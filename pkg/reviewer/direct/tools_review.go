package direct

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

// unwrapJSON tolerates a common LLM mistake: a nested object/array sent as a
// JSON-encoded string (double-encoding). If raw is a JSON string, its decoded
// contents are returned; otherwise raw is returned unchanged.
func unwrapJSON(raw json.RawMessage) json.RawMessage {
	t := bytes.TrimSpace(raw)
	if len(t) > 0 && t[0] == '"' {
		var s string
		if json.Unmarshal(t, &s) == nil {
			return json.RawMessage(s)
		}
	}
	return raw
}

// Review-type identifiers, in canonical order (R1..R5).
const (
	rtArchitecture = "architecture"
	rtCode         = "code"
	rtSecurity     = "security"
	rtTests        = "tests"
	rtOperability  = "operability"
)

// Schema property key / tool-name string constants, centralised so the schema
// builders and handlers don't repeat the same literal three or more times.
const (
	toolSubmitReview = "submit_review"

	fSummary     = "summary"
	fIsAccepted  = "isAccepted"
	fMarkdown    = "markdown"
	fReviewType  = "reviewType"
	fSeverity    = "severity"
	fFile        = "file"
	fFileType    = "fileType"
	fLocalID     = "localId"
	fTitle       = "title"
	fDescription = "description"
	fIssues      = "issues"
)

// reviewTypes are the five review groups, in canonical order.
var reviewTypes = []string{rtArchitecture, rtCode, rtSecurity, rtTests, rtOperability}

// mdPrefixByType maps a review type to the R*.md filename prefix that
// ctl.FindMDFiles discovers on upload (R1=architecture … R5=operability).
var mdPrefixByType = map[string]string{
	rtArchitecture: "R1",
	rtCode:         "R2",
	rtSecurity:     "R3",
	rtTests:        "R4",
	rtOperability:  "R5",
}

// mdSuffix is the fixed suffix for direct-runner markdown files; FindMDFiles
// matches on the "R1." prefix, so the middle segment is free.
const mdSuffix = ".ai.md"

// findingHeaderRe matches a localId-style finding header inside a markdown body.
var findingHeaderRe = regexp.MustCompile(`(?m)^#{2,4}\s+[A-Za-z]\d+\.`)

// reviewBuilder accumulates the review across the incremental tools (set_group,
// add_issues) so each tool call stays small. A large monolithic submit_review
// payload overflows a small model's output cap and arrives as truncated JSON.
type reviewBuilder struct {
	mu     sync.Mutex
	groups map[string]groupData
	issues []rest.ReviewDraftIssue
}

type groupData struct {
	summary    string
	isAccepted bool
	markdown   string
}

func newReviewBuilder() *reviewBuilder {
	return &reviewBuilder{groups: make(map[string]groupData)}
}

func (b *reviewBuilder) setGroup(rt, summary string, isAccepted bool, markdown string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.groups[rt] = groupData{summary: summary, isAccepted: isAccepted, markdown: markdown}
}

func (b *reviewBuilder) addIssues(iss []rest.ReviewDraftIssue) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.issues = append(b.issues, iss...)
	return len(b.issues)
}

func (b *reviewBuilder) snapshot() (map[string]groupData, []rest.ReviewDraftIssue) {
	b.mu.Lock()
	defer b.mu.Unlock()
	g := make(map[string]groupData, len(b.groups))
	for k, v := range b.groups {
		g[k] = v
	}
	return g, append([]rest.ReviewDraftIssue(nil), b.issues...)
}

