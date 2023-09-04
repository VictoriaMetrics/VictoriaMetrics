package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseTasks(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []task
		wantErr bool
	}{
		{
			name: "parse ok",
			args: args{
				data: []byte(`[
  {
    "ID": "t4rdm7j2y9yctbrksiwvsgpu5",
    "Version": {
      "Index": 23
    },
    "Spec": {
      "ContainerSpec": {
        "Image": "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
        "Init": false,
        "Labels": {
	    "label1": "value1"
        }
      },
      "Resources": {
        "Limits": {},
        "Reservations": {}
      },
      "Placement": {
        "Platforms": [
          {
            "Architecture": "amd64",
            "OS": "linux"
          }
        ]
      },
      "ForceUpdate": 0
    },
    "ServiceID": "t91nf284wzle1ya09lqvyjgnq",
    "Slot": 1,
    "NodeID": "qauwmifceyvqs0sipvzu8oslu",
    "Status": {
      "State": "running",
      "ContainerStatus": {
        "ContainerID": "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
        "ExitCode": 0
      },
      "PortStatus": {}
    },
    "DesiredState": "running"
  }
]
`),
			},
			want: []task{
				{
					ID:        "t4rdm7j2y9yctbrksiwvsgpu5",
					ServiceID: "t91nf284wzle1ya09lqvyjgnq",
					NodeID:    "qauwmifceyvqs0sipvzu8oslu",
					Spec: struct {
						ContainerSpec struct {
							Labels map[string]string
						}
					}{
						ContainerSpec: struct {
							Labels map[string]string
						}{
							Labels: map[string]string{
								"label1": "value1",
							},
						},
					},
					DesiredState: "running",
					Slot:         1,
					Status: struct {
						State           string
						ContainerStatus struct{ ContainerID string }
						PortStatus      struct{ Ports []portConfig }
					}{
						State: "running",
						ContainerStatus: struct{ ContainerID string }{
							ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
						},
						PortStatus: struct{ Ports []portConfig }{}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTasks(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTasks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTasks() got\n%v\nwant\n%v", got, tt.want)
			}
		})
	}
}

func Test_addTasksLabels(t *testing.T) {
	type args struct {
		tasks          []task
		nodesLabels    []*promutils.Labels
		servicesLabels []*promutils.Labels
		networksLabels map[string]*promutils.Labels
		services       []service
		port           int
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "adds 1 task with nodes labels",
			args: args{
				port: 9100,
				tasks: []task{
					{
						ID:           "t4rdm7j2y9yctbrksiwvsgpu5",
						ServiceID:    "t91nf284wzle1ya09lqvyjgnq",
						NodeID:       "qauwmifceyvqs0sipvzu8oslu",
						DesiredState: "running",
						Slot:         1,
						Status: struct {
							State           string
							ContainerStatus struct{ ContainerID string }
							PortStatus      struct{ Ports []portConfig }
						}{
							State: "running",
							ContainerStatus: struct{ ContainerID string }{
								ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
							},
							PortStatus: struct{ Ports []portConfig }{
								Ports: []portConfig{
									{
										PublishMode:   "ingress",
										Name:          "redis",
										Protocol:      "tcp",
										PublishedPort: 6379,
									},
								},
							}},
					},
				},
				nodesLabels: []*promutils.Labels{
					promutils.NewLabelsFromMap(map[string]string{
						"__address__":                                   "172.31.40.97:9100",
						"__meta_dockerswarm_node_address":               "172.31.40.97",
						"__meta_dockerswarm_node_availability":          "active",
						"__meta_dockerswarm_node_engine_version":        "19.03.11",
						"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
						"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
						"__meta_dockerswarm_node_platform_architecture": "x86_64",
						"__meta_dockerswarm_node_platform_os":           "linux",
						"__meta_dockerswarm_node_role":                  "manager",
						"__meta_dockerswarm_node_status":                "ready",
					}),
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                                   "172.31.40.97:6379",
					"__meta_dockerswarm_node_address":               "172.31.40.97",
					"__meta_dockerswarm_node_availability":          "active",
					"__meta_dockerswarm_node_engine_version":        "19.03.11",
					"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
					"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
					"__meta_dockerswarm_node_platform_architecture": "x86_64",
					"__meta_dockerswarm_node_platform_os":           "linux",
					"__meta_dockerswarm_node_role":                  "manager",
					"__meta_dockerswarm_node_status":                "ready",
					"__meta_dockerswarm_task_container_id":          "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
					"__meta_dockerswarm_task_desired_state":         "running",
					"__meta_dockerswarm_task_id":                    "t4rdm7j2y9yctbrksiwvsgpu5",
					"__meta_dockerswarm_task_port_publish_mode":     "ingress",
					"__meta_dockerswarm_task_slot":                  "1",
					"__meta_dockerswarm_task_state":                 "running",
				})},
		},
		{
			name: "adds 1 task with nodes, network and services labels",
			args: args{
				port: 9100,
				tasks: []task{
					{
						ID:           "t4rdm7j2y9yctbrksiwvsgpu5",
						ServiceID:    "tgsci5gd31aai3jyudv98pqxf",
						NodeID:       "qauwmifceyvqs0sipvzu8oslu",
						DesiredState: "running",
						Slot:         1,
						NetworksAttachments: []struct {
							Addresses []string
							Network   struct{ ID string }
						}{
							{
								Network: struct {
									ID string
								}{
									ID: "qs0hog6ldlei9ct11pr3c77v1",
								},
								Addresses: []string{"10.10.15.15/24"},
							},
						},
						Status: struct {
							State           string
							ContainerStatus struct{ ContainerID string }
							PortStatus      struct{ Ports []portConfig }
						}{
							State: "running",
							ContainerStatus: struct{ ContainerID string }{
								ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
							},
							PortStatus: struct{ Ports []portConfig }{}},
					},
				},
				networksLabels: map[string]*promutils.Labels{
					"qs0hog6ldlei9ct11pr3c77v1": promutils.NewLabelsFromMap(map[string]string{
						"__meta_dockerswarm_network_id":         "qs0hog6ldlei9ct11pr3c77v1",
						"__meta_dockerswarm_network_ingress":    "true",
						"__meta_dockerswarm_network_internal":   "false",
						"__meta_dockerswarm_network_label_key1": "value1",
						"__meta_dockerswarm_network_name":       "ingress",
						"__meta_dockerswarm_network_scope":      "swarm",
					}),
				},
				nodesLabels: []*promutils.Labels{
					promutils.NewLabelsFromMap(map[string]string{
						"__address__":                                   "172.31.40.97:9100",
						"__meta_dockerswarm_node_address":               "172.31.40.97",
						"__meta_dockerswarm_node_availability":          "active",
						"__meta_dockerswarm_node_engine_version":        "19.03.11",
						"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
						"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
						"__meta_dockerswarm_node_platform_architecture": "x86_64",
						"__meta_dockerswarm_node_platform_os":           "linux",
						"__meta_dockerswarm_node_role":                  "manager",
						"__meta_dockerswarm_node_status":                "ready",
					}),
				},
				services: []service{
					{
						ID: "tgsci5gd31aai3jyudv98pqxf",
						Spec: struct {
							Labels       map[string]string
							Name         string
							TaskTemplate struct {
								ContainerSpec struct {
									Hostname string
									Image    string
								}
							}
							Mode struct {
								Global     interface{}
								Replicated interface{}
							}
						}{
							Labels: map[string]string{},
							Name:   "redis2",
							TaskTemplate: struct {
								ContainerSpec struct {
									Hostname string
									Image    string
								}
							}{
								ContainerSpec: struct {
									Hostname string
									Image    string
								}{
									Hostname: "node1",
									Image:    "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
								},
							},
							Mode: struct {
								Global     interface{}
								Replicated interface{}
							}{
								Replicated: map[string]interface{}{},
							},
						},
						Endpoint: struct {
							Ports      []portConfig
							VirtualIPs []struct {
								NetworkID string
								Addr      string
							}
						}{
							Ports: []portConfig{
								{
									Protocol:      "tcp",
									Name:          "redis",
									PublishMode:   "ingress",
									PublishedPort: 6379,
								},
							}, VirtualIPs: []struct {
								NetworkID string
								Addr      string
							}{
								{
									NetworkID: "qs0hog6ldlei9ct11pr3c77v1",
									Addr:      "10.0.0.3/24",
								},
							},
						},
					},
				},
				servicesLabels: []*promutils.Labels{},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                                   "10.10.15.15:6379",
					"__meta_dockerswarm_network_id":                 "qs0hog6ldlei9ct11pr3c77v1",
					"__meta_dockerswarm_network_ingress":            "true",
					"__meta_dockerswarm_network_internal":           "false",
					"__meta_dockerswarm_network_label_key1":         "value1",
					"__meta_dockerswarm_network_name":               "ingress",
					"__meta_dockerswarm_network_scope":              "swarm",
					"__meta_dockerswarm_node_address":               "172.31.40.97",
					"__meta_dockerswarm_node_availability":          "active",
					"__meta_dockerswarm_node_engine_version":        "19.03.11",
					"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
					"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
					"__meta_dockerswarm_node_platform_architecture": "x86_64",
					"__meta_dockerswarm_node_platform_os":           "linux",
					"__meta_dockerswarm_node_role":                  "manager",
					"__meta_dockerswarm_node_status":                "ready",
					"__meta_dockerswarm_task_container_id":          "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
					"__meta_dockerswarm_task_desired_state":         "running",
					"__meta_dockerswarm_task_id":                    "t4rdm7j2y9yctbrksiwvsgpu5",
					"__meta_dockerswarm_task_port_publish_mode":     "ingress",
					"__meta_dockerswarm_task_slot":                  "1",
					"__meta_dockerswarm_task_state":                 "running",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addTasksLabels(tt.args.tasks, tt.args.nodesLabels, tt.args.servicesLabels, tt.args.networksLabels, tt.args.services, tt.args.port)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
