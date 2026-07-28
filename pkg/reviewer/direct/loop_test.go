package direct

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// scriptedProvider returns a fixed sequence of responses and records the
// requests it saw, so loop behaviour can be asserted without a network.
type scriptedProvider struct {
	responses []Response
	i         int
	seen      []Request
}

func (s *scriptedProvider) Complete(_ context.Context, req Request) (Response, error) {
	s.seen = append(s.seen, req)
	if s.i >= len(s.responses) {
		return Response{}, context.Canceled
	}
	r := s.responses[s.i]
	s.i++
	return r, nil
}

func (s *scriptedProvider) Model() string    { return "fake-model" }
func (s *scriptedProvider) Pricing() Pricing { return Pricing{InputPerMTok: 1, OutputPerMTok: 1} }

func validSubmitArgs(t *testing.T, severity string) json.RawMessage {
	t.Helper()
	files := make([]map[string]any, len(reviewTypes))
	for i, rt := range reviewTypes {
		files[i] = map[string]any{"reviewType": rt, "summary": "summary " + rt, "isAccepted": true}
	}
	md := map[string]any{}
	for _, rt := range reviewTypes {
		md[rt] = "# " + rt
	}
	payload := map[string]any{
		"review": map[string]any{"effortMinutes": 30, "aiSlopScore": 0.1, "description": "looks ok"},
		"files":  files,
		"issues": []map[string]any{{
			"localId": "1", "severity": severity, "title": "bug", "description": "desc",
			"content": "code", "file": "main.go", "lines": "1-2", "issueType": "logic",
			"fileType": "code", "suggestedFix": "fix it",
		}},
		"markdown": md,
	}
	b, err := json.Marshal(payload)
	require.NoError(t, err)
	return b
}

func TestRunSubmitsReview(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))

	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "read_file", Args: json.RawMessage(`{"path":"main.go"}`)}}, Usage: Usage{InputTokens: 100, OutputTokens: 10}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "submit_review", Args: validSubmitArgs(t, "high")}}, Usage: Usage{InputTokens: 50, OutputTokens: 20, CacheReadTokens: 40}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.NoError(t, err)
	require.True(t, res.Submitted)
	require.Equal(t, "submitted", res.StopReason)
	require.Equal(t, 2, res.Rounds)
	require.Equal(t, 150, res.Usage.InputTokens)
	require.Equal(t, 30, res.Usage.OutputTokens)
	require.Equal(t, 40, res.Usage.CacheReadTokens)
	require.Equal(t, "fake-model", res.Model)
	require.InEpsilon(t, computeCost(Usage{InputTokens: 150, OutputTokens: 30}, prov.Pricing()), res.CostUsd, 1e-9)

	// review.json written and valid.
	data, err := os.ReadFile(filepath.Join(dir, "review.json"))
	require.NoError(t, err)
	require.Contains(t, string(data), "\"reviewType\": \"architecture\"")

	// All five R*.md bodies written.
	for _, prefix := range []string{"R1", "R2", "R3", "R4", "R5"} {
		_, err := os.Stat(filepath.Join(dir, prefix+mdSuffix))
		require.NoError(t, err, "missing %s", prefix)
	}
}

func TestRunNudgesOnBareTurn(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		{Text: "I'm done reviewing.", Usage: Usage{InputTokens: 10, OutputTokens: 5}},
		{ToolCalls: []ToolCall{{ID: "1", Name: "submit_review", Args: validSubmitArgs(t, "low")}}, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.NoError(t, err)
	require.True(t, res.Submitted)
	require.Len(t, prov.seen, 2)
	// The second request must carry the nudge as a trailing user message.
	last := prov.seen[1].Messages[len(prov.seen[1].Messages)-1]
	require.Equal(t, RoleUser, last.Role)
	require.Equal(t, nudgeSubmit, last.Text)
}

func TestRunInvalidSubmitRetries(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		// First submit has an invalid severity → Validate fails → not submitted.
		{ToolCalls: []ToolCall{{ID: "1", Name: "submit_review", Args: validSubmitArgs(t, "bogus")}}},
		// Second submit is valid.
		{ToolCalls: []ToolCall{{ID: "2", Name: "submit_review", Args: validSubmitArgs(t, "high")}}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.NoError(t, err)
	require.True(t, res.Submitted)
	require.Equal(t, 2, res.Rounds)
}

func TestRunEmitsTranscript(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644))
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		{Text: "let me look", ToolCalls: []ToolCall{{ID: "1", Name: "read_file", Args: json.RawMessage(`{"path":"main.go"}`)}}, Usage: Usage{InputTokens: 10, OutputTokens: 2}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "submit_review", Args: validSubmitArgs(t, "high")}}, Usage: Usage{InputTokens: 5, OutputTokens: 3}},
	}}

	var events []Event
	opts := Options{MaxRounds: 10, OnEvent: func(ev Event) { events = append(events, ev) }}
	res, err := Run(context.Background(), prov, reg, "sys", "review", opts)
	require.NoError(t, err)
	require.True(t, res.Submitted)

	kinds := map[string]int{}
	for _, e := range events {
		kinds[e.Kind]++
	}
	require.Equal(t, 1, kinds["system"], "kickoff system contract")
	require.Equal(t, 1, kinds["user"], "kickoff user task")
	require.Equal(t, 1, kinds["assistant"], "round-0 model text")
	require.Equal(t, 2, kinds["tool_call"], "read_file + submit_review")
	require.Equal(t, 2, kinds["tool_result"])
	require.Equal(t, 2, kinds["round"])
	require.Equal(t, 1, kinds["result"])

	// The kickoff input is recorded before any model output.
	require.Equal(t, "system", events[0].Kind)
	require.Equal(t, "user", events[1].Kind)

	last := events[len(events)-1]
	require.Equal(t, "result", last.Kind)
	require.True(t, last.Submitted)
	require.Equal(t, "fake-model", last.Model)
	require.Equal(t, 15, last.Usage.InputTokens) // 10 + 5 accumulated
}

