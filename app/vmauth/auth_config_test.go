package main

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"testing"

	"gopkg.in/yaml.v2"
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
	f(`
users:
- username: foo
  url_prefix: [bar]
`)

	// Username and bearer_token in a single config
	f(`
users:
- username: foo
  bearer_token: bbb
  url_prefix: http://foo.bar
`)

	// Bearer_token and password in a single config
	f(`
users:
- password: foo
  bearer_token: bbb
  url_prefix: http://foo.bar
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

	// Duplicate bearer_tokens
	f(`
users:
- bearer_token: foo
  url_prefix: http://foo.bar
- username: bar
  url_prefix: http://xxx.yyy
- bearer_token: foo
  url_prefix: https://sss.sss
`)

	// Missing url_prefix in url_map
	f(`
users:
- username: a
  url_map:
  - src_paths: ["/foo/bar"]
`)

	// Invalid url_prefix in url_map
	f(`
users:
- username: a
  url_map:
  - src_paths: ["/foo/bar"]
    url_prefix: foo.bar
`)

	// Missing src_paths in url_map
	f(`
users:
- username: a
  url_map:
  - url_prefix: http://foobar
`)

	// Invalid regexp in src_path.
	f(`
users:
- username: a
  url_map:
  - src_paths: ['fo[obar']
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
		if err := areEqualConfigs(m, expectedAuthConfig); err != nil {
			t.Fatal(err)
		}
	}

	// Single user
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://aaa:343/bbb
`, map[string]*UserInfo{
		getAuthToken("", "foo", "bar"): {
			Username:  "foo",
			Password:  "bar",
			URLPrefix: mustParseURL("http://aaa:343/bbb"),
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
		getAuthToken("", "foo", ""): {
			Username:  "foo",
			URLPrefix: mustParseURL("http://foo"),
		},
		getAuthToken("", "bar", ""): {
			Username:  "bar",
			URLPrefix: mustParseURL("https://bar/x"),
		},
	})

	// non-empty URLMap
	f(`
users:
- bearer_token: foo
  url_map:
  - src_paths: ["/api/v1/query","/api/v1/query_range","/api/v1/label/[^./]+/.+"]
    url_prefix: http://vmselect/select/0/prometheus
  - src_paths: ["/api/v1/write"]
    url_prefix: http://vminsert/insert/0/prometheus
`, map[string]*UserInfo{
		getAuthToken("foo", "", ""): {
			BearerToken: "foo",
			URLMap: []URLMap{
				{
					SrcPaths:  getSrcPaths([]string{"/api/v1/query", "/api/v1/query_range", "/api/v1/label/[^./]+/.+"}),
					URLPrefix: mustParseURL("http://vmselect/select/0/prometheus"),
				},
				{
					SrcPaths:  getSrcPaths([]string{"/api/v1/write"}),
					URLPrefix: mustParseURL("http://vminsert/insert/0/prometheus"),
				},
			},
		},
	})
}

func getSrcPaths(paths []string) []*SrcPath {
	var sps []*SrcPath
	for _, path := range paths {
		sps = append(sps, &SrcPath{
			sOriginal: path,
			re:        regexp.MustCompile("^(?:" + path + ")$"),
		})
	}
	return sps
}

func removeMetrics(m map[string]*UserInfo) {
	for _, info := range m {
		info.requests = nil
	}
}

func areEqualConfigs(a, b map[string]*UserInfo) error {
	aData, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("cannot marshal a: %w", err)
	}
	bData, err := yaml.Marshal(b)
	if err != nil {
		return fmt.Errorf("cannot marshal b: %w", err)
	}
	if !bytes.Equal(aData, bData) {
		return fmt.Errorf("unexpected configs;\ngot\n%s\nwant\n%s", aData, bData)
	}
	return nil
}

func mustParseURL(u string) *yamlURL {
	pu, err := url.Parse(u)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot parse %q: %w", u, err))
	}
	return &yamlURL{
		u: pu,
	}
}
