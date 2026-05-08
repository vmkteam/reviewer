package reviewer

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"
	"text/template"
)

//go:embed instructions_md.tmpl
var instructionsMarkdownTmpl string

var instructionsMarkdownTemplate = template.Must(template.New("instructions_markdown").
	Funcs(markdownTemplateFuncs()).
	Parse(instructionsMarkdownTmpl))

type reviewTypeGroup struct {
	ReviewType string
	Issues     Issues
}

type instructionsMarkdownData struct {
	Project   *Project
	Groups    []reviewTypeGroup
	Total     int
	Truncated bool
	Limit     int
}

// BuildInstructionsMarkdown renders ignored issues of a project as a markdown
// document for an LLM to synthesize project-specific review instructions.
// truncated indicates the issue list was capped at ProjectInstructionsIssueLimit.
func BuildInstructionsMarkdown(project *Project, issues Issues, truncated bool) (string, error) {
	groups := groupByReviewType(issues)

	var buf bytes.Buffer
	if err := instructionsMarkdownTemplate.Execute(&buf, instructionsMarkdownData{
		Project:   project,
		Groups:    groups,
		Total:     len(issues),
		Truncated: truncated,
		Limit:     ProjectInstructionsIssueLimit,
	}); err != nil {
		return "", fmt.Errorf("render instructions markdown: %w", err)
	}
	return buf.String(), nil
}

// groupByReviewType buckets issues by ReviewFile.ReviewType, ordering buckets
// by the canonical ReviewTypes order; unknown/empty types fall to the end.
func groupByReviewType(issues Issues) []reviewTypeGroup {
	buckets := make(map[string]Issues)
	for _, issue := range issues {
		rt := ""
		if issue.ReviewFile != nil {
			rt = issue.ReviewFile.ReviewType
		}
		buckets[rt] = append(buckets[rt], issue)
	}

	groups := make([]reviewTypeGroup, 0, len(buckets))
	for _, rt := range ReviewTypes {
		if items, ok := buckets[rt]; ok {
			groups = append(groups, reviewTypeGroup{ReviewType: rt, Issues: items})
			delete(buckets, rt)
		}
	}
	rest := make([]string, 0, len(buckets))
	for rt := range buckets {
		rest = append(rest, rt)
	}
	slices.Sort(rest)
	for _, rt := range rest {
		groups = append(groups, reviewTypeGroup{ReviewType: rt, Issues: buckets[rt]})
	}
	return groups
}
