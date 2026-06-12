package direct

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Handler executes a tool call: it receives the raw JSON arguments and returns
// the textual result fed back to the model. A returned error surfaces to the
// model as an error tool result; the loop continues.
type Handler func(ctx context.Context, args json.RawMessage) (string, error)

// ToolDef describes a tool to the model. Schema is a JSON Schema object that each
// provider translates to its SDK's tool-parameter shape.
type ToolDef struct {
	Name        string
	Description string
	Schema      map[string]any
}

// Registry holds the tool set for one run and dispatches calls. It also records
// whether submit_review has fired so the loop can terminate deterministically.
type Registry struct {
	mu        sync.Mutex
	order     []string
	defs      map[string]ToolDef
	handlers  map[string]Handler
	submitted bool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		defs:     map[string]ToolDef{},
		handlers: map[string]Handler{},
	}
}

// Register adds (or replaces) a tool. Insertion order is preserved so the tool
// list is deterministic — important for prompt-cache stability.
func (r *Registry) Register(def ToolDef, h Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.defs[def.Name]; !ok {
		r.order = append(r.order, def.Name)
	}
	r.defs[def.Name] = def
	r.handlers[def.Name] = h
}

// Defs returns the tool definitions in registration order.
func (r *Registry) Defs() []ToolDef {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ToolDef, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.defs[name])
	}
	return out
}

// Dispatch runs the named tool's handler.
func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	r.mu.Lock()
	h, ok := r.handlers[name]
	r.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown tool %q", name)
	}
	return h(ctx, args)
}

// markSubmitted records that submit_review wrote its artifacts successfully.
func (r *Registry) markSubmitted() {
	r.mu.Lock()
	r.submitted = true
	r.mu.Unlock()
}

// Submitted reports whether submit_review has fired.
func (r *Registry) Submitted() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.submitted
}
