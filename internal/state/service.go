package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hjosugi/chezemon/internal/command"
)

// maxDiffBytes bounds what a single diff response may carry. chezmoi will
// happily render a diff for a very large or effectively binary managed file,
// and the browser puts the whole thing in one <pre>. Truncating server-side
// keeps one oversized entry from freezing the review UI; the entry is still
// listed, and the reader is told the view was cut short.
const maxDiffBytes = 256 << 10

type Service struct {
	runner   command.Runner
	timeout  time.Duration
	mu       sync.Mutex
	cacheMu  sync.RWMutex
	cached   *Snapshot
	cachedAt time.Time
	cacheTTL time.Duration

	// chezmoi's version cannot change within one run of this process, so it is
	// read once rather than on every snapshot.
	versionMu sync.Mutex
	version   string
}

type configView struct {
	SourceDir string `json:"sourceDir"`
	DestDir   string `json:"destDir"`
}

func NewService(runner command.Runner, timeout time.Duration) *Service {
	return &Service{
		runner: runner, timeout: timeout, cacheTTL: 3 * time.Second,
	}
}

// cachedSnapshot returns the cached snapshot while it is still fresh.
func (s *Service) cachedSnapshot() (Snapshot, bool) {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()
	if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
		return *s.cached, true
	}
	return Snapshot{}, false
}

