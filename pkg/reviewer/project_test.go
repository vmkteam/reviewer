package reviewer

import (
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

func TestProjectManager_GetByID(t *testing.T) {
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

func TestProjectManager_GetByKey(t *testing.T) {
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

func TestProjectManager_List(t *testing.T) {
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

func TestProjectManager_Prompt(t *testing.T) {
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

		result, err := pm.Prompt(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "Common instructions")
		assert.Contains(t, result, "Architecture review")
		assert.Contains(t, result, "Code review")
		assert.Contains(t, result, "Security review")
		assert.Contains(t, result, "Tests review")
	})

	t.Run("with task tracker token substitution", func(t *testing.T) {
		prompt, clPrompt := test.Prompt(t, dbc, &db.Prompt{
			Title:    "Prompt with TT",
			Common:   "Common",
			Code:     "Code review",
			StatusID: db.StatusEnabled,
		})
		t.Cleanup(clPrompt)

		tt, clTT := test.TaskTracker(t, dbc, &db.TaskTracker{
			Title:       "TestTracker",
			AuthToken:   "secret-token-123",
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

		result, err := pm.Prompt(t.Context(), pr.ProjectKey)
		require.NoError(t, err)
		assert.Contains(t, result, "secret-token-123")
		assert.NotContains(t, result, "{{TOKEN}}")
	})

	t.Run("project not found returns empty", func(t *testing.T) {
		result, err := pm.Prompt(t.Context(), "00000000-0000-0000-0000-000000000000")
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}
