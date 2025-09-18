package main

import (
	"testing"
)

func TestBuildMatchWithFilter_Failure(t *testing.T) {
	f := func(filter, metricName string) {
		t.Helper()

		_, err := buildMatchWithFilter(filter, metricName)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// match with error
	f(`{cluster~=".*"}`, "http_request_count_total")
}

func TestBuildMatchWithFilter_Success(t *testing.T) {
	f := func(filter, metricName, resultExpected string) {
		t.Helper()

		result, err := buildMatchWithFilter(filter, metricName)
		if err != nil {
			t.Fatalf("buildMatchWithFilter() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// parsed metric with label
	f(`{__name__="http_request_count_total",cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// metric name with label
	f(`http_request_count_total{cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// parsed metric with regexp value
	f(`{__name__="http_request_count_total",cluster=~"kube.*"}`, "http_request_count_total", `{cluster=~"kube.*",__name__="http_request_count_total"}`)

	// only label with regexp
	f(`{cluster=~".*"}`, "http_request_count_total", `{cluster=~".*",__name__="http_request_count_total"}`)

	// only label with regexp, empty metric name
	f(`{cluster=~".*"}`, "", `{cluster=~".*"}`)

	// many labels in filter with regexp
	f(`{cluster=~".*",job!=""}`, "http_request_count_total", `{cluster=~".*",job!="",__name__="http_request_count_total"}`)

	// all names
	f(`{__name__!=""}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// with many underscores labels
	f(`{__name__!="", __meta__!=""}`, "http_request_count_total", `{__meta__!="",__name__="http_request_count_total"}`)

	// metric name has regexp
	f(`{__name__=~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// metric name has negative regexp
	f(`{__name__!~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// metric name has negative regex and metric name is empty
	f(`{__name__!~".*"}`, "", `{__name__!~".*"}`)
}
