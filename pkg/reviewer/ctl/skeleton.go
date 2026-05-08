package ctl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"reviewsrv/pkg/rest"
	"reviewsrv/pkg/reviewer"
)

// WriteReviewSkeleton writes a canonical empty review.json into dir,
// overwriting any existing file. Pre-creating the file removes the
// schema-drafting step where the model tends to invent its own field
// names (branch/baseBranch/tasks/summary, files[].path/reviewer, etc).
// The runner is then told to fill it in place.
//
// CI metadata fields fall back to placeholders (`%TITLE%`, `%COMMIT_HASH%`,
// etc.) when reviewctl runs outside CI without env vars. The prompt instructs
// the model to resolve those from git context — see promptReviewJSON.
func WriteReviewSkeleton(dir string, cfg *Config) error {
	files := make([]rest.ReviewDraftFile, len(reviewer.ReviewTypes))
	for i, rt := range reviewer.ReviewTypes {
		files[i] = rest.ReviewDraftFile{ReviewType: rt, IsAccepted: true}
	}

	draft := rest.ReviewDraft{
		Review: rest.ReviewDraftMeta{
			ExternalID:   orPlaceholder(cfg.ExternalID, "%EXTERNAL_ID%"),
			Title:        orPlaceholder(cfg.MRTitle, "%TITLE%"),
			CommitHash:   orPlaceholder(cfg.Commit, "%COMMIT_HASH%"),
			SourceBranch: orPlaceholder(cfg.SourceBranch, "%SOURCE_BRANCH%"),
			TargetBranch: orPlaceholder(cfg.TargetBranch, "%TARGET_BRANCH%"),
			Author:       orPlaceholder(cfg.Author, "%AUTHOR%"),
			// Sentinel that matches the prompt's "leave as-is" example.
			CreatedAt: time.Unix(0, 0).UTC(),
		},
		Files:  files,
		Issues: []rest.ReviewDraftIssue{},
	}

	data, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal review skeleton: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review.json"), data, 0o644); err != nil {
		return fmt.Errorf("write review skeleton: %w", err)
	}
	return nil
}

func orPlaceholder(value, placeholder string) string {
	if value == "" {
		return placeholder
	}
	return value
}
