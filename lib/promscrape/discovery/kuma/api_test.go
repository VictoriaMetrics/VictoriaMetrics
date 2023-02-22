package kuma

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func Test_buildAPIPath(t *testing.T) {
	type args struct {
		server string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "get api path ok",
			args: args{server: "http://localhost:5676"},
			want: "/v3/discovery:monitoringassignments",
		},
		{
			name:    "get api path incorrect server URL",
			args:    args{server: ":"},
			wantErr: true,
		},
		{
			name:    "get api path incorrect server URL err",
			args:    args{server: "api"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sdConf := &SDConfig{
				Server:            tt.args.server,
				HTTPClientConfig:  promauth.HTTPClientConfig{},
				ProxyClientConfig: promauth.ProxyClientConfig{},
			}
			apiConf, err := getAPIConfig(sdConf, ".")

			if tt.wantErr {
				if err == nil {
					t.Errorf("buildAPIPath() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if apiConf.path != tt.want {
				t.Errorf("buildAPIPath() got = %v, want = %v", apiConf.path, tt.want)
			}

			sdConf.MustStop()
		})
	}

}

func Test_parseAPIResponse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []kumaTarget
		wantErr bool
	}{

		{
			name: "parse ok",
			args: args{
				data: []byte(`{
    "version_info":"5dc9a5dd-2091-4426-a886-dfdc24fc99d7",
    "resources":[
       {
          "@type":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
          "mesh":"default",
          "service":"redis",
          "labels":{ "test":"test1" },
          "targets":[
             {
                "name":"redis",
                "scheme":"http",
                "address":"127.0.0.1:5670",
                "metrics_path":"/metrics",
                "labels":{ "kuma_io_protocol":"tcp" }
             }
          ]
       },
       {
          "@type":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
          "mesh":"default",
          "service":"app",
          "labels":{ "test":"test2" },
          "targets":[
             {
                "name":"app",
                "scheme":"http",
                "address":"127.0.0.1:5671",
                "metrics_path":"/metrics",
                "labels":{ "kuma_io_protocol":"http" }
             }
          ]
       }
    ],
    "type_url":"type.googleapis.com/kuma.observability.v1.MonitoringAssignment"
 }`),
			},
			want: []kumaTarget{
				{
					Mesh:        "default",
					Service:     "redis",
					DataPlane:   "redis",
					Instance:    "redis",
					Scheme:      "http",
					Address:     "127.0.0.1:5670",
					MetricsPath: "/metrics",
					Labels:      map[string]string{"kuma_io_protocol": "tcp", "test": "test1"},
				},
				{
					Mesh:        "default",
					Service:     "app",
					DataPlane:   "app",
					Instance:    "app",
					Scheme:      "http",
					Address:     "127.0.0.1:5671",
					MetricsPath: "/metrics",
					Labels:      map[string]string{"kuma_io_protocol": "http", "test": "test2"},
				},
			},
		},
		{
			name:    "parse err",
			args:    args{data: []byte(`[]`)},
			wantErr: true,
		},
		{
			name: "api version err",
			args: args{
				data: []byte(`{
    "resources":[
       {
          "@type":"type.googleapis.com/kuma.observability.v2.MonitoringAssignment",
          "mesh":"default",
          "service":"redis",
          "targets":[]
       }
    ],
    "type_url":"type.googleapis.com/kuma.observability.v2.MonitoringAssignment"
 }`),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseDiscoveryResponse(tt.args.data)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDiscoveryResponse() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			got := parseKumaTargets(resp)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseDiscoveryResponse() got = %v, want %v", got, tt.want)
			}
		})
	}
}
