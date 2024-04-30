package main

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestParseAuthConfigFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			return
		}
		users, err := parseAuthConfigUsers(ac)
		if err == nil {
			t.Fatalf("expecting non-nil error; got %v", users)
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
  url_prefix:
    bar: baz
`)
	f(`
users:
- username: foo
  url_prefix:
  - [foo]
`)

	// Invalid headers
	f(`
users:
- username: foo
  url_prefix: http://foo.bar
  headers: foobar
`)

	// empty url_prefix
	f(`
users:
- username: foo
  url_prefix: []
`)

	// auth_token and username in a single config
	f(`
users:
- auth_token: foo
  username: bbb
  url_prefix: http://foo.bar
`)

	// auth_token and bearer_token in a single config
	f(`
users:
- auth_token: foo
  bearer_token: bbb
  url_prefix: http://foo.bar
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
	// Duplicate users
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://foo.bar
- username: bar
  url_prefix: http://xxx.yyy
- username: foo
  password: bar
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
	f(`
users:
- username: a
  url_map:
  - src_hosts: ["foobar"]
`)

	// Invalid url_prefix in url_map
	f(`
users:
- username: a
  url_map:
  - src_paths: ["/foo/bar"]
    url_prefix: foo.bar
`)
	f(`
users:
- username: a
  url_map:
  - src_hosts: ["foobar"]
    url_prefix: foo.bar
`)

	// empty url_prefix in url_map
	f(`
users:
- username: a
  url_map:
  - src_paths: ['/foo/bar']
    url_prefix: []
`)
	f(`
users:
- username: a
  url_map:
  - src_phosts: ['foobar']
    url_prefix: []
`)

	// Missing src_paths and src_hosts in url_map
	f(`
users:
- username: a
  url_map:
  - url_prefix: http://foobar
`)

	// Invalid regexp in src_paths
	f(`
users:
- username: a
  url_map:
  - src_paths: ['fo[obar']
    url_prefix: http://foobar
`)

	// Invalid regexp in src_hosts
	f(`
users:
- username: a
  url_map:
  - src_hosts: ['fo[obar']
    url_prefix: http://foobar
`)

	// Invalid src_query_args
	f(`
users:
- username: a
  url_map:
  - src_query_args: abc
    url_prefix: http://foobar
`)

	// Invalid src_headers
	f(`
users:
- username: a
  url_map:
  - src_headers: abc
    url_prefix: http://foobar
`)

	// Invalid headers in url_map (missing ':')
	f(`
users:
- username: a
  url_map:
  - src_paths: ['/foobar']
    url_prefix: http://foobar
    headers:
    - foobar
`)
	// Invalid headers in url_map (dictionary instead of array)
	f(`
users:
- username: a
  url_map:
  - src_paths: ['/foobar']
    url_prefix: http://foobar
    headers:
      aaa: bbb
`)
	// Invalid metric label name
	f(`
users:
- username: foo
  url_prefix: http://foo.bar
  metric_labels:
    not-prometheus-compatible: value
`)
}

func TestParseAuthConfigSuccess(t *testing.T) {
	f := func(s string, expectedAuthConfig map[string]*UserInfo) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		m, err := parseAuthConfigUsers(ac)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		removeMetrics(m)
		if err := areEqualConfigs(m, expectedAuthConfig); err != nil {
			t.Fatal(err)
		}
	}

	insecureSkipVerifyTrue := true

	// Single user
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://aaa:343/bbb
  max_concurrent_requests: 5
  tls_insecure_skip_verify: true
`, map[string]*UserInfo{
		getHTTPAuthBasicToken("foo", "bar"): {
			Username:              "foo",
			Password:              "bar",
			URLPrefix:             mustParseURL("http://aaa:343/bbb"),
			MaxConcurrentRequests: 5,
			TLSInsecureSkipVerify: &insecureSkipVerifyTrue,
		},
	})

	// Single user with auth_token
	f(`
users:
- auth_token: foo
  url_prefix: https://aaa:343/bbb
  max_concurrent_requests: 5
  tls_insecure_skip_verify: true
  tls_server_name: "foo.bar"
  tls_ca_file: "foo/bar"
  tls_cert_file: "foo/baz"
  tls_key_file: "foo/foo"
`, map[string]*UserInfo{
		getHTTPAuthToken("foo"): {
			AuthToken:             "foo",
			URLPrefix:             mustParseURL("https://aaa:343/bbb"),
			MaxConcurrentRequests: 5,
			TLSInsecureSkipVerify: &insecureSkipVerifyTrue,
			TLSServerName:         "foo.bar",
			TLSCAFile:             "foo/bar",
			TLSCertFile:           "foo/baz",
			TLSKeyFile:            "foo/foo",
		},
	})

	// Multiple url_prefix entries
	insecureSkipVerifyFalse := false
	discoverBackendIPsTrue := true
	f(`
