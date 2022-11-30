package docker

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseContainers(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []container
		wantErr bool
	}{
		{
			name: "parse two containers",
			args: args{
				data: []byte(`[
  {
    "Id": "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
    "Names": [
      "/crow-server"
    ],
    "ImageID": "sha256:11045371758ccf9468d807d53d1e1faa100c8ebabe87296bc107d52bdf983378",
    "Created": 1624440429,
    "Ports": [
      {
        "IP": "0.0.0.0",
        "PrivatePort": 8080,
        "PublicPort": 18081,
        "Type": "tcp"
      }
    ],
    "Labels": {
      "com.docker.compose.config-hash": "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
      "com.docker.compose.container-number": "1",
      "com.docker.compose.oneoff": "False",
      "com.docker.compose.project": "crowserver",
      "com.docker.compose.service": "crow-server",
      "com.docker.compose.version": "1.11.2"
    },
    "State": "running",
    "Status": "Up 2 hours",
    "HostConfig": {
      "NetworkMode": "bridge"
    },
    "NetworkSettings": {
      "Networks": {
        "bridge": {
          "IPAMConfig": null,
          "Links": null,
          "Aliases": null,
          "NetworkID": "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
          "EndpointID": "2bcd751c98578ad1ce3d6798077a3e110535f3dcb0b0735cc84bd1e03905d907",
          "Gateway": "172.17.0.1",
          "IPAddress": "172.17.0.2",
          "IPPrefixLen": 16,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "DriverOpts": null
        }
      }
    }
  },
  {
    "Id": "0e0f72a6eb7d9fb443f0426a66f7b8dd7d3283ab7e3a308b2bed584ac03a33dc",
    "Names": [
      "/crow-web"
    ],
    "ImageID": "sha256:36e04dd2f67950179ab62a4ebd1b8b741232263fa1f210496a53335d5820b3af",
    "Created": 1618302442,
    "Ports": [
      {
        "IP": "0.0.0.0",
        "PrivatePort": 8080,
        "PublicPort": 18082,
        "Type": "tcp"
      }
    ],
    "Labels": {
      "com.docker.compose.config-hash": "d99ebd0fde8512366c2d78c367e95ddc74528bb60b7cf0c991c9f4835981e00e",
      "com.docker.compose.container-number": "1",
      "com.docker.compose.oneoff": "False",
      "com.docker.compose.project": "crowweb",
      "com.docker.compose.service": "crow-web",
      "com.docker.compose.version": "1.11.2"
    },
    "State": "running",
    "Status": "Up 2 months",
    "HostConfig": {
      "NetworkMode": "bridge"
    },
    "NetworkSettings": {
      "Networks": {
        "bridge": {
          "IPAMConfig": null,
          "Links": null,
          "Aliases": null,
          "NetworkID": "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
          "EndpointID": "78d751dfa31923d73e001e5dc5751ad6f9f5ffeb88056fc950f0a952d7f24302",
          "Gateway": "172.17.0.1",
          "IPAddress": "172.17.0.3",
          "IPPrefixLen": 16,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "DriverOpts": null
        }
      }
    }
  }
]`),
			},
			want: []container{
				{
					ID:    "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
					Names: []string{"/crow-server"},
					Labels: map[string]string{
						"com.docker.compose.config-hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
						"com.docker.compose.container-number": "1",
						"com.docker.compose.oneoff":           "False",
						"com.docker.compose.project":          "crowserver",
						"com.docker.compose.service":          "crow-server",
						"com.docker.compose.version":          "1.11.2",
					},
					Ports: []struct {
						IP          string
						PrivatePort int
						PublicPort  int
						Type        string
					}{{
						IP:          "0.0.0.0",
						PrivatePort: 8080,
						PublicPort:  18081,
						Type:        "tcp",
					}},
					HostConfig: struct {
						NetworkMode string
					}{
						NetworkMode: "bridge",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string
							NetworkID string
						}
					}{
						Networks: map[string]struct {
							IPAddress string
							NetworkID string
						}{
							"bridge": {
								IPAddress: "172.17.0.2",
								NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
							},
						},
					},
				},
				{
					ID:    "0e0f72a6eb7d9fb443f0426a66f7b8dd7d3283ab7e3a308b2bed584ac03a33dc",
					Names: []string{"/crow-web"},
					Labels: map[string]string{
						"com.docker.compose.config-hash":      "d99ebd0fde8512366c2d78c367e95ddc74528bb60b7cf0c991c9f4835981e00e",
						"com.docker.compose.container-number": "1",
						"com.docker.compose.oneoff":           "False",
						"com.docker.compose.project":          "crowweb",
						"com.docker.compose.service":          "crow-web",
						"com.docker.compose.version":          "1.11.2",
					},
					Ports: []struct {
						IP          string
						PrivatePort int
						PublicPort  int
						Type        string
					}{{
						IP:          "0.0.0.0",
						PrivatePort: 8080,
						PublicPort:  18082,
						Type:        "tcp",
					}},
					HostConfig: struct {
						NetworkMode string
					}{
						NetworkMode: "bridge",
					},
					NetworkSettings: struct {
						Networks map[string]struct {
							IPAddress string
							NetworkID string
						}
					}{
						Networks: map[string]struct {
							IPAddress string
							NetworkID string
						}{
							"bridge": {
								IPAddress: "172.17.0.3",
								NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseContainers(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseContainers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNetworks() \ngot  %v, \nwant %v", got, tt.want)
			}
		})
	}
}

func Test_addContainerLabels(t *testing.T) {
	data := []byte(`[
  {
    "Name": "host",
    "Id": "6a1989488dcb847c052eda939924d997457d5ecd994f76f35472996c4c75279a",
    "Created": "2020-08-18T17:18:18.439033107+08:00",
    "Scope": "local",
    "Driver": "host",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": []
    },
    "Internal": false,
    "Attachable": false,
    "Ingress": false,
    "ConfigFrom": {
      "Network": ""
    },
    "ConfigOnly": false,
    "Containers": {},
    "Options": {},
    "Labels": {}
  },
  {
    "Name": "none",
    "Id": "c9668d06973d976527e913ba207a3819275649f347390379ec8356db375cfde3",
    "Created": "2020-08-18T17:18:18.428827132+08:00",
    "Scope": "local",
    "Driver": "null",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": []
    },
    "Internal": false,
    "Attachable": false,
    "Ingress": false,
    "ConfigFrom": {
      "Network": ""
    },
    "ConfigOnly": false,
    "Containers": {},
    "Options": {},
    "Labels": {}
  },
  {
    "Name": "bridge",
    "Id": "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
    "Created": "2021-03-18T14:36:04.290821903+08:00",
    "Scope": "local",
    "Driver": "bridge",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "172.17.0.0/16",
          "Gateway": "172.17.0.1"
        }
      ]
    },
    "Internal": false,
    "Attachable": false,
    "Ingress": false,
    "ConfigFrom": {
      "Network": ""
    },
    "ConfigOnly": false,
    "Containers": {},
    "Options": {
      "com.docker.network.bridge.default_bridge": "true",
      "com.docker.network.bridge.enable_icc": "true",
      "com.docker.network.bridge.enable_ip_masquerade": "true",
      "com.docker.network.bridge.host_binding_ipv4": "0.0.0.0",
      "com.docker.network.bridge.name": "docker0",
      "com.docker.network.driver.mtu": "1500"
    },
    "Labels": {}
  }
]`)
	networks, err := parseNetworks(data)
	if err != nil {
		t.Fatalf("fail to parse networks: %v", err)
	}
	networkLabels := getNetworkLabelsByNetworkID(networks)

	tests := []struct {
		name    string
		c       container
		want    []*promutils.Labels
		wantErr bool
	}{
		{
			name: "NetworkMode!=host",
			c: container{
				ID:    "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
				Names: []string{"/crow-server"},
				Labels: map[string]string{
					"com.docker.compose.config-hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"com.docker.compose.container-number": "1",
					"com.docker.compose.oneoff":           "False",
					"com.docker.compose.project":          "crowserver",
					"com.docker.compose.service":          "crow-server",
					"com.docker.compose.version":          "1.11.2",
				},
				HostConfig: struct {
					NetworkMode string
				}{
					NetworkMode: "bridge",
				},
				NetworkSettings: struct {
					Networks map[string]struct {
						IPAddress string
						NetworkID string
					}
				}{
					Networks: map[string]struct {
						IPAddress string
						NetworkID string
					}{
						"host": {
							IPAddress: "172.17.0.2",
							NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                "172.17.0.2:8012",
					"__meta_docker_container_id": "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
					"__meta_docker_container_label_com_docker_compose_config_hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"__meta_docker_container_label_com_docker_compose_container_number": "1",
					"__meta_docker_container_label_com_docker_compose_oneoff":           "False",
					"__meta_docker_container_label_com_docker_compose_project":          "crowserver",
					"__meta_docker_container_label_com_docker_compose_service":          "crow-server",
					"__meta_docker_container_label_com_docker_compose_version":          "1.11.2",
					"__meta_docker_container_name":                                      "/crow-server",
					"__meta_docker_container_network_mode":                              "bridge",
					"__meta_docker_network_id":                                          "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
					"__meta_docker_network_ingress":                                     "false",
					"__meta_docker_network_internal":                                    "false",
					"__meta_docker_network_ip":                                          "172.17.0.2",
					"__meta_docker_network_name":                                        "bridge",
					"__meta_docker_network_scope":                                       "local",
				}),
			},
		},
		{
			name: "NetworkMode=host",
			c: container{
				ID:    "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
				Names: []string{"/crow-server"},
				Labels: map[string]string{
					"com.docker.compose.config-hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"com.docker.compose.container-number": "1",
					"com.docker.compose.oneoff":           "False",
					"com.docker.compose.project":          "crowserver",
					"com.docker.compose.service":          "crow-server",
					"com.docker.compose.version":          "1.11.2",
				},
				HostConfig: struct {
					NetworkMode string
				}{
					NetworkMode: "host",
				},
				NetworkSettings: struct {
					Networks map[string]struct {
						IPAddress string
						NetworkID string
					}
				}{
					Networks: map[string]struct {
						IPAddress string
						NetworkID string
					}{
						"host": {
							IPAddress: "172.17.0.2",
							NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                "foobar",
					"__meta_docker_container_id": "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
					"__meta_docker_container_label_com_docker_compose_config_hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"__meta_docker_container_label_com_docker_compose_container_number": "1",
					"__meta_docker_container_label_com_docker_compose_oneoff":           "False",
					"__meta_docker_container_label_com_docker_compose_project":          "crowserver",
					"__meta_docker_container_label_com_docker_compose_service":          "crow-server",
					"__meta_docker_container_label_com_docker_compose_version":          "1.11.2",
					"__meta_docker_container_name":                                      "/crow-server",
					"__meta_docker_container_network_mode":                              "host",
					"__meta_docker_network_id":                                          "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
					"__meta_docker_network_ingress":                                     "false",
					"__meta_docker_network_internal":                                    "false",
					"__meta_docker_network_ip":                                          "172.17.0.2",
					"__meta_docker_network_name":                                        "bridge",
					"__meta_docker_network_scope":                                       "local",
				}),
			},
		},
		{
			name: "get labels from a container",
			c: container{
				ID:    "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
				Names: []string{"/crow-server"},
				Labels: map[string]string{
					"com.docker.compose.config-hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"com.docker.compose.container-number": "1",
					"com.docker.compose.oneoff":           "False",
					"com.docker.compose.project":          "crowserver",
					"com.docker.compose.service":          "crow-server",
					"com.docker.compose.version":          "1.11.2",
				},
				Ports: []struct {
					IP          string
					PrivatePort int
					PublicPort  int
					Type        string
				}{{
					IP:          "0.0.0.0",
					PrivatePort: 8080,
					PublicPort:  18081,
					Type:        "tcp",
				}},
				HostConfig: struct {
					NetworkMode string
				}{
					NetworkMode: "bridge",
				},
				NetworkSettings: struct {
					Networks map[string]struct {
						IPAddress string
						NetworkID string
					}
				}{
					Networks: map[string]struct {
						IPAddress string
						NetworkID string
					}{
						"bridge": {
							IPAddress: "172.17.0.2",
							NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                "172.17.0.2:8080",
					"__meta_docker_container_id": "90bc3b31aa13da5c0b11af2e228d54b38428a84e25d4e249ae9e9c95e51a0700",
					"__meta_docker_container_label_com_docker_compose_config_hash":      "c9f0bd5bb31921f94cff367d819a30a0cc08d4399080897a6c5cd74b983156ec",
					"__meta_docker_container_label_com_docker_compose_container_number": "1",
					"__meta_docker_container_label_com_docker_compose_oneoff":           "False",
					"__meta_docker_container_label_com_docker_compose_project":          "crowserver",
					"__meta_docker_container_label_com_docker_compose_service":          "crow-server",
					"__meta_docker_container_label_com_docker_compose_version":          "1.11.2",
					"__meta_docker_container_name":                                      "/crow-server",
					"__meta_docker_container_network_mode":                              "bridge",
					"__meta_docker_network_id":                                          "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
					"__meta_docker_network_ingress":                                     "false",
					"__meta_docker_network_internal":                                    "false",
					"__meta_docker_network_ip":                                          "172.17.0.2",
					"__meta_docker_network_name":                                        "bridge",
					"__meta_docker_network_scope":                                       "local",
					"__meta_docker_port_private":                                        "8080",
					"__meta_docker_port_public":                                         "18081",
					"__meta_docker_port_public_ip":                                      "0.0.0.0",
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labelss := addContainersLabels([]container{tt.c}, networkLabels, 8012, "foobar")
			if (err != nil) != tt.wantErr {
				t.Errorf("addContainersLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			discoveryutils.TestEqualLabelss(t, labelss, tt.want)
		})
	}
}
