package main

import (
	"reflect"
	"testing"
)

func TestParseAuthConfigFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		_, err := parseAuthConfig([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// Empty config
	f(``)

	// Invalid entry
	f(`foobar`)
	f(`foobar: baz`)

	// Empty users
	f(`users: []`)

	// Missing url_prefix
	f(`
users:
- username: foo
`)

	// Invalid url_prefix
	f(`
users:
- username: foo
  url_prefix: bar
`)
	f(`
users:
- username: foo
  url_prefix: ftp://bar
`)
	f(`
users:
- username: foo
  url_prefix: //bar
`)

	// Duplicate users
	f(`
users:
- username: foo
  url_prefix: http://foo.bar
- username: bar
  url_prefix: http://xxx.yyy
- username: foo
  url_prefix: https://sss.sss
`)
}

func TestParseAuthConfigSuccess(t *testing.T) {
	f := func(s string, expectedAuthConfig map[string]*UserInfo) {
		t.Helper()
		m, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		removeMetrics(m)
		if !reflect.DeepEqual(m, expectedAuthConfig) {
			t.Fatalf("unexpected auth config\ngot\n%v\nwant\n%v", m, expectedAuthConfig)
		}
	}

	// Single user
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://aaa:343/bbb
`, map[string]*UserInfo{
		"foo": {
			Username:  "foo",
			Password:  "bar",
			URLPrefix: "http://aaa:343/bbb",
		},
	})

	// Multiple users
	f(`
users:
- username: foo
  url_prefix: http://foo
- username: bar
  url_prefix: https://bar/x///
`, map[string]*UserInfo{
		"foo": {
			Username:  "foo",
			URLPrefix: "http://foo",
		},
		"bar": {
			Username:  "bar",
			URLPrefix: "https://bar/x",
		},
	})
}

func removeMetrics(m map[string]*UserInfo) {
	for _, info := range m {
		info.requests = nil
	}
}
