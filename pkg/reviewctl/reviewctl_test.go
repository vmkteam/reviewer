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

	t.Run("returns tracker config out-of-band", func(t *testing.T) {
		primary := mkProfile(t, dbc, "tracker-primary", "direct")

		tt, clTT := dbtest.TaskTracker(t, dbc, &db.TaskTracker{
			Title:       "YT",
			URL:         "https://youtrack.example.com",
			AuthToken:   reviewer.Ptr("perm:secret"),
			FetchPrompt: "GET {{URL}}/api/issues/<ID>",
			StatusID:    db.StatusEnabled,
		})
		t.Cleanup(clTT)

		pr, cl := dbtest.Project(t, dbc, &db.Project{
			RunnerProfileID: reviewer.Ptr(primary.ID),
			TaskTrackerID:   reviewer.Ptr(tt.ID),
			StatusID:        db.StatusEnabled,
		}, dbtest.WithProjectRelations, dbtest.WithFakeProject)
		t.Cleanup(cl)

		got, err := svc.ReviewConfig(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, got.Tracker)
		assert.Equal(t, "https://youtrack.example.com", got.Tracker.URL)
		assert.Equal(t, "perm:secret", got.Tracker.Token)
	})

	t.Run("disabled tracker is not returned", func(t *testing.T) {
		primary := mkProfile(t, dbc, "tracker-off", "claude")

		tt, clTT := dbtest.TaskTracker(t, dbc, &db.TaskTracker{
			Title:       "Off",
			URL:         "https://off.example.com",
			FetchPrompt: "GET {{URL}}",
			StatusID:    db.StatusDisabled,
		})
		t.Cleanup(clTT)

		pr, cl := dbtest.Project(t, dbc, &db.Project{
			RunnerProfileID: reviewer.Ptr(primary.ID),
			TaskTrackerID:   reviewer.Ptr(tt.ID),
			StatusID:        db.StatusEnabled,
		}, dbtest.WithProjectRelations, dbtest.WithFakeProject)
		t.Cleanup(cl)

		got, err := svc.ReviewConfig(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		assert.Nil(t, got.Tracker)
	})

	t.Run("invalid project key", func(t *testing.T) {
		_, err := svc.ReviewConfig(t.Context(), "not-a-uuid")
		assert.ErrorIs(t, err, ErrInvalidProjectKey)
	})
}

func TestDBService_Prompt(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	svc := NewService(dbc)

	prompt, clP := dbtest.Prompt(t, dbc, &db.Prompt{Title: "P", Common: "Common", Code: "Code", StatusID: db.StatusEnabled})
	t.Cleanup(clP)

	tt, clTT := dbtest.TaskTracker(t, dbc, &db.TaskTracker{
		Title:       "YT",
		URL:         "https://yt.example.com",
		AuthToken:   reviewer.Ptr("perm:legacy-secret"),
		FetchPrompt: "curl -H 'Authorization: Bearer {{TOKEN}}' {{URL}}/api",
		StatusID:    db.StatusEnabled,
	})
	t.Cleanup(clTT)

	pr, cl := dbtest.Project(t, dbc, &db.Project{
		PromptID:      prompt.ID,
		TaskTrackerID: reviewer.Ptr(tt.ID),
		StatusID:      db.StatusEnabled,
	}, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(cl)

	t.Run("legacy client without tokenEnv gets the real token", func(t *testing.T) {
		got, err := svc.Prompt(t.Context(), pr.ProjectKey, nil)
		require.NoError(t, err)
		assert.Contains(t, got, "perm:legacy-secret")
		assert.NotContains(t, got, "$REVIEW_TRACKER_TOKEN")
	})

	t.Run("tokenEnv client gets the env reference, not the secret", func(t *testing.T) {
		got, err := svc.Prompt(t.Context(), pr.ProjectKey, reviewer.Ptr(true))
		require.NoError(t, err)
		assert.NotContains(t, got, "perm:legacy-secret")
		assert.Contains(t, got, "$REVIEW_TRACKER_TOKEN")
	})

	t.Run("invalid project key", func(t *testing.T) {
		_, err := svc.Prompt(t.Context(), "nope", nil)
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
