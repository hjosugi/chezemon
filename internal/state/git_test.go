package state

import "testing"

func TestParseGitStatus(t *testing.T) {
	output := []byte(" M file.txt\x00A  new.txt\x00R  old.txt\x00renamed.txt\x00")
	changes := parseGitStatus(output)
	if len(changes) != 3 {
		t.Fatalf("got %d changes, want 3", len(changes))
	}
	if changes[2].Path != "old.txt -> renamed.txt" {
		t.Fatalf("rename path = %q", changes[2].Path)
	}
}