func (s *Service) Snapshot(ctx context.Context, force bool) (Snapshot, error) {
	if !force {
		if snapshot, ok := s.cachedSnapshot(); ok {
			return snapshot, nil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Re-check after taking the lock: a concurrent caller may have refreshed
	// the cache while this one was waiting.
	if !force {
		if snapshot, ok := s.cachedSnapshot(); ok {
			return snapshot, nil
		}
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	configOutput, err := s.runner.Run(ctx, "chezmoi", "dump-config", "--format=json")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read chezmoi configuration: %w", err)
	}
	var cfg configView
	if err := json.Unmarshal(configOutput, &cfg); err != nil {
		return Snapshot{}, fmt.Errorf("parse chezmoi configuration: %w", err)
	}
	if cfg.SourceDir == "" || cfg.DestDir == "" {
		return Snapshot{}, errors.New("chezmoi returned an incomplete source/destination configuration")
	}

	// Everything below depends only on the configuration, not on each other,
	// so the remaining dozen or so chezmoi and git invocations run
	// concurrently rather than end to end.
	var (
		wait         sync.WaitGroup
		statusOutput []byte
		statusErr    error
		version      string
		meta         sourceMetadata
		gitState     GitState
	)
	wait.Add(4)
	go func() {
		defer wait.Done()
		statusOutput, statusErr = s.runner.Run(ctx, "chezmoi", "--color=false", "status", "--path-style=absolute")
	}()
	go func() {
		defer wait.Done()
		version = s.chezmoiVersion(ctx)
	}()
	go func() {
		defer wait.Done()
		meta = loadSourceMetadata(ctx, s.runner)
	}()
	go func() {
		defer wait.Done()
		gitState = loadGit(ctx, s.runner, cfg.SourceDir)
	}()
	wait.Wait()

	if statusErr != nil {
		return Snapshot{}, fmt.Errorf("read chezmoi status: %w", statusErr)
	}

	entries := parseStatus(string(statusOutput), cfg.DestDir, meta)
	counts := countEntries(entries)
	snapshot := Snapshot{
		GeneratedAt:    time.Now(),
		DurationMS:     time.Since(start).Milliseconds(),
		ChezmoiVersion: version,
		SourceDir:      cfg.SourceDir,
		DestDir:        cfg.DestDir,
		ReadOnly:       true,
		Entries:        entries,
		Counts:         counts,
		Git:            gitState,
		Workflow:       buildWorkflow(counts, gitState),
		Notices:        buildNotices(counts, gitState),
	}

	s.cacheMu.Lock()
	s.cached = &snapshot
	s.cachedAt = time.Now()
	s.cacheMu.Unlock()
	return snapshot, nil
}

// chezmoiVersion reads the version once and reuses it.
//
// A failure is not cached and not fatal: the version is informational, so a
// later snapshot retries, and until one succeeds the field reads "unknown"
// rather than refusing to show the drift the user came to look at.
func (s *Service) chezmoiVersion(ctx context.Context) string {
	s.versionMu.Lock()
	defer s.versionMu.Unlock()
	if s.version != "" {
		return s.version
	}
	output, err := s.runner.Run(ctx, "chezmoi", "--version")
	if err != nil {
		return "unknown"
	}
	s.version = strings.TrimSpace(string(output))
	if s.version == "" {
		return "unknown"
	}
	return s.version
}

func (s *Service) Diff(ctx context.Context, target string, reveal bool) (Diff, error) {
	snapshot, err := s.Snapshot(ctx, false)
	if err != nil {
		return Diff{}, err
	}
	entry, ok := findEntry(snapshot.Entries, target)
	if !ok {
		return Diff{}, errors.New("target is not present in the current review queue")
	}
	if entry.Sensitive && !reveal {
		return Diff{
			Path: entry.Path, Sensitive: true, Revealed: false,
			Message: "This path may contain secrets. Its diff is masked until you explicitly reveal it.",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	output, runErr := s.runner.Run(
		ctx,
		"chezmoi",
		"--color=false",
		"--no-pager",
		"--use-builtin-diff",
		"diff",
		entry.Path,
	)
	content, truncated := truncateDiff(string(output))
	diff := Diff{
		Path: entry.Path, Content: content, Truncated: truncated,
		Sensitive: entry.Sensitive, Revealed: reveal,
	}
	if truncated {
		diff.Message = fmt.Sprintf(
			"This diff is larger than %d KiB and is shown truncated. Use `chezmoi diff %s` for the whole thing.",
			maxDiffBytes>>10, entry.DisplayPath,
		)
	}

	// A failed render is reported in the payload rather than as a transport
	// error, because chezmoi often writes a usable partial diff before
	// failing. Discarding it threw away the most useful thing available at
	// exactly the moment the reader needed it.
	if runErr != nil {
		diff.Error = runErr.Error()
		return diff, nil
	}

	if sourceOutput, sourceErr := s.runner.Run(ctx, "chezmoi", "source-path", entry.Path); sourceErr == nil {
		diff.SourcePath = strings.TrimSpace(string(sourceOutput))
	}
	return diff, nil
}

func (s *Service) Doctor(ctx context.Context) DoctorResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	output, err := s.runner.Run(ctx, "chezmoi", "--color=false", "--no-pager", "doctor", "--no-network")
	return DoctorResult{Output: string(output), OK: err == nil}
}

// truncateDiff caps a diff at maxDiffBytes, cutting back to the last complete
// line so the result still reads as a diff rather than stopping mid-token.
func truncateDiff(content string) (string, bool) {
	if len(content) <= maxDiffBytes {
		return content, false
	}
	cut := content[:maxDiffBytes]
	if newline := strings.LastIndexByte(cut, '\n'); newline > 0 {
		cut = cut[:newline]
	}
	return cut, true
}

func findEntry(entries []Entry, target string) (Entry, bool) {
	clean := filepath.Clean(target)
	for _, entry := range entries {
		if clean == filepath.Clean(entry.Path) || target == entry.DisplayPath {
			return entry, true
		}
	}
	return Entry{}, false
}

func buildNotices(counts Counts, git GitState) []Notice {
	var notices []Notice
	if counts.Critical > 0 {
		notices = append(notices, Notice{
			Level:   "critical",
			Title:   "Resolve diverged files first",
			Message: fmt.Sprintf("%d file(s) changed on both sides. Avoid a global apply or re-add until they are reviewed.", counts.Critical),
		})
	}
	if counts.Scripts > 0 {
		notices = append(notices, Notice{
			Level:   "warning",
			Title:   "Apply will run scripts",
			Message: fmt.Sprintf("%d script(s) are pending. File diffs do not describe every side effect a script may have.", counts.Scripts),
		})
	}
	if git.Available && git.Clean && counts.Total > 0 {
		notices = append(notices, Notice{
			Level:   "info",
			Title:   "Git is clean, but home is not synchronized",
			Message: "Source Git and chezmoi's rendered/live drift are separate states. Review the queue before assuming this machine is up to date.",
		})
	}
	return notices
}
