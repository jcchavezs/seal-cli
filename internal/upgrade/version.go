package upgrade

import (
	"fmt"
	"strconv"
	"strings"
)

type semver struct {
	major int
	minor int
	patch int
}

func UpgradeAvailable(currentTag, latestTag string) (bool, error) {
	current, err := parseStableVersion(currentTag)
	if err != nil {
		return false, fmt.Errorf("current version %q is not a stable release tag", currentTag)
	}
	latest, err := parseStableVersion(latestTag)
	if err != nil {
		return false, fmt.Errorf("latest version %q is not a stable release tag", latestTag)
	}
	return compareVersion(latest, current) > 0, nil
}

func LatestStableRelease(releases []Release) (Release, error) {
	var best Release
	var bestVersion semver
	found := false
	for _, release := range releases {
		if release.Prerelease {
			continue
		}
		version, err := parseStableVersion(release.TagName)
		if err != nil {
			continue
		}
		if !found || compareVersion(version, bestVersion) > 0 {
			best = release
			bestVersion = version
			found = true
		}
	}
	if !found {
		return Release{}, fmt.Errorf("no stable release found")
	}
	return best, nil
}

func parseStableVersion(tag string) (semver, error) {
	if !strings.HasPrefix(tag, "v") {
		return semver{}, fmt.Errorf("missing v prefix")
	}
	rest := strings.TrimPrefix(tag, "v")
	if strings.Contains(rest, "-") || strings.Contains(rest, "+") {
		return semver{}, fmt.Errorf("not a stable version")
	}
	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return semver{}, fmt.Errorf("expected major.minor.patch")
	}
	nums := make([]int, 3)
	for i, part := range parts {
		if part == "" {
			return semver{}, fmt.Errorf("empty version component")
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return semver{}, err
		}
		nums[i] = n
	}
	return semver{major: nums[0], minor: nums[1], patch: nums[2]}, nil
}

func compareVersion(a, b semver) int {
	switch {
	case a.major != b.major:
		return a.major - b.major
	case a.minor != b.minor:
		return a.minor - b.minor
	default:
		return a.patch - b.patch
	}
}
