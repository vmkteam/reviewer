package ctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	md := []byte("# Hello\n\nSome `inline code` and:\n\n```go\nfmt.Println(\"hello\")\n```\n")
	html, err := RenderMarkdown(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := string(html)
	if !strings.Contains(result, "<h1>Hello</h1>") {
		t.Errorf("should contain h1, got: %s", result)
	}
	if !strings.Contains(result, "<code") {
		t.Errorf("should contain code block, got: %s", result)
	}
}

func TestRenderMarkdown_Mermaid(t *testing.T) {
	md := []byte("```mermaid\nsequenceDiagram\n    A->>B: Hello\n```\n")
	html, err := RenderMarkdown(md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := string(html)
	if !strings.Contains(result, `<pre class="mermaid">`) {
		t.Errorf("should contain mermaid pre, got: %s", result)
	}
	if !strings.Contains(result, "sequenceDiagram") {
		t.Errorf("should contain diagram content, got: %s", result)
	}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	outPath := filepath.Join(tmpDir, "review.html")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read review.html: %v", err)
	}

	html := string(data)
	if !strings.Contains(html, "Test Review") {
		t.Error("should contain title")
	}
	if !strings.Contains(html, "Architecture") {
		t.Error("should contain Architecture section")
	}
	if !strings.Contains(html, "Code") {
		t.Error("should contain Code section")
	}
	if !strings.Contains(html, `<pre class="mermaid">`) {
		t.Error("should contain mermaid diagram")
	}
	if !strings.Contains(html, "mermaid.min.js") {
		t.Error("should contain mermaid script")
	}
}
