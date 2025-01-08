package marathon

import (
	"encoding/json"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetAppLabels(t *testing.T) {
	jsonResponse := `{
    "apps": [
        {
            "id": "/app-test",
            "tasks": [
                {
                    "id": "app-test.b44e0f85-a586-11ef-b308-02429c08177d",
                    "host": "pre2",
                    "ports": [
                        20651
                    ],
                    "ipAddresses": [
                        {
                            "ipAddress": "172.17.0.12",
                            "protocol": "IPv4"
                        }
                    ]
                },
                {
                    "id": "app-test.7cbfcce5-a586-11ef-b308-02429c08177d",
                    "host": "pre3",
                    "ports": [
                        20681
                    ],
                    "ipAddresses": [
                        {
                            "ipAddress": "172.17.0.19",
                            "protocol": "IPv4"
                        }
                    ]
                },
                {
                    "id": "app-test.b0a7c3d4-a586-11ef-b308-02429c08177d",
                    "host": "pre1",
                    "ports": [
                        20337
                    ],
                    "ipAddresses": [
                        {
                            "ipAddress": "172.17.0.13",
                            "protocol": "IPv4"
                        }
                    ]
                },
                {
                    "id": "app-test.7c26c12c-a586-11ef-b308-02429c08177d",
                    "host": "pre2",
                    "ports": [
                        20668
                    ],
                    "ipAddresses": [
                        {
                            "ipAddress": "172.17.0.9",
                            "protocol": "IPv4"
                        }
                    ]
                }
            ],
            "tasksRunning": 4,
            "labels": {
                "HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
                "HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
                "HAPROXY_0_PATH": "-i /app-test",
                "HAPROXY_0_VHOST": "pre1,pre2,pre3,pre4,pre1:9000,pre",
                "HAPROXY_GROUP": "local",
                "micrometer_prometheus": "/actuator/prometheus"
            },
            "container": {
                "docker": {
                    "image": "docker.local/app-test:1.0.0",
                    "portMappings": null
                },
                "portMappings": [
                    {
                        "labels": {
							"portMappingsLabel1": "portMappingsValue1"
						},
                        "containerPort": 8080,
                        "hostPort": 0,
                        "servicePort": 11002
                    }
                ]
            },
            "portDefinitions": [],
            "networks": [
                {
                    "name": "",
                    "mode": "container/bridge"
                }
            ],
            "requirePorts": false
        },
{
            "id": "/app-test-for-port-definition",
            "tasks": [
                {
                    "id": "app-test.b44e0f85-a586-11ef-b308-02429c08177d",
                    "host": "pre2",
                    "ports": [
                        20651
                    ],
                    "ipAddresses": [
                        {
                            "ipAddress": "172.17.0.12",
                            "protocol": "IPv4"
                        }
                    ]
                }
            ],
            "tasksRunning": 1,
            "labels": {
                "HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
                "HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
                "HAPROXY_0_PATH": "-i /app-test",
                "HAPROXY_0_VHOST": "pre1,pre2,pre3,pre4,pre1:9000,pre",
                "HAPROXY_GROUP": "local",
                "micrometer_prometheus": "/actuator/prometheus"
            },
            "container": {
                "docker": {
                    "image": "docker.local/app-test:1.0.0",
                    "portMappings": null
                },
                "portMappings": []
            },
            "portDefinitions": [
				{
					"port" : 9091,
					"name" : "prometheus",
					"labels": {
						"metrics": "/metrics"
					}
				}
			],
            "networks": [
                {
                    "name": "",
                    "mode": "container/bridge"
                }
            ],
            "requirePorts": false
        }
    ]
}`
	var appList *AppList
	if err := json.Unmarshal([]byte(jsonResponse), &appList); err != nil {
		t.Fatalf("unmarshal jsonResponse failed: %s", err)
	}
	result := getAppsLabels(appList)
	expect := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":         "pre2:20651",
			"__meta_marathon_app": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_PATH":                        "-i /app-test",
			"__meta_marathon_app_label_HAPROXY_0_VHOST":                       "pre1,pre2,pre3,pre4,pre1:9000,pre",
			"__meta_marathon_app_label_HAPROXY_GROUP":                         "local",
			"__meta_marathon_app_label_micrometer_prometheus":                 "/actuator/prometheus",
			"__meta_marathon_image":                                           "docker.local/app-test:1.0.0",
			"__meta_marathon_port_index":                                      "0",
			"__meta_marathon_task":                                            "app-test.b44e0f85-a586-11ef-b308-02429c08177d",
			"__meta_marathon_port_mapping_label_portMappingsLabel1":           "portMappingsValue1",
		}), promutils.NewLabelsFromMap(map[string]string{
			"__address__":         "pre3:20681",
			"__meta_marathon_app": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_PATH":                        "-i /app-test",
			"__meta_marathon_app_label_HAPROXY_0_VHOST":                       "pre1,pre2,pre3,pre4,pre1:9000,pre",
			"__meta_marathon_app_label_HAPROXY_GROUP":                         "local",
			"__meta_marathon_app_label_micrometer_prometheus":                 "/actuator/prometheus",
			"__meta_marathon_image":                                           "docker.local/app-test:1.0.0",
			"__meta_marathon_port_index":                                      "0",
			"__meta_marathon_task":                                            "app-test.7cbfcce5-a586-11ef-b308-02429c08177d",
			"__meta_marathon_port_mapping_label_portMappingsLabel1":           "portMappingsValue1",
		}), promutils.NewLabelsFromMap(map[string]string{
			"__address__":         "pre1:20337",
			"__meta_marathon_app": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_PATH":                        "-i /app-test",
			"__meta_marathon_app_label_HAPROXY_0_VHOST":                       "pre1,pre2,pre3,pre4,pre1:9000,pre",
			"__meta_marathon_app_label_HAPROXY_GROUP":                         "local",
			"__meta_marathon_app_label_micrometer_prometheus":                 "/actuator/prometheus",
			"__meta_marathon_image":                                           "docker.local/app-test:1.0.0",
			"__meta_marathon_port_index":                                      "0",
			"__meta_marathon_task":                                            "app-test.b0a7c3d4-a586-11ef-b308-02429c08177d",
			"__meta_marathon_port_mapping_label_portMappingsLabel1":           "portMappingsValue1",
		}), promutils.NewLabelsFromMap(map[string]string{
			"__address__":         "pre2:20668",
			"__meta_marathon_app": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_PATH":                        "-i /app-test",
			"__meta_marathon_app_label_HAPROXY_0_VHOST":                       "pre1,pre2,pre3,pre4,pre1:9000,pre",
			"__meta_marathon_app_label_HAPROXY_GROUP":                         "local",
			"__meta_marathon_app_label_micrometer_prometheus":                 "/actuator/prometheus",
			"__meta_marathon_image":                                           "docker.local/app-test:1.0.0",
			"__meta_marathon_port_index":                                      "0",
			"__meta_marathon_task":                                            "app-test.7c26c12c-a586-11ef-b308-02429c08177d",
			"__meta_marathon_port_mapping_label_portMappingsLabel1":           "portMappingsValue1",
		}), promutils.NewLabelsFromMap(map[string]string{
			"__address__":         "pre2:20651",
			"__meta_marathon_app": "/app-test-for-port-definition",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_GLUE": "  reqirep  \"^([^ :]*)\\ {proxypath}/?(.*)\" \"\\1\\ /\\2\"\n",
			"__meta_marathon_app_label_HAPROXY_0_HTTP_BACKEND_PROXYPASS_PATH": "/app-test",
			"__meta_marathon_app_label_HAPROXY_0_PATH":                        "-i /app-test",
			"__meta_marathon_app_label_HAPROXY_0_VHOST":                       "pre1,pre2,pre3,pre4,pre1:9000,pre",
			"__meta_marathon_app_label_HAPROXY_GROUP":                         "local",
			"__meta_marathon_app_label_micrometer_prometheus":                 "/actuator/prometheus",
			"__meta_marathon_image":                                           "docker.local/app-test:1.0.0",
			"__meta_marathon_port_index":                                      "0",
			"__meta_marathon_task":                                            "app-test.b44e0f85-a586-11ef-b308-02429c08177d",
			"__meta_marathon_port_definition_label_metrics":                   "/metrics",
		}),
	}
	discoveryutils.TestEqualLabelss(t, result, expect)
}
