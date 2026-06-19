package reviewctl

import (
	"context"
	"testing"

	"reviewsrv/pkg/db"
	dbtest "reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkProfile(t *testing.T, dbc db.DB, title, runner string) *db.RunnerProfile {
	rp, _ := dbtest.RunnerProfile(t, dbc, &db.RunnerProfile{
		Title: title, Runner: runner, Token: reviewer.Ptr("tok-" + title), StatusID: db.StatusEnabled,
	})
	t.Cleanup(func() {
		_, _ = dbc.ModelContext(context.Background(), &db.RunnerProfile{ID: rp.ID}).WherePK().Delete()
	})
	return rp
}

func TestDBService_ReviewConfig(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	svc := NewService(dbc)

	t.Run("returns primary, panel and judge with real tokens", func(t *testing.T) {
		primary := mkProfile(t, dbc, "primary", "claude")
		member := mkProfile(t, dbc, "member", "codex")
		judge := mkProfile(t, dbc, "judge", "claude")

		pr, cl := dbtest.Project(t, dbc, &db.Project{
			RunnerProfileID:      reviewer.Ptr(primary.ID),
			RunnerProfileIDs:     db.ProjectRunnerProfileIDs{member.ID},
			JudgeRunnerProfileID: reviewer.Ptr(judge.ID),
			StatusID:             db.StatusEnabled,
		}, dbtest.WithProjectRelations, dbtest.WithFakeProject)
		t.Cleanup(cl)

		got, err := svc.ReviewConfig(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, got.Primary)
		assert.Equal(t, primary.ID, got.Primary.RunnerProfileID)
		assert.Equal(t, "tok-primary", got.Primary.Token)
		require.Len(t, got.Panel, 1)
		assert.Equal(t, member.ID, got.Panel[0].RunnerProfileID)
		require.NotNil(t, got.Judge)
		assert.Equal(t, judge.ID, got.Judge.RunnerProfileID)
	})

	t.Run("invalid project key", func(t *testing.T) {
		_, err := svc.ReviewConfig(t.Context(), "not-a-uuid")
		assert.ErrorIs(t, err, ErrInvalidProjectKey)
	})
}

func TestDBService_FusionPrompt(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	svc := NewService(dbc)

	t.Run("returns the built-in fusion prompt", func(t *testing.T) {
		got, err := svc.FusionPrompt(t.Context(), "00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.NotEmpty(t, got)
		assert.Equal(t, reviewer.FusionPrompt, got)
	})

	t.Run("invalid project key", func(t *testing.T) {
		_, err := svc.FusionPrompt(t.Context(), "nope")
		assert.ErrorIs(t, err, ErrInvalidProjectKey)
	})
}
