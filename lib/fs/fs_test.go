package fs

import (
	"testing"
)

func TestIsTemporaryFileName(t *testing.T) {
	f := func(s string, resultExpected bool) {
		t.Helper()
		result := IsTemporaryFileName(s)
		if result != resultExpected {
			t.Fatalf("unexpected IsTemporaryFileName(%q); got %v; want %v", s, result, resultExpected)
		}
	}
	f("", false)
	f(".", false)
	f(".tmp", false)
	f("tmp.123", false)
	f(".tmp.123.xx", false)
	f(".tmp.1", true)
	f("asdf.dff.tmp.123", true)
	f("asdf.sdfds.tmp.dfd", false)
	f("dfd.sdfds.dfds.1232", false)
}

func TestIsHTTPURLSuccess(t *testing.T) {
	f := func(s string, expected bool) {
		t.Helper()
		res := isHTTPURL(s)
		if res != expected {
			t.Fatalf("expecting %t, got %t", expected, res)
		}
	}
	f("http://isvalid:8000/filepath", true)  // test http
	f("https://isvalid:8000/filepath", true) // test https
	f("tcp://notvalid:8000/filepath", false) // test tcp
	f("0/filepath", false)                   // something invalid
	f("filepath.extension", false)           // something invalid
}