users:
- username: foo
  password: bar
  url_prefix:
  - http://node1:343/bbb
  - http://srv+node2:343/bbb
  tls_insecure_skip_verify: false
  retry_status_codes: [500, 501]
  load_balancing_policy: first_available
  drop_src_path_prefix_parts: 1
  discover_backend_ips: true
`, map[string]*UserInfo{
		getHTTPAuthBasicToken("foo", "bar"): {
			Username: "foo",
			Password: "bar",
			URLPrefix: mustParseURLs([]string{
				"http://node1:343/bbb",
				"http://srv+node2:343/bbb",
			}),
			TLSInsecureSkipVerify:  &insecureSkipVerifyFalse,
			RetryStatusCodes:       []int{500, 501},
			LoadBalancingPolicy:    "first_available",
			DropSrcPathPrefixParts: intp(1),
			DiscoverBackendIPs:     &discoverBackendIPsTrue,
		},
	})

	// Multiple users
	f(`
users:
- username: foo
  url_prefix: http://foo
- username: bar
  url_prefix: https://bar/x/
`, map[string]*UserInfo{
		getHTTPAuthBasicToken("foo", ""): {
			Username:  "foo",
			URLPrefix: mustParseURL("http://foo"),
		},
		getHTTPAuthBasicToken("bar", ""): {
			Username:  "bar",
			URLPrefix: mustParseURL("https://bar/x/"),
		},
	})

	// non-empty URLMap
	sharedUserInfo := &UserInfo{
		BearerToken: "foo",
		URLMaps: []URLMap{
			{
				SrcPaths:  getRegexs([]string{"/api/v1/query", "/api/v1/query_range", "/api/v1/label/[^./]+/.+"}),
				URLPrefix: mustParseURL("http://vmselect/select/0/prometheus"),
			},
			{
				SrcHosts: getRegexs([]string{"foo\\.bar", "baz:1234"}),
				SrcPaths: getRegexs([]string{"/api/v1/write"}),
				SrcQueryArgs: []*QueryArg{
					mustNewQueryArg("foo=b.+ar"),
					mustNewQueryArg("baz=~.*x=y.+"),
				},
				SrcHeaders: []*Header{
					mustNewHeader("'TenantID: 345'"),
				},
				URLPrefix: mustParseURLs([]string{
					"http://vminsert1/insert/0/prometheus",
					"http://vminsert2/insert/0/prometheus",
				}),
				HeadersConf: HeadersConf{
					RequestHeaders: []*Header{
						mustNewHeader("'foo: bar'"),
						mustNewHeader("'xxx: y'"),
					},
				},
			},
		},
	}
	f(`
users:
- bearer_token: foo
  url_map:
  - src_paths: ["/api/v1/query","/api/v1/query_range","/api/v1/label/[^./]+/.+"]
    url_prefix: http://vmselect/select/0/prometheus
  - src_paths: ["/api/v1/write"]
    src_hosts: ["foo\\.bar", "baz:1234"]
    src_query_args: ['foo=b.+ar', 'baz=~.*x=y.+']
    src_headers: ['TenantID: 345']
    url_prefix: ["http://vminsert1/insert/0/prometheus","http://vminsert2/insert/0/prometheus"]
    headers:
    - "foo: bar"
    - "xxx: y"
`, map[string]*UserInfo{
		getHTTPAuthBearerToken("foo"):    sharedUserInfo,
		getHTTPAuthBasicToken("foo", ""): sharedUserInfo,
	})

	// Multiple users with the same name - this should work, since these users have different passwords
	f(`
users:
- username: foo-same
  password: baz
  url_prefix: http://foo
- username: foo-same
  password: bar
  url_prefix: https://bar/x
`, map[string]*UserInfo{
		getHTTPAuthBasicToken("foo-same", "baz"): {
			Username:  "foo-same",
			Password:  "baz",
			URLPrefix: mustParseURL("http://foo"),
		},
		getHTTPAuthBasicToken("foo-same", "bar"): {
			Username:  "foo-same",
			Password:  "bar",
			URLPrefix: mustParseURL("https://bar/x"),
		},
	})

	// with default url
	f(`
users:
- bearer_token: foo
  url_map:
  - src_paths: ["/api/v1/query","/api/v1/query_range","/api/v1/label/[^./]+/.+"]
    url_prefix: http://vmselect/select/0/prometheus
  - src_paths: ["/api/v1/write"]
    url_prefix: ["http://vminsert1/insert/0/prometheus","http://vminsert2/insert/0/prometheus"]
    headers:
    - "foo: bar"
    - "xxx: y"
  default_url:
  - http://default1/select/0/prometheus
  - http://default2/select/0/prometheus
