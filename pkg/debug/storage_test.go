package debug

import (
	"strconv"
	"sync"
	"testing"
)

func TestStorage_AddAssignsID(t *testing.T) {
	s := New(3, 3)
	b := &Bundle{ProjectKey: "p"}
	s.Add(b)

	if b.ID == "" {
		t.Fatal("Add must assign ID when empty")
	}
	if b.Timestamp.IsZero() {
		t.Fatal("Add must set Timestamp when zero")
	}
}

func TestStorage_RingEvictsOldest(t *testing.T) {
	s := New(2, 2)
	a := &Bundle{ID: "a"}
	b := &Bundle{ID: "b"}
	c := &Bundle{ID: "c"}

	s.Add(a)
	s.Add(b)
	s.Add(c)

	if got := s.Get("a"); got != nil {
		t.Errorf("oldest bundle should be evicted, got %v", got)
	}
	if got := s.Get("b"); got == nil {
		t.Error("bundle b should still be present")
	}
	if got := s.Get("c"); got == nil {
		t.Error("bundle c should still be present")
	}
}

func TestStorage_ListNewestFirst(t *testing.T) {
	s := New(3, 3)
	s.Add(&Bundle{ID: "1"})
	s.Add(&Bundle{ID: "2"})
	s.Add(&Bundle{ID: "3"})

	got := s.List()
	want := []string{"3", "2", "1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, b := range got {
		if b.ID != want[i] {
			t.Errorf("List[%d] = %s, want %s", i, b.ID, want[i])
		}
	}
}

func TestStorage_GetFile(t *testing.T) {
	s := New(1, 1)
	s.Add(&Bundle{
		ID:    "x",
		Files: map[string]File{"review.json": {Gzip: []byte("gz"), Size: 2}},
	})

	f, ok := s.GetFile("x", "review.json")
	if !ok || f.Size != 2 || string(f.Gzip) != "gz" {
		t.Errorf("GetFile = %+v, %v", f, ok)
	}

	if _, ok := s.GetFile("x", "missing.txt"); ok {
		t.Error("GetFile must return false for missing file")
	}
	if _, ok := s.GetFile("missing", "review.json"); ok {
		t.Error("GetFile must return false for missing bundle")
	}
}

func TestStorage_ConcurrentAddGet(t *testing.T) {
	s := New(50, 50)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Add(&Bundle{ID: strconv.Itoa(i)})
		}(i)
	}
	for range 100 {
		wg.Go(func() {
			_ = s.List()
		})
	}
	wg.Wait()

	if got := len(s.List()); got != 50 {
		t.Errorf("len after concurrent Add = %d, want 50", got)
	}
}

func TestStorage_NonPositiveCapacityDefaultsToOne(t *testing.T) {
	s := New(0, 0)
	s.Add(&Bundle{ID: "a"})
	s.Add(&Bundle{ID: "b"})

	if got := len(s.List()); got != 1 {
		t.Errorf("len = %d, want 1", got)
	}
	if s.Get("b") == nil {
		t.Error("newest bundle should remain after eviction")
	}
}

func TestStorage_EvictsFilesBeyondFilesCapacity(t *testing.T) {
	s := New(5, 2)
	for _, id := range []string{"1", "2", "3"} {
		s.Add(&Bundle{ID: id, Files: map[string]File{"review.json": {Size: 2}}})
	}
	old := s.Get("1")
	if old == nil {
		t.Fatal("metadata of the oldest bundle must be kept")
	}
	if old.Files != nil || !old.FilesEvicted {
		t.Errorf("oldest bundle must lose its files: files=%v evicted=%v", old.Files, old.FilesEvicted)
	}
	if _, ok := s.GetFile("3", "review.json"); !ok {
		t.Error("newest bundle must keep its files")
	}
	if _, ok := s.GetFile("2", "review.json"); !ok {
		t.Error("second newest bundle must keep its files")
	}
}

func TestStorage_EvictionDoesNotMutateSharedBundle(t *testing.T) {
	s := New(3, 1)
	first := &Bundle{ID: "1", Files: map[string]File{"a": {Size: 1}}}
	s.Add(first)
	s.Add(&Bundle{ID: "2", Files: map[string]File{"a": {Size: 1}}})
	// A reader holding the old pointer (a page being rendered) keeps a consistent view.
	if first.Files == nil || first.FilesEvicted {
		t.Error("eviction must replace the stored bundle, not mutate it")
	}
}

func TestNew_ClampsFilesCapacity(t *testing.T) {
	s := New(2, 10)
	if s.filesCapacity != 2 {
		t.Errorf("filesCapacity = %d, want clamped to capacity 2", s.filesCapacity)
	}
	if s = New(0, 0); s.capacity != 1 || s.filesCapacity != 1 {
		t.Errorf("non-positive capacities must default to 1, got %d/%d", s.capacity, s.filesCapacity)
	}
}
