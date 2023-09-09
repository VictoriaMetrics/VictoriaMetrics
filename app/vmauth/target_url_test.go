package main

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
)

func TestCreateTargetURLSuccess(t *testing.T) {
	f := func(ui *UserInfo, requestURI, expectedTarget, expectedRequestHeaders, expectedResponseHeaders string, expectedRetryStatusCodes []int) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc, retryStatusCodes := ui.getURLPrefixAndHeaders(u)
		if up == nil {
			t.Fatalf("cannot determie backend: %s", err)
		}
		bu := up.getLeastLoadedBackendURL()
		target := mergeURLs(bu.url, u)
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
	}
	// Simple routing with `url_prefix`
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "", "http://foo.bar/.", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
		HeadersConf: HeadersConf{
			RequestHeaders: []Header{{
				Name:  "bb",
				Value: "aaa",
			}},
		},
		RetryStatusCodes: []int{503, 501},
	}, "/", "http://foo.bar", `[{"bb" "aaa"}]`, `[]`, []int{503, 501})
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar/federate"),
	}, "/", "http://foo.bar/federate", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "a/b?c=d", "http://foo.bar/a/b?c=d", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/z", "https://sss:3894/x/y/z", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/../../aaa", "https://sss:3894/x/y/aaa", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s%2F..%2Fd", "[]", "[]", nil)

	// Complex routing with `url_map`
	ui := &UserInfo{
		URLMaps: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/query"}),
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
				RetryStatusCodes: []int{503, 500, 501},
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
		RetryStatusCodes: []int{502},
	}
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up", `[{"xx" "aa"} {"yy" "asdf"}]`, `[{"qwe" "rty"}]`, []int{503, 500, 501})
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "[]", "[]", nil)
	f(ui, "/api/v1/query_range", "http://default-server/api/v1/query_range", `[{"bb" "aaa"}]`, `[{"x" "y"}]`, []int{502})

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
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up", "[]", "[]", nil)
	f(ui, "/api/v1/query_range?query=up", "http://vmselect/0/prometheus/api/v1/query_range?query=up", "[]", "[]", nil)
	f(ui, "/api/v1/label/foo/values", "http://vmselect/0/prometheus/api/v1/label/foo/values", "[]", "[]", nil)
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write", "[]", "[]", nil)
	f(ui, "/api/v1/foo/bar", "http://default-server/api/v1/foo/bar", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=dev"),
	}, "/api/v1/query", "http://foo.bar/api/v1/query?extra_label=team=dev", "[]", "[]", nil)
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=mobile"),
	}, "/api/v1/query?extra_label=team=dev", "http://foo.bar/api/v1/query?extra_label=team%3Dmobile", "[]", "[]", nil)
}

func TestCreateTargetURLFailure(t *testing.T) {
	f := func(ui *UserInfo, requestURI string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		u = normalizeURL(u)
		up, hc, retryStatusCodes := ui.getURLPrefixAndHeaders(u)
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
