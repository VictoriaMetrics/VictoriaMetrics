package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
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
	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
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

func TestGetEndpointsLabels(t *testing.T) {
	type testArgs struct {
		containerPorts map[string][]ContainerPort
		endpointPorts  []EndpointPort
	}
	f := func(t *testing.T, args testArgs, wantLabels []*promutils.Labels) {
		t.Helper()
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
				ClusterIP: "1.2.3.4",
				Type:      "service-type",
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
				UID:       "pod-uid",
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: PodSpec{
				NodeName: "test-node",
			},
			Status: PodStatus{
				Phase:  "abc",
				PodIP:  "192.168.15.1",
				HostIP: "4.5.6.7",
			},
		}
		node := Node{
			Metadata: ObjectMeta{
				Labels: promutils.NewLabelsFromMap(map[string]string{"node-label": "xyz"}),
			},
		}
		for cn, ports := range args.containerPorts {
			pod.Spec.Containers = append(pod.Spec.Containers, Container{
				Name:  cn,
				Image: "test-image",
				Ports: ports,
			})
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
			"node": {
				role: "node",
				objectsByKey: map[string]object{
					"/test-node": &node,
				},
			},
		}
		gw.attachNodeMetadata = true
		var sortedLabelss []*promutils.Labels
		gotLabels := eps.getTargetLabels(&gw)
		for _, lbs := range gotLabels {
			lbs.Sort()
			sortedLabelss = append(sortedLabelss, lbs)
		}
		if !areEqualLabelss(sortedLabelss, wantLabels) {
			t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, wantLabels)
		}
	}

	t.Run("1 port from endpoint", func(t *testing.T) {
		f(t, testArgs{
			endpointPorts: []EndpointPort{
				{
					Name:     "web",
					Port:     8081,
					Protocol: "foobar",
				},
			},
		}, []*promutils.Labels{
			promutils.NewLabelsFromMap(map[string]string{
				"__address__": "10.13.15.15:8081",
				"__meta_kubernetes_endpoint_address_target_kind": "Pod",
				"__meta_kubernetes_endpoint_address_target_name": "test-pod",
				"__meta_kubernetes_endpoint_port_name":           "web",
				"__meta_kubernetes_endpoint_port_protocol":       "foobar",
				"__meta_kubernetes_endpoint_ready":               "true",
				"__meta_kubernetes_endpoints_name":               "test-eps",
				"__meta_kubernetes_namespace":                    "default",
				"__meta_kubernetes_node_label_node_label":        "xyz",
				"__meta_kubernetes_node_labelpresent_node_label": "true",
				"__meta_kubernetes_node_name":                    "test-node",
				"__meta_kubernetes_pod_host_ip":                  "4.5.6.7",
				"__meta_kubernetes_pod_ip":                       "192.168.15.1",
				"__meta_kubernetes_pod_name":                     "test-pod",
				"__meta_kubernetes_pod_node_name":                "test-node",
				"__meta_kubernetes_pod_phase":                    "abc",
				"__meta_kubernetes_pod_ready":                    "unknown",
				"__meta_kubernetes_pod_uid":                      "pod-uid",
				"__meta_kubernetes_service_cluster_ip":           "1.2.3.4",
				"__meta_kubernetes_service_name":                 "test-eps",
				"__meta_kubernetes_service_type":                 "service-type",
			}),
		})
	})

	t.Run("1 port from endpoint and 1 from pod", func(t *testing.T) {
		f(t, testArgs{
			containerPorts: map[string][]ContainerPort{"metrics": {{
				Name:          "http-metrics",
				ContainerPort: 8428,
				Protocol:      "foobar",
			}}},
			endpointPorts: []EndpointPort{
				{
					Name:     "web",
					Port:     8081,
					Protocol: "https",
				},
			},
		}, []*promutils.Labels{
			promutils.NewLabelsFromMap(map[string]string{
				"__address__": "10.13.15.15:8081",
				"__meta_kubernetes_endpoint_address_target_kind": "Pod",
				"__meta_kubernetes_endpoint_address_target_name": "test-pod",
				"__meta_kubernetes_endpoint_port_name":           "web",
				"__meta_kubernetes_endpoint_port_protocol":       "https",
				"__meta_kubernetes_endpoint_ready":               "true",
				"__meta_kubernetes_endpoints_name":               "test-eps",
				"__meta_kubernetes_namespace":                    "default",
				"__meta_kubernetes_node_label_node_label":        "xyz",
				"__meta_kubernetes_node_labelpresent_node_label": "true",
				"__meta_kubernetes_node_name":                    "test-node",
				"__meta_kubernetes_pod_host_ip":                  "4.5.6.7",
				"__meta_kubernetes_pod_ip":                       "192.168.15.1",
				"__meta_kubernetes_pod_name":                     "test-pod",
				"__meta_kubernetes_pod_node_name":                "test-node",
				"__meta_kubernetes_pod_phase":                    "abc",
				"__meta_kubernetes_pod_ready":                    "unknown",
				"__meta_kubernetes_pod_uid":                      "pod-uid",
				"__meta_kubernetes_service_cluster_ip":           "1.2.3.4",
				"__meta_kubernetes_service_name":                 "test-eps",
				"__meta_kubernetes_service_type":                 "service-type",
			}),
			promutils.NewLabelsFromMap(map[string]string{
				"__address__": "192.168.15.1:8428",
				"__meta_kubernetes_endpoint_address_target_kind": "Pod",
				"__meta_kubernetes_endpoint_address_target_name": "test-pod",
				"__meta_kubernetes_endpoints_name":               "test-eps",
				"__meta_kubernetes_namespace":                    "default",
				"__meta_kubernetes_node_label_node_label":        "xyz",
				"__meta_kubernetes_node_labelpresent_node_label": "true",
				"__meta_kubernetes_node_name":                    "test-node",
				"__meta_kubernetes_pod_container_image":          "test-image",
				"__meta_kubernetes_pod_container_name":           "metrics",
				"__meta_kubernetes_pod_container_port_name":      "http-metrics",
				"__meta_kubernetes_pod_container_port_number":    "8428",
				"__meta_kubernetes_pod_container_port_protocol":  "foobar",
				"__meta_kubernetes_pod_host_ip":                  "4.5.6.7",
				"__meta_kubernetes_pod_ip":                       "192.168.15.1",
				"__meta_kubernetes_pod_name":                     "test-pod",
				"__meta_kubernetes_pod_node_name":                "test-node",
				"__meta_kubernetes_pod_phase":                    "abc",
				"__meta_kubernetes_pod_ready":                    "unknown",
				"__meta_kubernetes_pod_uid":                      "pod-uid",
				"__meta_kubernetes_service_cluster_ip":           "1.2.3.4",
				"__meta_kubernetes_service_name":                 "test-eps",
				"__meta_kubernetes_service_type":                 "service-type",
			}),
		})
	})

	t.Run("1 port from endpoint", func(t *testing.T) {
		f(t, testArgs{
			containerPorts: map[string][]ContainerPort{"metrics": {{
				Name:          "web",
				ContainerPort: 8428,
				Protocol:      "sdc",
			}}},
			endpointPorts: []EndpointPort{
				{
					Name:     "web",
					Port:     8428,
					Protocol: "xabc",
				},
			},
		}, []*promutils.Labels{
			promutils.NewLabelsFromMap(map[string]string{
				"__address__": "10.13.15.15:8428",
				"__meta_kubernetes_endpoint_address_target_kind": "Pod",
				"__meta_kubernetes_endpoint_address_target_name": "test-pod",
				"__meta_kubernetes_endpoint_port_name":           "web",
				"__meta_kubernetes_endpoint_port_protocol":       "xabc",
				"__meta_kubernetes_endpoint_ready":               "true",
				"__meta_kubernetes_endpoints_name":               "test-eps",
				"__meta_kubernetes_namespace":                    "default",
				"__meta_kubernetes_node_label_node_label":        "xyz",
				"__meta_kubernetes_node_labelpresent_node_label": "true",
				"__meta_kubernetes_node_name":                    "test-node",
				"__meta_kubernetes_pod_container_image":          "test-image",
				"__meta_kubernetes_pod_container_name":           "metrics",
				"__meta_kubernetes_pod_container_port_name":      "web",
				"__meta_kubernetes_pod_container_port_number":    "8428",
				"__meta_kubernetes_pod_container_port_protocol":  "sdc",
				"__meta_kubernetes_pod_host_ip":                  "4.5.6.7",
				"__meta_kubernetes_pod_ip":                       "192.168.15.1",
				"__meta_kubernetes_pod_name":                     "test-pod",
				"__meta_kubernetes_pod_node_name":                "test-node",
				"__meta_kubernetes_pod_phase":                    "abc",
				"__meta_kubernetes_pod_ready":                    "unknown",
				"__meta_kubernetes_pod_uid":                      "pod-uid",
				"__meta_kubernetes_service_cluster_ip":           "1.2.3.4",
				"__meta_kubernetes_service_name":                 "test-eps",
				"__meta_kubernetes_service_type":                 "service-type",
			}),
		})
	})
}