// setGroupTool sets one review group (summary + isAccepted + markdown body) — a
// small payload, called once per group.
func setGroupTool(b *reviewBuilder) (ToolDef, Handler) {
	def := ToolDef{
		Name: "set_group",
		Description: "Set one review group: its one-line summary, isAccepted, and full markdown body. " +
			"Call once per group (architecture, code, security, tests, operability). Small payload — preferred over packing all groups into submit_review.",
		Schema: objSchema(map[string]any{
			fReviewType: reviewTypeProp(),
			fSummary:    strProp("One-line summary for this group"),
			fIsAccepted: boolProp(),
			fMarkdown:   strProp("Full markdown body; head each finding with ### C1. Title"),
		}, fReviewType, fSummary, fIsAccepted, fMarkdown),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			ReviewType string `json:"reviewType"`
			Summary    string `json:"summary"`
			IsAccepted bool   `json:"isAccepted"`
			Markdown   string `json:"markdown"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("set_group: bad arguments: %w", err)
		}
		if !reviewer.IsValidReviewType(a.ReviewType) {
			return "", fmt.Errorf("set_group: invalid reviewType %q", a.ReviewType)
		}
		if strings.TrimSpace(a.Summary) == "" || strings.TrimSpace(a.Markdown) == "" {
			return "", fmt.Errorf("set_group: summary and markdown are required for %s", a.ReviewType)
		}
		b.setGroup(a.ReviewType, a.Summary, a.IsAccepted, a.Markdown)
		return fmt.Sprintf("group %q set", a.ReviewType), nil
	}
	return def, h
}

// addIssuesTool appends a batch of issues — called one or more times, small.
func addIssuesTool(b *reviewBuilder) (ToolDef, Handler) {
	def := ToolDef{
		Name: "add_issues",
		Description: "Append a batch of issues to the review. Call one or more times; keep batches small to avoid output truncation. " +
			"Every ### finding in a group's markdown must have a matching issue.",
		Schema: objSchema(map[string]any{
			fIssues: arrayOf(issueItemSchema()),
		}, fIssues),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Issues []rest.ReviewDraftIssue `json:"issues"`
		}
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("add_issues: bad arguments: %w", err)
		}
		for i, iss := range a.Issues {
			if !reviewer.IsValidSeverity(iss.Severity) {
				return "", fmt.Errorf("add_issues: invalid severity at [%d] (localId=%s): %q", i, iss.LocalID, iss.Severity)
			}
			if !reviewer.IsValidReviewType(iss.FileType) {
				return "", fmt.Errorf("add_issues: invalid fileType at [%d] (localId=%s): %q", i, iss.LocalID, iss.FileType)
			}
		}
		total := b.addIssues(a.Issues)
		return fmt.Sprintf("added %d issues (total %d)", len(a.Issues), total), nil
	}
	return def, h
}

// submitReviewTool finalizes the review. Preferred: set_group ×5 + add_issues,
// then submit_review with only the overall metadata. Dual-mode: a capable model
// may also pass files/issues/markdown directly for a one-shot submit; the two are
// merged. Writes review.json + R*.md and marks the run submitted.
func submitReviewTool(dir string, b *reviewBuilder, reg *Registry) (ToolDef, Handler) {
	def := ToolDef{
		Name: toolSubmitReview,
		Description: "Finalize and submit the review (writes review.json + R*.md, ends the run). " +
			"Preferred: call set_group for all five groups and add_issues first, then submit_review with only the review metadata. " +
			"You MAY also pass files/issues/markdown here for a one-shot submit if your output fits.",
		Schema: objSchema(map[string]any{
			"review": objectProp(map[string]any{
				fDescription:    strProp("Overall verdict"),
				"effortMinutes": intProp("Estimated minutes to address the findings"),
				"aiSlopScore":   numberProp(),
				fTitle:          strProp("Optional title override"),
			}),
			"files":   arrayOf(fileItemSchema()),
			fIssues:   arrayOf(issueItemSchema()),
			fMarkdown: freeObject(),
		}),
	}
	h := func(_ context.Context, raw json.RawMessage) (string, error) {
		var a struct {
			Review struct {
				Description   string  `json:"description"`
				EffortMinutes int     `json:"effortMinutes"`
				AiSlopScore   float32 `json:"aiSlopScore"`
				Title         string  `json:"title"`
			} `json:"review"`
			Files    []rest.ReviewDraftFile  `json:"files"`
			Issues   []rest.ReviewDraftIssue `json:"issues"`
			Markdown map[string]string       `json:"markdown"`
		}
		if len(raw) > 0 {
			// Parse field-by-field and unwrap any double-encoded (stringified)
			// object/array — small models often send "review"/"issues" as a JSON
			// string. The streamed builder already holds the content, so a fumbled
			// one-shot field is non-fatal; the completeness check is the safety net.
			var top map[string]json.RawMessage
			if err := json.Unmarshal(raw, &top); err != nil {
				return "", fmt.Errorf("submit_review: bad arguments: %w", err)
			}
			_ = json.Unmarshal(unwrapJSON(top["review"]), &a.Review)
			_ = json.Unmarshal(unwrapJSON(top["files"]), &a.Files)
			_ = json.Unmarshal(unwrapJSON(top[fIssues]), &a.Issues)
			_ = json.Unmarshal(unwrapJSON(top["markdown"]), &a.Markdown)
		}

		// Merge the incremental builder with any one-shot args.
		groups, issues := b.snapshot()
		for _, f := range a.Files {
			g := groups[f.ReviewType]
			g.summary, g.isAccepted = f.Summary, f.IsAccepted
			groups[f.ReviewType] = g
		}
		for rt, md := range a.Markdown {
			g := groups[rt]
			g.markdown = md
			groups[rt] = g
		}
		issues = append(issues, a.Issues...)

		if err := checkGroupsComplete(groups, issues); err != nil {
			return "", err
		}

		// Merge onto the skeleton the controller wrote, so CI metadata
		// placeholders and the CreatedAt sentinel survive; fillMetadata fills the
		// rest afterwards.
		draft := loadOrNewDraft(dir)
		draft.Files = make([]rest.ReviewDraftFile, 0, len(reviewTypes))
		for _, rt := range reviewTypes {
			g := groups[rt]
			draft.Files = append(draft.Files, rest.ReviewDraftFile{ReviewType: rt, Summary: g.summary, IsAccepted: g.isAccepted})
		}
		draft.Issues = issues
		draft.Review.Description = a.Review.Description
		draft.Review.EffortMinutes = a.Review.EffortMinutes
		draft.Review.AiSlopScore = a.Review.AiSlopScore
		if a.Review.Title != "" {
			draft.Review.Title = a.Review.Title
		}

		if err := draft.Validate(); err != nil {
			return "", fmt.Errorf("submit_review: %w (fix and call submit_review again)", err)
		}

		data, err := json.MarshalIndent(draft, "", "  ")
		if err != nil {
			return "", fmt.Errorf("submit_review: marshal review.json: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "review.json"), data, 0o644); err != nil {
			return "", fmt.Errorf("submit_review: write review.json: %w", err)
		}
		for rt, g := range groups {
			prefix := mdPrefixByType[rt]
			if prefix == "" || strings.TrimSpace(g.markdown) == "" {
				continue
			}
			if err := os.WriteFile(filepath.Join(dir, prefix+mdSuffix), []byte(g.markdown), 0o644); err != nil {
				return "", fmt.Errorf("submit_review: write %s: %w", prefix, err)
			}
		}

		reg.markSubmitted()
		return fmt.Sprintf("review submitted: %d files, %d issues", len(draft.Files), len(draft.Issues)), nil
	}
	return def, h
}

// checkGroupsComplete enforces that every group has a non-empty summary and
// markdown body, and that every markdown finding ("### C1.") has an issue.
func checkGroupsComplete(groups map[string]groupData, issues []rest.ReviewDraftIssue) error {
	var noMarkdown, noSummary []string
	findings := 0
	for _, rt := range reviewTypes {
		g := groups[rt]
		if strings.TrimSpace(g.markdown) == "" {
			noMarkdown = append(noMarkdown, rt)
		}
		if strings.TrimSpace(g.summary) == "" {
			noSummary = append(noSummary, rt)
		}
		findings += len(findingHeaderRe.FindAllString(g.markdown, -1))
	}
	if len(noMarkdown) > 0 {
		return fmt.Errorf("submit_review: missing group(s): %s — call set_group for each (summary + markdown), then submit_review again", strings.Join(noMarkdown, ", "))
	}
	if len(noSummary) > 0 {
		return fmt.Errorf("submit_review: missing summary for: %s — set it via set_group, then submit_review again", strings.Join(noSummary, ", "))
	}
	if findings > len(issues) {
		return fmt.Errorf("submit_review: markdown has %d findings (### headers) but only %d issues — add_issues for the rest, then submit_review again", findings, len(issues))
	}
	return nil
}

// loadOrNewDraft reads the existing review.json skeleton, falling back to an
// empty draft if it is missing or unparseable.
func loadOrNewDraft(dir string) rest.ReviewDraft {
	var d rest.ReviewDraft
	if data, err := os.ReadFile(filepath.Join(dir, "review.json")); err == nil {
		_ = json.Unmarshal(data, &d)
	}
	return d
}

// reviewTypeProp builds a string schema property constrained to the five
// review-type values.
func reviewTypeProp() map[string]any {
	return enumProp("", reviewTypes...)
}

func issueItemSchema() map[string]any {
	return objSchema(map[string]any{
		fLocalID:       strProp("Stable id, matches a ### header"),
		fSeverity:      enumProp("", "critical", "high", "medium", "low"),
		fTitle:         strProp("Issue title"),
		fDescription:   strProp("What and why"),
		"content":      strProp("Optional code excerpt"),
		fFile:          strProp("Path relative to repository root"),
		"lines":        strProp("Line range, e.g. 10-20"),
		"issueType":    strProp("Free-form category"),
		fFileType:      reviewTypeProp(),
		"suggestedFix": strProp("Optional suggested fix"),
	}, fLocalID, fSeverity, fTitle, fDescription, fFile, fFileType)
}

func fileItemSchema() map[string]any {
	return objSchema(map[string]any{
		fReviewType: reviewTypeProp(),
		fSummary:    strProp("Short summary for this review group"),
		fIsAccepted: boolProp(),
	}, fReviewType, fSummary, fIsAccepted)
}
