package fs

import "testing"

func TestIsSinglePathSegment(t *testing.T) {
	ok := []string{"part", "0000000000000001", "20241124120000-ABCDEF"}
	bad := []string{"", ".", "..", "a/b", `a\b`, "/etc/passwd", "../x", ". ", ".. ", "..  ", "...", "... ", " . . ", " "}
	for _, name := range ok {
		if !IsSinglePathSegment(name) {
			t.Fatalf("expected %q to be a single segment", name)
		}
	}
	for _, name := range bad {
		if IsSinglePathSegment(name) {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}
