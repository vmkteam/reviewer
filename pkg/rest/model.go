package rest

import (
	"fmt"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"
)

type ReviewDraft struct {
	Review struct {
		ExternalID    string             `json:"externalId"`
		Title         string             `json:"title"`
		Description   string             `json:"description"`
		CommitHash    string             `json:"commitHash"`
		SourceBranch  string             `json:"sourceBranch"`
		TargetBranch  string             `json:"targetBranch"`
		Author        string             `json:"author"`
		CreatedAt     time.Time          `json:"createdAt"`
		DurationMs    int                `json:"durationMs"`
		EffortMinutes int                `json:"effortMinutes"`
		AiSlopScore   float32            `json:"aiSlopScore"`
		ModelInfo     db.ReviewModelInfo `json:"modelInfo"`
	} `json:"review"`
	Files []struct {
		ReviewType string `json:"reviewType"`
		Summary    string `json:"summary"`
		IsAccepted bool   `json:"isAccepted"`
	} `json:"files"`
	Issues []ReviewDraftIssue `json:"issues"`
}

// ReviewDraftIssue represents a single issue in the review draft.
type ReviewDraftIssue struct {
	LocalID      string `json:"localId"`
	Severity     string `json:"severity"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Content      string `json:"content"`
	File         string `json:"file"`
	Lines        string `json:"lines"`
	IssueType    string `json:"issueType"`
	FileType     string `json:"fileType"`
	SuggestedFix string `json:"suggestedFix"`
}

// Validate checks that all reviewType and fileType values are valid.
// Errors include the offending index and value so the failure points at the
// specific element, not just the field name.
func (rd ReviewDraft) Validate() error {
	for i, f := range rd.Files {
		if !reviewer.IsValidReviewType(f.ReviewType) {
			return fmt.Errorf("invalid reviewType at files[%d]: %q", i, f.ReviewType)
		}
	}
	for i, iss := range rd.Issues {
		if !reviewer.IsValidReviewType(iss.FileType) {
			return fmt.Errorf("invalid fileType at issues[%d] (localId=%s): %q", i, iss.LocalID, iss.FileType)
		}
		if !reviewer.IsValidSeverity(iss.Severity) {
			return fmt.Errorf("invalid severity at issues[%d] (localId=%s): %q", i, iss.LocalID, iss.Severity)
		}
	}
	return nil
}

// ToModel converts ReviewDraft to reviewer.Review with nested ReviewFiles and Issues.
func (rd ReviewDraft) ToModel() reviewer.Review {
	rv := reviewer.Review{
		Review: db.Review{
			Title:         rd.Review.Title,
			Description:   rd.Review.Description,
			ExternalID:    rd.Review.ExternalID,
			CommitHash:    rd.Review.CommitHash,
			SourceBranch:  rd.Review.SourceBranch,
			TargetBranch:  rd.Review.TargetBranch,
			Author:        rd.Review.Author,
			CreatedAt:     rd.Review.CreatedAt,
			DurationMS:    rd.Review.DurationMs,
			EffortMinutes: ptrInt(rd.Review.EffortMinutes),
			AiSlopScore:   ptrFloat32(rd.Review.AiSlopScore),
			ModelInfo:     rd.Review.ModelInfo,
		},
	}

	issuesByType := make(map[string]reviewer.Issues, len(rd.Files))
	for _, iss := range rd.Issues {
		issuesByType[iss.FileType] = append(issuesByType[iss.FileType], reviewer.Issue{
			Issue: db.Issue{
				LocalID:      ptrString(iss.LocalID),
				IssueType:    iss.IssueType,
				Title:        iss.Title,
				Severity:     iss.Severity,
				Description:  iss.Description,
				Content:      iss.Content,
				File:         iss.File,
				Lines:        iss.Lines,
				SuggestedFix: ptrString(iss.SuggestedFix),
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

func ptrInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func ptrFloat32(v float32) *float32 {
	if v == 0 {
		return nil
	}
	return &v
}
