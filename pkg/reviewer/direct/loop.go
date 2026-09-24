package direct

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"reviewsrv/pkg/reviewer"
)

// errMaxRounds is returned when the loop exhausts MaxRounds without a submit.
var errMaxRounds = reviewer.WithRunReason(reviewer.RunReasonMaxRounds,
	errors.New("direct: max rounds reached without submit_review"))

// errTruncatedRounds aborts the loop when several rounds in a row are cut by the
// output-token cap — each such round burns the full max_tokens budget without
// making progress, so grinding on to MaxRounds only wastes money.
var errTruncatedRounds = reviewer.WithRunReason(reviewer.RunReasonTruncated,
	errors.New("direct: consecutive rounds truncated by the output-token limit"))

// maxConsecutiveTruncated is how many truncated rounds in a row abort the run.
const maxConsecutiveTruncated = 3

// nudgeSubmit is injected once if the model stops producing tool calls before
// submitting the review.
const nudgeSubmit = "You have not called submit_review yet. " +
	"Finish the review now by calling the submit_review tool with the full review.json content and the R1..R5 markdown bodies."

// noticeTruncated tells the model its last turn was cut by the output-token
// cap, so tool calls may have arrived with empty or partial arguments.
const noticeTruncated = "NOTE: your previous response was cut off by the output-token limit, " +
	"so any tool call in it may have been received with empty or partial arguments. " +
	"Re-issue the affected tool calls with smaller payloads (e.g. fewer issues per add_issues batch)."

// Provider stop reasons meaning the response was cut by the output-token cap.
const (
	stopMaxTokens = "max_tokens" // Anthropic stop_reason
	stopLength    = "length"     // OpenAI finish_reason
)

// IsTruncated reports whether a provider stop reason means the response was cut
// by the output-token cap.
func IsTruncated(stopReason string) bool {
	return stopReason == stopMaxTokens || stopReason == stopLength
}

// Round budget. At budgetWarnAt rounds left the model is told to start
// delivering the review, at budgetFinalAt to deliver it now. Once MaxRounds is
// spent, up to graceRounds more rounds run with only the review tools working:
// a review that is mid-delivery still lands instead of being thrown away with
// everything it cost.
const (
	budgetWarnAt  = 10
	budgetFinalAt = 3
	graceRounds   = 5
	// maxDeniedRounds grace rounds in a row made only of refused calls end the
	// run: the model is not delivering the review, and each such round re-sends
	// the whole history.
	maxDeniedRounds = 2
)

// budgetDenied is the tool error for a read/search call during the grace rounds.
const budgetDenied = "round budget spent — only set_group, add_issues and submit_review work now; deliver the review from what you have"

// budgetNotice returns the notice for the given number of rounds left, or "".
func budgetNotice(left int) string {
	switch left {
	case budgetWarnAt:
		return fmt.Sprintf("NOTE: %d rounds of your round budget are left. Wrap up exploring and start delivering the review: "+
			"set_group for all five groups and add_issues, then submit_review.", left)
	case budgetFinalAt:
		return fmt.Sprintf("NOTE: only %d rounds are left. Stop exploring and deliver the review now with set_group, add_issues "+
			"and submit_review, batching the calls in the same step.", left)
	case 0:
		return fmt.Sprintf("NOTE: the round budget is spent — read and search tools are disabled from now on. Deliver the review "+
			"from what you have within %d steps: set_group for all five groups, add_issues, then submit_review.", graceRounds)
	}
	return ""
}

// appendNotice folds a harness notice into the last tool result, so the model
// gets it without an extra user message (which would put two consecutive user
// turns in the history — the Anthropic API rejects that).
func appendNotice(results []ToolResult, notice string) {
	if len(results) == 0 {
		return
	}
	last := &results[len(results)-1]
	last.Content = strings.TrimSpace(last.Content + "\n\n" + notice)
}

// Result is the outcome of a direct run, mapped to ClaudeResult by the ctl adapter.
type Result struct {
	Usage         Usage
	Rounds        int
	StopReason    string // "submitted" | "end_turn" | "max_rounds" | "error"
	Submitted     bool
	Model         string
	CostUsd       float64
	DurationAPIMs int // cumulative time spent in provider Complete calls
}

