package fscore

import (
	"testing"
)

func TestIsHTTPURL(t *testing.T) {
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
