package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredEnv(t *testing.T) {
	const v = "REVIEWSRV_TEST_CRED_VAR"

	t.Run("empty token injects nothing", func(t *testing.T) {
		t.Setenv(v, "") // ambient unset
		assert.Nil(t, credEnv(v, ""))
	})

	t.Run("token injected when ambient var is unset", func(t *testing.T) {
		t.Setenv(v, "")
		assert.Equal(t, []string{v + "=tok"}, credEnv(v, "tok"))
	})

	t.Run("ambient env wins over the profile token", func(t *testing.T) {
		t.Setenv(v, "ambient")
		assert.Nil(t, credEnv(v, "tok"), "an already-set env var is not overridden")
	})
}