func TestCompactMessages(t *testing.T) {
	// mk builds a realistic history: user task, then alternating assistant/tool
	// turns (odd index = assistant, even = tool with one result).
	mk := func(n int) []Message {
		m := make([]Message, n)
		m[0] = Message{Role: RoleUser, Text: "task"}
		for i := 1; i < n; i++ {
			if i%2 == 1 {
				m[i] = Message{Role: RoleAssistant, Text: fmt.Sprintf("a%d", i)}
			} else {
				m[i] = Message{Role: RoleTool, ToolResults: []ToolResult{{CallID: fmt.Sprintf("c%d", i), Content: fmt.Sprintf("r%d", i)}}}
			}
		}
		return m
	}

	// Short conversation: returned unchanged.
	require.Len(t, compactMessages(mk(3), 12), 3)

	// Long: the head must stay byte-identical (mutating it would invalidate the
	// whole provider prompt cache) — the marker lands in the first tool result of
	// the tail instead, and the tail resumes on an assistant turn.
	src := mk(40)
	out := compactMessages(src, 5)
	require.Equal(t, "task", out[0].Text, "head must stay byte-identical for cache reuse")
	require.Equal(t, RoleUser, out[0].Role)
	require.Equal(t, RoleAssistant, out[1].Role, "tail must resume on an assistant turn")
	require.Len(t, out, 1+5)
	require.Equal(t, RoleTool, out[2].Role)
	require.Contains(t, out[2].ToolResults[0].Content, "compacted")
	// The source history must not be mutated (its ToolResults backing is shared).
	require.NotContains(t, src[36].ToolResults[0].Content, "compacted")

	// Tail without tool results (text-only turns): fall back to the head marker.
	textOnly := []Message{
		{Role: RoleUser, Text: "task"},
		{Role: RoleAssistant, Text: "a1"},
		{Role: RoleUser, Text: "u1"},
		{Role: RoleAssistant, Text: "a2"},
		{Role: RoleUser, Text: "u2"},
		{Role: RoleAssistant, Text: "a3"},
		{Role: RoleUser, Text: "u3"},
		{Role: RoleAssistant, Text: "a4"},
	}
	fb := compactMessages(textOnly, 2)
	require.Contains(t, fb[0].Text, "compacted")

	// C3: a tail boundary landing on a mid-history user message (e.g. a nudge)
	// must advance to the next assistant turn, never leaving two consecutive
	// user messages (which the Anthropic API rejects).
	withNudge := []Message{
		{Role: RoleUser, Text: "task"},
		{Role: RoleAssistant, Text: "a1"},
		{Role: RoleTool},
		{Role: RoleUser, Text: "nudge"},
		{Role: RoleAssistant, Text: "a2"},
		{Role: RoleTool},
	}
	got := compactMessages(withNudge, 3) // cut lands on the nudge(user)
	require.Equal(t, RoleUser, got[0].Role)
	require.NotEqual(t, RoleUser, got[1].Role, "must not produce two consecutive user messages")

	// Tail entirely tool messages: no assistant to resume on, so compaction is
	// skipped rather than dropping the whole tail.
	allTail := make([]Message, 20)
	allTail[0] = Message{Role: RoleUser, Text: "task"}
	for i := 1; i < 20; i++ {
		allTail[i] = Message{Role: RoleTool}
	}
	require.Len(t, compactMessages(allTail, 3), 20)
}

