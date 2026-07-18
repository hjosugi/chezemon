package state

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/hjosugi/chezemon/internal/command"
)

// sourceMetadata maps live target paths back to the chezmoi source entries
// that produce them.
type sourceMetadata struct {
	sourceRelative map[string]string
	encrypted      map[string]bool
}

type managedEntry struct {
	Absolute       string `json:"absolute"`
	SourceRelative string `json:"sourceRelative"`
}

// loadSourceMetadata asks chezmoi which source entry produces each target and
// which of those targets chezmoi encrypts.
//
// Both lookups are best-effort. A failure degrades entry labelling and drops
// one masking signal, which is preferable to failing the whole snapshot: the
// status output on its own is still useful.
func loadSourceMetadata(ctx context.Context, runner command.Runner) sourceMetadata {
	meta := sourceMetadata{
		sourceRelative: map[string]string{},
		encrypted:      map[string]bool{},
	}

	if output, err := runner.Run(ctx, "chezmoi", "managed", "--path-style=all", "--format=json"); err == nil {
		var managed map[string]managedEntry
		if json.Unmarshal(output, &managed) == nil {
			for _, entry := range managed {
				if entry.Absolute != "" && entry.SourceRelative != "" {
					meta.sourceRelative[filepath.Clean(entry.Absolute)] = entry.SourceRelative
				}
			}
		}
	}

	// chezmoi knows exactly which targets it encrypts, so this is an
	// authoritative sensitivity signal rather than a guess about filenames.
	//
	// Deliberately NOT used here: the "private_" source attribute. It sets
	// file mode 0600 and says nothing about secrecy — users apply it to whole
	// config trees. On the repository this was developed against, keying off
	// "private_" would have masked 14 of 49 queued entries (29%), including
	// gtk bookmarks and fcitx5 keyboard settings, while adding no file that
	// the filename heuristic did not already catch.
	if output, err := runner.Run(ctx, "chezmoi", "managed", "--include=encrypted", "--path-style=absolute"); err == nil {
		for _, line := range strings.Split(string(output), "\n") {
			if path := strings.TrimSpace(line); path != "" {
				meta.encrypted[filepath.Clean(path)] = true
			}
		}
	}
	return meta
}

// scriptDisplay converts a chezmoi source script name into the name a reader
// would recognise plus a plain-language description of when it runs.
//
// chezmoi reports a pending script against a nominal path in the destination
// directory, but a script is executed, never written there. Naming the source
// entry keeps the queue honest about what apply will actually do.
func scriptDisplay(sourceRelative string) (name, timing string) {
	name = strings.TrimSuffix(filepath.Base(sourceRelative), ".tmpl")
	rest, ok := strings.CutPrefix(name, "run_")
	if !ok {
		return name, ""
	}

	var conditions []string
	if trimmed, ok := strings.CutPrefix(rest, "once_"); ok {
		rest = trimmed
		conditions = append(conditions, "only once")
	}
	if trimmed, ok := strings.CutPrefix(rest, "onchange_"); ok {
		rest = trimmed
		conditions = append(conditions, "when its contents change")
	}
	if trimmed, ok := strings.CutPrefix(rest, "before_"); ok {
		rest = trimmed
		conditions = append(conditions, "before apply")
	} else if trimmed, ok := strings.CutPrefix(rest, "after_"); ok {
		rest = trimmed
		conditions = append(conditions, "after apply")
	}

	if rest == "" {
		rest = name
	}
	return rest, strings.Join(conditions, ", ")
}
