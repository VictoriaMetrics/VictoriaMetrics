package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

func TestDropPrefixParts(t *testing.T) {
	f := func(path string, parts int, expectedResult string) {
		t.Helper()

		result := dropPrefixParts(path, parts)
		if result != expectedResult {
			t.Fatalf("unexpected result; got %q; want %q", result, expectedResult)
		}
	}

	f("", 0, "")
	f("", 1, "")
	f("", 10, "")
	f("foo", 0, "foo")
	f("foo", -1, "foo")
	f("foo", 1, "")

	f("/foo", 0, "/foo")
	f("/foo/bar", 0, "/foo/bar")
	f("/foo/bar/baz", 0, "/foo/bar/baz")

	f("foo", 0, "foo")
	f("foo/bar", 0, "foo/bar")
	f("foo/bar/baz", 0, "foo/bar/baz")

	f("/foo/", 0, "/foo/")
	f("/foo/bar/", 0, "/foo/bar/")
	f("/foo/bar/baz/", 0, "/foo/bar/baz/")

	f("/foo", 1, "")
	f("/foo/bar", 1, "/bar")
	f("/foo/bar/baz", 1, "/bar/baz")

	f("foo", 1, "")
	f("foo/bar", 1, "/bar")
	f("foo/bar/baz", 1, "/bar/baz")

	f("/foo/", 1, "/")
	f("/foo/bar/", 1, "/bar/")
	f("/foo/bar/baz/", 1, "/bar/baz/")

	f("/foo", 2, "")
	f("/foo/bar", 2, "")
	f("/foo/bar/baz", 2, "/baz")

	f("foo", 2, "")
	f("foo/bar", 2, "")
	f("foo/bar/baz", 2, "/baz")

	f("/foo/", 2, "")
	f("/foo/bar/", 2, "/")
	f("/foo/bar/baz/", 2, "/baz/")

	f("/foo", 3, "")
	f("/foo/bar", 3, "")
	f("/foo/bar/baz", 3, "")

	f("foo", 3, "")
	f("foo/bar", 3, "")
	f("foo/bar/baz", 3, "")

	f("/foo/", 3, "")
	f("/foo/bar/", 3, "")
	f("/foo/bar/baz/", 3, "/")

	f("/foo/", 4, "")
	f("/foo/bar/", 4, "")
	f("/foo/bar/baz/", 4, "")
}

