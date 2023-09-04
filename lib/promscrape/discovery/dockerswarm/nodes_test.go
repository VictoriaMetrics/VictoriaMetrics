package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseNodes(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []node
		wantErr bool
	}{
		{
			name: "parse ok",
			args: args{
				data: []byte(`[
  {
    "ID": "qauwmifceyvqs0sipvzu8oslu",
    "Version": {
      "Index": 16
    },
    "Spec": {
      "Role": "manager",
      "Availability": "active"
    },
    "Description": {
      "Hostname": "ip-172-31-40-97",
      "Platform": {
        "Architecture": "x86_64",
        "OS": "linux"
      },
      "Resources": {
        "NanoCPUs": 1000000000,
        "MemoryBytes": 1026158592
      },
      "Engine": {
        "EngineVersion": "19.03.11"
      }
    },
    "Status": {
      "State": "ready",
      "Addr": "172.31.40.97"
    }
  }
]
`),
			},
			want: []node{
				{
					ID: "qauwmifceyvqs0sipvzu8oslu",
					Spec: struct {
						Labels       map[string]string
						Role         string
						Availability string
					}{Role: "manager", Availability: "active"},
					Status: struct {
						State   string
						Message string
						Addr    string
					}{State: "ready", Addr: "172.31.40.97"},
					Description: struct {
						Hostname string
						Platform struct {
							Architecture string
							OS           string
						}
						Engine struct{ EngineVersion string }
					}{
						Hostname: "ip-172-31-40-97",
						Platform: struct {
							Architecture string
							OS           string
						}{
							Architecture: "x86_64",
							OS:           "linux",
						},
						Engine: struct{ EngineVersion string }{
							EngineVersion: "19.03.11",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNodes(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNodes() \ngot  %v, \nwant %v", got, tt.want)
			}
		})
	}
}

func Test_addNodeLabels(t *testing.T) {
	type args struct {
		nodes []node
		port  int
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "add labels to one node",
			args: args{
				nodes: []node{
					{
						ID: "qauwmifceyvqs0sipvzu8oslu",
						Spec: struct {
							Labels       map[string]string
							Role         string
							Availability string
						}{Role: "manager", Availability: "active"},
						Status: struct {
							State   string
							Message string
							Addr    string
						}{State: "ready", Addr: "172.31.40.97"},
						Description: struct {
							Hostname string
							Platform struct {
								Architecture string
								OS           string
							}
							Engine struct{ EngineVersion string }
						}{
							Hostname: "ip-172-31-40-97",
							Platform: struct {
								Architecture string
								OS           string
							}{
								Architecture: "x86_64",
								OS:           "linux",
							},
							Engine: struct{ EngineVersion string }{
								EngineVersion: "19.03.11",
							},
						},
					},
				},
				port: 9100,
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                                   "172.31.40.97:9100",
					"__meta_dockerswarm_node_address":               "172.31.40.97",
					"__meta_dockerswarm_node_availability":          "active",
					"__meta_dockerswarm_node_engine_version":        "19.03.11",
					"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
					"__meta_dockerswarm_node_manager_address":       "",
					"__meta_dockerswarm_node_manager_leader":        "false",
					"__meta_dockerswarm_node_manager_reachability":  "",
					"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
					"__meta_dockerswarm_node_platform_architecture": "x86_64",
					"__meta_dockerswarm_node_platform_os":           "linux",
					"__meta_dockerswarm_node_role":                  "manager",
					"__meta_dockerswarm_node_status":                "ready",
				})},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addNodeLabels(tt.args.nodes, tt.args.port)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
