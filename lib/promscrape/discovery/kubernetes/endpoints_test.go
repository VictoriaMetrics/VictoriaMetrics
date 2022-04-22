package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseEndpointsListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		objectsByKey, _, err := parseEndpointsList(r)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(objectsByKey) != 0 {
			t.Fatalf("unexpected non-empty objectsByKey: %v", objectsByKey)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
	f(`{"items":[{"metadata":{"labels":[1]}}]}`)
}

func TestParseEndpointsListSuccess(t *testing.T) {
	data := `
{
  "kind": "EndpointsList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/endpoints",
    "resourceVersion": "128055"
  },
  "items": [
    {
      "metadata": {
        "name": "kubernetes",
        "namespace": "default",
        "selfLink": "/api/v1/namespaces/default/endpoints/kubernetes",
        "uid": "0972c7d9-c267-4b93-a090-a417eeb9b385",
        "resourceVersion": "150",
        "creationTimestamp": "2020-03-16T20:44:25Z",
        "labels": {
          "foo": "bar"
        },
        "annotations": {
            "x": "y"
        }
      },
      "subsets": [
        {
          "addresses": [
            {
	      "hostname": "aaa.bbb",
	      "nodeName": "test-node",
              "ip": "172.17.0.2",
              "targetRef": {
                "kind": "Pod",
                "namespace": "kube-system",
                "name": "coredns-6955765f44-lnp6t",
                "uid": "cbddb2b6-5b85-40f1-8819-9a59385169bb",
                "resourceVersion": "124878"
              }
            }
          ],
          "ports": [
            {
              "name": "https",
              "port": 8443,
              "protocol": "TCP"
            }
          ]
        }
      ]
    }
  ]
}
`
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseEndpointsList(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "128055"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}

	sortedLabelss := getSortedLabelss(objectsByKey)
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "172.17.0.2:8443",
			"__meta_kubernetes_endpoint_address_target_kind":  "Pod",
			"__meta_kubernetes_endpoint_address_target_name":  "coredns-6955765f44-lnp6t",
			"__meta_kubernetes_endpoint_hostname":             "aaa.bbb",
			"__meta_kubernetes_endpoint_node_name":            "test-node",
			"__meta_kubernetes_endpoint_port_name":            "https",
			"__meta_kubernetes_endpoint_port_protocol":        "TCP",
			"__meta_kubernetes_endpoint_ready":                "true",
			"__meta_kubernetes_endpoints_name":                "kubernetes",
			"__meta_kubernetes_endpoints_annotation_x":        "y",
			"__meta_kubernetes_endpoints_annotationpresent_x": "true",
			"__meta_kubernetes_endpoints_label_foo":           "bar",
			"__meta_kubernetes_endpoints_labelpresent_foo":    "true",
			"__meta_kubernetes_namespace":                     "default",
		}),
	}
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}