func TestCreateTargetURLSuccess(t *testing.T) {
	f := func(ui *UserInfo, requestURI, expectedTarget, expectedRequestHeaders, expectedResponseHeaders string,
		expectedRetryStatusCodes []int, expectedLoadBalancingPolicy string, expectedDropSrcPathPrefixParts int) {
		t.Helper()
		if err := ui.initURLs(); err != nil {
			t.Fatalf("cannot initialize urls inside UserInfo: %s", err)
		}
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc := ui.getURLPrefixAndHeaders(u, nil)
		if up == nil {
			t.Fatalf("cannot match available backend: %s", err)
		}
		bu := up.getBackendURL()
		target := mergeURLs(bu.url, u, up.dropSrcPathPrefixParts)
		bu.put()

		gotTarget := target.String()
		if gotTarget != expectedTarget {
			t.Fatalf("unexpected target; \ngot:\n%q;\nwant:\n%q", gotTarget, expectedTarget)
		}
		if s := headersToString(hc.RequestHeaders); s != expectedRequestHeaders {
			t.Fatalf("unexpected request headers; got %q; want %q", s, expectedRequestHeaders)
		}
		if s := headersToString(hc.ResponseHeaders); s != expectedResponseHeaders {
			t.Fatalf("unexpected response headers; got %q; want %q", s, expectedResponseHeaders)
		}
		if !reflect.DeepEqual(up.retryStatusCodes, expectedRetryStatusCodes) {
			t.Fatalf("unexpected retryStatusCodes; got %d; want %d", up.retryStatusCodes, expectedRetryStatusCodes)
		}
		if up.loadBalancingPolicy != expectedLoadBalancingPolicy {
			t.Fatalf("unexpected loadBalancingPolicy; got %q; want %q", up.loadBalancingPolicy, expectedLoadBalancingPolicy)
		}
		if up.dropSrcPathPrefixParts != expectedDropSrcPathPrefixParts {
			t.Fatalf("unexpected dropSrcPathPrefixParts; got %d; want %d", up.dropSrcPathPrefixParts, expectedDropSrcPathPrefixParts)
		}
	}
	// Simple routing with `url_prefix`
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "", "http://foo.bar/.", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
		HeadersConf: HeadersConf{
			RequestHeaders: []*Header{
				mustNewHeader("'bb: aaa'"),
			},
			ResponseHeaders: []*Header{
				mustNewHeader("'x: y'"),
			},
		},
		RetryStatusCodes:       []int{503, 501},
		LoadBalancingPolicy:    "first_available",
		DropSrcPathPrefixParts: intp(2),
	}, "/a/b/c", "http://foo.bar/c", `bb: aaa`, `x: y`, []int{503, 501}, "first_available", 2)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar/federate"),
	}, "/", "http://foo.bar/federate", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "a/b?c=d", "http://foo.bar/a/b?c=d", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/z", "https://sss:3894/x/y/z", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/../../aaa", "https://sss:3894/x/y/aaa", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s%2F..%2Fd", "", "", nil, "least_loaded", 0)

	// Complex routing with `url_map`
	ui := &UserInfo{
		URLMaps: []URLMap{
			{
				SrcHosts: getRegexs([]string{"host42"}),
				SrcPaths: getRegexs([]string{"/vmsingle/api/v1/query"}),
				SrcQueryArgs: []*QueryArg{
					mustNewQueryArg("db=foo"),
				},
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
				HeadersConf: HeadersConf{
					RequestHeaders: []*Header{
						mustNewHeader("'xx: aa'"),
						mustNewHeader("'yy: asdf'"),
					},
					ResponseHeaders: []*Header{
						mustNewHeader("'qwe: rty'"),
					},
				},
				RetryStatusCodes:       []int{503, 500, 501},
				LoadBalancingPolicy:    "first_available",
				DropSrcPathPrefixParts: intp(1),
			},
			{
				SrcPaths:               getRegexs([]string{"/api/v1/write"}),
				URLPrefix:              mustParseURL("http://vminsert/0/prometheus"),
				RetryStatusCodes:       []int{},
				DropSrcPathPrefixParts: intp(0),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
		HeadersConf: HeadersConf{
			RequestHeaders: []*Header{
				mustNewHeader("'bb: aaa'"),
			},
			ResponseHeaders: []*Header{
				mustNewHeader("'x: y'"),
			},
		},
		RetryStatusCodes:       []int{502},
		DropSrcPathPrefixParts: intp(2),
	}
	f(ui, "http://host42/vmsingle/api/v1/query?query=up&db=foo", "http://vmselect/0/prometheus/api/v1/query?db=foo&query=up",
		"xx: aa\nyy: asdf", "qwe: rty", []int{503, 500, 501}, "first_available", 1)
	f(ui, "http://host123/vmsingle/api/v1/query?query=up", "http://default-server/v1/query?query=up",
		"bb: aaa", "x: y", []int{502}, "least_loaded", 2)
	f(ui, "https://foo-host/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "", "", []int{}, "least_loaded", 0)
	f(ui, "https://foo-host/foo/bar/api/v1/query_range", "http://default-server/api/v1/query_range", "bb: aaa", "x: y", []int{502}, "least_loaded", 2)

	// Complex routing regexp paths in `url_map`
	ui = &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getRegexs([]string{"/api/v1/query(_range)?", "/api/v1/label/[^/]+/values"}),
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
			},
			{
				SrcPaths:  getRegexs([]string{"/api/v1/write"}),
				URLPrefix: mustParseURL("http://vminsert/0/prometheus"),
			},
			{
				SrcHosts:  getRegexs([]string{"vmui\\..+"}),
				URLPrefix: mustParseURL("http://vmui.host:1234/vmui/"),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
	}
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up", "", "", nil, "least_loaded", 0)
	f(ui, "/api/v1/query_range?query=up", "http://vmselect/0/prometheus/api/v1/query_range?query=up", "", "", nil, "least_loaded", 0)
	f(ui, "/api/v1/label/foo/values", "http://vmselect/0/prometheus/api/v1/label/foo/values", "", "", nil, "least_loaded", 0)
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "", "", nil, "least_loaded", 0)
	f(ui, "/api/v1/foo/bar", "http://default-server/api/v1/foo/bar", "", "", nil, "least_loaded", 0)
	f(ui, "https://vmui.foobar.com/a/b?c=d", "http://vmui.host:1234/vmui/a/b?c=d", "", "", nil, "least_loaded", 0)

	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=dev"),
	}, "/api/v1/query", "http://foo.bar/api/v1/query?extra_label=team=dev", "", "", nil, "least_loaded", 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=mobile"),
	}, "/api/v1/query?extra_label=team=dev", "http://foo.bar/api/v1/query?extra_label=team%3Dmobile", "", "", nil, "least_loaded", 0)

	// Complex routing regexp query args in `url_map`
	ui = &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths: getRegexs([]string{"/api/v1/query"}),
				SrcQueryArgs: []*QueryArg{
					mustNewQueryArg(`query=~.*{.*env="dev".*}*.`),
				},
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
			},
			{
				SrcPaths: getRegexs([]string{"/api/v1/query"}),
				SrcQueryArgs: []*QueryArg{
					mustNewQueryArg(`query=~.*{.*env="prod".*}.*`),
				},
				URLPrefix: mustParseURL("http://vmselect/1/prometheus"),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
	}
	f(ui, `/api/v1/query?query=up{env="prod"}`, `http://vmselect/1/prometheus/api/v1/query?query=up%7Benv%3D%22prod%22%7D`, "", "", nil, "least_loaded", 0)
	f(ui, `/api/v1/query?query=up{foo="bar",env="dev",pod!=""}`, `http://vmselect/0/prometheus/api/v1/query?query=up%7Bfoo%3D%22bar%22%2Cenv%3D%22dev%22%2Cpod%21%3D%22%22%7D`, "", "", nil, "least_loaded", 0)
	f(ui, `/api/v1/query?query=up{foo="bar"}`, `http://default-server/api/v1/query?query=up%7Bfoo%3D%22bar%22%7D`, "", "", nil, "least_loaded", 0)

	customResolver := &fakeResolver{
		Resolver: &net.Resolver{},
		lookupSRVResults: map[string][]*net.SRV{
			"vmselect": {
				{
					Target: "10.6.142.50",
					Port:   8481,
				},
				{
					Target: "10.6.142.51",
					Port:   8481,
				},
			},
		},
		lookupIPAddrResults: map[string][]net.IPAddr{
			"vminsert": {
				{
					IP: net.ParseIP("10.6.142.52"),
				},
			},
		},
	}
	netutil.Resolver = customResolver

	// Discover backendURL
	allowed := true
	ui = &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getRegexs([]string{"/select/.+"}),
				URLPrefix: mustParseURL("http://srv+vmselect"),
			},
			{
				SrcPaths:  getRegexs([]string{"/insert/.+"}),
				URLPrefix: mustParseURL("http://vminsert:8480"),
			},
		},
		DiscoverBackendIPs: &allowed,
		URLPrefix:          mustParseURL("http://non-exist-dns-addr"),
	}
	f(ui, `/select/0/prometheus/api/v1/query?query=up`, "http://10.6.142.51:8481/select/0/prometheus/api/v1/query?query=up", "", "", nil, "least_loaded", 0)
	// url_prefix counter will be reset, still go to 10.6.142.51
	f(ui, `/select/0/prometheus/api/v1/query?query=up`, "http://10.6.142.51:8481/select/0/prometheus/api/v1/query?query=up", "", "", nil, "least_loaded", 0)
	f(ui, `/insert/0/prometheus/api/v1/write`, "http://10.6.142.52:8480/insert/0/prometheus/api/v1/write", "", "", nil, "least_loaded", 0)
	// unsuccessful dns resolve
	f(ui, `/test`, "http://non-exist-dns-addr/test", "", "", nil, "least_loaded", 0)
}

