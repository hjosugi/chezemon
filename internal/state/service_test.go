package state

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type recordingRunner struct {
	calls []string
	dest  string
	path  string
}

func (r *recordingRunner) Run(_ context.Context, program string, args ...string) ([]byte, error) {
	call := program + " " + strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "chezmoi dump-config --format=json":
		config, err := json.Marshal(configView{SourceDir: r.dest, DestDir: r.dest})
		return config, err
	case call == "chezmoi --version":
		return []byte("chezmoi version v2.71.0"), nil
	case strings.Contains(call, " status "):
		return []byte(" M " + r.path + "\n"), nil
	case strings.Contains(call, "rev-parse --is-inside-work-tree"):
		return nil, errors.New("not a Git working tree")
	default:
		return nil, errors.New("unexpected command: " + call)
	}
}

func TestDiffMasksSensitiveContentBeforeRunningDiff(t *testing.T) {
	dest := t.TempDir()
	target := filepath.Join(dest, ".config", "rclone", "rclone.conf")
	runner := &recordingRunner{dest: dest, path: target}
	service := NewService(runner, time.Second)

	diff, err := service.Diff(context.Background(), target, false)
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
