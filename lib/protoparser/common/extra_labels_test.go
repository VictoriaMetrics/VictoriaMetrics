package common

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func TestGetExtraLabelsSuccess(t *testing.T) {
	f := func(requestURI, expectedLabels string) {
		t.Helper()
		fullURL := "http://fobar" + requestURI
		req, err := http.NewRequest(http.MethodGet, fullURL, nil)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", fullURL, err)
		}
		extraLabels, err := GetExtraLabels(req)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		labelsStr := getLabelsString(extraLabels)
		if labelsStr != expectedLabels {
			t.Fatalf("unexpected labels;\ngot\n%s\nwant\n%s", labelsStr, expectedLabels)
		}
	}
	f("", `{}`)
	f("/foo/bar", `{}`)
	f("/foo?extra_label=foo=bar", `{foo="bar"}`)
	f("/foo?extra_label=a=x&extra_label=b=y", `{a="x",b="y"}`)
	f("/metrics/job/foo", `{job="foo"}`)
	f("/metrics/job/foo?extra_label=a=b", `{a="b",job="foo"}`)
	f("/metrics/job/foo/b/bcd?extra_label=a=b&extra_label=qwe=rty", `{a="b",b="bcd",job="foo",qwe="rty"}`)
	f("/metrics/job/titan/name/%CE%A0%CF%81%CE%BF%CE%BC%CE%B7%CE%B8%CE%B5%CF%8D%CF%82", `{job="titan",name="Προμηθεύς"}`)
	f("/metrics/job/titan/name@base64/zqDPgc6_zrzOt864zrXPjc-C", `{job="titan",name="Προμηθεύς"}`)
}

func TestGetPushgatewayLabelsSuccess(t *testing.T) {
	f := func(path, expectedLabels string) {
		t.Helper()
		labels, err := getPushgatewayLabels(path)
		if err != nil {
			t.Fatalf("unexpected error in getPushgatewayLabels(%q): %s", path, err)
		}
		labelsStr := getLabelsString(labels)
		if labelsStr != expectedLabels {
			t.Fatalf("unexpected labels returned from getPushgatewayLabels(%q);\ngot\n%s\nwant\n%s", path, labelsStr, expectedLabels)
		}
	}
	f("", "{}")
	f("/foo/bar", "{}")
	f("/metrics/foo/bar", "{}")
	f("/metrics/job", "{}")
	f("/metrics/job@base64", "{}")
	f("/metrics/job/", "{}")
	f("/metrics/job/foo", `{job="foo"}`)
	f("/foo/metrics/job/foo", `{job="foo"}`)
	f("/api/v1/import/prometheus/metrics/job/foo", `{job="foo"}`)
	f("/foo/metrics/job@base64/Zm9v", `{job="foo"}`)
	f("/foo/metrics/job/x/a/foo/aaa/bar", `{a="foo",aaa="bar",job="x"}`)
	f("/foo/metrics/job/x/a@base64/Zm9v", `{a="foo",job="x"}`)
	f("/metrics/job/test/region@base64/YXotc291dGhlYXN0LTEtZjAxL3d6eS1hei1zb3V0aGVhc3QtMQ", `{job="test",region="az-southeast-1-f01/wzy-az-southeast-1"}`)
	f("/metrics/job/test/empty@base64/=", `{job="test"}`)
	f("/metrics/job/test/test@base64/PT0vPT0", `{job="test",test="==/=="}`)
}

func TestGetPushgatewayLabelsFailure(t *testing.T) {
	f := func(path string) {
		t.Helper()
		labels, err := getPushgatewayLabels(path)
		if err == nil {
			labelsStr := getLabelsString(labels)
			t.Fatalf("expecting non-nil error for getPushgatewayLabels(%q); got labels %s", path, labelsStr)
		}
	}
	// missing bar value
	f("/metrics/job/foo/bar")
	// invalid base64 encoding for job
	f("/metrics/job@base64/#$%")
	// invalid base64 encoding for non-job label
	f("/metrics/job/foo/bar@base64/#$%")
}

func getLabelsString(labels []prompbmarshal.Label) string {
	a := make([]string, len(labels))
	for i, label := range labels {
		a[i] = fmt.Sprintf("%s=%q", label.Name, label.Value)
	}
	sort.Strings(a)
	return "{" + strings.Join(a, ",") + "}"
}