// Run drives the agent loop: send system + history + tools to the provider, run
// any requested tools, feed results back, and repeat until the model calls
// submit_review (success), stops without tools (end_turn), or MaxRounds plus the
// review-only grace rounds are spent.
func Run(ctx context.Context, p LLMProvider, reg *Registry, system, userPrompt string, opts Options) (*Result, error) {
	if opts.MaxRounds <= 0 {
		opts.MaxRounds = DefaultOptions().MaxRounds
	}

	msgs := []Message{{Role: RoleUser, Text: userPrompt}}
	var (
		total           Usage
		apiMs           int // cumulative provider Complete time (vs total wall-clock)
		nudged          bool
		truncatedRounds int // consecutive rounds cut by the output-token cap
		deniedRounds    int // consecutive grace rounds whose every call was refused
	)

	// Record the kickoff input (system contract + user task with the preloaded
	// diff/files) so the transcript is a full input/output log, not just the
	// model's outputs and tool I/O.
	opts.OnEvent.emit(Event{Kind: "system", Text: system})
	opts.OnEvent.emit(Event{Kind: "user", Text: userPrompt})

	// finish builds the result and records it in the transcript.
	finish := func(rounds int, stop string, submitted bool) *Result {
		r := makeResult(total, rounds, stop, submitted, p)
		r.DurationAPIMs = apiMs
		opts.OnEvent.emit(Event{Kind: "result", Rounds: r.Rounds, Usage: &r.Usage, StopReason: r.StopReason, CostUsd: r.CostUsd, Submitted: r.Submitted, Model: r.Model})
		return r
	}

	limit := opts.MaxRounds + graceRounds
	for round := range limit {
		if err := ctx.Err(); err != nil {
			return finish(round, "cancelled", reg.Submitted()), err
		}
		if round == opts.MaxRounds {
			reg.restrict(budgetDenied, toolSetGroup, toolAddIssues, toolSubmitReview)
		}
		resp, roundMs, err := completeRound(ctx, p, Request{System: system, Messages: msgs, Tools: reg.Defs(), Effort: opts.Effort}, opts.OnEvent, round)
		apiMs += roundMs
		if err != nil {
			return finish(round, "error", reg.Submitted()), reviewer.WithRunReason(providerReason(err), fmt.Errorf("round %d: %w", round, err))
		}
		total = sumUsage(total, resp.Usage)
		emitRound(opts.OnEvent, round, resp, roundMs)
		msgs = append(msgs, Message{Role: RoleAssistant, Text: resp.Text, ToolCalls: resp.ToolCalls, Raw: resp.Raw})

		// A truncated round burned the whole max_tokens budget (thinking + partial
		// output); tell the model so it retries smaller, and bail out after a few
		// in a row — repeating the same over-budget turn never converges. The
		// abort happens AFTER dispatching whatever tool calls did arrive, so a
		// submit_review or issue batch that survived the cut is not thrown away.
		truncated := IsTruncated(resp.StopReason)
		truncatedRounds = consecutive(truncatedRounds, truncated)
		abort := truncatedRounds >= maxConsecutiveTruncated

		if len(resp.ToolCalls) == 0 {
			if truncated && !reg.Submitted() {
				if abort {
					return finish(round+1, "error", false), fmt.Errorf("round %d: %w", round, errTruncatedRounds)
				}
				// Cut off mid-text before any tool call: ask for a retry rather than
				// treating it as a deliberate bare turn.
				msgs = append(msgs, Message{Role: RoleUser, Text: noticeTruncated})
				continue
			}
			// Model produced only text. If it hasn't submitted, nudge once; on a
			// second bare turn, give up cleanly.
			if !reg.Submitted() && !nudged {
				nudged = true
				msgs = append(msgs, Message{Role: RoleUser, Text: nudgeSubmit})
				continue
			}
			return finish(round+1, "end_turn", reg.Submitted()), nil
		}

		toolMsg, allDenied := runTools(ctx, reg, opts, round, resp.ToolCalls, truncated)
		msgs = append(msgs, toolMsg)

		if reg.Submitted() {
			return finish(round+1, "submitted", true), nil
		}
		if abort {
			return finish(round+1, "error", false), fmt.Errorf("round %d: %w", round, errTruncatedRounds)
		}
		deniedRounds = consecutive(deniedRounds, allDenied)
		if deniedRounds >= maxDeniedRounds {
			return finish(round+1, "max_rounds", false), errMaxRounds
		}
		msgs = maybeCompact(msgs, opts, round)
	}

	return finish(limit, "max_rounds", reg.Submitted()), errMaxRounds
}

// runTools dispatches a round's tool calls and returns the tool message fed back
// to the model, with any harness notices (truncation, round budget) folded into
// its last result, and whether the round budget refused every call.
func runTools(ctx context.Context, reg *Registry, opts Options, round int, calls []ToolCall, truncated bool) (Message, bool) {
	denied0 := reg.deniedCalls()
	results := dispatchParallel(ctx, reg, calls)
	allDenied := reg.deniedCalls()-denied0 == len(calls)
	if truncated {
		appendNotice(results, noticeTruncated)
	}
	if !reg.Submitted() {
		noteBudget(opts, round, results)
	}
	emitToolResults(opts.OnEvent, round, results)
	return Message{Role: RoleTool, ToolResults: results}, allDenied
}

// noteBudget folds the round-budget notice due after this round (if any) into
// its tool results and records it in the transcript.
func noteBudget(opts Options, round int, results []ToolResult) {
	notice := budgetNotice(opts.MaxRounds - (round + 1))
	if notice == "" {
		return
	}
	appendNotice(results, notice)
	opts.OnEvent.emit(Event{Round: round, Kind: "notice", Text: notice})
}

// emitToolResults records each tool result in the transcript, clipped to keep
// the log readable.
func emitToolResults(s Sink, round int, results []ToolResult) {
	for _, tr := range results {
		s.emit(Event{Round: round, Kind: "tool_result", Tool: tr.Name, Content: clipN(tr.Content, logContentClip), IsError: tr.IsError})
	}
}

