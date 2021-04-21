package main

import (
	"net/url"
	"testing"
)

func TestCreateTargetURLSuccess(t *testing.T) {
	f := func(ui *UserInfo, requestURI, expectedTarget string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		target, err := createTargetURL(ui, u)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if target.String() != expectedTarget {
			t.Fatalf("unexpected target; got %q; want %q", target, expectedTarget)
		}
	}
	// Simple routing with `url_prefix`
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "", "http://foo.bar/.")
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "/", "http://foo.bar/")
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar"),
	}, "a/b?c=d", "http://foo.bar/a/b?c=d")
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/z", "https://sss:3894/x/y/z")
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/../../aaa", "https://sss:3894/x/y/aaa")
	f(&UserInfo{
		URLPrefix: mustParseURL("https://sss:3894/x/y"),
	}, "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s%2F..%2Fd")

	// Complex routing with `url_map`
	ui := &UserInfo{
		URLMap: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/query"}),
				URLPrefix: mustParseURL("http://vmselect/0/prometheus"),
			},
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/write"}),
				URLPrefix: mustParseURL("http://vminsert/0/prometheus"),
			},
		},
		URLPrefix: mustParseURL("http://default-server"),
	}
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up")
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write")
	f(ui, "/api/v1/query_range", "http://default-server/api/v1/query_range")

	// Complex routing regexp paths in `url_map`
	ui = &UserInfo{
		URLMap: []URLMap{
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
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up")
	f(ui, "/api/v1/query_range?query=up", "http://vmselect/0/prometheus/api/v1/query_range?query=up")
	f(ui, "/api/v1/label/foo/values", "http://vmselect/0/prometheus/api/v1/label/foo/values")
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write")
	f(ui, "/api/v1/foo/bar", "http://default-server/api/v1/foo/bar")
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=dev"),
	}, "/api/v1/query", "http://foo.bar/api/v1/query?extra_label=team=dev")
	f(&UserInfo{
		URLPrefix: mustParseURL("http://foo.bar?extra_label=team=mobile"),
	}, "/api/v1/query?extra_label=team=dev", "http://foo.bar/api/v1/query?extra_label=team%3Dmobile")

}

func TestCreateTargetURLFailure(t *testing.T) {
	f := func(ui *UserInfo, requestURI string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		target, err := createTargetURL(ui, u)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if target != nil {
			t.Fatalf("unexpected target=%q; want empty string", target)
		}
	}
	f(&UserInfo{}, "/foo/bar")
	f(&UserInfo{
		URLMap: []URLMap{
			{
				SrcPaths:  getSrcPaths([]string{"/api/v1/query"}),
				URLPrefix: mustParseURL("http://foobar/baz"),
			},
		},
	}, "/api/v1/write")
}
