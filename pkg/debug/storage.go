// Package debug provides an in-memory ring buffer of recent reviewctl runs.
// reviewctl uploads artifacts (claude-output.json, opencode-output.jsonl,
// review.json, R*.md) here when a review fails in CI, where GitLab job
// artifacts are unavailable. The buffer is intentionally small and ephemeral —
// restart of reviewsrv drops everything.
package debug

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Bundle is a single captured run. Files holds raw (un-gzipped) bytes keyed
// by original filename (e.g. "review.json", "R1.feat-foo.md").
type Bundle struct {
	ID           string
	Timestamp    time.Time
	ProjectKey   string
	MRIid        string
	ExternalID   string
	Runner       string
	Model        string
	SourceBranch string
	TargetBranch string
	CommitHash   string
	ErrorMsg     string
	Files        map[string][]byte
}

// Storage is a thread-safe ring buffer of Bundle values, newest last.
type Storage struct {
	mu       sync.RWMutex
	capacity int
	items    []*Bundle
}

// New returns a Storage with the given capacity. Once full, Add evicts
// the oldest bundle. capacity must be positive; non-positive values default to 1.
func New(capacity int) *Storage {
	if capacity <= 0 {
		capacity = 1
	}
	return &Storage{
		capacity: capacity,
		items:    make([]*Bundle, 0, capacity),
	}
}

// Add stores b, generating an ID if empty, and evicts the oldest entry when full.
func (s *Storage) Add(b *Bundle) {
	if b.ID == "" {
		b.ID = newID()
	}
	if b.Timestamp.IsZero() {
		b.Timestamp = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) >= s.capacity {
		// Nil the evicted slot first so the previous bundle's bytes can be
		// reclaimed; plain s.items[1:] would keep them pinned in the backing array.
		s.items[0] = nil
		s.items = s.items[1:]
	}
	s.items = append(s.items, b)
}

// List returns a snapshot of bundles, newest first.
func (s *Storage) List() []*Bundle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]*Bundle, len(s.items))
	for i, b := range s.items {
		out[len(s.items)-1-i] = b
	}
	return out
}

// Get returns the bundle with the given ID, or nil if absent.
func (s *Storage) Get(id string) *Bundle {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, b := range s.items {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// GetFile returns the raw file content from the bundle by ID and filename.
// The boolean is false when either the bundle or the file is missing.
func (s *Storage) GetFile(id, filename string) ([]byte, bool) {
	b := s.Get(id)
	if b == nil {
		return nil, false
	}
	data, ok := b.Files[filename]
	return data, ok
}

// newID returns a 12-char hex id derived from a UUIDv4. Short enough
// for readable URLs, wide enough to avoid collisions in a 10-slot buffer.
func newID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}