// consecutive advances a run-of-rounds counter (truncated rounds, fully refused
// rounds): a round without the condition resets it.
func consecutive(prev int, hit bool) int {
	if !hit {
		return 0
	}
	return prev + 1
}

// completeRound sends one round to the provider, recording each retry as it
// happens (a failed round's too), and returns its duration.
func completeRound(ctx context.Context, p LLMProvider, req Request, s Sink, round int) (Response, int, error) {
	req.OnRetry = func(err error) { s.emit(Event{Round: round, Kind: "retry", Text: err.Error()}) }
	t0 := time.Now()
	resp, err := p.Complete(ctx, req)
	return resp, int(time.Since(t0).Milliseconds()), err
}

// maybeCompact prunes the history once it crosses the compaction threshold,
// recording a transcript event: compaction rewrites the provider prompt cache,
// so a sudden cost jump must be traceable to its round.
func maybeCompact(msgs []Message, opts Options, round int) []Message {
	if opts.CompactAt <= 0 || estimateTokens(msgs) <= opts.CompactAt {
		return msgs
	}
	before := len(msgs)
	out := compactMessages(msgs, opts.KeepTail)
	if len(out) < before {
		opts.OnEvent.emit(Event{Round: round, Kind: "compact", Text: fmt.Sprintf("compacted history: %d -> %d messages", before, len(out))})
	}
	return out
}

// emitRound records the model's text, requested tool calls and per-round usage.
func emitRound(s Sink, round int, resp Response, durationMs int) {
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
	s.emit(Event{Round: round, Kind: "round", Usage: &u, StopReason: resp.StopReason, DurationMs: durationMs})
}

func makeResult(total Usage, rounds int, stop string, submitted bool, p LLMProvider) *Result {
	return &Result{
		Usage:      total,
		Rounds:     rounds,
		StopReason: stop,
		Submitted:  submitted,
		Model:      p.Model(),
		CostUsd:    p.Pricing().Cost(total),
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
			// A panicking tool handler must not crash the whole review: recover it
			// into an error result. The deferred func also writes results[i] on
			// every path (each goroutine owns its index, so there is no race).
			defer func() {
				if r := recover(); r != nil {
					tr.Content = fmt.Sprintf("tool panicked: %v", r)
					tr.IsError = true
				}
				results[i] = tr
			}()
			// Skip work if the run was cancelled (timeout / manual abort).
			if err := ctx.Err(); err != nil {
				tr.Content = "tool skipped: " + err.Error()
				tr.IsError = true
				return
			}
			out, err := reg.Dispatch(ctx, call.Name, call.Args)
			if err != nil {
				tr.Content = "tool error: " + err.Error()
				tr.IsError = true
			} else {
				tr.Content = out
			}
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
	// The kept head is the user task, so the tail must resume on an assistant
	// turn: advance off any leading tool result (which would lead without its
	// tool_use turn) AND any leading user message (which would collide with the
	// head into two consecutive user messages — the Anthropic API rejects that).
	for cut < len(msgs) && msgs[cut].Role != RoleAssistant {
		cut++
	}
	// If no assistant turn remains in the tail, advancing past everything would
	// drop it all — skip compaction this round rather than lose the tail.
	if cut >= len(msgs) {
		return msgs
	}
	dropped := cut - keepHead
	if dropped <= 0 {
		return msgs
	}
	marker := fmt.Sprintf("[compacted: %d earlier messages omitted to fit context]", dropped)
	out := make([]Message, 0, 1+(len(msgs)-cut))
	out = append(out, msgs[0]) // keepHead == 1
	out = append(out, msgs[cut:]...)
	// Drop the provider-native snapshots of the kept assistant turns so they are
	// rebuilt from Text+ToolCalls: their reasoning is bound to the pre-compaction
	// history, and Anthropic rejects replayed thinking blocks whose prefix
	// changed. Turns appended afterwards keep theirs.
	for i := range out {
		out[i].Raw = nil
	}
	// Place the marker inside the tail, keeping the head byte-identical: mutating
	// the head changes the first bytes after system+tools and invalidates the
	// provider prompt cache for the ENTIRE history, while the tail is re-written
	// anyway (its positions shifted). A standalone marker message would also
	// break the user/assistant alternation the Anthropic API requires, so it is
	// folded into the first tool-result of the tail instead.
	for i := 1; i < len(out); i++ {
		if out[i].Role == RoleTool && len(out[i].ToolResults) > 0 {
			trs := append([]ToolResult(nil), out[i].ToolResults...) // keep msgs' backing array intact
			trs[0].Content = marker + "\n\n" + trs[0].Content
			out[i].ToolResults = trs
			return out
		}
	}
	// Tail has no tool message (text-only turns) — fall back to folding the
	// marker into the head; correctness beats cache preservation here.
	head := out[0]
	head.Text = strings.TrimSpace(head.Text + "\n\n" + marker)
	out[0] = head
	return out
}
