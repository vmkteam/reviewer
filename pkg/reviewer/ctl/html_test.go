package ctl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdown(t *testing.T) {
	md := []byte("# Hello\n\nSome `inline code` and:\n\n```go\nfmt.Println(\"hello\")\n```\n")
	html, err := RenderMarkdown(md)
	require.NoError(t, err)

	result := string(html)
	assert.Contains(t, result, "<h1>Hello</h1>")
	assert.Contains(t, result, "<code")
}

func TestRenderMarkdown_Mermaid(t *testing.T) {
	md := []byte("```mermaid\nsequenceDiagram\n    A->>B: Hello\n```\n")
	html, err := RenderMarkdown(md)
	require.NoError(t, err)

	result := string(html)
	assert.Contains(t, result, `<pre class="mermaid">`)
	assert.Contains(t, result, "sequenceDiagram")
}

func TestGenerateHTML(t *testing.T) {
	tmpDir := t.TempDir()

	mdFiles := map[string]string{
		"architecture": "testdata/R1.architecture.md",
		"code":         "testdata/R2.code.md",
		"security":     "testdata/R3.security.md",
		"tests":        "testdata/R4.tests.md",
	}

	err := GenerateHTML(tmpDir, "Test Review", mdFiles)
	require.NoError(t, err)

	outPath := filepath.Join(tmpDir, "review.html")
	data, err := os.ReadFile(outPath)
	require.NoError(t, err)

	html := string(data)
	assert.Contains(t, html, "Test Review")
	assert.Contains(t, html, "Architecture")
	assert.Contains(t, html, "Code")
	assert.Contains(t, html, `<pre class="mermaid">`)
	assert.Contains(t, html, "mermaid.min.js")
}