`, map[string]*UserInfo{
		getHTTPAuthBearerToken("foo"): {
			BearerToken: "foo",
			URLMaps: []URLMap{
				{
					SrcPaths:  getRegexs([]string{"/api/v1/query", "/api/v1/query_range", "/api/v1/label/[^./]+/.+"}),
					URLPrefix: mustParseURL("http://vmselect/select/0/prometheus"),
				},
				{
					SrcPaths: getRegexs([]string{"/api/v1/write"}),
					URLPrefix: mustParseURLs([]string{
						"http://vminsert1/insert/0/prometheus",
						"http://vminsert2/insert/0/prometheus",
					}),
					HeadersConf: HeadersConf{
						RequestHeaders: []*Header{
							mustNewHeader("'foo: bar'"),
							mustNewHeader("'xxx: y'"),
						},
					},
				},
			},
			DefaultURL: mustParseURLs([]string{
				"http://default1/select/0/prometheus",
				"http://default2/select/0/prometheus",
			}),
		},
		getHTTPAuthBasicToken("foo", ""): {
			BearerToken: "foo",
			URLMaps: []URLMap{
				{
					SrcPaths:  getRegexs([]string{"/api/v1/query", "/api/v1/query_range", "/api/v1/label/[^./]+/.+"}),
					URLPrefix: mustParseURL("http://vmselect/select/0/prometheus"),
				},
				{
					SrcPaths: getRegexs([]string{"/api/v1/write"}),
					URLPrefix: mustParseURLs([]string{
						"http://vminsert1/insert/0/prometheus",
						"http://vminsert2/insert/0/prometheus",
					}),
					HeadersConf: HeadersConf{
						RequestHeaders: []*Header{
							mustNewHeader("'foo: bar'"),
							mustNewHeader("'xxx: y'"),
						},
					},
				},
			},
			DefaultURL: mustParseURLs([]string{
				"http://default1/select/0/prometheus",
				"http://default2/select/0/prometheus",
			}),
		},
	})

	// With metric_labels
	f(`
users:
- username: foo-same
  password: baz
  url_prefix: http://foo
  metric_labels:
    dc: eu
    team: dev
- username: foo-same
  password: bar
  url_prefix: https://bar/x
  metric_labels:
    backend_env: test
    team: accounting
`, map[string]*UserInfo{
		getHTTPAuthBasicToken("foo-same", "baz"): {
			Username:  "foo-same",
			Password:  "baz",
			URLPrefix: mustParseURL("http://foo"),
			MetricLabels: map[string]string{
				"dc":   "eu",
				"team": "dev",
			},
		},
		getHTTPAuthBasicToken("foo-same", "bar"): {
			Username:  "foo-same",
			Password:  "bar",
			URLPrefix: mustParseURL("https://bar/x"),
			MetricLabels: map[string]string{
				"backend_env": "test",
				"team":        "accounting",
			},
		},
	})
}

func TestParseAuthConfigPassesTLSVerificationConfig(t *testing.T) {
	c := `
users:
- username: foo
  password: bar
  url_prefix: https://aaa/bbb
  max_concurrent_requests: 5
  tls_insecure_skip_verify: true

unauthorized_user:
  url_prefix: http://aaa:343/bbb
  max_concurrent_requests: 5
  tls_insecure_skip_verify: false
