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

// pathspec returns the trailing git pathspec: scoped to a single path when one is
// given, otherwise the whole tree minus vendored/generated trees (which would
// otherwise swamp the review diff).
func pathspec(path string) []string {
	if strings.TrimSpace(path) != "" {
		return []string{"--", path}
	}
	return []string{"--", ".",
		":(exclude)vendor", ":(exclude)node_modules",
		":(exclude)frontend/dist", ":(exclude)frontend/dist-vt"}
}

// withExcludes appends the whole-tree pathspec (vendored/generated trees excluded).
func withExcludes(args ...string) []string {
	return append(args, pathspec("")...)
}

// validPath guards the git_diff path argument: relative, no traversal, not an
// option. It sits after "--" in argv so option injection is already blocked;
// this also keeps the diff scoped inside the repository.
func validPath(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "-") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return false
		}
	}
	return true
}

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
			"Pass path to get the FULL diff of a single file — use this when the pre-loaded diff was truncated. " +
			"Defaults come from the runner config.",
		Schema: objSchema(map[string]any{
			"base": strProp("Base ref (optional; defaults to the configured target branch)"),
			"head": strProp("Head ref (optional; defaults to the configured source branch). Leave empty to include uncommitted work."),
			"path": strProp("Optional: limit the diff to a single changed file (path relative to the repository root)."),
		}),
	}
	h := func(ctx context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Base string `json:"base"`
			Head string `json:"head"`
			Path string `json:"path"`
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
		if a.Path != "" && !validPath(a.Path) {
			return "", fmt.Errorf("git_diff: invalid path %q", a.Path)
		}
		if base == "" && head != "" {
			return "", fmt.Errorf("git_diff: head=%q requires a base ref; pass base, or omit head to diff the working tree", head)
		}

		out, err := gitDiff(ctx, root, base, head, a.Path)
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
func gitDiff(ctx context.Context, root, base, head, path string) (string, error) {
	ps := pathspec(path)
	if base != "" && head != "" {
		return runGit(ctx, root, false, append([]string{"--no-pager", "diff", base + "..." + head}, ps...)...)
	}

	var b strings.Builder
	trackedArgs := []string{"--no-pager", "diff"}
	if base != "" {
		trackedArgs = append(trackedArgs, base)
	}
	tracked, err := runGit(ctx, root, false, append(trackedArgs, ps...)...)
	if err != nil {
		return "", err
	}
	b.WriteString(tracked)

	// Untracked files only matter for the whole-tree view; with a specific path
	// the caller already named the file, so skip the untracked scan.
	if strings.TrimSpace(path) != "" {
		return b.String(), nil
	}

	// Untracked listing is best-effort: on failure the tracked diff is still
	// useful, so discard the error and inline whatever (if anything) we got.
	untracked, _ := runGit(ctx, root, false, withExcludes("ls-files", "--others", "--exclude-standard")...)
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
