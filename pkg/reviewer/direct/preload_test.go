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
