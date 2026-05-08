package ctl

import (
	"os"
	"path/filepath"
	"testing"

	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteReviewSkeleton(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		ExternalID:   "599",
		MRTitle:      "Add discovery RPC",
		Commit:       "deadbeef",
		SourceBranch: "feat/x",
		TargetBranch: "master",
		Author:       "alice",
	}

	require.NoError(t, WriteReviewSkeleton(dir, cfg))

	draft, err := ReadReviewJSON(dir)
	require.NoError(t, err, "skeleton must satisfy our own validator")

	assert.Equal(t, "599", draft.Review.ExternalID)
	assert.Equal(t, "Add discovery RPC", draft.Review.Title)
	assert.Equal(t, "deadbeef", draft.Review.CommitHash)
	assert.Equal(t, "feat/x", draft.Review.SourceBranch)
	assert.Equal(t, "master", draft.Review.TargetBranch)
	assert.Equal(t, "alice", draft.Review.Author)

	require.Len(t, draft.Files, len(reviewer.ReviewTypes), "skeleton must mirror canonical list")
	for i, want := range reviewer.ReviewTypes {
		assert.Equal(t, want, draft.Files[i].ReviewType, "files[%d].reviewType", i)
	}
	assert.Empty(t, draft.Issues, "skeleton issues must start empty")
}

func TestWriteReviewSkeleton_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	// Stale draft from a prior run that the validator would reject.
	stalePath := filepath.Join(dir, "review.json")
	require.NoError(t, os.WriteFile(stalePath, []byte(`{"branch": "old"}`), 0o644))

	require.NoError(t, WriteReviewSkeleton(dir, &Config{}))

	draft, err := ReadReviewJSON(dir)
	require.NoError(t, err)
	assert.Len(t, draft.Files, 5, "skeleton must replace the stale file")
}
