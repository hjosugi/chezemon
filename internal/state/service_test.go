package state

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingRunner struct {
	calls []string
}

func (r *recordingRunner) Run(_ context.Context, program string, args ...string) ([]byte, error) {
	call := program + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "chezmoi dump-config --format=json":
		return []byte(`{"sourceDir":"/source","destDir":"/home/test"}`), nil
	case call == "chezmoi --version":
		return []byte("chezmoi version v2.71.0"), nil
	case strings.Contains(call, " status "):
		return []byte(" M /home/test/.config/rclone/rclone.conf\n"), nil
	case strings.Contains(call, "rev-parse --is-inside-work-tree"):
		return nil, errors.New("not a Git working tree")
	default:
		return nil, errors.New("unexpected command: " + call)
	}
}

func TestDiffMasksSensitiveContentBeforeRunningDiff(t *testing.T) {
	runner := &recordingRunner{}
	service := NewService(runner, time.Second)

	diff, err := service.Diff(context.Background(), "/home/test/.config/rclone/rclone.conf", false)
	if err != nil {
		t.Fatal(err)
	}
	if !diff.Sensitive || diff.Revealed || diff.Content != "" {
		t.Fatalf("sensitive diff was not masked: %#v", diff)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, " diff ") {
			t.Fatalf("diff command ran before explicit reveal: %q", call)
		}
	}
}
