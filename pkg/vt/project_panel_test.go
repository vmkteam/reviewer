package vt

import (
	"context"
	"testing"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hasField(errs []FieldError, name string) bool {
	for _, e := range errs {
		if e.Field == name {
			return true
		}
	}
	return false
}

// The project panel fields (judgeRunnerProfileId + runnerProfileIds) are FK-validated
// in code, like the existing runnerProfileId. Validate runs on an unpersisted draft
// (ID=0), so no project row is needed — only the referenced profiles/prompt.
func TestDB_ProjectService_PanelValidation(t *testing.T) {
	dbc, logger := test.Setup(t)
	srv := NewProjectService(dbc, logger, "")
	ctx := t.Context()

	prompt, clP := test.Prompt(t, dbc, &db.Prompt{Title: "vt-panel-prompt", StatusID: db.StatusEnabled})
	t.Cleanup(clP)

	// The generated test.RunnerProfile cleaner deletes via t.Context() (canceled
	// before cleanups); delete with a background context instead.
	rp, _ := test.RunnerProfile(t, dbc, &db.RunnerProfile{Title: "vt-panel-rp", Runner: runner.RunnerClaude, StatusID: db.StatusEnabled})
	t.Cleanup(func() {
		_, _ = dbc.ModelContext(context.Background(), &db.RunnerProfile{ID: rp.ID}).WherePK().Delete()
	})

	base := func() Project {
		return Project{Title: "t", VcsURL: "https://x.example", Language: "Go", PromptID: prompt.ID, StatusID: db.StatusEnabled}
	}

	const badID = 999999999

	t.Run("bad judge id is flagged", func(t *testing.T) {
		p := base()
		j := badID
		p.JudgeRunnerProfileID = &j
		errs, err := srv.Validate(ctx, p)
		require.NoError(t, err)
		assert.True(t, hasField(errs, "judgeRunnerProfileId"))
	})

	t.Run("bad panel member id is flagged", func(t *testing.T) {
		p := base()
		p.RunnerProfileIDs = []int{rp.ID, badID}
		errs, err := srv.Validate(ctx, p)
		require.NoError(t, err)
		assert.True(t, hasField(errs, "runnerProfileIds"))
	})

	t.Run("valid judge and duplicate members pass", func(t *testing.T) {
		p := base()
		j := rp.ID
		p.JudgeRunnerProfileID = &j
		p.RunnerProfileIDs = []int{rp.ID, rp.ID} // duplicates allowed (self-fusion)
		errs, err := srv.Validate(ctx, p)
		require.NoError(t, err)
		assert.False(t, hasField(errs, "judgeRunnerProfileId"))
		assert.False(t, hasField(errs, "runnerProfileIds"))
	})
}
