package http

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestAddHTTPTargetLabels(t *testing.T) {
	f := func(src []httpGroupTarget, labelssExpected []*promutils.Labels) {
		t.Helper()

		labelss := addHTTPTargetLabels(src, "http://foo.bar/baz?aaa=bb")
		discoveryutils.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// add ok
	src := []httpGroupTarget{
		{
			Targets: []string{"127.0.0.1:9100", "127.0.0.2:91001"},
			Labels:  promutils.NewLabelsFromMap(map[string]string{"__meta_kubernetes_pod": "pod-1", "__meta_consul_dc": "dc-2"}),
		},
	}
	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.1:9100",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.2:91001",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
	}
	f(src, labelssExpected)
}
