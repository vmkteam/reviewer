package reviewer

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed fixmd.tmpl
var fixMarkdownTmpl string

type fixField struct {
	Label string
	Body  string
}

// SafeUntrustedBody neutralises injected closing tags so untrusted content
// cannot escape its <untrusted-data> wrapper: a zero-width space between the
// name and '>' breaks the literal match while keeping the visual output
// identical. The single definition shared by the markdown templates and the
// direct runner's http_fetch tool.
func SafeUntrustedBody(s string) string {
	return strings.ReplaceAll(s, "</untrusted-data>", "</untrusted-data\u200b>")
}

// markdownTemplateFuncs returns the shared FuncMap used by issue-list markdown
// templates (fix prompts, project instructions).
func markdownTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"inc":      func(i int) int { return i + 1 },
		"upper":    strings.ToUpper,
		"deref":    derefString,
		"trim":     strings.TrimSpace,
		"field":    func(label, body string) fixField { return fixField{Label: label, Body: body} },
		"safeBody": SafeUntrustedBody,
	}
}

var fixMarkdownTemplate = template.Must(template.New("fix_markdown").
	Funcs(markdownTemplateFuncs()).
	Parse(fixMarkdownTmpl))

type fixMarkdownData struct {
	Review  *Review
	Project *Project
	Issues  Issues
}

// BuildFixMarkdown renders valid issues of a review as a fix-task markdown
// document for Claude Code.
func BuildFixMarkdown(rv *Review, project *Project, issues Issues) (string, error) {
	var buf bytes.Buffer
	if err := fixMarkdownTemplate.Execute(&buf, fixMarkdownData{
		Review:  rv,
		Project: project,
		Issues:  issues,
	}); err != nil {
		return "", fmt.Errorf("render fix markdown: %w", err)
	}
	return buf.String(), nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
