package ctl

import (
	"cmp"
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
			ExternalID:   cmp.Or(cfg.ExternalID, PlaceholderExternalID),
			Title:        cmp.Or(cfg.MRTitle, PlaceholderTitle),
			CommitHash:   cmp.Or(cfg.Commit, PlaceholderCommitHash),
			SourceBranch: cmp.Or(cfg.SourceBranch, PlaceholderSourceBranch),
			TargetBranch: cmp.Or(cfg.TargetBranch, PlaceholderTargetBranch),
			Author:       cmp.Or(cfg.Author, PlaceholderAuthor),
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

// clearPlaceholders blanks CI skeleton placeholders (%AUTHOR% …) that a local
// run leaves unsubstituted, so literal placeholders never reach the server and
// the cfg/git fallbacks in fillMetadata get a chance to fill the fields. Kept
// next to WriteReviewSkeleton: this table must mirror the placeholder set the
// skeleton writes above.
func clearPlaceholders(draft *rest.ReviewDraft) {
	fields := []struct {
		v  *string
		ph string
	}{
		{&draft.Review.ExternalID, PlaceholderExternalID},
		{&draft.Review.Author, PlaceholderAuthor},
		{&draft.Review.SourceBranch, PlaceholderSourceBranch},
		{&draft.Review.TargetBranch, PlaceholderTargetBranch},
		{&draft.Review.CommitHash, PlaceholderCommitHash},
		{&draft.Review.Title, PlaceholderTitle},
		{&draft.Review.Title, PlaceholderMRTitle},
	}
	for _, f := range fields {
		if *f.v == f.ph {
			*f.v = ""
		}
	}
}
