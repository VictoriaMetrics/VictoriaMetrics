package logger

import (
	"fmt"
	"testing"
)

func TestFormatLogMessage(t *testing.T) {
	f := func(format string, args []any, maxArgLen int, expectedResult string) {
		t.Helper()
		result := formatLogMessage(maxArgLen, format, args)
		if result != expectedResult {
			t.Fatalf("unexpected result; got\n%q\nwant\n%q", result, expectedResult)
		}
	}

	// Zero format args
	f("foobar", nil, 1, "foobar")

	// Format args not exceeding the maxArgLen
	f("foo: %d, %s, %s, %s", []any{123, "bar", []byte("baz"), fmt.Errorf("abc")}, 3, "foo: 123, bar, baz, abc")

	// Format args exceeding the maxArgLen
	f("foo: %s, %q, %s", []any{"abcde", fmt.Errorf("foo bar baz"), "xx"}, 4, `foo: a..e, "f..z", xx`)
}

func TestSimplifyFilePath(t *testing.T) {
	f := func(path, expectedResult string) {
		t.Helper()
		result := simplifyFilePath(path)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %q, want %q", result, expectedResult)
		}
	}

	// log in VictoriaMetrics repo
	f(
		`/VictoriaMetrics/VictoriaMetrics/1.go`,
		"VictoriaMetrics/1.go",
	)

	// used in other repo
	f(
		`/VictoriaMetrics/VictoriaTraces/1.go`,
		"VictoriaTraces/1.go",
	)

	// used in other repo as vendor
	f(
		`/VictoriaMetrics/VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go`,
		"VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)

	// used in other repo as vendor with version num
	f(
		`/VictoriaMetrics/VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics@v0.0.0-00010101000000-000000000000/1.go`,
		"VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)

	// The following tests are for local builds, for which path is the absolute path.
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces/1.go`,
		"VictoriaTraces/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go`,
		"VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics@v0.0.0-00010101000000-000000000000/1.go`,
		"VictoriaTraces/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaMetrics-enterprise/1.go`,
		"VictoriaMetrics-enterprise/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces-enterprise/1.go`,
		"VictoriaTraces-enterprise/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces-enterprise/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go`,
		"VictoriaTraces-enterprise/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)
	f(
		`/Users/jiekun/repo/github.com/VictoriaMetrics/VictoriaTraces-enterprise/vendor/github.com/VictoriaMetrics/VictoriaMetrics@v0.0.0-00010101000000-000000000000/1.go`,
		"VictoriaTraces-enterprise/vendor/github.com/VictoriaMetrics/VictoriaMetrics/1.go",
	)

	// special cases that user may rename the repo to whatever they want and does not contain `/VictoriaMetrics/`.
	f(
		`/Users/jiekun/repo/github.com/VictoriaTraces/1.go`,
		"/Users/jiekun/repo/github.com/VictoriaTraces/1.go",
	)
	f(
		`/what_ever_path/1.go`,
		"/what_ever_path/1.go",
	)
}
