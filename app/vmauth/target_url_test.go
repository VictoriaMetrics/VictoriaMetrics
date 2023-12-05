package main

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
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
		expectedRetryStatusCodes []int, expectedDropSrcPathPrefixParts int) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc, retryStatusCodes, dropSrcPathPrefixParts := ui.getURLPrefixAndHeaders(u)
		if up == nil {
			t.Fatalf("cannot determie backend: %s", err)
		}
		bu := up.getLeastLoadedBackendURL()
		target := mergeURLs(bu.url, u, dropSrcPathPrefixParts)
		bu.put()
		if target.String() != expectedTarget {
			t.Fatalf("unexpected target; got %q; want %q", target, expectedTarget)
		}
		headersStr := fmt.Sprintf("%q", hc.RequestHeaders)
		if headersStr != expectedRequestHeaders {
			t.Fatalf("unexpected request headers; got %s; want %s", headersStr, expectedRequestHeaders)
		}
		if !reflect.DeepEqual(retryStatusCodes, expectedRetryStatusCodes) {
			t.Fatalf("unexpected retryStatusCodes; got %d; want %d", retryStatusCodes, expectedRetryStatusCodes)
		}
		if dropSrcPathPrefixParts != expectedDropSrcPathPrefixParts {
			t.Fatalf("unexpected dropSrcPathPrefixParts; got %d; want %d", dropSrcPathPrefixParts, expectedDropSrcPathPrefixParts)
		}
	}
	// Simple routing with `url_prefix`
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "", "http://foo.bar/.", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
		HeadersConf: HeadersConf{
			RequestHeaders: []Header{{
				Name:  "bb",
				Value: "aaa",
			}},
		},
		RetryStatusCodes:       []int{503, 501},
		DropSrcPathPrefixParts: 2,
	}, "/a/b/c", "http://foo.bar/c", `[{"bb" "aaa"}]`, `[]`, []int{503, 501}, 2)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar/federate"),
	}, "/", "http://foo.bar/federate", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "a/b?c=d", "http://foo.bar/a/b?c=d", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/z", "https://sss:3894/x/y/z", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/../../aaa", "https://sss:3894/x/y/aaa", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s%2F..%2Fd", "[]", "[]", nil, 0)

	// Complex routing with `url_map`
	ui := &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/vmsingle/api/v1/query"}),
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
				HeadersConf: HeadersConf{
					RequestHeaders: []Header{
						{
							Name:  "xx",
							Value: "aa",
						},
						{
							Name:  "yy",
							Value: "asdf",
						},
					},
					ResponseHeaders: []Header{
						{
							Name:  "qwe",
							Value: "rty",
						},
					},
				},
				RetryStatusCodes:       []int{503, 500, 501},
				DropSrcPathPrefixParts: 1,
			},
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/write"}),
				URLPrefix: mustParseURL("http://vminsert/0/prometheus"),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
		HeadersConf: HeadersConf{
			RequestHeaders: []Header{{
				Name:  "bb",
				Value: "aaa",
			}},
			ResponseHeaders: []Header{{
				Name:  "x",
				Value: "y",
			}},
		},
		RetryStatusCodes:       []int{502},
		DropSrcPathPrefixParts: 2,
	}
	f(ui, "/vmsingle/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up", `[{"xx" "aa"} {"yy" "asdf"}]`, `[{"qwe" "rty"}]`, []int{503, 500, 501}, 1)
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "[]", "[]", nil, 0)
	f(ui, "/foo/bar/api/v1/query_range", "http://default-server/api/v1/query_range", `[{"bb" "aaa"}]`, `[{"x" "y"}]`, []int{502}, 2)

	// Complex routing regexp paths in `url_map`
	ui = &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/query(_range)?", "/api/v1/label/[^/]+/values"}),
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
			},
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/write"}),
				URLPrefix: mustParseURL("http://vminsert/0/prometheus"),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
	}
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up", "[]", "[]", nil, 0)
	f(ui, "/api/v1/query_range?query=up", "http://vmselect/0/prometheus/api/v1/query_range?query=up", "[]", "[]", nil, 0)
	f(ui, "/api/v1/label/foo/values", "http://vmselect/0/prometheus/api/v1/label/foo/values", "[]", "[]", nil, 0)
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "[]", "[]", nil, 0)
	f(ui, "/api/v1/foo/bar", "http://default-server/api/v1/foo/bar", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=dev"),
	}, "/api/v1/query", "http://foo.bar/api/v1/query?extra_label=team=dev", "[]", "[]", nil, 0)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=mobile"),
	}, "/api/v1/query?extra_label=team=dev", "http://foo.bar/api/v1/query?extra_label=team%3Dmobile", "[]", "[]", nil, 0)
}

func TestCreateTargetURLFailure(t *testing.T) {
	f := func(ui *UserInfo, requestURI string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc, retryStatusCodes, dropSrcPathPrefixParts := ui.getURLPrefixAndHeaders(u)
		if up != nil {
			t.Fatalf("unexpected non-empty up=%#v", up)
		}
		if hc.RequestHeaders != nil {
			t.Fatalf("unexpected non-empty request headers=%q", hc.RequestHeaders)
		}
		if hc.ResponseHeaders != nil {
			t.Fatalf("unexpected non-empty response headers=%q", hc.ResponseHeaders)
		}
		if retryStatusCodes != nil {
			t.Fatalf("unexpected non-empty retryStatusCodes=%d", retryStatusCodes)
		}
		if dropSrcPathPrefixParts != 0 {
			t.Fatalf("unexpected non-zero dropSrcPathPrefixParts=%d", dropSrcPathPrefixParts)
		}
	}
	f(&UserInfo{}, "/foo/bar")
	f(&UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/query"}),
				URLPrefix: mustParseURL("http://foobar/baz"),
			},
		},
	}, "/api/v1/write")
}
