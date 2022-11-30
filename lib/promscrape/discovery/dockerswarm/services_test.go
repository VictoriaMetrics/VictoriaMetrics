package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseServicesResponse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []service
		wantErr bool
	}{
		{
			name: "parse ok",
			args: args{
				data: []byte(`[
  {
    "ID": "tgsci5gd31aai3jyudv98pqxf",
    "Version": {
      "Index": 25
    },
    "CreatedAt": "2020-10-06T11:17:31.948808444Z",
    "UpdatedAt": "2020-10-06T11:17:31.950195138Z",
    "Spec": {
      "Name": "redis2",
      "Labels": {},
      "TaskTemplate": {
        "ContainerSpec": {
          "Image": "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
          "Init": false,
          "DNSConfig": {},
          "Isolation": "default"
        },
        "Resources": {
          "Limits": {},
          "Reservations": {}
        }
      },
      "Mode": {
        "Replicated": {}
      },
      "EndpointSpec": {
        "Mode": "vip",
        "Ports": [
          {
            "Protocol": "tcp",
            "TargetPort": 6379,
            "PublishedPort": 8081,
            "PublishMode": "ingress"
          }
        ]
      }
    },
    "Endpoint": {
      "Spec": {
        "Mode": "vip",
        "Ports": [
          {
            "Protocol": "tcp",
            "TargetPort": 6379,
            "PublishedPort": 8081,
            "PublishMode": "ingress"
          }
        ]
      },
      "Ports": [
        {
          "Protocol": "tcp",
          "TargetPort": 6379,
          "PublishedPort": 8081,
          "PublishMode": "ingress"
        }
      ],
      "VirtualIPs": [
        {
          "NetworkID": "qs0hog6ldlei9ct11pr3c77v1",
          "Addr": "10.0.0.3/24"
        }
      ]
    }
  }
]`),
			},
			want: []service{
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
								Hostname: "",
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
					}{Ports: []portConfig{
						{
							Protocol:      "tcp",
							PublishMode:   "ingress",
							PublishedPort: 8081,
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServicesResponse(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseServicesResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseServicesResponse() \ngot  %v, \nwant %v", got, tt.want)
			}
		})
	}
}

func Test_addServicesLabels(t *testing.T) {
	type args struct {
		services       []service
		networksLabels map[string]*promutils.Labels
		port           int
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "add 2 services with network labels join",
			args: args{
				port: 9100,
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
						}{Ports: []portConfig{
							{
								Protocol:    "tcp",
								Name:        "redis",
								PublishMode: "ingress",
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
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                                           "10.0.0.3:0",
					"__meta_dockerswarm_network_id":                         "qs0hog6ldlei9ct11pr3c77v1",
					"__meta_dockerswarm_network_ingress":                    "true",
					"__meta_dockerswarm_network_internal":                   "false",
					"__meta_dockerswarm_network_label_key1":                 "value1",
					"__meta_dockerswarm_network_name":                       "ingress",
					"__meta_dockerswarm_network_scope":                      "swarm",
					"__meta_dockerswarm_service_endpoint_port_name":         "redis",
					"__meta_dockerswarm_service_endpoint_port_publish_mode": "ingress",
					"__meta_dockerswarm_service_id":                         "tgsci5gd31aai3jyudv98pqxf",
					"__meta_dockerswarm_service_mode":                       "replicated",
					"__meta_dockerswarm_service_name":                       "redis2",
					"__meta_dockerswarm_service_task_container_hostname":    "node1",
					"__meta_dockerswarm_service_task_container_image":       "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
					"__meta_dockerswarm_service_updating_status":            "",
				})},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addServicesLabels(tt.args.services, tt.args.networksLabels, tt.args.port)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
