package reviewer

import "unicode/utf8"

// ClipUTF8 cuts s to at most n bytes without splitting a rune.
func ClipUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
