package state

import (
	"path/filepath"
	"testing"
)

func TestScriptDisplayNamesSourceAndTiming(t *testing.T) {
	tests := []struct {
		source string
		name   string
		timing string
	}{
		{"run_after_50-quake-terminal-settings.sh.tmpl", "50-quake-terminal-settings.sh", "after apply"},
		{"run_onchange_after_30-keyd.sh.tmpl", "30-keyd.sh", "when its contents change, after apply"},
		{"run_once_install-packages.sh", "install-packages.sh", "only once"},
		{"run_before_setup.sh", "setup.sh", "before apply"},
		{"run_plain.sh", "plain.sh", ""},
		// Not a run_ script: the name is passed through unchanged.
		{"dot_bashrc", "dot_bashrc", ""},
	}
	for _, test := range tests {
		name, timing := scriptDisplay(test.source)
		if name != test.name || timing != test.timing {
			t.Errorf("scriptDisplay(%q) = (%q, %q), want (%q, %q)",
				test.source, name, timing, test.name, test.timing)
		}
	}
}

// parseOneEntry parses a single status line, with the target path built via
// testDest so the metadata key and the parsed path agree on every platform.
func parseOneEntry(t *testing.T, code string, segments []string, meta func(target string) sourceMetadata) Entry {
	t.Helper()
	dest, path := testDest(t)
	target := path(segments...)
	entries := parseStatus(code+" "+target+"\n", dest, meta(target))
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	return entries[0]
}

// A pending script must not be listed at its nominal destination path, which
// is never written. See the "~/50-quake-terminal-settings.sh" case that
// prompted this: the file does not and will not exist.
func TestParseStatusRenamesScriptsToTheirSource(t *testing.T) {
	entry := parseOneEntry(t, " R", []string{"50-quake.sh"}, func(target string) sourceMetadata {
		return sourceMetadata{
			sourceRelative: map[string]string{target: "run_after_50-quake.sh.tmpl"},
			encrypted:      map[string]bool{},
		}
	})
	if entry.DisplayPath != "50-quake.sh" {
		t.Errorf("displayPath = %q, want %q (not a ~/ home path)", entry.DisplayPath, "50-quake.sh")
	}
	if entry.ScriptTiming != "after apply" {
		t.Errorf("scriptTiming = %q, want %q", entry.ScriptTiming, "after apply")
	}
	if entry.SourcePath != "run_after_50-quake.sh.tmpl" {
		t.Errorf("sourcePath = %q", entry.SourcePath)
	}
}

// chezmoi's own encryption is authoritative, so it must mask a file whose name
// gives no hint that it holds secrets.
func TestParseStatusMasksEncryptedEntriesRegardlessOfName(t *testing.T) {
	entry := parseOneEntry(t, " M", []string{".config", "app", "boring.txt"}, func(target string) sourceMetadata {
		return sourceMetadata{
			sourceRelative: map[string]string{},
			encrypted:      map[string]bool{target: true},
		}
	})
	if !entry.Sensitive {
		t.Fatal("an encrypted entry was not masked")
	}
	if entry.SensitiveReason != "chezmoi encrypts this file" {
		t.Errorf("sensitiveReason = %q", entry.SensitiveReason)
	}
}

// "private_" is a file-mode attribute (0600), not a secrecy marker. Treating
// it as one masked 14 of 49 queued entries on the repository this was
// developed against, including gtk bookmarks and fcitx5 keyboard settings.
func TestParseStatusDoesNotMaskOnPrivateSourceAttribute(t *testing.T) {
	source := filepath.Join("dot_config", "private_fcitx5", "private_conf", "private_mozc.conf")
	entry := parseOneEntry(t, " M", []string{".config", "fcitx5", "conf", "mozc.conf"}, func(target string) sourceMetadata {
		return sourceMetadata{
			sourceRelative: map[string]string{target: source},
			encrypted:      map[string]bool{},
		}
	})
	// Guard against the assertion below passing merely because the metadata
	// lookup missed, which is what made this test portable-fragile.
	if entry.SourcePath != source {
		t.Fatalf("metadata lookup missed: sourcePath = %q, want %q", entry.SourcePath, source)
	}
	if entry.Sensitive {
		t.Error("a private_ source attribute must not mask an ordinary config file")
	}
}
