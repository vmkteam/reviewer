package ctl

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"reviewsrv/pkg/reviewer"

	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

//go:embed review.html.tmpl
var reviewHTMLTmpl string

var reviewHTMLTemplate = template.Must(template.New("review.html").Parse(reviewHTMLTmpl))

// ReviewHTML holds data for the HTML template.
type ReviewHTML struct {
	Title    string
	Sections []ReviewSection
}

// ReviewSection represents one review type section.
type ReviewSection struct {
	Type    string
	Content template.HTML
}

// mermaidRenderer renders fenced code blocks with language "mermaid" as <pre class="mermaid">.
type mermaidRenderer struct{}

func (r *mermaidRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *mermaidRenderer) renderFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.FencedCodeBlock)
	if !ok {
		return ast.WalkContinue, nil
	}

	lang := string(n.Language(source))
	if lang != "mermaid" {
		return ast.WalkContinue, nil
	}

	if entering {
		_, _ = w.WriteString(`<pre class="mermaid">`)
		lines := n.Lines()
		for i := range lines.Len() {
			line := lines.At(i)
			_, _ = w.Write(line.Value(source))
		}
	} else {
		_, _ = w.WriteString("</pre>\n")
	}

	return ast.WalkSkipChildren, nil
}

var mdRenderer = goldmark.New(
	goldmark.WithExtensions(
		highlighting.NewHighlighting(
			highlighting.WithStyle("monokai"),
		),
	),
	goldmark.WithRendererOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(&mermaidRenderer{}, 100),
		),
	),
)

// RenderMarkdown converts markdown to HTML bytes.
func RenderMarkdown(md []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := mdRenderer.Convert(md, &buf); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateHTML generates review.html from R*.md files.
func GenerateHTML(dir string, title string, mdFiles map[string]string) error {
	data := ReviewHTML{
		Title: title,
	}

	typeOrder := reviewer.ReviewTypes
	for _, rt := range typeOrder {
		filePath, ok := mdFiles[rt]
		if !ok {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}

		html, err := RenderMarkdown(content)
		if err != nil {
			return fmt.Errorf("render %s: %w", rt, err)
		}

		data.Sections = append(data.Sections, ReviewSection{
			Type:    capitalizeFirst(rt),
			Content: template.HTML(html),
		})
	}

	var buf bytes.Buffer
	if err := reviewHTMLTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute html template: %w", err)
	}

	outPath := filepath.Join(dir, "review.html")
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write review.html: %w", err)
	}

	return nil
}
