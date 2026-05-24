package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/SamyGhannad/seal-cli/internal/upgrade"
)

type ReleaseLister interface {
	ListReleasesContext(ctx context.Context) ([]upgrade.Release, error)
}

type Installer interface {
	Install(ctx context.Context) error
}

type UpgradeOpts struct {
	CurrentVersion string
	ExePath        string
	Stderr         io.Writer
	CheckOnly      bool

	GoOS   string
	GoArch string
	Env    upgrade.GoEnv

	ReleaseLister     ReleaseLister
	Installer         Installer
	Download          func(ctx context.Context, rawURL string) ([]byte, error)
	ReplaceExecutable func(path string, data []byte) error
}

func RunUpgrade(opts UpgradeOpts) int {
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	if _, err := upgrade.UpgradeAvailable(opts.CurrentVersion, opts.CurrentVersion); err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	releases, err := releaseLister(opts).ListReleasesContext(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	latest, err := upgrade.LatestStableRelease(releases)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	available, err := upgrade.UpgradeAvailable(opts.CurrentVersion, latest.TagName)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	if !available {
		fmt.Fprintf(stderr, "seal is already up to date (%s)\n", opts.CurrentVersion)
		return 0
	}
	if opts.CheckOnly {
		fmt.Fprintf(stderr, "seal %s is available (current: %s)\n", latest.TagName, opts.CurrentVersion)
		return 0
	}

	exePath, err := executablePath(opts.ExePath)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	if upgrade.DetectInstallMethod(exePath, goEnv(opts.Env)) == upgrade.InstallMethodGoInstall {
		if err := installer(opts).Install(ctx); err != nil {
			fmt.Fprintf(stderr, "seal: upgrade: go install failed: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "Updated seal to %s via go install\n", latest.TagName)
		return 0
	}

	goos := opts.GoOS
	if goos == "" {
		goos = runtime.GOOS
	}
	goarch := opts.GoArch
	if goarch == "" {
		goarch = runtime.GOARCH
	}

	asset, err := upgrade.FindPlatformAsset(latest, goos, goarch)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	sidecar, err := findAsset(latest, upgrade.ChecksumAssetName(asset.Name))
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}

	download := downloader(opts)
	binaryBytes, err := download(ctx, asset.BrowserDownloadURL)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: download %s: %v\n", asset.Name, err)
		return 2
	}
	sidecarBytes, err := download(ctx, sidecar.BrowserDownloadURL)
	if err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: download %s: %v\n", sidecar.Name, err)
		return 2
	}
	if err := upgrade.VerifySidecarChecksum(binaryBytes, sidecarBytes, asset.Name); err != nil {
		fmt.Fprintf(stderr, "seal: upgrade: %v\n", err)
		return 2
	}
	if err := replacer(opts)(exePath, binaryBytes); err != nil {
		var manual *upgrade.ManualReplaceError
		if errors.As(err, &manual) {
			fmt.Fprintf(stderr, "Downloaded and verified seal %s. Complete the upgrade manually:\nmove /Y %q %q\n", latest.TagName, manual.TempPath, manual.ExePath)
			return 0
		}
		fmt.Fprintf(stderr, "seal: upgrade: replace executable: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "Updated seal to %s\n", latest.TagName)
	return 0
}

func releaseLister(opts UpgradeOpts) ReleaseLister {
	if opts.ReleaseLister != nil {
		return opts.ReleaseLister
	}
	client := upgrade.DefaultGitHubClient()
	return client
}

func installer(opts UpgradeOpts) Installer {
	if opts.Installer != nil {
		return opts.Installer
	}
	return upgrade.GoInstaller{}
}

func downloader(opts UpgradeOpts) func(ctx context.Context, rawURL string) ([]byte, error) {
	if opts.Download != nil {
		return opts.Download
	}
	return func(ctx context.Context, rawURL string) ([]byte, error) {
		return upgrade.DownloadURL(ctx, nil, rawURL)
	}
}

func replacer(opts UpgradeOpts) func(path string, data []byte) error {
	if opts.ReplaceExecutable != nil {
		return opts.ReplaceExecutable
	}
	return upgrade.ReplaceExecutable
}

func executablePath(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return path, nil
	}
	return os.Executable()
}

func goEnv(env upgrade.GoEnv) upgrade.GoEnv {
	if env.GOBIN != "" || env.GOPATH != "" || env.Home != "" {
		return env
	}
	return upgrade.CurrentGoEnv()
}

func findAsset(release upgrade.Release, name string) (upgrade.Asset, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return asset, nil
		}
	}
	return upgrade.Asset{}, fmt.Errorf("release %s has no asset named %s", release.TagName, name)
}
