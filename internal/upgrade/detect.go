package upgrade

import (
	"os"
	"path/filepath"
	"strings"
)

type InstallMethod string

const (
	InstallMethodGoInstall     InstallMethod = "go-install"
	InstallMethodReleaseBinary InstallMethod = "release-binary"
)

type GoEnv struct {
	GOBIN  string
	GOPATH string
	Home   string
}

func CurrentGoEnv() GoEnv {
	home, _ := os.UserHomeDir()
	return GoEnv{
		GOBIN:  os.Getenv("GOBIN"),
		GOPATH: os.Getenv("GOPATH"),
		Home:   home,
	}
}

func DetectInstallMethod(exe string, env GoEnv) InstallMethod {
	exe = filepath.Clean(exe)
	for _, dir := range goInstallDirs(env) {
		if pathWithin(exe, dir) {
			return InstallMethodGoInstall
		}
	}
	return InstallMethodReleaseBinary
}

func goInstallDirs(env GoEnv) []string {
	var dirs []string
	if env.GOBIN != "" {
		dirs = append(dirs, env.GOBIN)
	}
	if env.GOPATH != "" {
		dirs = append(dirs, filepath.Join(env.GOPATH, "bin"))
	}
	if env.Home != "" {
		dirs = append(dirs, filepath.Join(env.Home, "go", "bin"))
	}
	return dirs
}

func pathWithin(path, dir string) bool {
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != "" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
