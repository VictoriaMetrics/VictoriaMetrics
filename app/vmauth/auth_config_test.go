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
	f(`
users:
- username: foo
  url_prefix: http:///bar
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

	// Missing url_prefix in url_map
	f(`
users:
- username: a
  url_map:
  - src_paths: ["/foo/bar"]
`)

	// Missing src_paths in url_map
	f(`
users:
- username: a
  url_map:
  - url_prefix: http://foobar
`)

	// src_path not starting with `/`
	f(`
users:
- username: a
  url_map:
  - src_paths: [foobar]
    url_prefix: http://foobar
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

	// non-empty URLMap
	f(`
users:
- username: foo
  url_map:
  - src_paths: ["/api/v1/query","/api/v1/query_range"]
    url_prefix: http://vmselect/select/0/prometheus
  - src_paths: ["/api/v1/write"]
    url_prefix: http://vminsert/insert/0/prometheus
`, map[string]*UserInfo{
		"foo": {
			Username: "foo",
			URLMap: []URLMap{
				{
					SrcPaths:  []string{"/api/v1/query", "/api/v1/query_range"},
					URLPrefix: "http://vmselect/select/0/prometheus",
				},
				{
					SrcPaths:  []string{"/api/v1/write"},
					URLPrefix: "http://vminsert/insert/0/prometheus",
				},
			},
		},
	})
}

func removeMetrics(m map[string]*UserInfo) {
	for _, info := range m {
		info.requests = nil
	}
}
