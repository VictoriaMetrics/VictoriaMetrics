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
		if target != expectedTarget {
			t.Fatalf("unexpected target; got %q; want %q", target, expectedTarget)
		}
	}
	// Simple routing with `url_prefix`
	f(&UserInfo{
		URLPrefix: "http://foo.bar",
	}, "", "http://foo.bar/.")
	f(&UserInfo{
		URLPrefix: "http://foo.bar",
	}, "/", "http://foo.bar/")
	f(&UserInfo{
		URLPrefix: "http://foo.bar",
	}, "a/b?c=d", "http://foo.bar/a/b?c=d")
	f(&UserInfo{
		URLPrefix: "https://sss:3894/x/y",
	}, "/z", "https://sss:3894/x/y/z")
	f(&UserInfo{
		URLPrefix: "https://sss:3894/x/y",
	}, "/../../aaa", "https://sss:3894/x/y/aaa")
	f(&UserInfo{
		URLPrefix: "https://sss:3894/x/y",
	}, "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s/../d")

	// Complex routing with `url_map`
	ui := &UserInfo{
		URLMap: []URLMap{
			{
				SrcPaths:  []string{"/api/v1/query"},
				URLPrefix: "http://vmselect/0/prometheus",
			},
			{
				SrcPaths:  []string{"/api/v1/write"},
				URLPrefix: "http://vminsert/0/prometheus",
			},
		},
		URLPrefix: "http://default-server",
	}
	f(ui, "/api/v1/query?query=up", "http://vmselect/0/prometheus/api/v1/query?query=up")
	f(ui, "/api/v1/write", "http://vminsert/0/prometheus/api/v1/write")
	f(ui, "/api/v1/query_range", "http://default-server/api/v1/query_range")
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
		if target != "" {
			t.Fatalf("unexpected target=%q; want empty string", target)
		}
	}
	f(&UserInfo{}, "/foo/bar")
	f(&UserInfo{
		URLMap: []URLMap{
			{
				SrcPaths:  []string{"/api/v1/query"},
				URLPrefix: "http://foobar/baz",
			},
		},
	}, "/api/v1/write")
}
