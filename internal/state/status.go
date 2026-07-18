package state

import (
	"bufio"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func parseStatus(output string, destDir string, meta sourceMetadata) []Entry {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 || !isStatusLine(line) {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[2:])
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(destDir, path)
		}
		path = filepath.Clean(path)

		class := classifyStatus(code)
		sourceRelative := meta.sourceRelative[path]

		entry := Entry{
			Code:              code,
			Path:              path,
			DisplayPath:       displayPath(path, destDir),
			SourcePath:        sourceRelative,
			Kind:              class.Kind,
			Label:             class.Label,
			Risk:              class.Risk,
			Explanation:       class.Explanation,
			RecommendedAction: class.Action,
		}

		// A pending script is executed, not written to its nominal target
		// path, so showing that path as "~/name.sh" implies a file that will
		// never exist. Name the source script and say when it runs instead.
		if class.Kind == "script" {
			if sourceRelative != "" {
				entry.DisplayPath, entry.ScriptTiming = scriptDisplay(sourceRelative)
			} else {
				entry.DisplayPath = filepath.Base(path)
			}
			if entry.ScriptTiming != "" {
				entry.Explanation = fmt.Sprintf("%s This one runs %s.", entry.Explanation, entry.ScriptTiming)
			}
		}

		switch {
		case meta.encrypted[path]:
			entry.Sensitive, entry.SensitiveReason = true, "chezmoi encrypts this file"
		case isSensitivePath(path):
			entry.Sensitive, entry.SensitiveReason = true, "this path usually holds secrets"
		}

		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		left, right := riskRank(entries[i].Risk), riskRank(entries[j].Risk)
		if left != right {
			return left < right
		}
		return entries[i].DisplayPath < entries[j].DisplayPath
	})
	return entries
}

// isStatusLine reports whether a line has the "XY <path>" shape chezmoi uses,
// where each of X and Y is a space or an uppercase status letter.
//
// This is defence in depth against non-status text reaching the parser. Prose
// such as "chezmoi: warning: ..." would otherwise be read as code "ch" with a
// path of "ezmoi: warning: ...", and two non-space characters classify as a
// critical divergence — the loudest thing the queue can show. Unrecognised
// uppercase codes still pass through and land in the "unknown" classification,
// so a future chezmoi status code is reported rather than silently dropped.
func isStatusLine(line string) bool {
	if len(line) < 3 || line[2] != ' ' {
		return false
	}
	for _, char := range line[:2] {
		if char != ' ' && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

// classification is the plain-language reading of a two-character chezmoi
// status code.
type classification struct {
	Kind        string
	Label       string
	Risk        string
	Explanation string
	Action      string
}

func classifyStatus(code string) classification {
	if len(code) != 2 {
		return classification{"unknown", "Unknown", "medium",
			"Chezemon could not classify this status.",
			"Inspect the entry before changing it."}
	}
	actual, target := code[0], code[1]
	if actual != ' ' && target != ' ' {
		return classification{"diverged", "Both sides changed", "critical",
			"The live file changed and the rendered target would also change it. Applying or re-adding blindly may lose work.",
			"Review a three-way diff and merge the changes."}
	}

	switch target {
	case 'R':
		return classification{"script", "Script will run", "high",
			"A chezmoi script is scheduled to run on apply. Scripts may change files, install software, use the network, or request privileges.",
			"Inspect the script and run reason before applying."}
	case 'D':
		return classification{"pending-delete", "Will be deleted", "high",
			"The rendered target says this live path should be removed.",
			"Confirm the deletion in the diff before applying."}
	case 'M':
		return classification{"pending-modify", "Pending apply", "medium",
			"The rendered target differs from the live file. Applying will modify the live file.",
			"Review the diff, then apply this file if the target is correct."}
	case 'A':
		return classification{"pending-create", "Will be created", "low",
			"The rendered target contains a path that is not present in the live home.",
			"Review the new content, then apply this file."}
	}

	switch actual {
	case 'M':
		return classification{"local-change", "Live file changed", "medium",
			"The live file changed since chezmoi last wrote it.",
			"Decide whether to keep the live change, discard it, or merge it."}
	case 'D':
		return classification{"local-delete", "Live file deleted", "high",
			"The live path was deleted since chezmoi last wrote it.",
			"Decide whether the deletion should be captured or the file restored."}
	case 'A':
		return classification{"local-create", "Live file created", "low",
			"A new live path is relevant to the managed state.",
			"Inspect it before adding it to source."}
	}

	return classification{"unknown", fmt.Sprintf("Status %q", code), "medium",
		"Chezemon does not yet have a plain-language explanation for this status.",
		"Inspect the raw chezmoi status and diff."}
}

func displayPath(path, destDir string) string {
	rel, err := filepath.Rel(destDir, path)
	if err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "~/" + filepath.ToSlash(rel)
	}
	if path == filepath.Clean(destDir) {
		return "~"
	}
	return filepath.ToSlash(path)
}

func isSensitivePath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	segments := strings.Split(lower, "/")
	for _, segment := range segments {
		switch segment {
		// ".git-credentials" and any "credentials" file are already covered
		// by the "credential" substring rule below.
		case ".ssh", ".gnupg", ".aws", ".kube", ".password-store",
			".docker", ".age", ".pki",
			".netrc", "netrc", ".npmrc", "npmrc", ".pypirc", "pypirc",
			"hosts.yml":
			return true
		}
		if segment == ".env" || strings.HasPrefix(segment, ".env.") {
			return true
		}
	}
	sensitiveWords := []string{
		"secret", "token", "credential", "private_key", "id_rsa", "id_ed25519",
		"rclone.conf", "auth.json", "keyring",
	}
	for _, word := range sensitiveWords {
		if strings.Contains(lower, word) {
			return true
		}
	}
	// Key material is identified by extension rather than substring so that
	// names like "keybindings.json" are not masked as secrets.
	for _, suffix := range []string{".pem", ".key", ".p12", ".pfx", ".jks", ".keystore"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func riskRank(risk string) int {
	switch risk {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	default:
		return 4
	}
}

func countEntries(entries []Entry) Counts {
	counts := Counts{Total: len(entries)}
	for _, entry := range entries {
		switch entry.Risk {
		case "critical":
			counts.Critical++
		case "high":
			counts.High++
		}
		switch entry.Kind {
		case "script":
			counts.Scripts++
		case "local-change", "local-delete", "local-create":
			counts.LocalChanges++
		case "pending-create":
			counts.Pending++
			counts.PendingCreate++
		case "pending-modify":
			counts.Pending++
			counts.PendingModify++
		case "pending-delete":
			counts.Pending++
			counts.PendingDelete++
		case "diverged":
			counts.Pending++
			counts.LocalChanges++
		}
	}
	return counts
}
