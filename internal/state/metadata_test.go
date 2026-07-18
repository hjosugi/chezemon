package state

import "testing"

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

// A pending script must not be listed at its nominal destination path, which
// is never written. See the "~/50-quake-terminal-settings.sh" case that
// prompted this: the file does not and will not exist.
func TestParseStatusRenamesScriptsToTheirSource(t *testing.T) {
	meta := sourceMetadata{
		sourceRelative: map[string]string{
			"/home/test/50-quake.sh": "run_after_50-quake.sh.tmpl",
		},
		encrypted: map[string]bool{},
	}
	entries := parseStatus(" R /home/test/50-quake.sh\n", "/home/test", meta)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	entry := entries[0]
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
	meta := sourceMetadata{
		sourceRelative: map[string]string{},
		encrypted:      map[string]bool{"/home/test/.config/app/boring.txt": true},
	}
	entries := parseStatus(" M /home/test/.config/app/boring.txt\n", "/home/test", meta)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].Sensitive {
		t.Fatal("an encrypted entry was not masked")
	}
	if entries[0].SensitiveReason != "chezmoi encrypts this file" {
		t.Errorf("sensitiveReason = %q", entries[0].SensitiveReason)
	}
}

// "private_" is a file-mode attribute (0600), not a secrecy marker. Treating
// it as one masked 14 of 49 queued entries on the repository this was
// developed against, including gtk bookmarks and fcitx5 keyboard settings.
func TestParseStatusDoesNotMaskOnPrivateSourceAttribute(t *testing.T) {
	meta := sourceMetadata{
		sourceRelative: map[string]string{
			"/home/test/.config/fcitx5/conf/mozc.conf": "dot_config/private_fcitx5/private_conf/private_mozc.conf",
		},
		encrypted: map[string]bool{},
	}
	entries := parseStatus(" M /home/test/.config/fcitx5/conf/mozc.conf\n", "/home/test", meta)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Sensitive {
		t.Error("a private_ source attribute must not mask an ordinary config file")
	}
}
