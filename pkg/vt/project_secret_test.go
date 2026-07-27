package vt

import (
	"testing"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_SlackChannelWebhookSetOrKeep(t *testing.T) {
	ctx := t.Context()
	srv := NewSlackChannelService(test.Setup(t))
	sp := func(s string) *string { return &s }

	out, err := srv.Add(ctx, SlackChannel{Title: "sc-secret", Channel: "#rev", WebhookURL: sp("https://hooks.slack.com/T1"), StatusID: db.StatusEnabled})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = srv.projectRepo.DeleteSlackChannel(ctx, out.ID) })

	// Add never returns the raw URL to the admin API.
	assert.Nil(t, out.WebhookURL)
	assert.True(t, out.HasWebhookURL)

	stored := func() string {
		t.Helper()
		dbc, berr := srv.byID(ctx, out.ID)
		require.NoError(t, berr)
		return dbc.WebhookURL
	}
	update := func(u *string) {
		t.Helper()
		_, uerr := srv.Update(ctx, SlackChannel{ID: out.ID, Title: "sc-secret", Channel: "#rev", WebhookURL: u, StatusID: db.StatusEnabled})
		require.NoError(t, uerr)
	}

	require.Equal(t, "https://hooks.slack.com/T1", stored())
	update(nil)
	assert.Equal(t, "https://hooks.slack.com/T1", stored(), "nil URL on update keeps the stored one")
	update(sp(""))
	assert.Equal(t, "https://hooks.slack.com/T1", stored(), "blank keeps too — a stale client echoing the empty write-only field must not erase it")
	update(sp("https://hooks.slack.com/T2"))
	assert.Equal(t, "https://hooks.slack.com/T2", stored(), "a new value replaces")
}

func TestDB_TaskTrackerTokenSetOrKeepAndURLTrim(t *testing.T) {
	ctx := t.Context()
	srv := NewTaskTrackerService(test.Setup(t))
	sp := func(s string) *string { return &s }

	out, err := srv.Add(ctx, TaskTracker{Title: "tt-secret", URL: "https://bugs.example.com/", FetchPrompt: "p", AuthToken: sp("tok-1"), StatusID: db.StatusEnabled})
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = srv.projectRepo.DeleteTaskTracker(ctx, out.ID) })

	assert.Nil(t, out.AuthToken)
	assert.True(t, out.HasToken)

	dbt, err := srv.byID(ctx, out.ID)
	require.NoError(t, err)
	// Trailing slash is normalized on write: {{URL}}+"/api/..." with a doubled
	// slash makes SPA trackers (YouTrack) answer with HTML instead of JSON.
	assert.Equal(t, "https://bugs.example.com", dbt.URL)

	stored := func() string {
		t.Helper()
		dbt, berr := srv.byID(ctx, out.ID)
		require.NoError(t, berr)
		require.NotNil(t, dbt.AuthToken)
		return *dbt.AuthToken
	}
	update := func(tok *string) {
		t.Helper()
		_, uerr := srv.Update(ctx, TaskTracker{ID: out.ID, Title: "tt-secret", URL: "https://bugs.example.com", FetchPrompt: "p", AuthToken: tok, StatusID: db.StatusEnabled})
		require.NoError(t, uerr)
	}

	require.Equal(t, "tok-1", stored())
	update(nil)
	assert.Equal(t, "tok-1", stored(), "nil token on update keeps the stored one")
	update(sp(""))
	assert.Equal(t, "tok-1", stored(), "blank token keeps too")
	update(sp("tok-2"))
	assert.Equal(t, "tok-2", stored())
}
