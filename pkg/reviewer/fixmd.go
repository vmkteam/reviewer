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

var fixMarkdownTemplate = template.Must(template.New("fix_markdown").Funcs(template.FuncMap{
	"inc":   func(i int) int { return i + 1 },
	"upper": strings.ToUpper,
	"deref": derefString,
	"trim":  strings.TrimSpace,
	"field": func(label, body string) fixField { return fixField{Label: label, Body: body} },
	// neutralise injected closing tags so attacker cannot escape <untrusted-data>.
	// zero-width space between the name and '>' breaks the literal match while
	// keeping the visual output identical.
	"safeBody": func(s string) string {
		return strings.ReplaceAll(s, "</untrusted-data>", "</untrusted-data\u200b>")
	},
}).Parse(fixMarkdownTmpl))

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