func TestGetEndpointLabels(t *testing.T) {
	type testArgs struct {
		containerPorts map[string][]ContainerPort
		endpointPorts  []EndpointPort
	}
	f := func(name string, args testArgs, wantLabels [][]prompbmarshal.Label) {
		t.Run(name, func(t *testing.T) {
			eps := Endpoints{
				Metadata: ObjectMeta{
					Name:      "test-eps",
					Namespace: "default",
				},
				Subsets: []EndpointSubset{
					{
						Ports: args.endpointPorts,
						Addresses: []EndpointAddress{
							{
								IP: "10.13.15.15",
								TargetRef: ObjectReference{
									Kind:      "Pod",
									Namespace: "default",
									Name:      "test-pod",
								},
							},
						},
					},
				},
			}
			svc := Service{
				Metadata: ObjectMeta{
					Name:      "test-eps",
					Namespace: "default",
				},
				Spec: ServiceSpec{
					Ports: []ServicePort{
						{
							Name: "test-port",
							Port: 8081,
						},
					},
				},
			}
			pod := Pod{
				Metadata: ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: PodStatus{PodIP: "192.168.15.1"},
			}
			for cn, ports := range args.containerPorts {
				pod.Spec.Containers = append(pod.Spec.Containers, Container{Name: cn, Ports: ports})
			}
			var gw groupWatcher
			gw.m = map[string]*urlWatcher{
				"pod": {
					role: "pod",
					objectsByKey: map[string]object{
						"default/test-pod": &pod,
					},
				},
				"service": {
					role: "service",
					objectsByKey: map[string]object{
						"default/test-eps": &svc,
					},
				},
			}
			var sortedLabelss [][]prompbmarshal.Label
			gotLabels := eps.getTargetLabels(&gw)
			for _, lbs := range gotLabels {
				sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(lbs))
			}
			if !areEqualLabelss(sortedLabelss, wantLabels) {
				t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, wantLabels)

			}
		})
	}

	f("1 port from endpoint", testArgs{
		endpointPorts: []EndpointPort{
			{
				Name: "web",
				Port: 8081,
			},
		},
	}, [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.13.15.15:8081",
			"__meta_kubernetes_endpoint_address_target_kind": "Pod",
			"__meta_kubernetes_endpoint_address_target_name": "test-pod",
			"__meta_kubernetes_endpoint_port_name":           "web",
			"__meta_kubernetes_endpoint_port_protocol":       "",
			"__meta_kubernetes_endpoint_ready":               "true",
			"__meta_kubernetes_endpoints_name":               "test-eps",
			"__meta_kubernetes_namespace":                    "default",
			"__meta_kubernetes_pod_host_ip":                  "",
			"__meta_kubernetes_pod_ip":                       "192.168.15.1",
			"__meta_kubernetes_pod_name":                     "test-pod",
			"__meta_kubernetes_pod_node_name":                "",
			"__meta_kubernetes_pod_phase":                    "",
			"__meta_kubernetes_pod_ready":                    "unknown",
			"__meta_kubernetes_pod_uid":                      "",
			"__meta_kubernetes_service_cluster_ip":           "",
			"__meta_kubernetes_service_name":                 "test-eps",
			"__meta_kubernetes_service_type":                 "",
		}),
	})

	f("1 port from endpoint and 1 from pod", testArgs{
		containerPorts: map[string][]ContainerPort{"metrics": {{
			Name:          "http-metrics",
			ContainerPort: 8428,
		}}},
		endpointPorts: []EndpointPort{
			{
				Name: "web",
				Port: 8081,
			},
		},
	}, [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.13.15.15:8081",
			"__meta_kubernetes_endpoint_address_target_kind": "Pod",
			"__meta_kubernetes_endpoint_address_target_name": "test-pod",
			"__meta_kubernetes_endpoint_port_name":           "web",
			"__meta_kubernetes_endpoint_port_protocol":       "",
			"__meta_kubernetes_endpoint_ready":               "true",
			"__meta_kubernetes_endpoints_name":               "test-eps",
			"__meta_kubernetes_namespace":                    "default",
			"__meta_kubernetes_pod_host_ip":                  "",
			"__meta_kubernetes_pod_ip":                       "192.168.15.1",
			"__meta_kubernetes_pod_name":                     "test-pod",
			"__meta_kubernetes_pod_node_name":                "",
			"__meta_kubernetes_pod_phase":                    "",
			"__meta_kubernetes_pod_ready":                    "unknown",
			"__meta_kubernetes_pod_uid":                      "",
			"__meta_kubernetes_service_cluster_ip":           "",
			"__meta_kubernetes_service_name":                 "test-eps",
			"__meta_kubernetes_service_type":                 "",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                                   "192.168.15.1:8428",
			"__meta_kubernetes_namespace":                   "default",
			"__meta_kubernetes_pod_container_name":          "metrics",
			"__meta_kubernetes_pod_container_port_name":     "http-metrics",
			"__meta_kubernetes_pod_container_port_number":   "8428",
			"__meta_kubernetes_pod_container_port_protocol": "",
			"__meta_kubernetes_pod_host_ip":                 "",
			"__meta_kubernetes_pod_ip":                      "192.168.15.1",
			"__meta_kubernetes_pod_name":                    "test-pod",
			"__meta_kubernetes_pod_node_name":               "",
			"__meta_kubernetes_pod_phase":                   "",
			"__meta_kubernetes_pod_ready":                   "unknown",
			"__meta_kubernetes_pod_uid":                     "",
			"__meta_kubernetes_service_cluster_ip":          "",
			"__meta_kubernetes_service_name":                "test-eps",
			"__meta_kubernetes_service_type":                "",
		}),
	})

	f("1 port from endpoint", testArgs{
		containerPorts: map[string][]ContainerPort{"metrics": {{
			Name:          "web",
			ContainerPort: 8428,
		}}},
		endpointPorts: []EndpointPort{
			{
				Name: "web",
				Port: 8428,
			},
		},
	}, [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.13.15.15:8428",
			"__meta_kubernetes_endpoint_address_target_kind": "Pod",
			"__meta_kubernetes_endpoint_address_target_name": "test-pod",
			"__meta_kubernetes_endpoint_port_name":           "web",
			"__meta_kubernetes_endpoint_port_protocol":       "",
			"__meta_kubernetes_endpoint_ready":               "true",
			"__meta_kubernetes_endpoints_name":               "test-eps",
			"__meta_kubernetes_namespace":                    "default",
			"__meta_kubernetes_pod_container_name":           "metrics",
			"__meta_kubernetes_pod_container_port_name":      "web",
			"__meta_kubernetes_pod_container_port_number":    "8428",
			"__meta_kubernetes_pod_container_port_protocol":  "",
			"__meta_kubernetes_pod_host_ip":                  "",
			"__meta_kubernetes_pod_ip":                       "192.168.15.1",
			"__meta_kubernetes_pod_name":                     "test-pod",
			"__meta_kubernetes_pod_node_name":                "",
			"__meta_kubernetes_pod_phase":                    "",
			"__meta_kubernetes_pod_ready":                    "unknown",
			"__meta_kubernetes_pod_uid":                      "",
			"__meta_kubernetes_service_cluster_ip":           "",
			"__meta_kubernetes_service_name":                 "test-eps",
			"__meta_kubernetes_service_type":                 "",
		}),
	})
}
