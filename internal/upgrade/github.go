package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultGitHubAPI = "https://api.github.com"
	defaultOwner     = "SamyGhannad"
	defaultRepo      = "seal-cli"
)

type Release struct {
	TagName    string  `json:"tag_name"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type GitHubClient struct {
	HTTPClient *http.Client
	BaseURL    string
	Owner      string
	Repo       string
}

func DefaultGitHubClient() GitHubClient {
	return GitHubClient{
		HTTPClient: http.DefaultClient,
		BaseURL:    defaultGitHubAPI,
		Owner:      defaultOwner,
		Repo:       defaultRepo,
	}
}

func (c GitHubClient) ListReleases() ([]Release, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return c.ListReleasesContext(ctx)
}

func (c GitHubClient) ListReleasesContext(ctx context.Context) ([]Release, error) {
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultGitHubAPI
	}
	owner := c.Owner
	if owner == "" {
		owner = defaultOwner
	}
	repo := c.Repo
	if repo == "" {
		repo = defaultRepo
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	endpoint := strings.TrimRight(baseURL, "/") + "/repos/" + owner + "/" + repo + "/releases"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("GitHub releases API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func PlatformAssetName(goos, goarch string) string {
	name := "seal-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func FindPlatformAsset(release Release, goos, goarch string) (Asset, error) {
	want := PlatformAssetName(goos, goarch)
	for _, asset := range release.Assets {
		if asset.Name != want {
			continue
		}
		if err := validateHTTPS(asset.BrowserDownloadURL); err != nil {
			return Asset{}, fmt.Errorf("asset %s: %w", asset.Name, err)
		}
		return asset, nil
	}
	return Asset{}, fmt.Errorf("release %s has no asset named %s", release.TagName, want)
}

func ChecksumAssetName(assetName string) string {
	return assetName + ".sha256"
}

func VerifySidecarChecksum(payload, sidecar []byte, assetName string) error {
	want, err := parseSidecar(sidecar, assetName)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payload)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func DownloadURL(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	if err := validateHTTPS(rawURL); err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func parseSidecar(sidecar []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(sidecar), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] != assetName {
			continue
		}
		if len(fields[0]) != sha256.Size*2 {
			return "", fmt.Errorf("invalid checksum length for %s", assetName)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return "", fmt.Errorf("invalid checksum for %s: %w", assetName, err)
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("checksum sidecar has no entry for %s", assetName)
}

func validateHTTPS(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("download URL must be HTTPS")
	}
	return nil
}
