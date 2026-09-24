package reviewer

import "testing"

func TestClipUTF8(t *testing.T) {
	for _, tt := range []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exact", 5, "exact"},
		{"ошибка", 5, "ош"}, // 2 bytes per rune: a cut at 5 must not split one
		{"ошибка", 0, ""},
	} {
		if got := ClipUTF8(tt.s, tt.n); got != tt.want {
			t.Errorf("ClipUTF8(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}