`

	ac, err := parseAuthConfig([]byte(c))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	m, err := parseAuthConfigUsers(ac)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	ui := m[getHTTPAuthBasicToken("foo", "bar")]
	if !isSetBool(ui.TLSInsecureSkipVerify, true) {
		t.Fatalf("unexpected TLSInsecureSkipVerify value for user foo")
	}

	if !isSetBool(ac.UnauthorizedUser.TLSInsecureSkipVerify, false) {
		t.Fatalf("unexpected TLSInsecureSkipVerify value for unauthorized_user")
	}
}

func TestUserInfoGetMetricLabels(t *testing.T) {
	t.Run("empty-labels", func(t *testing.T) {
		ui := &UserInfo{
			Username: "user1",
		}
		labels, err := ui.getMetricLabels()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		labelsExpected := `{username="user1"}`
		if labels != labelsExpected {
			t.Fatalf("unexpected labels; got %s; want %s", labels, labelsExpected)
		}
	})
	t.Run("non-empty-username", func(t *testing.T) {
		ui := &UserInfo{
			Username: "user1",
			MetricLabels: map[string]string{
				"env":        "prod",
				"datacenter": "dc1",
			},
		}
		labels, err := ui.getMetricLabels()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		labelsExpected := `{datacenter="dc1",env="prod",username="user1"}`
		if labels != labelsExpected {
			t.Fatalf("unexpected labels; got %s; want %s", labels, labelsExpected)
		}
	})
	t.Run("non-empty-name", func(t *testing.T) {
		ui := &UserInfo{
			Name:        "user1",
			BearerToken: "abc",
			MetricLabels: map[string]string{
				"env":        "prod",
				"datacenter": "dc1",
			},
		}
		labels, err := ui.getMetricLabels()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		labelsExpected := `{datacenter="dc1",env="prod",username="user1"}`
		if labels != labelsExpected {
			t.Fatalf("unexpected labels; got %s; want %s", labels, labelsExpected)
		}
	})
	t.Run("non-empty-bearer-token", func(t *testing.T) {
		ui := &UserInfo{
			BearerToken: "abc",
			MetricLabels: map[string]string{
				"env":        "prod",
				"datacenter": "dc1",
			},
		}
		labels, err := ui.getMetricLabels()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		labelsExpected := `{datacenter="dc1",env="prod",username="bearer_token:hash:44BC2CF5AD770999"}`
		if labels != labelsExpected {
			t.Fatalf("unexpected labels; got %s; want %s", labels, labelsExpected)
		}
	})
	t.Run("invalid-label", func(t *testing.T) {
		ui := &UserInfo{
			Username: "foo",
			MetricLabels: map[string]string{
				",{": "aaaa",
			},
		}
		_, err := ui.getMetricLabels()
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	})
}

func isSetBool(boolP *bool, expectedValue bool) bool {
	if boolP == nil {
		return false
	}
	return *boolP == expectedValue
}

func TestGetLeastLoadedBackendURL(t *testing.T) {
	up := mustParseURLs([]string{
		"http://node1:343",
		"http://node2:343",
		"http://node3:343",
	})
	up.loadBalancingPolicy = "least_loaded"

	fn := func(ns ...int) {
		t.Helper()
		bus := up.bus.Load()
		pbus := *bus
		for i, b := range pbus {
			got := int(b.concurrentRequests.Load())
			exp := ns[i]
			if got != exp {
				t.Fatalf("expected %q to have %d concurrent requests; got %d instead", b.url, exp, got)
			}
		}
	}

	up.getBackendURL()
	fn(0, 1, 0)
	up.getBackendURL()
	fn(0, 1, 1)
	up.getBackendURL()
	fn(1, 1, 1)

	up.getBackendURL()
	up.getBackendURL()
	fn(1, 2, 2)

	bus := up.bus.Load()
	pbus := *bus
	pbus[0].concurrentRequests.Add(2)
	pbus[2].concurrentRequests.Add(5)
	fn(3, 2, 7)

	up.getBackendURL()
	fn(3, 3, 7)

	up.getBackendURL()
	fn(3, 4, 7)

	up.getBackendURL()
	fn(4, 4, 7)

	up.getBackendURL()
	fn(5, 4, 7)

	up.getBackendURL()
	fn(5, 5, 7)

	up.getBackendURL()
	fn(6, 5, 7)

	up.getBackendURL()
	fn(6, 6, 7)

	up.getBackendURL()
	up.getBackendURL()
	fn(7, 7, 7)
}

func getRegexs(paths []string) []*Regex {
	var sps []*Regex
	for _, path := range paths {
		sps = append(sps, mustNewRegex(path))
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

func mustParseURL(u string) *URLPrefix {
	return mustParseURLs([]string{u})
}

func mustParseURLs(us []string) *URLPrefix {
	bus := make([]*backendURL, len(us))
	urls := make([]*url.URL, len(us))
	for i, u := range us {
		pu, err := url.Parse(u)
		if err != nil {
			panic(fmt.Errorf("BUG: cannot parse %q: %w", u, err))
		}
		bus[i] = &backendURL{
			url: pu,
		}
		urls[i] = pu
	}
	up := &URLPrefix{}
	if len(us) == 1 {
		up.vOriginal = us[0]
	} else {
		up.vOriginal = us
	}
	up.bus.Store(&bus)
	up.busOriginal = urls
	return up
}

func intp(n int) *int {
	return &n
}

func mustNewRegex(s string) *Regex {
	var re Regex
	if err := yaml.Unmarshal([]byte(s), &re); err != nil {
		logger.Panicf("cannot unmarshal regex %q: %s", s, err)
	}
	return &re
}

func mustNewQueryArg(s string) *QueryArg {
	var qa QueryArg
	if err := yaml.Unmarshal([]byte(s), &qa); err != nil {
		logger.Panicf("cannot unmarshal query arg filter %q: %s", s, err)
	}
	return &qa
}

func mustNewHeader(s string) *Header {
	var h Header
	if err := yaml.Unmarshal([]byte(s), &h); err != nil {
		logger.Panicf("cannot unmarshal header filter %q: %s", s, err)
	}
	return &h
}
