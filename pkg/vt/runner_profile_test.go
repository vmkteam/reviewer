package vt

import (
	"testing"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_RunnerProfileService(t *testing.T) {
	ctx := t.Context()
	srv := NewRunnerProfileService(test.Setup(t))
	require.NotNil(t, srv)

	sp := func(s string) *string { return &s }

	// Snapshot the pre-existing default so this test can restore it — the DB is
	// shared and "single default" is global state enforced only in code.
	wantDefault := true
	origDefault, _ := srv.projectRepo.OneRunnerProfile(ctx, &db.RunnerProfileSearch{IsDefault: &wantDefault})

	var created []int
	add := func(rp RunnerProfile) *RunnerProfile {
		rp.StatusID = db.StatusEnabled
		out, err := srv.Add(ctx, rp)
		require.NoError(t, err)
		created = append(created, out.ID)
		return out
	}

	t.Cleanup(func() {
		// Clear isDefault on everything this test created and soft-delete it
		// (Delete only flips statusId), then restore the original default so the
		// shared DB is left with exactly one enabled default for other tests.
		for _, id := range created {
			_, _ = srv.projectRepo.UpdateRunnerProfile(ctx, &db.RunnerProfile{ID: id, IsDefault: false}, db.WithColumns(db.Columns.RunnerProfile.IsDefault))
			_, _ = srv.projectRepo.DeleteRunnerProfile(ctx, id)
		}
		if origDefault != nil {
			_, _ = srv.projectRepo.UpdateRunnerProfile(ctx, &db.RunnerProfile{ID: origDefault.ID, IsDefault: true}, db.WithColumns(db.Columns.RunnerProfile.IsDefault))
		}
	})

	t.Run("single default is enforced on Add", func(t *testing.T) {
		a := add(RunnerProfile{Title: "rp-default-a", Runner: runner.RunnerClaude, IsDefault: true})
		b := add(RunnerProfile{Title: "rp-default-b", Runner: runner.RunnerClaude, IsDefault: true})

		dbA, err := srv.byID(ctx, a.ID)
		require.NoError(t, err)
		assert.False(t, dbA.IsDefault, "marking a new profile default must unset the previous default")

		dbB, err := srv.byID(ctx, b.ID)
		require.NoError(t, err)
		assert.True(t, dbB.IsDefault)
	})

	t.Run("a new default can be set after the old default was deleted", func(t *testing.T) {
		a := add(RunnerProfile{Title: "rp-del-default", Runner: runner.RunnerClaude, IsDefault: true})

		// Soft-delete only flips statusId, so the deleted row keeps isDefault.
		// With single-default enforced in code (no DB unique index), this must
		// not block marking another profile default.
		_, err := srv.Delete(ctx, a.ID)
		require.NoError(t, err)

		b := add(RunnerProfile{Title: "rp-new-default", Runner: runner.RunnerClaude, IsDefault: true})
		dbB, err := srv.byID(ctx, b.ID)
		require.NoError(t, err)
		assert.True(t, dbB.IsDefault, "must be able to set a new default after deleting the old one")
	})

	t.Run("token is write-only with set-or-keep on Update", func(t *testing.T) {
		c := add(RunnerProfile{Title: "rp-token", Runner: runner.RunnerClaude, Token: sp("secret-1")})

		// Add does not expose the raw token back to the admin API.
		assert.Nil(t, c.Token)
		assert.True(t, c.HasToken)

		dbC, err := srv.byID(ctx, c.ID)
		require.NoError(t, err)
		require.NotNil(t, dbC.Token)
		assert.Equal(t, "secret-1", *dbC.Token)

		// nil token on update keeps the stored secret.
		_, err = srv.Update(ctx, RunnerProfile{ID: c.ID, Title: "rp-token", Runner: runner.RunnerClaude, Token: nil, StatusID: db.StatusEnabled})
		require.NoError(t, err)
		dbC, err = srv.byID(ctx, c.ID)
		require.NoError(t, err)
		require.NotNil(t, dbC.Token)
		assert.Equal(t, "secret-1", *dbC.Token, "nil token on update must keep the stored token")

		// A new value replaces it.
		_, err = srv.Update(ctx, RunnerProfile{ID: c.ID, Title: "rp-token", Runner: runner.RunnerClaude, Token: sp("secret-2"), StatusID: db.StatusEnabled})
		require.NoError(t, err)
		dbC, err = srv.byID(ctx, c.ID)
		require.NoError(t, err)
		require.NotNil(t, dbC.Token)
		assert.Equal(t, "secret-2", *dbC.Token)
	})

	t.Run("apiProvider validation matches direct.NewProvider", func(t *testing.T) {
		_, err := srv.Add(ctx, RunnerProfile{Title: "rp-bad", Runner: runner.RunnerDirect, APIProvider: sp("nope"), StatusID: db.StatusEnabled})
		require.Error(t, err, "an unknown provider must be rejected")

		// openai is accepted by direct.NewProvider, so the admin must accept it too.
		ok := add(RunnerProfile{Title: "rp-openai", Runner: runner.RunnerDirect, APIProvider: sp("openai")})
		assert.NotZero(t, ok.ID)
	})
}
