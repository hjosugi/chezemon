package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner executes a program without involving a shell.
type Runner interface {
	Run(ctx context.Context, program string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, program string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"NO_COLOR=1",
		"PAGER=cat",
		"GIT_PAGER=cat",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s: %w", program, strings.Join(args, " "), err)
	}
	return output, nil
}
