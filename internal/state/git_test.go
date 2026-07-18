package state

import "testing"

func TestParseGitStatus(t *testing.T) {
	output := []byte(" M file.txt\x00A  new.txt\x00")
	changes := parseGitStatus(output)
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(changes))
	}
	if changes[0].Code != " M" || changes[0].Path != "file.txt" {
		t.Errorf("unexpected first change: %#v", changes[0])
	}
	if changes[1].Code != "A " || changes[1].Path != "new.txt" {
		t.Errorf("unexpected second change: %#v", changes[1])
	}
}

// The fixture below is the verbatim output of
// `git status --porcelain=v1 -z` after `git mv original.txt renamed.txt`:
// the NEW path comes first and the ORIGINAL path follows in the next
// NUL-separated field. The rendered arrow must therefore read
// "original -> new" to match git's own short format.
func TestParseGitStatusRenameReadsOriginalToNew(t *testing.T) {
	output := []byte("R  renamed.txt\x00original.txt\x00 M other.txt\x00")
	changes := parseGitStatus(output)
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2: %#v", len(changes), changes)
	}
	if changes[0].Path != "original.txt -> renamed.txt" {
		t.Errorf("rename path = %q, want %q", changes[0].Path, "original.txt -> renamed.txt")
	}
	if changes[1].Path != "other.txt" {
		t.Errorf("the original-path field was not consumed: %#v", changes)
	}
}
