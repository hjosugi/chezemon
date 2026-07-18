package state

import (
	"bufio"
	"context"
	"strconv"
	"strings"

	"github.com/hjosugi/chezemon/internal/command"
)

func loadGit(ctx context.Context, runner command.Runner, sourceDir string) GitState {
	state := GitState{}
	if sourceDir == "" {
		state.Error = "chezmoi source directory is unknown"
		return state
	}

	inside, err := runner.Run(ctx, "git", "-C", sourceDir, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(string(inside)) != "true" {
		state.Error = "chezmoi source directory is not a Git working tree"
		return state
	}
	state.Available = true

	if branch, err := runner.Run(ctx, "git", "-C", sourceDir, "branch", "--show-current"); err == nil {
		state.Branch = strings.TrimSpace(string(branch))
	}

	if output, err := runner.Run(ctx, "git", "-C", sourceDir, "status", "--porcelain=v1", "-z"); err == nil {
		state.Changes = parseGitStatus(output)
		state.Clean = len(state.Changes) == 0
	} else {
		state.Error = err.Error()
	}

	if output, err := runner.Run(ctx, "git", "-C", sourceDir, "rev-list", "--left-right", "--count", "@{upstream}...HEAD"); err == nil {
		fields := strings.Fields(string(output))
		if len(fields) == 2 {
			state.Behind, _ = strconv.Atoi(fields[0])
			state.Ahead, _ = strconv.Atoi(fields[1])
			state.HasUpstream = true
		}
	}

	if output, err := runner.Run(ctx, "git", "-C", sourceDir, "log", "-8", "--date=short", "--pretty=format:%h%x09%ad%x09%s"); err == nil {
		state.Commits = parseGitLog(string(output))
	}
	return state
}

func parseGitStatus(output []byte) []GitChange {
	parts := strings.Split(string(output), "\x00")
	changes := make([]GitChange, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) < 4 {
			continue
		}
		code := part[:2]
		path := part[3:]
		if code[0] == 'R' || code[0] == 'C' {
			if i+1 < len(parts) && parts[i+1] != "" {
				path += " -> " + parts[i+1]
				i++
			}
		}
		changes = append(changes, GitChange{Code: code, Path: path})
	}
	return changes
}

func parseGitLog(output string) []GitCommit {
	var commits []GitCommit
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		commits = append(commits, GitCommit{
			Hash: parts[0], Date: parts[1], Subject: parts[2],
		})
	}
	return commits
}
