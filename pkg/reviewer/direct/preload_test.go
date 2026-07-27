package direct

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreloadContext(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	write(t, dir, "tracked.go", "package x\n")
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-q", "-m", "init")

	// Uncommitted edit + a new untracked file: both must appear in the preload.
	write(t, dir, "tracked.go", "package x\n// edited\n")
	write(t, dir, "newpkg/brand.go", "package newpkg\n// brand new\n")

	pc, preloaded := PreloadContext(context.Background(), dir, "", "") // working tree vs HEAD + untracked
	require.Contains(t, pc, "### Diff")
	require.Contains(t, pc, "// edited", "uncommitted edit in the diff")
	require.Contains(t, pc, "===== tracked.go =====")
	require.Contains(t, pc, "===== newpkg/brand.go =====", "untracked file preloaded")
	require.Contains(t, pc, "// brand new", "untracked file content preloaded")
	require.ElementsMatch(t, []string{"tracked.go", "newpkg/brand.go"}, preloaded)
}

func TestChangedFilesIncludesUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	write(t, dir, "a.go", "package a\n")
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "a.go", "package a\n// x\n")
	write(t, dir, "b.go", "package b\n")

	files, err := changedFiles(context.Background(), dir, "", "")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"a.go", "b.go"}, files)
}

func TestPreloadSkipsReviewerArtifacts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	write(t, dir, "a.go", "package a\n")
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-q", "-m", "init")

	// Leftovers from a previous reviewctl run + local agent state: none of these
	// are the change under review and must never be presented as the diff.
	write(t, dir, "review.json", `{"review":{}}`)
	write(t, dir, "review.html", "<html>")
	write(t, dir, "R1.ai.md", "# arch")
	write(t, dir, "direct-output.jsonl", "{}")
	write(t, dir, "claude-output.json", "{}")
	write(t, dir, "opencode-output.jsonl", "{}")
	write(t, dir, ".claude/memory/project-index.md", "# index")
	// A real untracked change stays visible.
	write(t, dir, "b.go", "package b\n// real work\n")

	files, err := changedFiles(context.Background(), dir, "", "")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"b.go"}, files)

	pc, preloaded := PreloadContext(context.Background(), dir, "", "")
	require.Contains(t, pc, "===== b.go =====")
	require.NotContains(t, pc, "review.json")
	require.NotContains(t, pc, "R1.ai.md")
	require.NotContains(t, pc, "project-index.md")
	require.ElementsMatch(t, []string{"b.go"}, preloaded)
}

func TestGitDiffSkipsReviewerArtifacts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	write(t, dir, "a.go", "package a\n")
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "review.json", `{"review":{}}`)
	write(t, dir, ".claude/memory/project-index.md", "# index")
	write(t, dir, "b.go", "package b\n// real work\n")

	out, err := gitDiff(context.Background(), dir, "", "", "")
	require.NoError(t, err)
	require.Contains(t, out, "b.go")
	require.NotContains(t, out, "review.json")
	require.NotContains(t, out, "project-index.md")
}

func TestDetectBaseRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	newRepo := func(t *testing.T) string {
		dir := t.TempDir()
		gitExec(t, dir, "init", "-q")
		gitExec(t, dir, "config", "user.email", "t@t")
		gitExec(t, dir, "config", "user.name", "t")
		gitExec(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
		return dir
	}

	// No remote refs at all → nothing to detect.
	require.Empty(t, DetectBaseRef(context.Background(), newRepo(t)))

	// Only a known integration branch ref exists → fallback candidate wins.
	dir := newRepo(t)
	gitExec(t, dir, "update-ref", "refs/remotes/origin/devel", "HEAD")
	require.Equal(t, "origin/devel", DetectBaseRef(context.Background(), dir))

	// origin/HEAD is authoritative when present, even for a non-standard name.
	dir = newRepo(t)
	gitExec(t, dir, "update-ref", "refs/remotes/origin/trunk", "HEAD")
	gitExec(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	require.Equal(t, "origin/trunk", DetectBaseRef(context.Background(), dir))
}
