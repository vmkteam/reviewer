package ctl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthorName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"name and email — strips email", "Иван Иванов <ivan@example.com>", "Иван Иванов"},
		{"email only — falls back to input", "<bot@ci>", "<bot@ci>"},
		{"plain login — passes through", "plain-login", "plain-login"},
		{"empty — empty", "", ""},
		{"trailing junk after email — falls back", "Bot <bot@ci> trailing", "Bot <bot@ci> trailing"},
		{"plain email no brackets — bare", "user@example.com", "user@example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AuthorName(tc.in))
		})
	}
}

func TestEnvBool(t *testing.T) {
	t.Run("unset returns fallback", func(t *testing.T) {
		t.Setenv("REVIEW_TEST_BOOL_UNSET", "")
		assert.True(t, EnvBool("REVIEW_TEST_BOOL_UNSET", true))
		assert.False(t, EnvBool("REVIEW_TEST_BOOL_UNSET", false))
	})

	cases := []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"1", true},
		{"t", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"0", false},
		{"f", false},
	}
	for _, tc := range cases {
		t.Run("parses "+tc.val, func(t *testing.T) {
			t.Setenv("REVIEW_TEST_BOOL", tc.val)
			// fallback flipped to make sure value, not fallback, decides.
			assert.Equal(t, tc.want, EnvBool("REVIEW_TEST_BOOL", !tc.want))
		})
	}

	t.Run("unparseable falls back without panicking", func(t *testing.T) {
		t.Setenv("REVIEW_TEST_BOOL_BAD", "maybe")
		assert.True(t, EnvBool("REVIEW_TEST_BOOL_BAD", true))
		assert.False(t, EnvBool("REVIEW_TEST_BOOL_BAD", false))
	})
}

func TestEnvDefault(t *testing.T) {
	t.Run("unset returns fallback", func(t *testing.T) {
		t.Setenv("REVIEW_TEST_STR_UNSET", "")
		assert.Equal(t, "fb", EnvDefault("REVIEW_TEST_STR_UNSET", "fb"))
	})

	t.Run("set value wins over fallback", func(t *testing.T) {
		t.Setenv("REVIEW_TEST_STR", "actual")
		assert.Equal(t, "actual", EnvDefault("REVIEW_TEST_STR", "fb"))
	})
}
