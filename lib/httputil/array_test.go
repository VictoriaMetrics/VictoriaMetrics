package httputil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestGetQueryArgOrHeader(t *testing.T) {
	f := func(urlStr, header, body, resultExpected string) {
		t.Helper()

		r, err := http.NewRequest(http.MethodPost, urlStr, strings.NewReader(body))
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if header != "" {
			r.Header.Set("X-Foo", header)
		}

		result := GetQueryArgOrHeader(r, "foo", "X-Foo")
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("unexpected error when reading request body: %s", err)
		}
		if string(data) != body {
			t.Fatalf("unexpected request body; got %q; want %q", data, body)
		}
	}

	// missing value
	f("http://foo.bar/baz", "", "", "")

	// value from query arg
	f("http://foo.bar/baz?foo=bar", "", "", "bar")

	// value from header
	f("http://foo.bar/baz", "bar", "", "bar")

	// query arg takes precedence over header
	f("http://foo.bar/baz?foo=bar", "baz", "", "bar")

	// the value isn't read from the request body
	f("http://foo.bar/baz", "", "foo=bar", "")
	f("http://foo.bar/baz?foo=x", "", "foo=bar", "x")
	f("http://foo.bar/baz", "y", "foo=bar", "y")
}
