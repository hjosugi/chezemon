package state

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Snapshot fans its chezmoi and git invocations out across goroutines, so this
// runner is shared concurrently and must guard its recording.
type recordingRunner struct {
	mu    sync.Mutex
	calls []string
	dest  string
	path  string
}

func (r *recordingRunner) recorded() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *recordingRunner) Run(_ context.Context, program string, args ...string) ([]byte, error) {
	call := program + " " + strings.Join(args, " ")
	r.mu.Lock()
	r.calls = append(r.calls, call)
	r.mu.Unlock()
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
	for _, call := range runner.recorded() {
		if strings.Contains(call, " diff ") {
			t.Fatalf("diff command ran before explicit reveal: %q", call)
		}
	}
}

func TestTruncateDiff(t *testing.T) {
	small := "line one\nline two\n"
	if got, truncated := truncateDiff(small); truncated || got != small {
		t.Errorf("small diff was altered: truncated=%v", truncated)
	}

	oversized := strings.Repeat("a line of diff content\n", (maxDiffBytes/23)+64)
	got, truncated := truncateDiff(oversized)
	if !truncated {
		t.Fatal("oversized diff was not truncated")
	}
	if len(got) > maxDiffBytes {
		t.Errorf("truncated to %d bytes, want <= %d", len(got), maxDiffBytes)
	}
	// Cutting back to a line boundary keeps the result readable as a diff.
	if strings.HasSuffix(got, "\n") {
		t.Error("expected the trailing newline to be consumed by the line-boundary cut")
	}
	if !strings.HasSuffix(got, "content") {
		t.Errorf("truncation did not fall on a line boundary: %q", got[len(got)-30:])
	}
}

// chezmoi often writes a usable partial diff before failing. That output is
// the most useful thing available at exactly the moment it fails, so it must
// reach the caller rather than being replaced by a bare error.
type failingDiffRunner struct{ recordingRunner }

func (r *failingDiffRunner) Run(ctx context.Context, program string, args ...string) ([]byte, error) {
	if strings.Contains(strings.Join(args, " "), " diff ") {
		return []byte("diff --git a/x b/x\n+partial\n"), errors.New("exit status 1")
	}
	return r.recordingRunner.Run(ctx, program, args...)
}

func TestDiffKeepsPartialContentWhenRenderFails(t *testing.T) {
	dest := t.TempDir()
	target := filepath.Join(dest, ".bashrc")
	runner := &failingDiffRunner{recordingRunner{dest: dest, path: target}}
	service := NewService(runner, time.Second)

	diff, err := service.Diff(context.Background(), target, false)
	if err != nil {
		t.Fatalf("a failed render should be reported in the payload, not as an error: %v", err)
	}
	if !strings.Contains(diff.Content, "+partial") {
		t.Errorf("partial diff was discarded: %#v", diff)
	}
	if diff.Error == "" {
		t.Error("the render failure was not reported")
	}
}

func TestChezmoiVersionIsReadOnce(t *testing.T) {
	dest := t.TempDir()
	runner := &recordingRunner{dest: dest, path: filepath.Join(dest, ".bashrc")}
	service := NewService(runner, time.Second)

	for range 3 {
		if got := service.chezmoiVersion(context.Background()); got != "chezmoi version v2.71.0" {
			t.Fatalf("version = %q", got)
		}
	}
	var reads int
	for _, call := range runner.recorded() {
		if call == "chezmoi --version" {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("chezmoi --version ran %d times, want 1", reads)
	}
}
