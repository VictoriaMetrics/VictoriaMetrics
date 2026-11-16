package main

import (
	"net/http"
	"testing"
)

func TestNewAuthConfig(t *testing.T) {
	f := func(username, password, bearerToken, headers string, headersExpected map[string]string) {
		t.Helper()

		ac, err := newAuthConfig(username, password, bearerToken, headers)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		req, err := http.NewRequest(http.MethodGet, "http://foo", nil)
		if err != nil {
			t.Fatalf("cannot create request: %s", err)
		}
		if err := ac.SetHeaders(req, true); err != nil {
			t.Fatalf("cannot set headers: %s", err)
		}
		if len(req.Header) != len(headersExpected) {
			t.Fatalf("unexpected number of headers; got %v; want %v", req.Header, headersExpected)
		}
		for k, v := range headersExpected {
			if got := req.Header.Get(k); got != v {
				t.Fatalf("unexpected value for header %q; got %q; want %q", k, got, v)
			}
		}
	}

	// no auth
	f("", "", "", "", map[string]string{})

	// basic auth
	f("foo", "bar", "", "", map[string]string{
		"Authorization": "Basic Zm9vOmJhcg==",
	})

	// bearer token
	f("", "", "secret", "", map[string]string{
		"Authorization": "Bearer secret",
	})

	// custom headers
	f("", "", "", "X-Foo: bar^^x-baz:  qux ", map[string]string{
		"X-Foo": "bar",
		"X-Baz": "qux",
	})
}

func TestNewAuthConfigFailure(t *testing.T) {
	f := func(username, password, bearerToken, headers string) {
		t.Helper()

		if _, err := newAuthConfig(username, password, bearerToken, headers); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// basic auth and bearer token at the same time
	f("foo", "bar", "secret", "")

	// header without ':'
	f("", "", "", "X-Foo")
}
