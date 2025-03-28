package http

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAddHTTPTargetLabels(t *testing.T) {
	f := func(src []httpGroupTarget, labelssExpected []*promutil.Labels) {
		t.Helper()

		labelss := addHTTPTargetLabels(src, "http://foo.bar/baz?aaa=bb")
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// add ok
	src := []httpGroupTarget{
		{
			Targets: []string{"127.0.0.1:9100", "127.0.0.2:91001"},
			Labels:  promutil.NewLabelsFromMap(map[string]string{"__meta_kubernetes_pod": "pod-1", "__meta_consul_dc": "dc-2"}),
		},
	}
	labelssExpected := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.1:9100",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.2:91001",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
	}
	f(src, labelssExpected)
}
