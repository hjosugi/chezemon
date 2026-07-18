package command

import (
	"bytes"
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

// Run returns the program's standard output only.
//
// Standard error is deliberately kept separate rather than merged: chezmoi
// writes advisory warnings there ("config file template has changed, run
// chezmoi init ...") while still exiting 0, and merging them into stdout
// corrupts anything that parses the result line by line. A warning once
// reached parseStatus as a status line and surfaced as a phantom critical
// entry in the review queue.
//
// Standard error is still captured and attached to the error, so a genuine
// failure keeps its diagnosis.
func (ExecRunner) Run(ctx context.Context, program string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"NO_COLOR=1",
		"PAGER=cat",
		"GIT_PAGER=cat",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		invocation := program + " " + strings.Join(args, " ")
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return stdout.Bytes(), fmt.Errorf("%s: %w: %s", invocation, err, detail)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w", invocation, err)
	}
	return stdout.Bytes(), nil
}
