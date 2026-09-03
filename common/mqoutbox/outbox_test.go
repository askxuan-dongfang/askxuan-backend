package mqoutbox

import "testing"

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("ab", 3); got != "ab" {
		t.Fatalf("got %q", got)
	}
}
