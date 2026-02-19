package rest

import (
	"fmt"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"
)

type ReviewDraft struct {
	Review struct {
		ExternalID   string    `json:"externalId"`
		Title        string    `json:"title"`
		Description  string    `json:"description"`
		CommitHash   string    `json:"commitHash"`
		SourceBranch string    `json:"sourceBranch"`
		TargetBranch string    `json:"targetBranch"`
		Author       string    `json:"author"`
		CreatedAt    time.Time `json:"createdAt"`
		DurationMs   int       `json:"durationMs"`
		ModelInfo    struct {
			Model        string  `json:"model"`
			InputTokens  int     `json:"inputTokens"`
			OutputTokens int     `json:"outputTokens"`
			CostUsd      float64 `json:"costUsd"`
		} `json:"modelInfo"`
	} `json:"review"`
	Files []struct {
		ReviewType string `json:"reviewType"`
		Summary    string `json:"summary"`
		IsAccepted bool   `json:"isAccepted"`
	} `json:"files"`
	Issues []struct {
		LocalID     string `json:"localId"`
		Severity    string `json:"severity"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		File        string `json:"file"`
		Lines       string `json:"lines"`
		IssueType   string `json:"issueType"`
		FileType    string `json:"fileType"`
	} `json:"issues"`
}

// Validate checks that all reviewType and fileType values are valid.
func (rd ReviewDraft) Validate() error {
	for _, f := range rd.Files {
		if !reviewer.IsValidReviewType(f.ReviewType) {
			return fmt.Errorf("invalid reviewType: %s", f.ReviewType)
		}
	}
	for _, iss := range rd.Issues {
		if !reviewer.IsValidReviewType(iss.FileType) {
			return fmt.Errorf("invalid fileType: %s", iss.FileType)
		}
		if !reviewer.IsValidSeverity(iss.Severity) {
			return fmt.Errorf("invalid severity: %s", iss.Severity)
		}
	}
	return nil
}

// ToModel converts ReviewDraft to reviewer.Review with nested ReviewFiles and Issues.
func (rd ReviewDraft) ToModel() reviewer.Review {
	rv := reviewer.Review{
		Review: db.Review{
			Title:        rd.Review.Title,
			Description:  rd.Review.Description,
			ExternalID:   rd.Review.ExternalID,
			CommitHash:   rd.Review.CommitHash,
			SourceBranch: rd.Review.SourceBranch,
			TargetBranch: rd.Review.TargetBranch,
			Author:       rd.Review.Author,
			CreatedAt:    rd.Review.CreatedAt,
			DurationMS:   rd.Review.DurationMs,
			ModelInfo: db.ReviewModelInfo{
				Model:        rd.Review.ModelInfo.Model,
				InputTokens:  rd.Review.ModelInfo.InputTokens,
				OutputTokens: rd.Review.ModelInfo.OutputTokens,
				CostUsd:      rd.Review.ModelInfo.CostUsd,
			},
		},
	}

	issuesByType := make(map[string]reviewer.Issues, len(rd.Files))
	for _, iss := range rd.Issues {
		issuesByType[iss.FileType] = append(issuesByType[iss.FileType], reviewer.Issue{
			Issue: db.Issue{
				LocalID:     ptrString(iss.LocalID),
				IssueType:   iss.IssueType,
				Title:       iss.Title,
				Severity:    iss.Severity,
				Description: iss.Description,
				Content:     iss.Content,
				File:        iss.File,
				Lines:       iss.Lines,
			},
		})
	}

	rv.ReviewFiles = make(reviewer.ReviewFiles, len(rd.Files))
	for i, f := range rd.Files {
		rv.ReviewFiles[i] = reviewer.ReviewFile{
			ReviewFile: db.ReviewFile{
				ReviewType: f.ReviewType,
				Summary:    f.Summary,
				IsAccepted: f.IsAccepted,
			},
			Issues: issuesByType[f.ReviewType],
		}
	}

	return rv
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
