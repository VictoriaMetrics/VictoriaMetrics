package storage

import (
	"testing"
)

func TestPartHeaderParseFromPath(t *testing.T) {
	testParseFromPathError := func(path string) {
		t.Helper()

		var ph partHeader
		if err := ph.ParseFromPath(path); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	t.Run("Error", func(t *testing.T) {
		testParseFromPathError("")
		testParseFromPathError("foobar")
		testParseFromPathError("/foo/bar")
		testParseFromPathError("/rowscount_mintimestamp_maxtimestamp_garbage")
		testParseFromPathError("/rowscount_mintimestamp_maxtimestamp_garbage")
		testParseFromPathError("/12_3456_mintimestamp_maxtimestamp_garbage")
		testParseFromPathError("/12_3456_20181011010203.456_maxtimestamp_garbage")
		testParseFromPathError("/12_3456_20181011010203.456_20181011010202.456_garbage")
		testParseFromPathError("12_3456_20181011010203.456_20181011010203.457_garbage")
		testParseFromPathError("12_3456_20181011010203.456_20181011010203.457_garbage/")

		// MinTimestamp > MaxTimetamp
		testParseFromPathError("1233_456_20181011010203.456_20181011010202.457_garbage")

		// Zero rowsCount
		testParseFromPathError("0_123_20181011010203.456_20181011010203.457_garbage")

		// Zero blocksCount
		testParseFromPathError("123_0_20181011010203.456_20181011010203.457_garbage")

		// blocksCount > rowsCount
		testParseFromPathError("123_456_20181011010203.456_20181011010203.457_garbage")
	})

	testParseFromPathSuccess := func(path string, phStringExpected string) {
		t.Helper()

		var ph partHeader
		if err := ph.ParseFromPath(path); err != nil {
			t.Fatalf("unexpected error when parsing path %q: %s", path, err)
		}
		phString := ph.String()
		if phString != phStringExpected {
			t.Fatalf("unexpected partHeader string for path %q: got %q; want %q", path, phString, phStringExpected)
		}
	}

	t.Run("Success", func(t *testing.T) {
		testParseFromPathSuccess("/1233_456_20181011010203.456_20181011010203.457_garbage", "1233_456_20181011010203.456_20181011010203.457")
		testParseFromPathSuccess("/1233_456_20181011010203.456_20181011010203.457_garbage/", "1233_456_20181011010203.456_20181011010203.457")
		testParseFromPathSuccess("/1233_456_20181011010203.456_20181011010203.457_garbage///", "1233_456_20181011010203.456_20181011010203.457")
		testParseFromPathSuccess("/var/lib/tsdb/1233_456_20181011010203.456_20181011010203.457_garbage///", "1233_456_20181011010203.456_20181011010203.457")
		testParseFromPathSuccess("/var/lib/tsdb/456_456_20181011010203.456_20181011010203.457_232345///", "456_456_20181011010203.456_20181011010203.457")
	})
}
