package ctl

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

//go:embed gitlab_comment.tmpl
var gitlabCommentTmpl string

var gitlabCommentTemplate = template.Must(template.New("gitlab_comment").Parse(gitlabCommentTmpl))

// GitLabClient posts comments to GitLab MR.
type GitLabClient struct {
	httpClient *http.Client
	log        *slog.Logger
	token      string
	apiURL     string
	projectID  string
	mrIID      string
	baseSHA    string
	headSHA    string
}

// NewGitLabClient creates a new GitLabClient from Config.
func NewGitLabClient(cfg *Config, log *slog.Logger) *GitLabClient {
	return &GitLabClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		log:        log,
		token:      cfg.GitLabToken,
		apiURL:     strings.TrimRight(cfg.GitLabURL, "/"),
		projectID:  cfg.ProjectID,
		mrIID:      cfg.MRIID,
		baseSHA:    cfg.DiffBaseSHA,
		headSHA:    cfg.Commit,
	}
}

const reviewerMarker = "<!-- reviewer -->"

// PostAllComments cleans up previous inline discussions (without replies), then posts summary and new inline issues.
func (g *GitLabClient) PostAllComments(ctx context.Context, draft *rest.ReviewDraft, reviewURL string) {
	g.cleanupInlineDiscussions(ctx)

	if err := g.PostSummaryComment(ctx, draft, reviewURL); err != nil {
		g.log.WarnContext(ctx, "failed to post summary comment", "err", err)
	} else {
		g.log.InfoContext(ctx, "posted summary comment")
	}

	var inlineCount int
	for _, iss := range draft.Issues {
		if !isInlineSeverity(iss.Severity) {
			continue
		}
		if err := g.PostInlineCommentWithFallback(ctx, iss); err != nil {
			g.log.WarnContext(ctx, "failed to post inline comment", "localId", iss.LocalID, "err", err)
		} else {
			inlineCount++
			g.log.InfoContext(ctx, "posted inline comment", "localId", iss.LocalID, "severity", iss.Severity, "file", iss.File)
		}
	}

	g.log.InfoContext(ctx, "gitlab comments completed", "inlinePosted", inlineCount, "totalIssues", len(draft.Issues))
}

// PostSummaryComment posts the review summary as an MR note.
func (g *GitLabClient) PostSummaryComment(ctx context.Context, draft *rest.ReviewDraft, reviewURL string) error {
	body, err := renderSummaryComment(draft, reviewURL)
	if err != nil {
		return fmt.Errorf("render summary: %w", err)
	}

	return g.createNote(ctx, body)
}

// PostInlineCommentWithFallback tries inline discussion, falls back to plain note.
func (g *GitLabClient) PostInlineCommentWithFallback(ctx context.Context, issue rest.ReviewDraftIssue) error {
	line, ok := parseLinePosition(issue.Lines)
	if !ok || issue.File == "" {
		return g.createNote(ctx, formatIssueNote(issue))
	}

	err := g.createDiscussion(ctx, issue, line)
	if err != nil {
		g.log.WarnContext(ctx, "inline comment failed, falling back to note", "file", issue.File, "err", err)
		return g.createNote(ctx, formatIssueNote(issue))
	}

	return nil
}

func (g *GitLabClient) createNote(ctx context.Context, body string) error {
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%s/notes", g.apiURL, g.projectID, g.mrIID)
	payload, _ := json.Marshal(map[string]string{"body": body})
	return g.doJSONRequest(ctx, http.MethodPost, url, payload)
}

func (g *GitLabClient) createDiscussion(ctx context.Context, issue rest.ReviewDraftIssue, line int) error {
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", g.apiURL, g.projectID, g.mrIID)
	payload, _ := json.Marshal(map[string]any{
		"body": formatIssueNote(issue),
		"position": map[string]any{
			"base_sha":      g.baseSHA,
			"head_sha":      g.headSHA,
			"start_sha":     g.baseSHA,
			"position_type": "text",
			"new_path":      issue.File,
			"new_line":      line,
		},
	})
	return g.doJSONRequest(ctx, http.MethodPost, url, payload)
}

