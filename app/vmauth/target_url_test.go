package main

import (
	"net/url"
	"testing"
)

func TestCreateTargetURL(t *testing.T) {
	f := func(prefix, requestURI, expectedTarget string) {
		t.Helper()
		u, err := url.Parse(requestURI)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", requestURI, err)
		}
		target := createTargetURL(prefix, u)
		if target != expectedTarget {
			t.Fatalf("unexpected target; got %q; want %q", target, expectedTarget)
		}
	}
	f("http://foo.bar", "", "http://foo.bar/.")
	f("http://foo.bar", "/", "http://foo.bar/")
	f("http://foo.bar", "a/b?c=d", "http://foo.bar/a/b?c=d")
	f("https://sss:3894/x/y", "/z", "https://sss:3894/x/y/z")
	f("https://sss:3894/x/y", "/../../aaa", "https://sss:3894/x/y/aaa")
	f("https://sss:3894/x/y", "/./asd/../../aaa?a=d&s=s/../d", "https://sss:3894/x/y/aaa?a=d&s=s/../d")
}
