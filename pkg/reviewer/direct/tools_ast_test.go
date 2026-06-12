package direct

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidAstArg(t *testing.T) {
	require.True(t, validAstArg("Foo"))
	require.True(t, validAstArg("Foo.Bar"))
	require.True(t, validAstArg("*Mailer"))
	require.True(t, validAstArg("read_file"))

	require.False(t, validAstArg(""))
	require.False(t, validAstArg("--rebuild"))
	require.False(t, validAstArg("-l"))
	require.False(t, validAstArg("a b"))
	require.False(t, validAstArg("a;rm -rf /"))
}

func TestAstToolsRegistration(t *testing.T) {
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: t.TempDir()})
	names := make(map[string]bool)
	for _, d := range reg.Defs() {
		names[d.Name] = true
	}
	require.True(t, names["read_files"])
	require.True(t, names["submit_review"])

	// AST tools are registered iff the binary is on PATH.
	if astIndexAvailable() {
		require.True(t, names["ast_symbol"])
		require.True(t, names["ast_refs"])
		require.True(t, names["ast_changed"])
	} else {
		require.False(t, names["ast_symbol"])
	}
}

func TestAstSymbolIntegration(t *testing.T) {
	if !astIndexAvailable() {
		t.Skip("ast-index not installed")
	}
	dir := t.TempDir()
	write(t, dir, "main.go", "package main\n\nfunc HelloWorld() string { return \"hi\" }\n")

	ran, err := EnsureAstIndex(context.Background(), dir)
	require.True(t, ran)
	require.NoError(t, err)

	_, h := astSymbolTool(dir)
	out, err := call(t, h, `{"name":"HelloWorld"}`)
	require.NoError(t, err)
	require.Contains(t, out, "HelloWorld")
}
