package direct

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const (
	// diffClip caps git_diff output to keep the context window bounded.
	diffClip = 300_000
	// maxUntrackedFiles bounds how many new (untracked) files are inlined.
	maxUntrackedFiles = 200
	// emptyDiff is the sentinel returned when there is nothing to review.
	emptyDiff = "empty diff"
)

// refRe allows only ref-ish characters, blocking argv injection via base/head.
var refRe = regexp.MustCompile(`^[\w./@~^-]+$`)

func validRef(s string) bool {
	if s == "" {
		return true
	}
	return refRe.MatchString(s) && !strings.HasPrefix(s, "-")
}

// gitDiffTool shows the diff of the reviewed change. With both base and head it
// uses the three-dot merge-base diff (base...head) — the committed MR range. With
// only base (head empty, the local case) it shows the working tree against base
// INCLUDING uncommitted and new untracked files, so a review covers work that is
// not yet committed.
func gitDiffTool(root, defBase, defHead string) (ToolDef, Handler) {
	def := ToolDef{
		Name: "git_diff",
		Description: "Show the diff of the reviewed change. With base+head it is the committed range base...head. " +
			"With only base, it is the working tree vs base, including uncommitted edits and new untracked files. " +
			"Defaults come from the runner config.",
		Schema: objSchema(map[string]any{
			"base": strProp("Base ref (optional; defaults to the configured target branch)"),
			"head": strProp("Head ref (optional; defaults to the configured source branch). Leave empty to include uncommitted work."),
		}),
	}
	h := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Base string `json:"base"`
			Head string `json:"head"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &a); err != nil {
				return "", fmt.Errorf("git_diff: bad arguments: %w", err)
			}
		}
		base := cmp.Or(a.Base, defBase)
		head := cmp.Or(a.Head, defHead)
		if !validRef(base) || !validRef(head) {
			return "", fmt.Errorf("git_diff: invalid ref (base=%q head=%q)", base, head)
		}
		if base == "" && head != "" {
			return "", fmt.Errorf("git_diff: head=%q requires a base ref; pass base, or omit head to diff the working tree", head)
		}

		out, err := gitDiff(ctx, root, base, head)
		if err != nil {
			return "", fmt.Errorf("git_diff: %w", err)
		}
		if strings.TrimSpace(out) == "" {
			return emptyDiff, nil
		}
		return clipN(out, diffClip), nil
	}
	return def, h
}

// gitDiff computes the diff. base+head -> committed range. Otherwise the working
// tree vs base (vs HEAD when base is empty) plus every untracked file inlined as
// an addition, so uncommitted work is fully visible.
func gitDiff(ctx context.Context, root, base, head string) (string, error) {
	if base != "" && head != "" {
		return runGit(ctx, root, false, "--no-pager", "diff", base+"..."+head)
	}

	var b strings.Builder
	trackedArgs := []string{"--no-pager", "diff"}
	if base != "" {
		trackedArgs = append(trackedArgs, base)
	}
	tracked, err := runGit(ctx, root, false, trackedArgs...)
	if err != nil {
		return "", err
	}
	b.WriteString(tracked)

	// Untracked listing is best-effort: on failure the tracked diff is still
	// useful, so discard the error and inline whatever (if anything) we got.
	untracked, _ := runGit(ctx, root, false, "ls-files", "--others", "--exclude-standard")
	n := 0
	for _, f := range strings.Split(strings.TrimSpace(untracked), "\n") {
		if f == "" {
			continue
		}
		if n >= maxUntrackedFiles {
			b.WriteString("\n... [more untracked files omitted]\n")
			break
		}
		// --no-index against /dev/null renders the new file as an addition; it
		// exits 1 when content differs (expected), so allow exit code 1.
		d, derr := runGit(ctx, root, true, "--no-pager", "diff", "--no-index", "--", os.DevNull, f)
		if derr != nil {
			continue
		}
		b.WriteString(d)
		n++
	}
	return b.String(), nil
}

// runGit runs git -C root with the given args. When allowExit1 is set, an exit
// code of 1 (git diff "differences found") is treated as success.
func runGit(ctx context.Context, root string, allowExit1 bool, args ...string) (string, error) {
	full := append([]string{"-C", root}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	if err == nil {
		return string(out), nil
	}

	var ee *exec.ExitError
	switch {
	case !errors.As(err, &ee):
		return "", err
	case allowExit1 && ee.ExitCode() == 1:
		// git diff exits 1 when differences are found — treat as success.
		return string(out), nil
	case len(ee.Stderr) > 0:
		return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
	default:
		return "", err
	}
}
