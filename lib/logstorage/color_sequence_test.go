package logstorage

import (
	"testing"
)

func TestHasColorSequences(t *testing.T) {
	f := func(s string, resultExpected bool) {
		t.Helper()

		result := hasColorSequences(s)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("", false)
	f("foo", false)
	f("\x1babc", false)
	f("\x1b[abc", true)
	f("axxb\x1b[", true)
	f("axxb\x1b[abc", true)
}

func TestDropColorSequences(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		result := dropColorSequences(nil, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}

	// empty string
	f("", "")

	// zero color escape sequences
	f("a", "a")
	f("FooBar", "FooBar")

	// invalid color escape sequence
	f("foo\x1b[\x01", "foo\x01")

	// valid color escape sequence
	// See https://gist.github.com/ConnerWill/d4b6c776b509add763e17f9f113fd25b#colors--graphics-mode
	f("\x1b[mfoo\x1b[1;31mERROR bar\x1b[10;5H", "fooERROR bar")
	f("\x1b[mfoo\x1b[1;31mERROR bar\x1b[10;5Hbaz", "fooERROR barbaz")

	// valid erase escape sequence
	// See https://gist.github.com/ConnerWill/d4b6c776b509add763e17f9f113fd25b#erase-functions
	f("foo\x1b[2Jbar", "foobar")

	// valid cursor controls escape sequence
	// See https://gist.github.com/ConnerWill/d4b6c776b509add763e17f9f113fd25b#cursor-controls
	f("abc\x1b[65;81fdef", "abcdef")

	// valid operating system command sequence. It is left as is.
	f("\x1b]0;My Terminal Title\x07", "\x1b]0;My Terminal Title\x07")

	// valid device control string sequence. It is left as is.
	f("a\x1bP 1;2;3 qabc\x1b\\", "a\x1bP 1;2;3 qabc\x1b\\")

}
