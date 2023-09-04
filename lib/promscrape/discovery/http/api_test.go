package http

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseAPIResponse(t *testing.T) {
	type args struct {
		data []byte
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    []httpGroupTarget
		wantErr bool
	}{

		{
			name: "parse ok",
			args: args{
				path: "/ok",
				data: []byte(`[
                {"targets": ["http://target-1:9100","http://target-2:9150"],
                "labels": {"label-1":"value-1"} }
                ]`),
			},
			want: []httpGroupTarget{
				{
					Labels:  promutils.NewLabelsFromMap(map[string]string{"label-1": "value-1"}),
					Targets: []string{"http://target-1:9100", "http://target-2:9150"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIResponse(tt.args.data, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAPIResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAPIResponse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
