package direct

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// errMaxRounds is returned when the loop exhausts MaxRounds without a submit.
var errMaxRounds = errors.New("direct: max rounds reached without submit_review")

// nudgeSubmit is injected once if the model stops producing tool calls before
// submitting the review.
const nudgeSubmit = "You have not called submit_review yet. " +
	"Finish the review now by calling the submit_review tool with the full review.json content and the R1..R5 markdown bodies."

// Result is the outcome of a direct run, mapped to ClaudeResult by the ctl adapter.
type Result struct {
	Usage      Usage
	Rounds     int
	StopReason string // "submitted" | "end_turn" | "max_rounds" | "error"
	Submitted  bool
	Model      string
	CostUsd    float64
}

// Run drives the agent loop: send system + history + tools to the provider, run
// any requested tools, feed results back, and repeat until the model calls
// submit_review (success), stops without tools (end_turn), or MaxRounds is hit.
func Run(ctx context.Context, p LLMProvider, reg *Registry, system, userPrompt string, opts Options) (*Result, error) {
	if opts.MaxRounds <= 0 {
		opts.MaxRounds = DefaultOptions().MaxRounds
	}

	msgs := []Message{{Role: RoleUser, Text: userPrompt}}
	var total Usage
	nudged := false

	// finish builds the result and records it in the transcript.
	finish := func(rounds int, stop string, submitted bool) *Result {
		r := makeResult(total, rounds, stop, submitted, p)
		opts.OnEvent.emit(Event{Kind: "result", Rounds: r.Rounds, Usage: &r.Usage, StopReason: r.StopReason, CostUsd: r.CostUsd, Submitted: r.Submitted, Model: r.Model})
		return r
	}

	for round := range opts.MaxRounds {
		if err := ctx.Err(); err != nil {
			return finish(round, "cancelled", reg.Submitted()), err
		}
		resp, err := p.Complete(ctx, Request{System: system, Messages: msgs, Tools: reg.Defs(), Effort: opts.Effort})
		if err != nil {
			return finish(round, "error", reg.Submitted()), fmt.Errorf("round %d: %w", round, err)
		}
		total = sumUsage(total, resp.Usage)
		emitRound(opts.OnEvent, round, resp)
		msgs = append(msgs, Message{Role: RoleAssistant, Text: resp.Text, ToolCalls: resp.ToolCalls, Raw: resp.Raw})

		if len(resp.ToolCalls) == 0 {
			// Model produced only text. If it hasn't submitted, nudge once; on a
			// second bare turn, give up cleanly.
			if !reg.Submitted() && !nudged {
				nudged = true
				msgs = append(msgs, Message{Role: RoleUser, Text: nudgeSubmit})
				continue
			}
			return finish(round+1, "end_turn", reg.Submitted()), nil
		}

		results := dispatchParallel(ctx, reg, resp.ToolCalls)
		for _, tr := range results {
			opts.OnEvent.emit(Event{Round: round, Kind: "tool_result", Tool: tr.Name, Content: clipN(tr.Content, logContentClip), IsError: tr.IsError})
		}
		msgs = append(msgs, Message{Role: RoleTool, ToolResults: results})

		if reg.Submitted() {
			return finish(round+1, "submitted", true), nil
		}
		if opts.CompactAt > 0 && estimateTokens(msgs) > opts.CompactAt {
			msgs = compactMessages(msgs, opts.KeepTail)
		}
	}

	return finish(opts.MaxRounds, "max_rounds", reg.Submitted()), errMaxRounds
}

// emitRound records the model's text, requested tool calls and per-round usage.
func emitRound(s Sink, round int, resp Response) {
	if s == nil {
		return
	}
	if strings.TrimSpace(resp.Text) != "" {
		s.emit(Event{Round: round, Kind: "assistant", Text: resp.Text})
	}
	for _, tc := range resp.ToolCalls {
		s.emit(Event{Round: round, Kind: "tool_call", Tool: tc.Name, Args: tc.Args})
	}
	u := resp.Usage
	s.emit(Event{Round: round, Kind: "round", Usage: &u, StopReason: resp.StopReason})
}

func makeResult(total Usage, rounds int, stop string, submitted bool, p LLMProvider) *Result {
	return &Result{
		Usage:      total,
		Rounds:     rounds,
		StopReason: stop,
		Submitted:  submitted,
		Model:      p.Model(),
		CostUsd:    computeCost(total, p.Pricing()),
	}
}

// dispatchParallel runs all tool calls of one assistant turn concurrently and
// returns their results in call order. A tool error becomes an error result fed
// back to the model rather than failing the run.
func dispatchParallel(ctx context.Context, reg *Registry, calls []ToolCall) []ToolResult {
	results := make([]ToolResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Go(func() {
			tr := ToolResult{CallID: call.ID, Name: call.Name}
			// Skip work if the run was cancelled (timeout / manual abort) — the
			// goroutine writes its own index, so there is no race on results.
			if err := ctx.Err(); err != nil {
				tr.Content = "tool skipped: " + err.Error()
				tr.IsError = true
				results[i] = tr
				return
			}
			out, err := reg.Dispatch(ctx, call.Name, call.Args)
			if err != nil {
				tr.Content = "tool error: " + err.Error()
				tr.IsError = true
			} else {
				tr.Content = out
			}
			results[i] = tr
		})
	}
	wg.Wait()
	return results
}

// estimateTokens is a rough char/4 heuristic used only to trigger compaction.
func estimateTokens(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		n += len(m.Text) / 4
		for _, tc := range m.ToolCalls {
			n += len(tc.Args)/4 + len(tc.Name)
		}
		for _, tr := range m.ToolResults {
			n += len(tr.Content) / 4
		}
	}
	return n
}

// compactMessages prunes the middle of the conversation, keeping the first
// message (the task) and the last keepTail messages. The tail boundary is moved
// forward off a tool message so a tool result never leads without its assistant
// tool_use turn (which would break provider translation).
func compactMessages(msgs []Message, keepTail int) []Message {
	const keepHead = 1
	if keepTail <= 0 {
		keepTail = DefaultOptions().KeepTail
	}
	if len(msgs) <= keepHead+keepTail+1 {
		return msgs
	}
	cut := len(msgs) - keepTail
	for cut < len(msgs) && msgs[cut].Role == RoleTool {
		cut++
	}
	// If the whole tail is tool messages, advancing past them would drop
	// everything — skip compaction this round rather than lose the tail.
	if cut >= len(msgs) {
		return msgs
	}
	dropped := cut - keepHead
	if dropped <= 0 {
		return msgs
	}
	// Fold the compaction marker into the head (kept) message rather than
	// inserting a separate one — a standalone marker after the head user turn
	// would put two consecutive user messages in the history, which the Anthropic
	// API rejects (roles must alternate).
	head := msgs[0] // keepHead == 1
	head.Text = strings.TrimSpace(head.Text +
		fmt.Sprintf("\n\n[compacted: %d earlier messages omitted to fit context]", dropped))
	out := make([]Message, 0, 1+(len(msgs)-cut))
	out = append(out, head)
	out = append(out, msgs[cut:]...)
	return out
}
