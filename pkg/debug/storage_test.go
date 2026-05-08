package debug

import (
	"strconv"
	"sync"
	"testing"
)

func TestStorage_AddAssignsID(t *testing.T) {
	s := New(3)
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
	s := New(2)
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
	s := New(3)
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
	s := New(1)
	s.Add(&Bundle{
		ID:    "x",
		Files: map[string][]byte{"review.json": []byte("{}")},
	})

	data, ok := s.GetFile("x", "review.json")
	if !ok || string(data) != "{}" {
		t.Errorf("GetFile = %q, %v", data, ok)
	}

	if _, ok := s.GetFile("x", "missing.txt"); ok {
		t.Error("GetFile must return false for missing file")
	}
	if _, ok := s.GetFile("missing", "review.json"); ok {
		t.Error("GetFile must return false for missing bundle")
	}
}

func TestStorage_ConcurrentAddGet(t *testing.T) {
	s := New(50)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Add(&Bundle{ID: strconv.Itoa(i)})
		}(i)
	}
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.List()
		}()
	}
	wg.Wait()

	if got := len(s.List()); got != 50 {
		t.Errorf("len after concurrent Add = %d, want 50", got)
	}
}

func TestStorage_NonPositiveCapacityDefaultsToOne(t *testing.T) {
	s := New(0)
	s.Add(&Bundle{ID: "a"})
	s.Add(&Bundle{ID: "b"})

	if got := len(s.List()); got != 1 {
		t.Errorf("len = %d, want 1", got)
	}
	if s.Get("b") == nil {
		t.Error("newest bundle should remain after eviction")
	}
}
