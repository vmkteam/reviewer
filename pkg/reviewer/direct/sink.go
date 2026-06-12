package direct

import "encoding/json"

// Event is one record in the session transcript written for later analysis.
// Records are emitted in order from the loop's main goroutine, so a Sink need
// not be safe for concurrent use.
//
// Kinds:
//   - "assistant"   — model text for a round (Text)
//   - "tool_call"   — a tool the model requested (Tool, Args)
//   - "tool_result" — the tool's output, truncated (Tool, Content, IsError)
//   - "round"       — per-round usage and stop reason (Usage, StopReason)
//   - "result"      — final totals (Rounds, Usage, CostUsd, Submitted, Model, StopReason)
type Event struct {
	Round      int             `json:"round"`
	Kind       string          `json:"kind"`
	Text       string          `json:"text,omitempty"`
	Tool       string          `json:"tool,omitempty"`
	Args       json.RawMessage `json:"args,omitempty"`
	Content    string          `json:"content,omitempty"`
	IsError    bool            `json:"isError,omitempty"`
	Usage      *Usage          `json:"usage,omitempty"`
	StopReason string          `json:"stopReason,omitempty"`
	Rounds     int             `json:"rounds,omitempty"`
	CostUsd    float64         `json:"costUsd,omitempty"`
	Submitted  bool            `json:"submitted,omitempty"`
	Model      string          `json:"model,omitempty"`
}

// Sink receives transcript events. A nil Sink is a no-op.
type Sink func(Event)

func (s Sink) emit(ev Event) {
	if s != nil {
		s(ev)
	}
}

// logContentClip caps tool-result content recorded in the transcript; the full
// output already went to the model, the log only needs a readable excerpt.
const logContentClip = 4_000