func (g *GitLabClient) doJSONRequest(ctx context.Context, method, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.token)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// cleanupInlineDiscussions deletes previous reviewer inline discussions that have no replies.
// Discussions with replies (notes_count > 1) are preserved to keep conversation context.
// Summary notes are never deleted — they show review progress history.
func (g *GitLabClient) cleanupInlineDiscussions(ctx context.Context) {
	url := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions?per_page=100", g.apiURL, g.projectID, g.mrIID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		g.log.WarnContext(ctx, "cleanup: failed to create request", "err", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+g.token)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		g.log.WarnContext(ctx, "cleanup: failed to fetch discussions", "err", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var discussions []struct {
		ID    string `json:"id"`
		Notes []struct {
			ID     int    `json:"id"`
			Type   string `json:"type"`
			Body   string `json:"body"`
			System bool   `json:"system"`
		} `json:"notes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&discussions); err != nil {
		g.log.WarnContext(ctx, "cleanup: failed to decode discussions", "err", err)
		return
	}

	var deleted, skipped int
	for _, d := range discussions {
		if len(d.Notes) == 0 || d.Notes[0].System {
			continue
		}
		// Only clean up inline diff discussions, not summary notes.
		if d.Notes[0].Type != "DiffNote" {
			continue
		}
		if !strings.Contains(d.Notes[0].Body, reviewerMarker) {
			continue
		}
		// Skip discussions where someone replied.
		if len(d.Notes) > 1 {
			skipped++
			continue
		}
		// Delete our single-note discussion.
		noteID := d.Notes[0].ID
		delURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions/%s/notes/%d", g.apiURL, g.projectID, g.mrIID, d.ID, noteID)
		delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, delURL, nil)
		if err != nil {
			continue
		}
		delReq.Header.Set("Authorization", "Bearer "+g.token)
		delResp, err := g.httpClient.Do(delReq)
		if err != nil {
			g.log.WarnContext(ctx, "cleanup: failed to delete discussion", "discussionId", d.ID, "err", err)
			continue
		}
		delResp.Body.Close()
		deleted++
	}

	if deleted > 0 || skipped > 0 {
		g.log.InfoContext(ctx, "cleaned up inline discussions", "deleted", deleted, "skippedWithReplies", skipped)
	}
}

func isInlineSeverity(severity string) bool {
	return severity == reviewer.SeverityCritical || severity == reviewer.SeverityHigh
}

// parseLinePosition extracts the first line number from "42-45" or "42".
func parseLinePosition(lines string) (int, bool) {
	if lines == "" {
		return 0, false
	}
	parts := strings.SplitN(lines, "-", 2)
	n, err := strconv.Atoi(parts[0])
	return n, err == nil
}

func formatIssueNote(issue rest.ReviewDraftIssue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🔴 **%s. %s** (%s)\n\n", issue.LocalID, issue.Title, issue.IssueType)
	fmt.Fprintf(&b, "%s\n", issue.Description)

	if issue.SuggestedFix != "" {
		fmt.Fprintf(&b, "\n**Suggested fix:**\n%s\n", issue.SuggestedFix)
	}

	fmt.Fprintf(&b, "\n%s\n", reviewerMarker)
	return b.String()
}

// summaryData holds template data for the summary comment.
type summaryData struct {
	TrafficLightEmoji string
	TrafficLightText  string
	Model             string
	CostUsd           float64
	Duration          string
	EffortMinutes     int
	Description       string
	Files             []summaryFile
	CriticalIssues    []summaryIssue
	ReviewURL         string
}

type summaryFile struct {
	ReviewType        string
	Summary           string
	IssuesSummary     string
	TrafficLightEmoji string
}

type summaryIssue struct {
	LocalID     string
	Title       string
	File        string
	Lines       string
	IssueType   string
	Description string
}

func renderSummaryComment(draft *rest.ReviewDraft, reviewURL string) (string, error) {
	issuesByType := make(map[string]map[string]int)
	for _, iss := range draft.Issues {
		if issuesByType[iss.FileType] == nil {
			issuesByType[iss.FileType] = make(map[string]int)
		}
		issuesByType[iss.FileType][iss.Severity]++
	}

	data := summaryData{
		Model:       draft.Review.ModelInfo.Model,
		CostUsd:     draft.Review.ModelInfo.CostUsd,
		Duration:    formatDuration(draft.Review.DurationMs),
		Description: draft.Review.Description,
		ReviewURL:   reviewURL,
	}

	if draft.Review.EffortMinutes > 0 {
		data.EffortMinutes = draft.Review.EffortMinutes
	}

	// Determine traffic light using the same logic as pkg/reviewer.
	var totalCritical, totalHigh, totalMedium int
	for _, counts := range issuesByType {
		totalCritical += counts[reviewer.SeverityCritical]
		totalHigh += counts[reviewer.SeverityHigh]
		totalMedium += counts[reviewer.SeverityMedium]
	}
	data.TrafficLightEmoji, data.TrafficLightText = trafficLightDisplay(totalCritical, totalHigh, totalMedium)

	for _, f := range draft.Files {
		counts := issuesByType[f.ReviewType]
		sf := summaryFile{
			ReviewType:    capitalizeFirst(f.ReviewType),
			Summary:       f.Summary,
			IssuesSummary: formatIssueCounts(counts),
		}
		fc, fh, fm := counts[reviewer.SeverityCritical], counts[reviewer.SeverityHigh], counts[reviewer.SeverityMedium]
		sf.TrafficLightEmoji, _ = trafficLightDisplay(fc, fh, fm)
		data.Files = append(data.Files, sf)
	}

	for _, iss := range draft.Issues {
		if !isInlineSeverity(iss.Severity) {
			continue
		}
		data.CriticalIssues = append(data.CriticalIssues, summaryIssue{
			LocalID:     iss.LocalID,
			Title:       iss.Title,
			File:        iss.File,
			Lines:       iss.Lines,
			IssueType:   iss.IssueType,
			Description: iss.Description,
		})
	}

	var buf bytes.Buffer
	if err := gitlabCommentTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// trafficLightDisplay returns emoji and text using the canonical traffic light logic.
func trafficLightDisplay(critical, high, medium int) (emoji, text string) {
	tl := reviewer.CalcTrafficLight(reviewer.IssueStats{
		Critical: critical,
		High:     high,
		Medium:   medium,
	})
	switch tl {
	case "red":
		return "🔴", "Red Light"
	case "yellow":
		return "🟡", "Yellow Light"
	default:
		return "🟢", "Green Light"
	}
}

func formatDuration(ms int) string {
	d := time.Duration(ms) * time.Millisecond
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func formatIssueCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "0"
	}

	var parts []string
	for _, sev := range reviewer.Severities {
		if n := counts[sev]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, sev))
		}
	}
	if len(parts) == 0 {
		return "0"
	}
	return strings.Join(parts, ", ")
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
