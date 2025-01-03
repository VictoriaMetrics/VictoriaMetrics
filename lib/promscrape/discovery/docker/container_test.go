package docker

import (
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseContainers(t *testing.T) {
	f := func(data string, resultExpected []container) {
		t.Helper()

		result, err := parseContainers([]byte(data))
		if err != nil {
			t.Fatalf("parseContainers() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	data := `[
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
]`
	resultExpected := []container{
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
			Ports: []containerPort{
				{
					IP:          "0.0.0.0",
					PrivatePort: 8080,
					PublicPort:  18081,
					Type:        "tcp",
				},
			},
			HostConfig: containerHostConfig{
				NetworkMode: "bridge",
			},
			NetworkSettings: containerNetworkSettings{
				Networks: map[string]containerNetwork{
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
			Ports: []containerPort{
				{
					IP:          "0.0.0.0",
					PrivatePort: 8080,
					PublicPort:  18082,
					Type:        "tcp",
				},
			},
			HostConfig: containerHostConfig{
				NetworkMode: "bridge",
			},
			NetworkSettings: containerNetworkSettings{
				Networks: map[string]containerNetwork{
					"bridge": {
						IPAddress: "172.17.0.3",
						NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
					},
				},
			},
		},
	}

	f(data, resultExpected)
}

func TestAddContainerLabels(t *testing.T) {
	f := func(c container, networkLabels map[string]*promutils.Labels, labelssExpected []*promutils.Labels) {
		t.Helper()

		labelss := addContainersLabels([]container{c}, networkLabels, 8012, "foobar", false)
		discoveryutils.TestEqualLabelss(t, labelss, labelssExpected)
	}

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

	// NetworkMode != host
	c := container{
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
		HostConfig: containerHostConfig{
			NetworkMode: "bridge",
		},
		NetworkSettings: containerNetworkSettings{
			Networks: map[string]containerNetwork{
				"host": {
					IPAddress: "172.17.0.2",
					NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
				},
			},
		},
	}
	labelssExpected := []*promutils.Labels{
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
	}
	f(c, networkLabels, labelssExpected)

	// NetworkMode=host
	c = container{
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
		HostConfig: containerHostConfig{
			NetworkMode: "host",
		},
		NetworkSettings: containerNetworkSettings{
			Networks: map[string]containerNetwork{
				"host": {
					IPAddress: "172.17.0.2",
					NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
				},
			},
		},
	}
	labelssExpected = []*promutils.Labels{
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
	}
	f(c, networkLabels, labelssExpected)

	// get labels from a container
	c = container{
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
		Ports: []containerPort{
			{
				IP:          "0.0.0.0",
				PrivatePort: 8080,
				PublicPort:  18081,
				Type:        "tcp",
			},
		},
		HostConfig: containerHostConfig{
			NetworkMode: "bridge",
		},
		NetworkSettings: containerNetworkSettings{
			Networks: map[string]containerNetwork{
				"bridge": {
					IPAddress: "172.17.0.2",
					NetworkID: "1dd8d1a8bef59943345c7231d7ce8268333ff5a8c5b3c94881e6b4742b447634",
				},
			},
		},
	}
	labelssExpected = []*promutils.Labels{
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
	}
	f(c, networkLabels, labelssExpected)
}

func TestDockerMultiNetworkLabels(t *testing.T) {
	networkJSON := []byte(`[
  {
    "Name": "dockersd_private",
    "Id": "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
    "Created": "2022-03-25T09:21:17.718370976+08:00",
    "Scope": "local",
    "Driver": "bridge",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "172.20.0.1/16"
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
    "Options": {},
    "Labels": {}
  },
  {
    "Name": "dockersd_private1",
    "Id": "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
    "Created": "2022-03-25T09:21:17.718370976+08:00",
    "Scope": "local",
    "Driver": "bridge",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "172.21.0.1/16"
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
    "Options": {},
    "Labels": {}
  }
]`)
	containerJSON := []byte(`[
  {
    "Id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
    "Names": [
      "/dockersd_multi_networks"
    ],
    "Image": "mysql:5.7.29",
    "ImageID": "sha256:16ae2f4625ba63a250462bedeece422e741de9f0caf3b1d89fd5b257aca80cd1",
    "Command": "mysqld",
    "Created": 1616273136,
    "Ports": [
      {
        "PrivatePort": 3306,
        "Type": "tcp"
      },
      {
        "PrivatePort": 33060,
        "Type": "tcp"
      }
    ],
    "Labels": {
      "com.docker.compose.project": "dockersd",
      "com.docker.compose.service": "mysql",
      "com.docker.compose.version": "2.2.2"
    },
    "State": "running",
    "Status": "Up 40 seconds",
    "HostConfig": {
      "NetworkMode": "dockersd_private_none"
    },
    "NetworkSettings": {
      "Networks": {
        "dockersd_private": {
          "IPAMConfig": null,
          "Links": null,
          "Aliases": null,
          "NetworkID": "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
          "EndpointID": "972d6807997369605ace863af58de6cb90c787a5bf2ffc4105662d393ae539b7",
          "Gateway": "172.20.0.1",
          "IPAddress": "172.20.0.3",
          "IPPrefixLen": 16,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "MacAddress": "02:42:ac:14:00:02",
          "DriverOpts": null
        },
        "dockersd_private1": {
          "IPAMConfig": {},
          "Links": null,
          "Aliases": [
            "mysql",
            "mysql",
            "f9ade4b83199"
          ],
          "NetworkID": "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
          "EndpointID": "91a98405344ee1cb7d977cafabe634837876651544b32da20a5e0155868e6f5f",
          "Gateway": "172.21.0.1",
          "IPAddress": "172.21.0.3",
          "IPPrefixLen": 24,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "MacAddress": "02:42:ac:15:00:02",
          "DriverOpts": null
        }
      }
    },
    "Mounts": []
  }
]
`)

	networks, err := parseNetworks(networkJSON)
	if err != nil {
		t.Fatalf("fail to parse networks: %v", err)
	}
	networkLabels := getNetworkLabelsByNetworkID(networks)

	containers, err := parseContainers(containerJSON)
	if err != nil {
		t.Fatalf("parseContainers() error: %s", err)
	}

	// matchFirstNetwork = false
	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.3:3306",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.3",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "3306",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.3:33060",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.3",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "33060",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.21.0.3:3306",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.21.0.3",
			"__meta_docker_network_name":                               "dockersd_private1",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "3306",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.21.0.3:33060",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.21.0.3",
			"__meta_docker_network_name":                               "dockersd_private1",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "33060",
		}),
	}
	labelss := addContainersLabels(containers, networkLabels, 80, "localhost", false)
	discoveryutils.TestEqualLabelss(t, sortLabelss(labelss), sortLabelss(labelssExpected))

	// matchFirstNetwork = true, so labels of `dockersd_private1` are removed
	labelssExpected = []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.3:3306",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.3",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "3306",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.3:33060",
			"__meta_docker_container_id": "f84b2a0cfaa58d9e70b0657e2b3c6f44f0e973de4163a871299b4acf127b224f",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_multi_networks",
			"__meta_docker_container_network_mode":                     "dockersd_private_none",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.3",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "33060",
		}),
	}
	labelss = addContainersLabels(containers, networkLabels, 80, "localhost", true)
	discoveryutils.TestEqualLabelss(t, sortLabelss(labelss), sortLabelss(labelssExpected))
}

// TestDockerLinkedNetworkSettings test the case when `NetworkMode` of a container linked to another container:
// "container:f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8"
func TestDockerLinkedNetworkSettings(t *testing.T) {
	networkJSON := []byte(`[
  {
    "Name": "dockersd_private",
    "Id": "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
    "Created": "2022-03-25T09:21:17.718370976+08:00",
    "Scope": "local",
    "Driver": "bridge",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "172.20.0.1/16"
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
    "Options": {},
    "Labels": {}
  },
  {
    "Name": "dockersd_private1",
    "Id": "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
    "Created": "2022-03-25T09:21:17.718370976+08:00",
    "Scope": "local",
    "Driver": "bridge",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "172.21.0.1/16"
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
    "Options": {},
    "Labels": {}
  }
]`)
	containerJSON := []byte(`[
{
    "Id": "f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
    "Names": [
      "/dockersd_mysql"
    ],
    "Image": "mysql:5.7.29",
    "ImageID": "sha256:5d9483f9a7b21c87e0f5b9776c3e06567603c28c0062013eda127c968175f5e8",
    "Command": "mysqld",
    "Created": 1616273136,
    "Ports": [
      {
        "PrivatePort": 3306,
        "Type": "tcp"
      },
      {
        "PrivatePort": 33060,
        "Type": "tcp"
      }
    ],
    "Labels": {
      "com.docker.compose.project": "dockersd",
      "com.docker.compose.service": "mysql",
      "com.docker.compose.version": "2.2.2"
    },
    "State": "running",
    "Status": "Up 40 seconds",
    "HostConfig": {
      "NetworkMode": "dockersd_private"
    },
    "NetworkSettings": {
      "Networks": {
        "dockersd_private": {
          "IPAMConfig": null,
          "Links": null,
          "Aliases": null,
          "NetworkID": "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
          "EndpointID": "80f8a61b37701a9991bb98c75ddd23fd9b7c16b5575ca81343f6b44ff4a2a9d9",
          "Gateway": "172.20.0.1",
          "IPAddress": "172.20.0.2",
          "IPPrefixLen": 16,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "MacAddress": "02:42:ac:14:00:0a",
          "DriverOpts": null
        },
        "dockersd_private1": {
          "IPAMConfig": {},
          "Links": null,
          "Aliases": [
            "mysql",
            "mysql",
            "f9ade4b83199"
          ],
          "NetworkID": "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
          "EndpointID": "f80921d10e78c99a5907705aae75befea40c3d3e9f820e66ab392f7274be16b8",
          "Gateway": "172.21.0.1",
          "IPAddress": "172.21.0.2",
          "IPPrefixLen": 24,
          "IPv6Gateway": "",
          "GlobalIPv6Address": "",
          "GlobalIPv6PrefixLen": 0,
          "MacAddress": "02:42:ac:15:00:02",
          "DriverOpts": null
        }
      }
    },
    "Mounts": []
  },
  {
    "Id": "59bf76e8816af98856b90dd619c91027145ca501043b1c51756d03b085882e06",
    "Names": [
      "/dockersd_mysql_exporter"
    ],
    "Image": "prom/mysqld-exporter:latest",
    "ImageID": "sha256:121b8a7cd0525dd89aaec58ad7d34c3bb3714740e5a67daf6510ccf71ab219a9",
    "Command": "/bin/mysqld_exporter",
    "Created": 1616273136,
    "Ports": [
      {
        "PrivatePort": 9104,
        "Type": "tcp"
      }
    ],
    "Labels": {
      "com.docker.compose.project": "dockersd",
      "com.docker.compose.service": "mysqlexporter",
      "com.docker.compose.version": "2.2.2",
      "maintainer": "The Prometheus Authors <prometheus-developers@googlegroups.com>"
    },
    "State": "running",
    "Status": "Up 40 seconds",
    "HostConfig": {
      "NetworkMode": "container:f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8"
    },
    "NetworkSettings": {
      "Networks": {}
    },
    "Mounts": []
  }
]
`)

	networks, err := parseNetworks(networkJSON)
	if err != nil {
		t.Fatalf("fail to parse networks: %v", err)
	}
	networkLabels := getNetworkLabelsByNetworkID(networks)

	containers, err := parseContainers(containerJSON)
	if err != nil {
		t.Fatalf("parseContainers() error: %s", err)
	}

	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.2:3306",
			"__meta_docker_container_id": "f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_mysql",
			"__meta_docker_container_network_mode":                     "dockersd_private",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.2",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "3306",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.2:33060",
			"__meta_docker_container_id": "f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_mysql",
			"__meta_docker_container_network_mode":                     "dockersd_private",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.2",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "33060",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.21.0.2:3306",
			"__meta_docker_container_id": "f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_mysql",
			"__meta_docker_container_network_mode":                     "dockersd_private",
			"__meta_docker_network_id":                                 "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.21.0.2",
			"__meta_docker_network_name":                               "dockersd_private1",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "3306",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.21.0.2:33060",
			"__meta_docker_container_id": "f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysql",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_name":                             "/dockersd_mysql",
			"__meta_docker_container_network_mode":                     "dockersd_private",
			"__meta_docker_network_id":                                 "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.21.0.2",
			"__meta_docker_network_name":                               "dockersd_private1",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "33060",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.20.0.2:9104",
			"__meta_docker_container_id": "59bf76e8816af98856b90dd619c91027145ca501043b1c51756d03b085882e06",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysqlexporter",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_label_maintainer":                 "The Prometheus Authors <prometheus-developers@googlegroups.com>",
			"__meta_docker_container_name":                             "/dockersd_mysql_exporter",
			"__meta_docker_container_network_mode":                     "container:f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_network_id":                                 "e804771e55254a360fdb70dfdd78d3610fdde231b14ef2f837a00ac1eeb9e601",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.20.0.2",
			"__meta_docker_network_name":                               "dockersd_private",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "9104",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                "172.21.0.2:9104",
			"__meta_docker_container_id": "59bf76e8816af98856b90dd619c91027145ca501043b1c51756d03b085882e06",
			"__meta_docker_container_label_com_docker_compose_project": "dockersd",
			"__meta_docker_container_label_com_docker_compose_service": "mysqlexporter",
			"__meta_docker_container_label_com_docker_compose_version": "2.2.2",
			"__meta_docker_container_label_maintainer":                 "The Prometheus Authors <prometheus-developers@googlegroups.com>",
			"__meta_docker_container_name":                             "/dockersd_mysql_exporter",
			"__meta_docker_container_network_mode":                     "container:f9ade4b83199d6f83020b7c0bfd1e8281b19dbf9e6cef2cf89bc45c8f8d20fe8",
			"__meta_docker_network_id":                                 "bfcf66a6b64f7d518f009e34290dc3f3c66a08164257ad1afc3bd31d75f656e8",
			"__meta_docker_network_ingress":                            "false",
			"__meta_docker_network_internal":                           "false",
			"__meta_docker_network_ip":                                 "172.21.0.2",
			"__meta_docker_network_name":                               "dockersd_private1",
			"__meta_docker_network_scope":                              "local",
			"__meta_docker_port_private":                               "9104",
		}),
	}
	labelss := addContainersLabels(containers, networkLabels, 80, "localhost", false)

	discoveryutils.TestEqualLabelss(t, sortLabelss(labelss), sortLabelss(labelssExpected))

}

func sortLabelss(labelss []*promutils.Labels) []*promutils.Labels {
	sort.Slice(labelss, func(i, j int) bool {
		return labelss[i].Get("__address__")+labelss[i].Get("__meta_docker_container_id") < labelss[j].Get("__address__")+labelss[j].Get("__meta_docker_container_id")
	})
	return labelss
}
