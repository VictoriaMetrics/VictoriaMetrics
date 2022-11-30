package http

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_addHTTPTargetLabels(t *testing.T) {
	type args struct {
		src []httpGroupTarget
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "add ok",
			args: args{
				src: []httpGroupTarget{
					{
						Targets: []string{"127.0.0.1:9100", "127.0.0.2:91001"},
						Labels:  promutils.NewLabelsFromMap(map[string]string{"__meta_kubernetes_pod": "pod-1", "__meta_consul_dc": "dc-2"}),
					},
				},
			},
			want: []*promutils.Labels{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addHTTPTargetLabels(tt.args.src, "http://foo.bar/baz?aaa=bb")
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
