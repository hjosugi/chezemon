package state

import "fmt"

const (
	stepDone    = "done"
	stepCurrent = "current"
	stepQueued  = "queued"
)

func buildWorkflow(entries []Entry, git GitState) Workflow {
	counts := countEntries(entries)
	liveOnly := max(0, counts.LocalChanges-counts.Critical)
	pendingOnly := max(0, counts.Pending-counts.Critical)
	gitChanges := len(git.Changes)
	upstreamChanges := git.Ahead + git.Behind

	steps := []WorkflowStep{
		{
			ID:          "divergence",
			Title:       "Protect changes made on both sides",
			Description: "Compare source, rendered target, and live content. Merge deliberately before apply or re-add.",
			Count:       counts.Critical,
			QueueFilter: "critical",
		},
		{
			ID:          "live",
			Title:       "Decide what to do with live-only changes",
			Description: "Keep the live edit, restore the managed version, or move the useful part into source.",
			Count:       liveOnly,
			QueueFilter: "local",
		},
		{
			ID:          "git",
			Title:       "Stabilize the source Git working tree",
			Description: "Review and record intentional source edits so the desired state is not ambiguous.",
			Count:       gitChanges,
			QueueFilter: "git",
		},
		{
			ID:          "upstream",
			Title:       "Reconcile the source with its upstream",
			Description: "Pull behind commits carefully and push intentional local commits when the source is ready.",
			Count:       upstreamChanges,
			QueueFilter: "git",
		},
		{
			ID:          "scripts",
			Title:       "Inspect scripts and side effects",
			Description: "Check why each script will run, including network, package, and privilege effects.",
			Count:       counts.Scripts,
			QueueFilter: "script",
		},
		{
			ID:          "pending",
			Title:       "Review and apply desired file changes",
			Description: "Preview creates, updates, and deletes, then apply the smallest safe target set.",
			Count:       pendingOnly,
			QueueFilter: "pending",
		},
	}

	current := -1
	completed := 0
	for index := range steps {
		steps[index].Number = index + 1
		switch {
		case steps[index].Count == 0:
			steps[index].State = stepDone
			completed++
		case current == -1:
			steps[index].State = stepCurrent
			current = index
		default:
			steps[index].State = stepQueued
		}
	}

	if current == -1 {
		return Workflow{
			Phase:       "synchronized",
			PhaseLabel:  "Synchronized",
			Summary:     "No chezmoi drift or source Git work is currently visible.",
			Completed:   len(steps),
			Total:       len(steps),
			CurrentStep: 0,
			Steps:       steps,
		}
	}

	phase, label, summary := workflowPhase(steps[current], git)
	return Workflow{
		Phase:       phase,
		PhaseLabel:  label,
		Summary:     summary,
		Completed:   completed,
		Total:       len(steps),
		CurrentStep: current + 1,
		Steps:       steps,
	}
}

func workflowPhase(step WorkflowStep, git GitState) (phase, label, summary string) {
	switch step.ID {
	case "divergence":
		return "protect-work", "Protect work first",
			fmt.Sprintf("%d file(s) changed on both sides. Open a critical item, compare both versions, and merge before applying anything broadly.", step.Count)
	case "live":
		return "decide-live", "Decide what to keep",
			fmt.Sprintf("%d live-only change(s) need a keep, restore, or merge decision.", step.Count)
	case "git":
		return "stabilize-source", "Stabilize source",
			fmt.Sprintf("%d source Git change(s) need review before the desired state is considered durable.", step.Count)
	case "upstream":
		return "sync-source", "Sync source history",
			fmt.Sprintf("The last-known upstream position is %d ahead and %d behind. Reconcile source history before final apply.", git.Ahead, git.Behind)
	case "scripts":
		return "inspect-scripts", "Inspect side effects",
			fmt.Sprintf("%d script(s) would run. Inspect their reason and effects before apply.", step.Count)
	case "pending":
		return "review-apply", "Review desired changes",
			fmt.Sprintf("%d file change(s) remain. Preview each risky change, then apply a narrow target set.", step.Count)
	default:
		return "review", "Review state", "Review the remaining items in the queue."
	}
}
