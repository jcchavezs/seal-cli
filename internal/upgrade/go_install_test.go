package upgrade

import (
	"context"
	"strings"
	"testing"
)

type commandRunnerFunc func(ctx context.Context, name string, args ...string) error

func (f commandRunnerFunc) Run(ctx context.Context, name string, args ...string) error {
	return f(ctx, name, args...)
}

func TestGoInstallerRunsLatestPackage(t *testing.T) {
	var gotName string
	var gotArgs []string
	runner := commandRunnerFunc(func(ctx context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	})

	installer := GoInstaller{Runner: runner}
	if err := installer.Install(context.Background()); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if gotName != "go" {
		t.Fatalf("command = %q, want go", gotName)
	}
	if got := strings.Join(gotArgs, " "); got != "install github.com/SamyGhannad/seal-cli/cmd/seal@latest" {
		t.Fatalf("args = %q, want install package@latest", got)
	}
}
