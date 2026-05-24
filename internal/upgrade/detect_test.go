package upgrade

import "testing"

func TestDetectInstallMethodGoInstallPaths(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		env  GoEnv
	}{
		{
			name: "GOBIN",
			exe:  "/opt/go/bin/seal",
			env:  GoEnv{GOBIN: "/opt/go/bin", GOPATH: "/tmp/gopath", Home: "/home/samy"},
		},
		{
			name: "GOPATH bin",
			exe:  "/tmp/gopath/bin/seal",
			env:  GoEnv{GOPATH: "/tmp/gopath", Home: "/home/samy"},
		},
		{
			name: "default home go bin",
			exe:  "/home/samy/go/bin/seal",
			env:  GoEnv{Home: "/home/samy"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectInstallMethod(tc.exe, tc.env); got != InstallMethodGoInstall {
				t.Fatalf("DetectInstallMethod() = %q, want %q", got, InstallMethodGoInstall)
			}
		})
	}
}

func TestDetectInstallMethodReleaseBinary(t *testing.T) {
	env := GoEnv{GOBIN: "/opt/go/bin", GOPATH: "/tmp/gopath", Home: "/home/samy"}
	if got := DetectInstallMethod("/usr/local/bin/seal", env); got != InstallMethodReleaseBinary {
		t.Fatalf("DetectInstallMethod() = %q, want %q", got, InstallMethodReleaseBinary)
	}
}

func TestDetectInstallMethodDoesNotMatchSiblingPrefix(t *testing.T) {
	env := GoEnv{GOBIN: "/opt/go/bin", GOPATH: "/tmp/gopath", Home: "/home/samy"}
	if got := DetectInstallMethod("/opt/go/bin-extra/seal", env); got != InstallMethodReleaseBinary {
		t.Fatalf("DetectInstallMethod() = %q, want %q", got, InstallMethodReleaseBinary)
	}
}
