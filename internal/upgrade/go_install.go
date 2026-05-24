package upgrade

import (
	"context"
	"os"
	"os/exec"
)

const GoInstallPackage = "github.com/SamyGhannad/seal-cli/cmd/seal@latest"

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) error
}

type ExecCommandRunner struct{}

func (ExecCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type GoInstaller struct {
	Runner CommandRunner
}

func (g GoInstaller) Install(ctx context.Context) error {
	runner := g.Runner
	if runner == nil {
		runner = ExecCommandRunner{}
	}
	return runner.Run(ctx, "go", "install", GoInstallPackage)
}
