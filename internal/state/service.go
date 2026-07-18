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

type Service struct {
	runner   command.Runner
	timeout  time.Duration
	mu       sync.Mutex
	cacheMu  sync.RWMutex
	cached   *Snapshot
	cachedAt time.Time
	cacheTTL time.Duration
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

func (s *Service) Snapshot(ctx context.Context, force bool) (Snapshot, error) {
	if !force {
		s.cacheMu.RLock()
		if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
			snapshot := *s.cached
			s.cacheMu.RUnlock()
			return snapshot, nil
		}
		s.cacheMu.RUnlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !force {
		s.cacheMu.RLock()
		if s.cached != nil && time.Since(s.cachedAt) < s.cacheTTL {
			snapshot := *s.cached
			s.cacheMu.RUnlock()
			return snapshot, nil
		}
		s.cacheMu.RUnlock()
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

	versionOutput, err := s.runner.Run(ctx, "chezmoi", "--version")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read chezmoi version: %w", err)
	}

	statusOutput, err := s.runner.Run(ctx, "chezmoi", "--color=false", "status", "--path-style=absolute")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read chezmoi status: %w", err)
	}

	entries := parseStatus(string(statusOutput), cfg.DestDir)
	gitState := loadGit(ctx, s.runner, cfg.SourceDir)
	snapshot := Snapshot{
		GeneratedAt:    time.Now(),
		DurationMS:     time.Since(start).Milliseconds(),
		ChezmoiVersion: strings.TrimSpace(string(versionOutput)),
		SourceDir:      cfg.SourceDir,
		DestDir:        cfg.DestDir,
		ReadOnly:       true,
		Entries:        entries,
		Counts:         countEntries(entries),
		Git:            gitState,
		Workflow:       buildWorkflow(entries, gitState),
		Notices:        buildNotices(entries, gitState),
	}

	s.cacheMu.Lock()
	s.cached = &snapshot
	s.cachedAt = time.Now()
	s.cacheMu.Unlock()
	return snapshot, nil
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
	diff := Diff{
		Path: entry.Path, Content: string(output),
		Sensitive: entry.Sensitive, Revealed: reveal,
	}
	if runErr != nil {
		return diff, fmt.Errorf("render chezmoi diff: %w", runErr)
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

func findEntry(entries []Entry, target string) (Entry, bool) {
	clean := filepath.Clean(target)
	for _, entry := range entries {
		if clean == filepath.Clean(entry.Path) || target == entry.DisplayPath {
			return entry, true
		}
	}
	return Entry{}, false
}

func buildNotices(entries []Entry, git GitState) []Notice {
	counts := countEntries(entries)
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
	if git.Available && git.Clean && len(entries) > 0 {
		notices = append(notices, Notice{
			Level:   "info",
			Title:   "Git is clean, but home is not synchronized",
			Message: "Source Git and chezmoi's rendered/live drift are separate states. Review the queue before assuming this machine is up to date.",
		})
	}
	return notices
}
