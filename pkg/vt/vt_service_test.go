package vt

import (
	"context"
	"fmt"
	"testing"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDB_AuthService(t *testing.T) {
	ctx := t.Context()
	srv := NewAuthService(test.Setup(t))
	require.NotNil(t, srv)

	t.Run("Positive testing", func(t *testing.T) {
		t.Run("Login method with remember password", func(t *testing.T) {
			authKey, err := srv.Login(ctx, "admin", "12345", true)
			require.NoError(t, err)
			authKey2, err := srv.Login(ctx, "admin", "12345", true)
			require.NoError(t, err)
			assert.Equal(t, authKey2, authKey)
		})

		t.Run("Login without remember password", func(t *testing.T) {
			authKey, err := srv.Login(ctx, "admin", "12345", false)
			require.NoError(t, err)
			assert.Len(t, authKey, 32)

			u, err := srv.commonRepo.EnabledUserByAuthKey(ctx, authKey)
			require.NoError(t, err)
			userCtx := context.WithValue(ctx, userKey, u)

			t.Run("Get profile", func(t *testing.T) {
				user, err := srv.Profile(userCtx)
				require.NoError(t, err)
				assert.NotNil(t, user)
			})

			t.Run("Logout", func(t *testing.T) {
				ok, err := srv.Logout(userCtx)
				require.NoError(t, err)
				assert.True(t, ok)
			})
		})
	})

	t.Run("Negative testing", func(t *testing.T) {
		t.Run("Login not exists", func(t *testing.T) {
			_, err := srv.Login(ctx, "vova", "12345", false)
			require.Error(t, err)
		})

		t.Run("Wrong password", func(t *testing.T) {
			_, err := srv.Login(ctx, "admin", "admin", false)
			require.Error(t, err)
		})

		t.Run("Empty login/password", func(t *testing.T) {
			_, err := srv.Login(ctx, "", "", false)
			require.Error(t, err)
		})

		t.Run("Profile without user in context", func(t *testing.T) {
			u, err := srv.Profile(ctx)
			require.Error(t, err)
			assert.Nil(t, u)
		})

		t.Run("Logout without user in context", func(t *testing.T) {
			ok, err := srv.Logout(ctx)
			require.Error(t, err)
			assert.False(t, ok)
		})
	})
}

func TestDB_UserService(t *testing.T) {
	ctx := t.Context()
	srv := NewUserService(test.Setup(t))
	require.NotNil(t, srv)

	t.Run("Positive testing", func(t *testing.T) {
		t.Run("Test CRUD", func(t *testing.T) {
			login := fmt.Sprintf("ivan_%d", time.Now().Unix())
			password := fmt.Sprintf("pwd_%v", login)

			inUser := User{
				Login:    login,
				Password: password,
				StatusID: db.StatusEnabled,
			}

			// Add
			outUser, err := srv.Add(ctx, inUser)
			require.NoError(t, err)
			require.NotNil(t, outUser)

			assert.Positive(t, outUser.ID)
			assert.Equal(t, inUser.Login, outUser.Login)
			assert.Empty(t, outUser.Password)

			// GetByID
			u, err := srv.GetByID(ctx, outUser.ID)
			require.NoError(t, err)
			require.NotNil(t, u)

			assert.Equal(t, outUser.ID, u.ID)
			assert.Equal(t, outUser.Login, u.Login)
			assert.Equal(t, outUser.Password, u.Password)

			// Update
			u.Login = "test"

			ok, err := srv.Update(ctx, *u)
			require.NoError(t, err)
			assert.True(t, ok)

			updated, err := srv.GetByID(ctx, outUser.ID)
			require.NoError(t, err)
			assert.Equal(t, u.Login, updated.Login)

			// Delete
			ok, err = srv.Delete(ctx, outUser.ID)
			require.NoError(t, err)
			assert.True(t, ok)
		})
	})

	t.Run("Negative testing", func(t *testing.T) {
		t.Run("Create user with empty login", func(t *testing.T) {
			user := User{
				Login:    "",
				Password: "password",
				StatusID: db.StatusEnabled,
			}
			u, err := srv.Add(ctx, user)
			require.Error(t, err)
			assert.Nil(t, u)
		})

		t.Run("Create user with empty password", func(t *testing.T) {
			user := User{
				Login:    "vasya",
				Password: "",
				StatusID: db.StatusEnabled,
			}
			u, err := srv.Add(ctx, user)
			require.Error(t, err)
			assert.Nil(t, u)
		})

		t.Run("Create user with duplicate login", func(t *testing.T) {
			login := fmt.Sprintf("dup_%d", time.Now().UnixNano())
			user := User{
				Login:    login,
				Password: "pwd",
				StatusID: db.StatusEnabled,
			}
			u, err := srv.Add(ctx, user)
			require.NoError(t, err)
			require.NotNil(t, u)

			u2, err := srv.Add(ctx, user)
			require.Error(t, err)
			assert.Nil(t, u2)
		})
	})
}