func TestCreateTargetURLFailure(t *testing.T) {
	f := func(ui *UserInfo, requestURI string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc := ui.getURLPrefixAndHeaders(u, nil)
		if up != nil {
			t.Fatalf("unexpected non-empty up=%#v", up)
		}
		if hc.RequestHeaders != nil {
			t.Fatalf("unexpected non-empty request headers: %s", headersToString(hc.RequestHeaders))
		}
		if hc.ResponseHeaders != nil {
			t.Fatalf("unexpected non-empty response headers: %s", headersToString(hc.ResponseHeaders))
		}
	}
	f(&UserInfo{}, "/foo/bar")
	f(&UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getRegexs([]string{"/api/v1/query"}),
				URLPrefix: mustParseURL("http://foobar/baz"),
			},
		},
	}, "/api/v1/write")
}

func headersToString(hs []*Header) string {
	a := make([]string, len(hs))
	for i, h := range hs {
		a[i] = fmt.Sprintf("%s: %s", h.Name, h.Value)
	}
	return strings.Join(a, "\n")
}

type fakeResolver struct {
	Resolver            *net.Resolver
	lookupSRVResults    map[string][]*net.SRV
	lookupIPAddrResults map[string][]net.IPAddr
}

func (r *fakeResolver) LookupSRV(_ context.Context, _, _, name string) (string, []*net.SRV, error) {
	if results, ok := r.lookupSRVResults[name]; ok {
		return name, results, nil
	}

	return name, nil, fmt.Errorf("no srv results found for host: %s", name)
}

func (r *fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	if results, ok := r.lookupIPAddrResults[host]; ok {
		return results, nil
	}

	return nil, fmt.Errorf("no results found for host: %s", host)
}

func (r *fakeResolver) LookupMX(_ context.Context, _ string) ([]*net.MX, error) {
	return nil, nil
}
