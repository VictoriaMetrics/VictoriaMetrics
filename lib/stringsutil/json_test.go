package stringsutil

import (
	"testing"
)

func TestJSONString(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		result := JSONString(s)
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f(``, `""`)
	f(`foo`, `"foo"`)
	f("\n\b\f\t\"acЫВА'\\", `"\n\b\f\t\"acЫВА\u0027\\"`)
}
