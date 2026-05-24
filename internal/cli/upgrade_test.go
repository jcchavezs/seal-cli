package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/SamyGhannad/seal-cli/internal/upgrade"
)

type fakeReleaseLister struct {
	releases []upgrade.Release
	err      error
	called   bool
}

func (f *fakeReleaseLister) ListReleasesContext(ctx context.Context) ([]upgrade.Release, error) {
	f.called = true
	return f.releases, f.err
}

type fakeInstaller struct {
	called bool
	err    error
}

func (f *fakeInstaller) Install(ctx context.Context) error {
	f.called = true
	return f.err
}

func TestRunUpgradeRejectsDevBuildBeforeNetwork(t *testing.T) {
	var stderr bytes.Buffer
	lister := &fakeReleaseLister{}

	code := RunUpgrade(UpgradeOpts{
		CurrentVersion: "dev",
		Stderr:         &stderr,
		ReleaseLister:  lister,
	})

	if code != 2 {
		t.Fatalf("RunUpgrade() = %d, want 2", code)
	}
	if lister.called {
		t.Fatal("release lister was called for dev build")
	}
	if !strings.Contains(stderr.String(), "seal: upgrade: current version") {
		t.Fatalf("stderr = %q, want current version diagnostic", stderr.String())
	}
}

func TestRunUpgradeCheckOnlyReportsAvailableUpdate(t *testing.T) {
	var stderr bytes.Buffer

	code := RunUpgrade(UpgradeOpts{
		CurrentVersion: "v1.2.3",
		CheckOnly:      true,
		Stderr:         &stderr,
		ReleaseLister: &fakeReleaseLister{releases: []upgrade.Release{
			{TagName: "v1.2.4", Assets: []upgrade.Asset{{Name: "seal-darwin-arm64", BrowserDownloadURL: "https://example.com/seal-darwin-arm64"}}},
		}},
		GoOS:   "darwin",
		GoArch: "arm64",
	})

	if code != 0 {
		t.Fatalf("RunUpgrade() = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "seal v1.2.4 is available") {
		t.Fatalf("stderr = %q, want update available message", stderr.String())
	}
}

func TestRunUpgradeAlreadyLatest(t *testing.T) {
	var stderr bytes.Buffer

	code := RunUpgrade(UpgradeOpts{
		CurrentVersion: "v1.2.3",
		Stderr:         &stderr,
		ReleaseLister: &fakeReleaseLister{releases: []upgrade.Release{
			{TagName: "v1.2.3"},
		}},
	})

	if code != 0 {
		t.Fatalf("RunUpgrade() = %d, want 0", code)
	}
	if !strings.Contains(stderr.String(), "seal is already up to date") {
		t.Fatalf("stderr = %q, want already latest message", stderr.String())
	}
}

func TestRunUpgradeGoInstallPathRunsInstaller(t *testing.T) {
	var stderr bytes.Buffer
	installer := &fakeInstaller{}

	code := RunUpgrade(UpgradeOpts{
		CurrentVersion: "v1.2.3",
		ExePath:        "/home/samy/go/bin/seal",
		Stderr:         &stderr,
		ReleaseLister: &fakeReleaseLister{releases: []upgrade.Release{
			{TagName: "v1.2.4"},
		}},
		Env:       upgrade.GoEnv{Home: "/home/samy"},
		Installer: installer,
	})

	if code != 0 {
		t.Fatalf("RunUpgrade() = %d, want 0", code)
	}
	if !installer.called {
		t.Fatal("installer was not called")
	}
	if !strings.Contains(stderr.String(), "Updated seal to v1.2.4") {
		t.Fatalf("stderr = %q, want success message", stderr.String())
	}
}

func TestRunUpgradeReleaseBinaryDownloadsVerifiesAndReplaces(t *testing.T) {
	var stderr bytes.Buffer
	var replacedPath string
	var replacedBytes []byte
	binaryBytes := []byte("new binary")
	sum := sha256.Sum256(binaryBytes)
	downloads := map[string][]byte{
		"https://example.com/seal-darwin-arm64":        binaryBytes,
		"https://example.com/seal-darwin-arm64.sha256": []byte(fmt.Sprintf("%s  seal-darwin-arm64\n", hex.EncodeToString(sum[:]))),
	}

	code := RunUpgrade(UpgradeOpts{
		CurrentVersion: "v1.2.3",
		ExePath:        "/usr/local/bin/seal",
		Stderr:         &stderr,
		ReleaseLister: &fakeReleaseLister{releases: []upgrade.Release{
			{TagName: "v1.2.4", Assets: []upgrade.Asset{
				{Name: "seal-darwin-arm64", BrowserDownloadURL: "https://example.com/seal-darwin-arm64"},
				{Name: "seal-darwin-arm64.sha256", BrowserDownloadURL: "https://example.com/seal-darwin-arm64.sha256"},
			}},
		}},
		GoOS:   "darwin",
		GoArch: "arm64",
		Env:    upgrade.GoEnv{Home: "/home/samy"},
		Download: func(ctx context.Context, rawURL string) ([]byte, error) {
			data, ok := downloads[rawURL]
			if !ok {
				return nil, errors.New("unexpected URL: " + rawURL)
			}
			return data, nil
		},
		ReplaceExecutable: func(path string, data []byte) error {
			replacedPath = path
			replacedBytes = append([]byte(nil), data...)
			return nil
		},
	})

	if code != 0 {
		t.Fatalf("RunUpgrade() = %d, want 0; stderr=%q", code, stderr.String())
	}
	if replacedPath != "/usr/local/bin/seal" || string(replacedBytes) != "new binary" {
		t.Fatalf("replacement = (%q, %q), want exe path and new binary", replacedPath, replacedBytes)
	}
}
