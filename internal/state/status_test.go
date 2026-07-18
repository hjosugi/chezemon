package state

import (
	"testing"
)

func TestParseStatusAndCounts(t *testing.T) {
	dest, target := testDest(t)
	output := "MM " + target(".config", "app.json") +
		"\n M " + target(".bashrc") +
		"\n R " + target("setup.sh") +
		"\n A " + target(".new") +
		"\n D " + target(".old") + "\n"
	entries := parseStatus(output, dest, sourceMetadata{})
	if len(entries) != 5 {
		t.Fatalf("got %d entries, want 5", len(entries))
	}
	if entries[0].Kind != "diverged" || entries[0].Risk != "critical" {
		t.Fatalf("first entry = %#v, want critical divergence", entries[0])
	}
	counts := countEntries(entries)
	if counts.Critical != 1 || counts.Scripts != 1 || counts.Pending != 4 {
		t.Fatalf("unexpected counts: %#v", counts)
	}
}

func TestSensitivePaths(t *testing.T) {
	tests := map[string]bool{
		"/home/me/.ssh/config":                  true,
		"/home/me/.config/rclone/rclone.conf":   true,
		"/home/me/project/.env.production":      true,
		"/home/me/.config/fish/config.fish":     false,
		"/home/me/.config/app/credentials.json": true,
		"/home/me/.netrc":                       true,
		"/home/me/.npmrc":                       true,
		"/home/me/.docker/config.json":          true,
		"/home/me/.config/gh/hosts.yml":         true,
		"/home/me/certs/server.pem":             true,
		// Key material is matched by extension, so ordinary config files
		// whose names merely contain "key" must stay unmasked.
		"/home/me/.config/Code/User/keybindings.json": false,
		"/home/me/.config/app/hotkeys.json":           false,
	}
	for path, want := range tests {
		if got := isSensitivePath(path); got != want {
			t.Errorf("isSensitivePath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestFindEntryAllowsOnlyCurrentQueue(t *testing.T) {
	entries := []Entry{{Path: "/home/me/.bashrc", DisplayPath: "~/.bashrc"}}
	if _, ok := findEntry(entries, "/etc/passwd"); ok {
		t.Fatal("unexpectedly allowed a target outside the queue")
	}
	if _, ok := findEntry(entries, "~/.bashrc"); !ok {
		t.Fatal("display path did not resolve")
	}
}

func TestBuildWorkflowChoosesFirstOutstandingStep(t *testing.T) {
	dest, target := testDest(t)
	entries := parseStatus(
		"MM "+target(".config", "app.json")+
			"\n R "+target("setup.sh")+
			"\n M "+target(".bashrc")+"\n",
		dest,
		sourceMetadata{},
	)
	workflow := buildWorkflow(countEntries(entries), GitState{Available: true, Clean: true})

	if workflow.Phase != "protect-work" || workflow.CurrentStep != 1 {
		t.Fatalf("unexpected phase: %#v", workflow)
	}
	if workflow.Steps[0].State != stepCurrent {
		t.Fatalf("divergence state = %q, want current", workflow.Steps[0].State)
	}
	if workflow.Steps[4].State != stepQueued || workflow.Steps[5].State != stepQueued {
		t.Fatalf("later work should be queued: %#v", workflow.Steps)
	}
}

func TestBuildWorkflowSynchronized(t *testing.T) {
	workflow := buildWorkflow(Counts{}, GitState{Available: true, Clean: true})
	if workflow.Phase != "synchronized" || workflow.Clear != workflow.Total {
		t.Fatalf("unexpected synchronized workflow: %#v", workflow)
	}
}

// chezmoi writes advisory warnings to stderr while still exiting 0. One such
// warning reached this parser and became a phantom "critical divergence",
// because "chezmoi: warning: ..." reads as code "ch" with two non-space
// characters. Non-status text must be ignored.
func TestParseStatusIgnoresNonStatusText(t *testing.T) {
	dest, target := testDest(t)
	real := target(".real.json")
	output := "chezmoi: warning: config file template has changed, run chezmoi init to regenerate config file\n" +
		"MM " + real + "\n" +
		"error: something went wrong\n"
	entries := parseStatus(output, dest, sourceMetadata{})
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1: %#v", len(entries), entries)
	}
	if entries[0].Path != real {
		t.Errorf("parsed the wrong line: %#v", entries[0])
	}
}

func TestIsStatusLine(t *testing.T) {
	// chezmoi status codes are a space or an uppercase letter. An unfamiliar
	// uppercase code is still accepted so it reaches the "unknown"
	// classification rather than being silently dropped.
	valid := []string{"MM /path", " M /path", "A  /path", " R /path", "XY /path"}
	for _, line := range valid {
		if !isStatusLine(line) {
			t.Errorf("isStatusLine(%q) = false, want true", line)
		}
	}
	invalid := []string{
		"chezmoi: warning: config file template has changed",
		"error: something went wrong",
		"MMno-separator",
		"m  /lowercase-is-not-a-status-code",
		"",
	}
	for _, line := range invalid {
		if isStatusLine(line) {
			t.Errorf("isStatusLine(%q) = true, want false", line)
		}
	}
}
