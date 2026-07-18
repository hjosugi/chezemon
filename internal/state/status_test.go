package state

import (
	"testing"
)

func TestParseStatusAndCounts(t *testing.T) {
	output := "MM /home/test/.config/app.json\n M /home/test/.bashrc\n R /home/test/setup.sh\n A /home/test/.new\n D /home/test/.old\n"
	entries := parseStatus(output, "/home/test", sourceMetadata{})
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
	entries := parseStatus(
		"MM /home/test/.config/app.json\n R /home/test/setup.sh\n M /home/test/.bashrc\n",
		"/home/test",
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
