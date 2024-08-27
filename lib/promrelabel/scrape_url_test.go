package promrelabel

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetScrapeURL(t *testing.T) {
	f := func(labelsStr, expectedScrapeURL, expectedAddress string) {
		t.Helper()
		labels := promutils.MustNewLabelsFromString(labelsStr)
		scrapeURL, address := GetScrapeURL(labels, nil)
		if scrapeURL != expectedScrapeURL {
			t.Fatalf("unexpected scrapeURL; got %q; want %q", scrapeURL, expectedScrapeURL)
		}
		if address != expectedAddress {
			t.Fatalf("unexpected address; got %q; want %q", address, expectedAddress)
		}
	}

	// Missing __address__
	f("{}", "", "")
	f(`{foo="bar"}`, "", "")

	// __address__ without port
	f(`{__address__="foo"}`, "http://foo/metrics", "foo:80")

	// __address__ with explicit port
	f(`{__address__="foo:1234"}`, "http://foo:1234/metrics", "foo:1234")

	// explicit __scheme__
	f(`{__address__="foo",__scheme__="https"}`, "https://foo/metrics", "foo:443")
	f(`{__address__="foo:1234",__scheme__="https"}`, "https://foo:1234/metrics", "foo:1234")

	// explicit __metrics_path__
	f(`{__address__="foo",__metrics_path__="abc"}`, "http://foo/abc", "foo:80")
	f(`{__address__="foo",__metrics_path__="/abc"}`, "http://foo/abc", "foo:80")
	f(`{__address__="foo",__metrics_path__="/ab/c?d=ef&aa=bb"}`, "http://foo/ab/c?d=ef&aa=bb", "foo:80")

	// explitit __param_*
	f(`{__address__="foo",__param_x="y"}`, "http://foo/metrics?x=y", "foo:80")
	f(`{__address__="foo",__param_x="y",__param_y="aa"}`, "http://foo/metrics?x=y&y=aa", "foo:80")
	f(`{__address__="foo",__param_x="y",__metrics_path__="?abc=de"}`, "http://foo/?abc=de&x=y", "foo:80")
	f(`{__address__="foo",__param_abc="y",__metrics_path__="?abc=de"}`, "http://foo/?abc=de&abc=y", "foo:80")

	// __address__ with metrics path and/or scheme
	f(`{__address__="foo/bar/baz?abc=de"}`, "http://foo/bar/baz?abc=de", "foo:80")
	f(`{__address__="foo:784/bar/baz?abc=de"}`, "http://foo:784/bar/baz?abc=de", "foo:784")
	f(`{__address__="foo:784/bar/baz?abc=de",__param_xx="yy"}`, "http://foo:784/bar/baz?abc=de&xx=yy", "foo:784")
	f(`{__address__="foo:784/bar/baz?abc=de",__param_xx="yy",__scheme__="https"}`, "https://foo:784/bar/baz?abc=de&xx=yy", "foo:784")
	f(`{__address__="http://foo/bar/baz?abc=de",__param_xx="yy"}`, "http://foo/bar/baz?abc=de&xx=yy", "foo:80")
	f(`{__address__="https://foo/bar/baz?abc=de",__param_xx="yy"}`, "https://foo/bar/baz?abc=de&xx=yy", "foo:443")

	// __address__ already carry 80/443 port
	f(`{__address__="foo:80"}`, "http://foo:80/metrics", "foo:80")
	f(`{__address__="foo:443"}`, "http://foo:443/metrics", "foo:443")
	f(`{__address__="http://foo"}`, "http://foo/metrics", "foo:80")
	f(`{__address__="https://foo"}`, "https://foo/metrics", "foo:443")
	f(`{__address__="http://foo:80"}`, "http://foo:80/metrics", "foo:80")
	f(`{__address__="https://foo:443"}`, "https://foo:443/metrics", "foo:443")
}
