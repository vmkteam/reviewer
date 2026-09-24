// Package debug provides an in-memory ring buffer of recent reviewctl runs.
// reviewctl uploads artifacts (claude-output.json, opencode-output.jsonl,
// review.json, R*.md) here when a review fails in CI, where GitLab job
// artifacts are unavailable. Run metadata outlives the artifacts (only the
// newest bundles keep their files); a restart of reviewsrv drops everything —
// the reviewer_runs_total metric is the durable record.
package debug

import (
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// File is one artifact, kept gzip-compressed as uploaded: runner transcripts
// compress 5-15x, so the ring's memory is bounded by the upload cap.
type File struct {
	Gzip []byte // compressed content
	Size int    // decompressed length
}

// Bundle is a single captured run. Files holds the artifacts keyed by original
// filename (e.g. "review.json", "R1.feat-foo.md").
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
	Status       string  // reviewer.RunStatus*, normalized on upload
	Reason       string  // reviewer.RunReason*; empty for an ok run
	CostUsd      float64 // what the run spent, including a failed one
	ProjectTitle string  // resolved from ProjectKey; empty for an unknown key
	Files        map[string]File
	FilesEvicted bool // artifacts dropped to cap memory; metadata kept
}

// Storage is a thread-safe ring buffer of Bundle values, newest last.
type Storage struct {
	mu            sync.RWMutex
	capacity      int
	filesCapacity int
	items         []*Bundle
}

// New returns a Storage keeping up to capacity bundles, of which only the newest
// filesCapacity keep their artifact files: metadata is bytes, artifacts are
// megabytes. Once full, Add evicts the oldest bundle. Non-positive capacity
// defaults to 1; filesCapacity is clamped to [1, capacity].
func New(capacity, filesCapacity int) *Storage {
	capacity = max(capacity, 1)
	return &Storage{
		capacity:      capacity,
		filesCapacity: min(max(filesCapacity, 1), capacity),
		items:         make([]*Bundle, 0, capacity),
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
	s.evictFiles()
}

// evictFiles drops the artifacts of bundles beyond the newest filesCapacity
// that still hold files. A stored bundle is shared with concurrent readers, so
// it is replaced by a trimmed copy rather than mutated. Caller holds s.mu.
func (s *Storage) evictFiles() {
	kept := 0
	for i := len(s.items) - 1; i >= 0; i-- {
		b := s.items[i]
		if len(b.Files) == 0 {
			continue
		}
		kept++
		if kept <= s.filesCapacity {
			continue
		}
		trimmed := *b
		trimmed.Files = nil
		trimmed.FilesEvicted = true
		s.items[i] = &trimmed
	}
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

// GetFile returns a file of the bundle by ID and filename. The boolean is
// false when either the bundle or the file is missing.
func (s *Storage) GetFile(id, filename string) (File, bool) {
	b := s.Get(id)
	if b == nil {
		return File{}, false
	}
	f, ok := b.Files[filename]
	return f, ok
}

// newID returns a 12-char hex id derived from a UUIDv4. Short enough
// for readable URLs, wide enough to avoid collisions in a 10-slot buffer.
func newID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}
