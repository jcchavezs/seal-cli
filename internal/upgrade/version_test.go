package upgrade

import "testing"

func TestUpgradeAvailable(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "newer stable release", current: "v1.2.3", latest: "v1.2.4", want: true},
		{name: "already latest", current: "v1.2.3", latest: "v1.2.3", want: false},
		{name: "older latest", current: "v1.2.3", latest: "v1.2.2", want: false},
		{name: "patch upgrade", current: "v1.3.0", latest: "v1.3.1", want: true},
		{name: "minor upgrade", current: "v1.2.3", latest: "v1.3.0", want: true},
		{name: "major upgrade", current: "v1.3.0", latest: "v2.0.0", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UpgradeAvailable(tt.current, tt.latest)
			if err != nil {
				t.Fatalf("UpgradeAvailable() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("UpgradeAvailable(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestUpgradeAvailableRejectsNonReleaseCurrentVersion(t *testing.T) {
	for _, current := range []string{"dev", "", "v1.2.3-rc.1", "1.2.3"} {
		t.Run(current, func(t *testing.T) {
			if _, err := UpgradeAvailable(current, "v1.2.4"); err == nil {
				t.Fatalf("UpgradeAvailable(%q) error = nil, want error", current)
			}
		})
	}
}

func TestLatestStableRelease(t *testing.T) {
	releases := []Release{
		{TagName: "v1.2.0", Prerelease: false},
		{TagName: "v1.3.0-rc.1", Prerelease: true},
		{TagName: "v1.1.9", Prerelease: false},
		{TagName: "not-a-version", Prerelease: false},
		{TagName: "v1.2.1", Prerelease: false},
	}

	got, err := LatestStableRelease(releases)
	if err != nil {
		t.Fatalf("LatestStableRelease() error = %v", err)
	}
	if got.TagName != "v1.2.1" {
		t.Fatalf("LatestStableRelease() = %q, want v1.2.1", got.TagName)
	}
}
