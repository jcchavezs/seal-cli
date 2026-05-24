package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindPlatformAsset(t *testing.T) {
	release := Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: "seal-linux-amd64", BrowserDownloadURL: "https://example.com/seal-linux-amd64"},
			{Name: "seal-darwin-arm64", BrowserDownloadURL: "https://example.com/seal-darwin-arm64"},
			{Name: "seal-windows-amd64.exe", BrowserDownloadURL: "https://example.com/seal-windows-amd64.exe"},
		},
	}

	got, err := FindPlatformAsset(release, "windows", "amd64")
	if err != nil {
		t.Fatalf("FindPlatformAsset() error = %v", err)
	}
	if got.Name != "seal-windows-amd64.exe" {
		t.Fatalf("asset name = %q, want seal-windows-amd64.exe", got.Name)
	}
}

func TestFindPlatformAssetRequiresHTTPS(t *testing.T) {
	release := Release{
		TagName: "v1.2.3",
		Assets: []Asset{
			{Name: "seal-linux-amd64", BrowserDownloadURL: "http://example.com/seal-linux-amd64"},
		},
	}

	if _, err := FindPlatformAsset(release, "linux", "amd64"); err == nil {
		t.Fatal("FindPlatformAsset() error = nil, want HTTPS validation error")
	}
}

func TestVerifySidecarChecksum(t *testing.T) {
	payload := []byte("seal binary bytes")
	sum := sha256.Sum256(payload)
	sidecar := []byte(hex.EncodeToString(sum[:]) + "  seal-darwin-arm64\n")

	if err := VerifySidecarChecksum(payload, sidecar, "seal-darwin-arm64"); err != nil {
		t.Fatalf("VerifySidecarChecksum() error = %v", err)
	}
}

func TestVerifySidecarChecksumRejectsMismatch(t *testing.T) {
	payload := []byte("seal binary bytes")
	sidecar := []byte("0000000000000000000000000000000000000000000000000000000000000000  seal-darwin-arm64\n")

	if err := VerifySidecarChecksum(payload, sidecar, "seal-darwin-arm64"); err == nil {
		t.Fatal("VerifySidecarChecksum() error = nil, want mismatch")
	}
}

func TestParseSidecar(t *testing.T) {
	sidecar := []byte("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff  seal-linux-amd64\nABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD  seal-darwin-arm64\n")

	got, err := parseSidecar(sidecar, "seal-darwin-arm64")
	if err != nil {
		t.Fatalf("parseSidecar() error = %v", err)
	}
	if got != "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd" {
		t.Fatalf("parseSidecar() = %q, want lowercase matching checksum", got)
	}
}

func TestParseSidecarRejectsInvalidSidecars(t *testing.T) {
	tests := []struct {
		name    string
		sidecar []byte
	}{
		{
			name:    "missing asset",
			sidecar: []byte("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff  seal-linux-amd64\n"),
		},
		{
			name:    "invalid checksum length",
			sidecar: []byte("ffff  seal-darwin-arm64\n"),
		},
		{
			name:    "non hex checksum",
			sidecar: []byte("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz  seal-darwin-arm64\n"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseSidecar(tc.sidecar, "seal-darwin-arm64"); err == nil {
				t.Fatal("parseSidecar() error = nil, want error")
			}
		})
	}
}

func TestGitHubClientFetchesReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/SamyGhannad/seal-cli/releases" {
			t.Fatalf("path = %q, want releases API path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v1.2.3","prerelease":false,"assets":[{"name":"seal-linux-amd64","browser_download_url":"https://example.com/seal-linux-amd64"}]}]`))
	}))
	defer server.Close()

	client := GitHubClient{
		HTTPClient: server.Client(),
		BaseURL:    server.URL,
		Owner:      "SamyGhannad",
		Repo:       "seal-cli",
	}
	releases, err := client.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases() error = %v", err)
	}
	if len(releases) != 1 || releases[0].TagName != "v1.2.3" {
		t.Fatalf("releases = %#v, want v1.2.3", releases)
	}
}
