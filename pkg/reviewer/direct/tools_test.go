package direct

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func call(t *testing.T, h Handler, args string) (string, error) {
	t.Helper()
	return h(context.Background(), json.RawMessage(args))
}

func TestGlobToRegexp(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "pkg/main.go", false},
		{"**/*.go", "main.go", true},
		{"**/*.go", "pkg/db/model.go", true},
		{"pkg/**/*.go", "pkg/db/model.go", true},
		{"pkg/**/*.go", "cmd/main.go", false},
		{"**/*_test.go", "pkg/x/y_test.go", true},
		{"*.go", "main.txt", false},
	}
	for _, c := range cases {
		re, err := globToRegexp(c.pattern)
		require.NoError(t, err)
		require.Equalf(t, c.want, re.MatchString(c.path), "pattern=%q path=%q", c.pattern, c.path)
	}
}

func TestReadFileRange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\nb\nc\nd\n"), 0o644))
	_, h := readFileTool(dir, newReadTracker(nil))

	full, err := call(t, h, `{"path":"f.txt"}`)
	require.NoError(t, err)
	require.Equal(t, "a\nb\nc\nd\n", full)

	slice, err := call(t, h, `{"path":"f.txt","offset":2,"limit":2}`)
	require.NoError(t, err)
	require.Equal(t, "b\nc", slice)
}

func TestReadFileEscapeRejected(t *testing.T) {
	dir := t.TempDir()
	_, h := readFileTool(dir, newReadTracker(nil))
	_, err := call(t, h, `{"path":"../secret"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "escapes repository root")
}

func TestGlobTool(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pkg", "db"), 0o755))
	write(t, dir, "main.go", "package main")
	write(t, dir, "pkg/db/model.go", "package db")
	write(t, dir, "notes.txt", "x")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755))
	write(t, dir, "node_modules/skip.go", "package skip")

	_, h := globTool(dir)

	out, err := call(t, h, `{"pattern":"**/*.go"}`)
	require.NoError(t, err)
	require.Equal(t, "main.go\npkg/db/model.go", out) // node_modules skipped, sorted

	out, err = call(t, h, `{"pattern":"*.go"}`)
	require.NoError(t, err)
	require.Equal(t, "main.go", out)
}

func TestReadFilesTool(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "package a")
	write(t, dir, "sub/b.go", "package b")
	_, h := readFilesTool(dir, newReadTracker(nil))

	out, err := call(t, h, `{"paths":["a.go","sub/b.go","missing.go"]}`)
	require.NoError(t, err)
	require.Contains(t, out, "===== a.go =====")
	require.Contains(t, out, "package a")
	require.Contains(t, out, "===== sub/b.go =====")
	require.Contains(t, out, "package b")
	require.Contains(t, out, "===== missing.go =====")
	require.Contains(t, out, "ERROR:")

	_, err = call(t, h, `{"paths":[]}`)
	require.Error(t, err)
}

func TestReadDedup(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "package a\nfunc A(){}\n")
	rt := newReadTracker(nil)
	_, rf := readFileTool(dir, rt)
	_, rfs := readFilesTool(dir, rt)

	// First full read serves content.
	out, err := call(t, rf, `{"path":"a.go"}`)
	require.NoError(t, err)
	require.Contains(t, out, "package a")

	// Second full read is deduped to a stub.
	out, err = call(t, rf, `{"path":"a.go"}`)
	require.NoError(t, err)
	require.Contains(t, out, "already provided")
	require.NotContains(t, out, "package a")

	// A ranged read is still served (targeted slice, not deduped).
	out, err = call(t, rf, `{"path":"a.go","offset":1,"limit":1}`)
	require.NoError(t, err)
	require.Equal(t, "package a", out)

	// read_files stubs an already-read path.
	out, err = call(t, rfs, `{"paths":["a.go"]}`)
	require.NoError(t, err)
	require.Contains(t, out, "already provided")

	// Seeded (pre-loaded) path is deduped from the first read.
	rt2 := newReadTracker([]string{"a.go"})
	_, rf2 := readFileTool(dir, rt2)
	out, err = call(t, rf2, `{"path":"./a.go"}`) // normalised path still matches
	require.NoError(t, err)
	require.Contains(t, out, "already provided")
}

func TestGrepTool(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.go", "package a\nfunc Foo() {}\n")
	write(t, dir, "b.txt", "Foo here too\n")

	_, h := grepTool(dir)
	out, err := call(t, h, `{"pattern":"func Foo","glob":"**/*.go"}`)
	require.NoError(t, err)
	require.Equal(t, "a.go:2:func Foo() {}", out)
}

func TestGitDiffTool(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitExec(t, dir, "init", "-q")
	gitExec(t, dir, "config", "user.email", "t@t")
	gitExec(t, dir, "config", "user.name", "t")
	write(t, dir, "x.go", "package x\n")
	gitExec(t, dir, "add", ".")
	gitExec(t, dir, "commit", "-q", "-m", "init")
	write(t, dir, "x.go", "package x\n// changed\n")

	_, h := gitDiffTool(dir, "", "")
	out, err := call(t, h, `{}`)
	require.NoError(t, err)
	require.Contains(t, out, "+// changed")
}

func TestGitDiffIncludesUncommittedAndUntracked(t *testing.T) {
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

	// Uncommitted: edit a tracked file and add a brand-new untracked file.
	write(t, dir, "tracked.go", "package x\n// edited\n")
	write(t, dir, "newpkg/brand_new.go", "package newpkg\n// untracked addition\n")

	_, h := gitDiffTool(dir, "", "") // head empty -> working tree + untracked
	out, err := call(t, h, `{}`)
	require.NoError(t, err)
	require.Contains(t, out, "+// edited", "uncommitted edit to tracked file")
	require.Contains(t, out, "brand_new.go", "untracked file path present")
	require.Contains(t, out, "+// untracked addition", "untracked file content shown as addition")
}

func TestGitDiffRejectsBadRef(t *testing.T) {
	_, h := gitDiffTool(t.TempDir(), "", "")
	_, err := call(t, h, `{"base":"--upload-pack=evil"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid ref")
}

func TestGitDiffHeadRequiresBase(t *testing.T) {
	_, h := gitDiffTool(t.TempDir(), "", "") // no configured default base
	_, err := call(t, h, `{"head":"feature"}`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires a base")
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
}

func gitExec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
}
