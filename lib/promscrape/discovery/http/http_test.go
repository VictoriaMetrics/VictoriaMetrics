package http

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func Test_addHTTPTargetLabels(t *testing.T) {
	type args struct {
		src []httpGroupTarget
	}
	tests := []struct {
		name string
		args args
		want [][]prompbmarshal.Label
	}{
		{
			name: "add ok",
			args: args{
				src: []httpGroupTarget{
					{
						Targets: []string{"127.0.0.1:9100", "127.0.0.2:91001"},
						Labels:  map[string]string{"__meta__kubernetes_pod": "pod-1", "__meta_consul_dc": "dc-2"},
					},
				},
			},
			want: [][]prompbmarshal.Label{
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__":            "127.0.0.1:9100",
					"__meta__kubernetes_pod": "pod-1",
					"__meta_consul_dc":       "dc-2",
				}),
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__":            "127.0.0.2:91001",
					"__meta__kubernetes_pod": "pod-1",
					"__meta_consul_dc":       "dc-2",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addHTTPTargetLabels(tt.args.src)
			var sortedLabelss [][]prompbmarshal.Label
			for _, labels := range got {
				sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
			}
			if !reflect.DeepEqual(sortedLabelss, tt.want) {
				t.Errorf("addHTTPTargetLabels() \ngot  \n%v\n, \nwant \n%v\n", sortedLabelss, tt.want)
			}
		})
	}
}
