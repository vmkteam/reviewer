package reviewer

import (
	"context"
	"testing"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestProjectManager(t *testing.T) (*ProjectManager, db.DB) {
	dbc, _ := test.Setup(t)
	return NewProjectManager(dbc), dbc
}

func TestDBProjectManager_GetByID(t *testing.T) {
	pm, dbc := newTestProjectManager(t)

	pr, cl := test.Project(t, dbc, nil, test.WithProjectRelations, test.WithFakeProject)
	t.Cleanup(cl)

	t.Run("found", func(t *testing.T) {
		got, err := pm.GetByID(t.Context(), pr.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, pr.ID, got.ID)
		assert.Equal(t, pr.Title, got.Title)
	})

	t.Run("not found", func(t *testing.T) {
		got, err := pm.GetByID(t.Context(), -1)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestDBProjectManager_GetByKey(t *testing.T) {
	pm, dbc := newTestProjectManager(t)

	pr, cl := test.Project(t, dbc, nil, test.WithProjectRelations, test.WithFakeProject)
	t.Cleanup(cl)

	t.Run("found with relations", func(t *testing.T) {
		got, err := pm.GetByKey(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, pr.ID, got.ID)
		assert.Equal(t, pr.ProjectKey, got.ProjectKey)
		assert.NotNil(t, got.Prompt)
	})

	t.Run("not found", func(t *testing.T) {
		got, err := pm.GetByKey(t.Context(), "00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestDBProjectManager_List(t *testing.T) {
	pm, dbc := newTestProjectManager(t)

	pr, cl := test.Project(t, dbc, nil, test.WithProjectRelations, test.WithFakeProject)
	t.Cleanup(cl)

	projects, err := pm.List(t.Context())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(projects), 1)

	found := false
	for _, p := range projects {
		if p.ID == pr.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "created project should appear in list")
}

func TestDBProjectManager_ReviewProfiles(t *testing.T) {
	pm, dbc := newTestProjectManager(t)

	// The generated test.RunnerProfile cleaner deletes via t.Context(), which is
	// canceled before cleanups run; delete with a background context instead. The
	// profile FK is cleared first because the project (deleted by its own cleaner)
	// goes away earlier — subtest cleanups run before this outer-test cleanup.
	mk := func(title, rn string, status int) *db.RunnerProfile {
		rp, _ := test.RunnerProfile(t, dbc, &db.RunnerProfile{
			Title: title, Runner: rn, Token: Ptr("tok-" + title), StatusID: status,
		})
		t.Cleanup(func() {
			_, _ = dbc.ModelContext(context.Background(), &db.RunnerProfile{ID: rp.ID}).WherePK().Delete()
		})
		return rp
	}

	t.Run("panel with judge resolves primary, ordered members, judge", func(t *testing.T) {
		primary := mk("primary", "claude", db.StatusEnabled)
		m1 := mk("m1", "codex", db.StatusEnabled)
		m2 := mk("m2", "opencode", db.StatusEnabled)
		judge := mk("judge", "claude", db.StatusEnabled)

		pr, cl := test.Project(t, dbc, &db.Project{
			RunnerProfileID:      Ptr(primary.ID),
			RunnerProfileIDs:     db.ProjectRunnerProfileIDs{m1.ID, m2.ID},
			JudgeRunnerProfileID: Ptr(judge.ID),
			StatusID:             db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(cl)

		setup, err := pm.ReviewProfiles(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, setup)
		require.NotNil(t, setup.Primary)
		assert.Equal(t, primary.ID, setup.Primary.ID)
		assert.Equal(t, "tok-primary", *setup.Primary.Token, "real token is returned")
		require.Len(t, setup.Panel, 2)
		assert.Equal(t, m1.ID, setup.Panel[0].ID, "members keep runnerProfileIds order")
		assert.Equal(t, m2.ID, setup.Panel[1].ID)
		require.NotNil(t, setup.Judge)
		assert.Equal(t, judge.ID, setup.Judge.ID)
	})

	t.Run("no judge → judge nil, no panel (single review)", func(t *testing.T) {
		primary := mk("primarySingle", "claude", db.StatusEnabled)
		pr, cl := test.Project(t, dbc, &db.Project{
			RunnerProfileID: Ptr(primary.ID),
			StatusID:        db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(cl)

		setup, err := pm.ReviewProfiles(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, setup)
		require.NotNil(t, setup.Primary)
		assert.Empty(t, setup.Panel)
		assert.Nil(t, setup.Judge)
	})

	t.Run("disabled panel member is skipped", func(t *testing.T) {
		primary := mk("primaryDis", "claude", db.StatusEnabled)
		live := mk("live", "codex", db.StatusEnabled)
		disabled := mk("disabled", "claude", db.StatusDisabled)

		pr, cl := test.Project(t, dbc, &db.Project{
			RunnerProfileID:  Ptr(primary.ID),
			RunnerProfileIDs: db.ProjectRunnerProfileIDs{disabled.ID, live.ID},
			StatusID:         db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(cl)

		setup, err := pm.ReviewProfiles(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		require.NotNil(t, setup)
		require.Len(t, setup.Panel, 1, "the disabled member is dropped, not fatal")
		assert.Equal(t, live.ID, setup.Panel[0].ID)
	})

	t.Run("unknown project key → nil setup", func(t *testing.T) {
		setup, err := pm.ReviewProfiles(t.Context(), "00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.Nil(t, setup)
	})
}

func TestDBProjectManager_Prompt(t *testing.T) {
	pm, dbc := newTestProjectManager(t)

	t.Run("with prompt template", func(t *testing.T) {
		prompt, clPrompt := test.Prompt(t, dbc, &db.Prompt{
			Title:        "Test Prompt",
			Common:       "Common instructions",
			Architecture: "Architecture review",
			Code:         "Code review",
			Security:     "Security review",
			Tests:        "Tests review",
			StatusID:     db.StatusEnabled,
		})
		t.Cleanup(clPrompt)

		pr, clPr := test.Project(t, dbc, &db.Project{
			PromptID: prompt.ID,
			StatusID: db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(clPr)

		result, err := pm.Prompt(t.Context(), pr.ProjectKey, true)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Common instructions")
		assert.Contains(t, result, "Architecture review")
		assert.Contains(t, result, "Code review")
		assert.Contains(t, result, "Security review")
		assert.Contains(t, result, "Tests review")
	})

	t.Run("with task tracker token kept out of the prompt", func(t *testing.T) {
		prompt, clPrompt := test.Prompt(t, dbc, &db.Prompt{
			Title:    "Prompt with TT",
			Common:   "Common",
			Code:     "Code review",
			StatusID: db.StatusEnabled,
		})
		t.Cleanup(clPrompt)

		tt, clTT := test.TaskTracker(t, dbc, &db.TaskTracker{
			Title:       "TestTracker",
			AuthToken:   Ptr("secret-token-123"),
			FetchPrompt: "curl -H 'Bearer {{TOKEN}}' https://api/issues",
			StatusID:    db.StatusEnabled,
		})
		t.Cleanup(clTT)

		pr, clPr := test.Project(t, dbc, &db.Project{
			PromptID:      prompt.ID,
			TaskTrackerID: Ptr(tt.ID),
			StatusID:      db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(clPr)

		result, err := pm.Prompt(t.Context(), pr.ProjectKey, true)
		require.NoError(t, err)
		// The real secret never enters the prompt: {{TOKEN}} renders as the env
		// reference reviewctl exports to runner processes.
		assert.NotContains(t, result, "secret-token-123")
		assert.Contains(t, result, "$REVIEW_TRACKER_TOKEN")
		assert.NotContains(t, result, "{{TOKEN}}")

		// Legacy clients (tokenEnv=false) keep the historical real-token
		// substitution so pre-update CI images stay functional.
		legacy, err := pm.Prompt(t.Context(), pr.ProjectKey, false)
		require.NoError(t, err)
		assert.Contains(t, legacy, "secret-token-123")
		assert.NotContains(t, legacy, "{{TOKEN}}")
		assert.NotContains(t, legacy, "$REVIEW_TRACKER_TOKEN")
	})

	t.Run("with task tracker URL substitution", func(t *testing.T) {
		prompt, clPrompt := test.Prompt(t, dbc, &db.Prompt{
			Title:    "Prompt with URL",
			Common:   "Common",
			Code:     "Code review",
			StatusID: db.StatusEnabled,
		})
		t.Cleanup(clPrompt)

		tt, clTT := test.TaskTracker(t, dbc, &db.TaskTracker{
			Title:       "URLTracker",
			URL:         "https://youtrack.example.com",
			AuthToken:   Ptr("token-456"),
			FetchPrompt: "curl -X GET \"{{URL}}/api/issues/PLF-731\" -H 'Authorization: Bearer {{TOKEN}}'",
			StatusID:    db.StatusEnabled,
		})
		t.Cleanup(clTT)

		pr, clPr := test.Project(t, dbc, &db.Project{
			PromptID:      prompt.ID,
			TaskTrackerID: Ptr(tt.ID),
			StatusID:      db.StatusEnabled,
		}, test.WithProjectRelations, test.WithFakeProject)
		t.Cleanup(clPr)

		result, err := pm.Prompt(t.Context(), pr.ProjectKey, true)
		require.NoError(t, err)
		assert.Contains(t, result, "https://youtrack.example.com")
		assert.NotContains(t, result, "{{URL}}")
		assert.NotContains(t, result, "token-456")
		assert.Contains(t, result, "$REVIEW_TRACKER_TOKEN")
		assert.NotContains(t, result, "{{TOKEN}}")
	})

	t.Run("project not found returns empty", func(t *testing.T) {
		result, err := pm.Prompt(t.Context(), "00000000-0000-0000-0000-000000000000", true)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
