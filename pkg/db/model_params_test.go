package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReviewModelInfo_Add(t *testing.T) {
	t.Run("sums all numeric fields", func(t *testing.T) {
		base := ReviewModelInfo{
			Model:                    "claude-opus",
			Runner:                   "claude",
			InputTokens:              100,
			OutputTokens:             200,
			CostUsd:                  0.50,
			CacheCreationInputTokens: 10,
			CacheReadInputTokens:     20,
			NumTurns:                 5,
			SessionID:                "ses_1",
			DurationAPIMs:            1000,
			DurationTotalMs:          1200,
			CacheCreate1hInputTokens: 1,
			CacheCreate5mInputTokens: 2,
			WebSearchRequests:        3,
			WebFetchRequests:         4,
			StopReason:               "end_turn",
		}
		other := ReviewModelInfo{
			InputTokens:              50,
			OutputTokens:             75,
			CostUsd:                  0.25,
			CacheCreationInputTokens: 5,
			CacheReadInputTokens:     10,
			NumTurns:                 2,
			DurationAPIMs:            500,
			DurationTotalMs:          600,
			CacheCreate1hInputTokens: 1,
			CacheCreate5mInputTokens: 1,
			WebSearchRequests:        1,
			WebFetchRequests:         2,
		}

		base.Add(other)

		assert.Equal(t, 150, base.InputTokens)
		assert.Equal(t, 275, base.OutputTokens)
		assert.InDelta(t, 0.75, base.CostUsd, 0.0001)
		assert.Equal(t, 15, base.CacheCreationInputTokens)
		assert.Equal(t, 30, base.CacheReadInputTokens)
		assert.Equal(t, 7, base.NumTurns)
		assert.Equal(t, 1500, base.DurationAPIMs)
		assert.Equal(t, 1800, base.DurationTotalMs)
		assert.Equal(t, 2, base.CacheCreate1hInputTokens)
		assert.Equal(t, 3, base.CacheCreate5mInputTokens)
		assert.Equal(t, 4, base.WebSearchRequests)
		assert.Equal(t, 6, base.WebFetchRequests)
	})

	t.Run("identity-shaped fields are not touched", func(t *testing.T) {
		base := ReviewModelInfo{
			Model:          "claude-opus",
			Runner:         "claude",
			SessionID:      "ses_1",
			StopReason:     "end_turn",
			TerminalReason: "ok",
			IsError:        false,
		}
		other := ReviewModelInfo{
			Model:          "claude-haiku",
			Runner:         "opencode",
			SessionID:      "ses_2",
			StopReason:     "max_tokens",
			TerminalReason: "limit",
			IsError:        true,
		}

		base.Add(other)

		assert.Equal(t, "claude-opus", base.Model, "Model must come from primary run")
		assert.Equal(t, "claude", base.Runner)
		assert.Equal(t, "ses_1", base.SessionID)
		assert.Equal(t, "end_turn", base.StopReason)
		assert.Equal(t, "ok", base.TerminalReason)
		assert.False(t, base.IsError)
	})

	t.Run("merges Models map: same key sums, new key inserts", func(t *testing.T) {
		base := ReviewModelInfo{
			Models: map[string]ModelUseStats{
				"opus":  {InputTokens: 100, OutputTokens: 200, CostUsd: 0.5},
				"haiku": {InputTokens: 10, OutputTokens: 20, CostUsd: 0.01},
			},
		}
		other := ReviewModelInfo{
			Models: map[string]ModelUseStats{
				"opus":   {InputTokens: 50, OutputTokens: 100, CostUsd: 0.25},
				"sonnet": {InputTokens: 30, OutputTokens: 40, CostUsd: 0.05},
			},
		}

		base.Add(other)

		require := assert.New(t)
		require.Len(base.Models, 3)
		require.Equal(150, base.Models["opus"].InputTokens)
		require.Equal(300, base.Models["opus"].OutputTokens)
		require.InDelta(0.75, base.Models["opus"].CostUsd, 0.0001)
		require.Equal(10, base.Models["haiku"].InputTokens)  // untouched
		require.Equal(30, base.Models["sonnet"].InputTokens) // inserted
		require.InDelta(0.05, base.Models["sonnet"].CostUsd, 0.0001)
	})

	t.Run("nil Models map is created when other has entries", func(t *testing.T) {
		base := ReviewModelInfo{}
		other := ReviewModelInfo{
			Models: map[string]ModelUseStats{
				"opus": {InputTokens: 10, OutputTokens: 20, CostUsd: 0.1},
			},
		}

		base.Add(other)

		assert.NotNil(t, base.Models)
		assert.Equal(t, 10, base.Models["opus"].InputTokens)
	})

	t.Run("empty other Models map leaves base.Models alone", func(t *testing.T) {
		base := ReviewModelInfo{
			Models: map[string]ModelUseStats{
				"opus": {InputTokens: 10},
			},
		}
		other := ReviewModelInfo{}

		base.Add(other)

		assert.Len(t, base.Models, 1)
		assert.Equal(t, 10, base.Models["opus"].InputTokens)
	})

	t.Run("zero other is a no-op on numeric fields", func(t *testing.T) {
		base := ReviewModelInfo{
			InputTokens:  100,
			OutputTokens: 200,
			CostUsd:      0.5,
		}
		base.Add(ReviewModelInfo{})

		assert.Equal(t, 100, base.InputTokens)
		assert.Equal(t, 200, base.OutputTokens)
		assert.InDelta(t, 0.5, base.CostUsd, 0.0001)
	})
}