func TestRunRespectsContextCancel(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "glob", Args: json.RawMessage(`{"pattern":"*"}`)}}},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the first round

	res, err := Run(ctx, prov, reg, "sys", "go", Options{MaxRounds: 5})
	require.Error(t, err)
	require.Equal(t, "cancelled", res.StopReason)
	require.Empty(t, prov.seen, "provider must not be called once ctx is cancelled")
}

func TestRunMaxRoundsWithoutSubmit(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		{ToolCalls: []ToolCall{{ID: "1", Name: "glob", Args: json.RawMessage(`{"pattern":"**/*.go"}`)}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "glob", Args: json.RawMessage(`{"pattern":"**/*.go"}`)}}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 2})
	require.ErrorIs(t, err, errMaxRounds)
	require.False(t, res.Submitted)
	require.Equal(t, "max_rounds", res.StopReason)
}

func TestDispatchParallelRecoversPanic(t *testing.T) {
	reg := NewRegistry()
	reg.Register(ToolDef{Name: "boom"}, func(context.Context, json.RawMessage) (string, error) {
		panic("kaboom")
	})
	reg.Register(ToolDef{Name: "ok"}, func(context.Context, json.RawMessage) (string, error) {
		return "fine", nil
	})

	// A panicking tool must yield an error result, not crash the process, and
	// must not stop the sibling tool in the same turn from completing.
	results := dispatchParallel(context.Background(), reg, []ToolCall{
		{ID: "1", Name: "boom"},
		{ID: "2", Name: "ok"},
	})
	require.Len(t, results, 2)
	require.True(t, results[0].IsError)
	require.Contains(t, results[0].Content, "panicked")
	require.False(t, results[1].IsError)
	require.Equal(t, "fine", results[1].Content)
}

func TestRunTruncatedToolRoundNotifiesModel(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		// max_tokens cut the turn: add_issues arrived with empty args.
		{StopReason: stopMaxTokens, ToolCalls: []ToolCall{{ID: "1", Name: toolAddIssues, Args: json.RawMessage(`{}`)}}},
		{ToolCalls: []ToolCall{{ID: "2", Name: "submit_review", Args: validSubmitArgs(t, "high")}}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.NoError(t, err)
	require.True(t, res.Submitted)
	require.Len(t, prov.seen, 2)

	// The tool turn fed back must carry an error for the empty batch plus the
	// truncation notice folded into the last result.
	last := prov.seen[1].Messages[len(prov.seen[1].Messages)-1]
	require.Equal(t, RoleTool, last.Role)
	require.Len(t, last.ToolResults, 1)
	require.True(t, last.ToolResults[0].IsError)
	require.Contains(t, last.ToolResults[0].Content, "empty issues array")
	require.Contains(t, last.ToolResults[0].Content, noticeTruncated)
}

func TestRunTruncatedBareTurnRetries(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	prov := &scriptedProvider{responses: []Response{
		// Cut off mid-text before any tool call.
		{Text: "Now I will add the iss", StopReason: stopMaxTokens},
		{ToolCalls: []ToolCall{{ID: "1", Name: "submit_review", Args: validSubmitArgs(t, "low")}}},
	}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.NoError(t, err)
	require.True(t, res.Submitted)
	require.Len(t, prov.seen, 2)
	last := prov.seen[1].Messages[len(prov.seen[1].Messages)-1]
	require.Equal(t, RoleUser, last.Role)
	require.Equal(t, noticeTruncated, last.Text)
}

func TestRunAbortsOnConsecutiveTruncatedRounds(t *testing.T) {
	dir := t.TempDir()
	reg := NewReviewRegistry(ReviewToolsConfig{Dir: dir})
	trunc := Response{StopReason: stopMaxTokens, ToolCalls: []ToolCall{{ID: "1", Name: toolAddIssues, Args: json.RawMessage(`{}`)}}}
	prov := &scriptedProvider{responses: []Response{trunc, trunc, trunc, trunc}}

	res, err := Run(context.Background(), prov, reg, "system", "review this", Options{MaxRounds: 10})
	require.ErrorIs(t, err, errTruncatedRounds)
	require.Equal(t, "error", res.StopReason)
	require.False(t, res.Submitted)
	require.Equal(t, maxConsecutiveTruncated, res.Rounds)
}

func TestIsTruncated(t *testing.T) {
	require.True(t, IsTruncated(stopMaxTokens), "Anthropic stop_reason")
	require.True(t, IsTruncated(stopLength), "OpenAI finish_reason")
	require.False(t, IsTruncated("end_turn"))
	require.False(t, IsTruncated(""))
}
