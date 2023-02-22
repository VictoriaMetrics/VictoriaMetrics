package kuma

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_kumaTargetsToLabels(t *testing.T) {
	type args struct {
		src []kumaTarget
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "convert to labels ok",
			args: args{
				src: []kumaTarget{
					{
						Mesh:        "default",
						Service:     "redis",
						DataPlane:   "redis",
						Instance:    "redis",
						Scheme:      "http",
						Address:     "127.0.0.1:5670",
						MetricsPath: "/metrics",
						Labels:      map[string]string{"kuma_io_protocol": "tcp", "kuma_io_service": "redis"},
					},
					{
						Mesh:        "default",
						Service:     "app",
						DataPlane:   "app",
						Instance:    "app",
						Scheme:      "http",
						Address:     "127.0.0.1:5671",
						MetricsPath: "/vm/metrics",
						Labels:      map[string]string{"kuma_io_protocol": "http", "kuma_io_service": "app"},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"instance":                           "redis",
					"__address__":                        "127.0.0.1:5670",
					"__scheme__":                         "http",
					"__metrics_path__":                   "/metrics",
					"__meta_server":                      "http://localhost:5676",
					"__meta_kuma_mesh":                   "default",
					"__meta_kuma_service":                "redis",
					"__meta_kuma_dataplane":              "redis",
					"__meta_kuma_label_kuma_io_protocol": "tcp",
					"__meta_kuma_label_kuma_io_service":  "redis",
				}),
				promutils.NewLabelsFromMap(map[string]string{
					"instance":                           "app",
					"__address__":                        "127.0.0.1:5671",
					"__scheme__":                         "http",
					"__metrics_path__":                   "/vm/metrics",
					"__meta_server":                      "http://localhost:5676",
					"__meta_kuma_mesh":                   "default",
					"__meta_kuma_service":                "app",
					"__meta_kuma_dataplane":              "app",
					"__meta_kuma_label_kuma_io_protocol": "http",
					"__meta_kuma_label_kuma_io_service":  "app",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kumaTargetsToLabels(tt.args.src, "http://localhost:5676")
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
