package direct

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
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

// ciClone builds the checkout a GitLab MR pipeline sees: an upstream with a
// "main" integration branch and a "feature-x" MR branch, cloned and detached at
// the MR commit — remote-tracking refs only, no local branch names.
func ciClone(t *testing.T) (clone, baseSHA string) {
	t.Helper()
	up := t.TempDir()
	gitExec(t, up, "init", "-q", "-b", "main")
	gitExec(t, up, "config", "user.email", "t@t")
	gitExec(t, up, "config", "user.name", "t")
	write(t, up, "a.go", "package a\n")
	gitExec(t, up, "add", ".")
	gitExec(t, up, "commit", "-q", "-m", "init")
	baseSHA = strings.TrimSpace(gitOut(t, up, "rev-parse", "HEAD"))
	gitExec(t, up, "checkout", "-q", "-b", "feature-x")
	write(t, up, "a.go", "package a\n// changed in the MR\n")
	gitExec(t, up, "commit", "-q", "-am", "change")

	clone = filepath.Join(t.TempDir(), "clone")
	gitExec(t, up, "clone", "-q", "--no-local", up, clone)
	gitExec(t, clone, "checkout", "-q", "--detach", "origin/feature-x")
	gitExec(t, clone, "branch", "-q", "-D", "feature-x")
	return clone, baseSHA
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...).Output()
	require.NoError(t, err)
	return string(out)
}

func TestResolveDiffRangeInCIClone(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	clone, baseSHA := ciClone(t)

	// The bare branch names don't resolve in the clone — the preload was empty.
	pc, _ := PreloadContext(ctx, clone, "main", "feature-x")
	require.Empty(t, pc)

	base, head := ResolveDiffRange(ctx, clone, "", "main", "feature-x")
	require.Equal(t, "origin/main", base)
	require.Equal(t, "HEAD", head)
	pc, preloaded := PreloadContext(ctx, clone, base, head)
	require.Contains(t, pc, "// changed in the MR")
	require.Equal(t, []string{"a.go"}, preloaded)

	// The MR diff base SHA wins when it is in the checkout.
	base, head = ResolveDiffRange(ctx, clone, baseSHA, "main", "feature-x")
	require.Equal(t, baseSHA, base)
	require.Equal(t, "HEAD", head)

	// An unknown base SHA (cut off by a shallow clone) falls through.
	base, _ = ResolveDiffRange(ctx, clone, strings.Repeat("a", 40), "main", "feature-x")
	require.Equal(t, "origin/main", base)
}

func TestResolveDiffRangeLocal(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q", "-b", "main")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	gitExec(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	gitExec(t, dir, "checkout", "-q", "-b", "feature-x")

	// Local branch names resolve as given.
	base, head := ResolveDiffRange(ctx, dir, "", "main", "feature-x")
	require.Equal(t, "main", base)
	require.Equal(t, "feature-x", head)

	// No CI metadata: nothing to resolve, head stays empty (working-tree mode).
	base, head = ResolveDiffRange(ctx, dir, "", "", "")
	require.Empty(t, base)
	require.Empty(t, head)

	// Option-like names are never passed to git.
	base, head = ResolveDiffRange(ctx, dir, "", "--output=/tmp/x", "--exec=x")
	require.Empty(t, base)
	require.Equal(t, "HEAD", head)
}
