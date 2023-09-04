package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseEndpointSliceListFail(t *testing.T) {
	f := func(data string) {
		r := bytes.NewBufferString(data)
		objectsByKey, _, err := parseEndpointSliceList(r)
		if err == nil {
			t.Errorf("unexpected result, test must fail! data: %s", data)
		}
		if len(objectsByKey) != 0 {
			t.Errorf("EndpointSliceList must be emptry, got: %v", objectsByKey)
		}
	}

	f(``)
	f(`{"items": [1,2,3]`)
	f(`{"items": [
    {
      "metadata": {
        "name": "kubernetes"}]}`)

}

func TestParseEndpointSliceListSuccess(t *testing.T) {
	data := `{
  "kind": "EndpointSliceList",
  "apiVersion": "discovery.k8s.io/v1",
  "metadata": {
    "selfLink": "/apis/discovery.k8s.io/v1/endpointslices",
    "resourceVersion": "1177"
  },
  "items": [
    {
      "metadata": {
        "name": "kubernetes",
        "namespace": "default",
        "selfLink": "/apis/discovery.k8s.io/v1/namespaces/default/endpointslices/kubernetes",
        "uid": "a60d9173-5fe4-4bc3-87a6-269daee71f8a",
        "resourceVersion": "159",
        "generation": 1,
        "creationTimestamp": "2020-09-07T14:27:22Z",
        "labels": {
          "kubernetes.io/service-name": "kubernetes"
        },
        "managedFields": [
          {
            "manager": "kube-apiserver",
            "operation": "Update",
            "apiVersion": "discovery.k8s.io/v1",
            "time": "2020-09-07T14:27:22Z",
            "fieldsType": "FieldsV1",
            "fieldsV1": {"f:addressType":{},"f:endpoints":{},"f:metadata":{"f:labels":{".":{},"f:kubernetes.io/service-name":{}}},"f:ports":{}}
          }
        ]
      },
      "addressType": "IPv4",
      "endpoints": [
        {
          "addresses": [
            "172.18.0.2"
          ],
          "conditions": {
            "ready": true
          }
        }
      ],
      "ports": [
        {
          "name": "https",
          "protocol": "TCP",
          "port": 6443
        }
      ]
    },
    {
      "metadata": {
        "name": "kube-dns-22mvb",
        "generateName": "kube-dns-",
        "namespace": "kube-system",
        "selfLink": "/apis/discovery.k8s.io/v1/namespaces/kube-system/endpointslices/kube-dns-22mvb",
        "uid": "7c95c854-f34c-48e1-86f5-bb8269113c11",
        "resourceVersion": "604",
        "generation": 5,
        "creationTimestamp": "2020-09-07T14:27:39Z",
        "labels": {
          "endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
          "kubernetes.io/service-name": "kube-dns"
        },
        "annotations": {
          "endpoints.kubernetes.io/last-change-trigger-time": "2020-09-07T14:28:35Z"
        },
        "ownerReferences": [
          {
            "apiVersion": "v1",
            "kind": "Service",
            "name": "kube-dns",
            "uid": "509e80d8-6d05-487b-bfff-74f5768f1024",
            "controller": true,
            "blockOwnerDeletion": true
          }
        ],
        "managedFields": [
          {
            "manager": "kube-controller-manager",
            "operation": "Update",
            "apiVersion": "discovery.k8s.io/v1",
            "time": "2020-09-07T14:28:35Z",
            "fieldsType": "FieldsV1",
            "fieldsV1": {"f:addressType":{},"f:endpoints":{},"f:metadata":{"f:annotations":{".":{},"f:endpoints.kubernetes.io/last-change-trigger-time":{}},"f:generateName":{},"f:labels":{".":{},"f:endpointslice.kubernetes.io/managed-by":{},"f:kubernetes.io/service-name":{}},"f:ownerReferences":{".":{},"k:{\"uid\":\"509e80d8-6d05-487b-bfff-74f5768f1024\"}":{".":{},"f:apiVersion":{},"f:blockOwnerDeletion":{},"f:controller":{},"f:kind":{},"f:name":{},"f:uid":{}}}},"f:ports":{}}
          }
        ]
      },
      "addressType": "IPv4",
      "endpoints": [
        {
          "addresses": [
            "10.244.0.3"
          ],
          "conditions": {
            "ready": true
          },
          "targetRef": {
            "kind": "Pod",
            "namespace": "kube-system",
            "name": "coredns-66bff467f8-z8czk",
            "uid": "36a545ff-dbba-4192-a5f6-1dbb0c21c73d",
            "resourceVersion": "603"
          },
          "topology": {
            "kubernetes.io/hostname": "kind-control-plane"
          }
        }
      ],
      "ports": [
        {
          "name": "metrics",
          "protocol": "TCP",
          "port": 9153
        },
        {
          "name": "dns",
          "protocol": "UDP",
          "port": 53
        }
      ]
    }
  ]
}`
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseEndpointSliceList(r)
	if err != nil {
		t.Errorf("cannot parse data for EndpointSliceList: %v", err)
		return
	}
	expectedResourceVersion := "1177"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}
	sortedLabelss := getSortedLabelss(objectsByKey)
	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__": "10.244.0.3:53",
			"__meta_kubernetes_endpointslice_address_target_kind":                                                "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                                                "coredns-66bff467f8-z8czk",
			"__meta_kubernetes_endpointslice_address_type":                                                       "IPv4",
			"__meta_kubernetes_endpointslice_annotation_endpoints_kubernetes_io_last_change_trigger_time":        "2020-09-07T14:28:35Z",
			"__meta_kubernetes_endpointslice_annotationpresent_endpoints_kubernetes_io_last_change_trigger_time": "true",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                                          "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":                           "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname":                   "true",
			"__meta_kubernetes_endpointslice_label_endpointslice_kubernetes_io_managed_by":                       "endpointslice-controller.k8s.io",
			"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":                                   "kube-dns",
			"__meta_kubernetes_endpointslice_labelpresent_endpointslice_kubernetes_io_managed_by":                "true",
			"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name":                            "true",
			"__meta_kubernetes_endpointslice_name":                                                               "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                                               "53",
			"__meta_kubernetes_endpointslice_port_name":                                                          "dns",
			"__meta_kubernetes_endpointslice_port_protocol":                                                      "UDP",
			"__meta_kubernetes_namespace":                                                                        "kube-system",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__": "10.244.0.3:9153",
			"__meta_kubernetes_endpointslice_address_target_kind":                                                "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                                                "coredns-66bff467f8-z8czk",
			"__meta_kubernetes_endpointslice_address_type":                                                       "IPv4",
			"__meta_kubernetes_endpointslice_annotation_endpoints_kubernetes_io_last_change_trigger_time":        "2020-09-07T14:28:35Z",
			"__meta_kubernetes_endpointslice_annotationpresent_endpoints_kubernetes_io_last_change_trigger_time": "true",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                                          "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":                           "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname":                   "true",
			"__meta_kubernetes_endpointslice_label_endpointslice_kubernetes_io_managed_by":                       "endpointslice-controller.k8s.io",
			"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":                                   "kube-dns",
			"__meta_kubernetes_endpointslice_labelpresent_endpointslice_kubernetes_io_managed_by":                "true",
			"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name":                            "true",
			"__meta_kubernetes_endpointslice_name":                                                               "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                                               "9153",
			"__meta_kubernetes_endpointslice_port_name":                                                          "metrics",
			"__meta_kubernetes_endpointslice_port_protocol":                                                      "TCP",
			"__meta_kubernetes_namespace":                                                                        "kube-system",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__": "172.18.0.2:6443",
			"__meta_kubernetes_endpointslice_address_type":                            "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":               "true",
			"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":        "kubernetes",
			"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name": "true",
			"__meta_kubernetes_endpointslice_name":                                    "kubernetes",
			"__meta_kubernetes_endpointslice_port":                                    "6443",
			"__meta_kubernetes_endpointslice_port_name":                               "https",
			"__meta_kubernetes_endpointslice_port_protocol":                           "TCP",
			"__meta_kubernetes_namespace":                                             "default",
		}),
	}
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels,\ngot:\n%v,\nwant:\n%v", sortedLabelss, expectedLabelss)
	}

}

func TestGetEndpointsliceLabels(t *testing.T) {
	type testArgs struct {
		containerPorts map[string][]ContainerPort
		endpointPorts  []EndpointPort
	}
	f := func(t *testing.T, args testArgs, wantLabels []*promutils.Labels) {
		t.Helper()
		eps := EndpointSlice{
			Metadata: ObjectMeta{
				Name:      "test-eps",
				Namespace: "default",
				Labels: promutils.NewLabelsFromMap(map[string]string{
					"kubernetes.io/service-name": "test-svc",
				}),
			},
			Endpoints: []Endpoint{
				{
					Addresses: []string{
						"10.13.15.15",
					},
					Conditions: EndpointConditions{
						Ready: true,
					},
					Hostname: "foo.bar",
					TargetRef: ObjectReference{
						Kind:      "Pod",
						Namespace: "default",
						Name:      "test-pod",
					},
					Topology: map[string]string{
						"x": "y",
					},
				},
			},
			AddressType: "foobar",
			Ports:       args.endpointPorts,
		}
		svc := Service{
			Metadata: ObjectMeta{
				Name:      "test-svc",
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
					"default/test-svc": &svc,
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
				"__meta_kubernetes_endpointslice_address_target_kind":                     "Pod",
				"__meta_kubernetes_endpointslice_address_target_name":                     "test-pod",
				"__meta_kubernetes_endpointslice_address_type":                            "foobar",
				"__meta_kubernetes_endpointslice_endpoint_conditions_ready":               "true",
				"__meta_kubernetes_endpointslice_endpoint_hostname":                       "foo.bar",
				"__meta_kubernetes_endpointslice_endpoint_topology_present_x":             "true",
				"__meta_kubernetes_endpointslice_endpoint_topology_x":                     "y",
				"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":        "test-svc",
				"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name": "true",
				"__meta_kubernetes_endpointslice_name":                                    "test-eps",
				"__meta_kubernetes_endpointslice_port":                                    "8081",
				"__meta_kubernetes_endpointslice_port_name":                               "web",
				"__meta_kubernetes_endpointslice_port_protocol":                           "foobar",
				"__meta_kubernetes_namespace":                                             "default",
				"__meta_kubernetes_node_label_node_label":                                 "xyz",
				"__meta_kubernetes_node_labelpresent_node_label":                          "true",
				"__meta_kubernetes_node_name":                                             "test-node",
				"__meta_kubernetes_pod_host_ip":                                           "4.5.6.7",
				"__meta_kubernetes_pod_ip":                                                "192.168.15.1",
				"__meta_kubernetes_pod_name":                                              "test-pod",
				"__meta_kubernetes_pod_node_name":                                         "test-node",
				"__meta_kubernetes_pod_phase":                                             "abc",
				"__meta_kubernetes_pod_ready":                                             "unknown",
				"__meta_kubernetes_pod_uid":                                               "pod-uid",
				"__meta_kubernetes_service_cluster_ip":                                    "1.2.3.4",
				"__meta_kubernetes_service_name":                                          "test-svc",
				"__meta_kubernetes_service_type":                                          "service-type",
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
				"__meta_kubernetes_endpointslice_address_target_kind":                     "Pod",
				"__meta_kubernetes_endpointslice_address_target_name":                     "test-pod",
				"__meta_kubernetes_endpointslice_address_type":                            "foobar",
				"__meta_kubernetes_endpointslice_endpoint_conditions_ready":               "true",
				"__meta_kubernetes_endpointslice_endpoint_hostname":                       "foo.bar",
				"__meta_kubernetes_endpointslice_endpoint_topology_present_x":             "true",
				"__meta_kubernetes_endpointslice_endpoint_topology_x":                     "y",
				"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":        "test-svc",
				"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name": "true",
				"__meta_kubernetes_endpointslice_name":                                    "test-eps",
				"__meta_kubernetes_endpointslice_port":                                    "8081",
				"__meta_kubernetes_endpointslice_port_name":                               "web",
				"__meta_kubernetes_endpointslice_port_protocol":                           "https",
				"__meta_kubernetes_namespace":                                             "default",
				"__meta_kubernetes_node_label_node_label":                                 "xyz",
				"__meta_kubernetes_node_labelpresent_node_label":                          "true",
				"__meta_kubernetes_node_name":                                             "test-node",
				"__meta_kubernetes_pod_host_ip":                                           "4.5.6.7",
				"__meta_kubernetes_pod_ip":                                                "192.168.15.1",
				"__meta_kubernetes_pod_name":                                              "test-pod",
				"__meta_kubernetes_pod_node_name":                                         "test-node",
				"__meta_kubernetes_pod_phase":                                             "abc",
				"__meta_kubernetes_pod_ready":                                             "unknown",
				"__meta_kubernetes_pod_uid":                                               "pod-uid",
				"__meta_kubernetes_service_cluster_ip":                                    "1.2.3.4",
				"__meta_kubernetes_service_name":                                          "test-svc",
				"__meta_kubernetes_service_type":                                          "service-type",
			}),
			promutils.NewLabelsFromMap(map[string]string{
				"__address__": "192.168.15.1:8428",
				"__meta_kubernetes_endpointslice_address_target_kind":                     "Pod",
				"__meta_kubernetes_endpointslice_address_target_name":                     "test-pod",
				"__meta_kubernetes_endpointslice_address_type":                            "foobar",
				"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":        "test-svc",
				"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name": "true",
				"__meta_kubernetes_endpointslice_name":                                    "test-eps",
				"__meta_kubernetes_namespace":                                             "default",
				"__meta_kubernetes_node_label_node_label":                                 "xyz",
				"__meta_kubernetes_node_labelpresent_node_label":                          "true",
				"__meta_kubernetes_node_name":                                             "test-node",
				"__meta_kubernetes_pod_container_image":                                   "test-image",
				"__meta_kubernetes_pod_container_name":                                    "metrics",
				"__meta_kubernetes_pod_container_port_name":                               "http-metrics",
				"__meta_kubernetes_pod_container_port_number":                             "8428",
				"__meta_kubernetes_pod_container_port_protocol":                           "foobar",
				"__meta_kubernetes_pod_host_ip":                                           "4.5.6.7",
				"__meta_kubernetes_pod_ip":                                                "192.168.15.1",
				"__meta_kubernetes_pod_name":                                              "test-pod",
				"__meta_kubernetes_pod_node_name":                                         "test-node",
				"__meta_kubernetes_pod_phase":                                             "abc",
				"__meta_kubernetes_pod_ready":                                             "unknown",
				"__meta_kubernetes_pod_uid":                                               "pod-uid",
				"__meta_kubernetes_service_cluster_ip":                                    "1.2.3.4",
				"__meta_kubernetes_service_name":                                          "test-svc",
				"__meta_kubernetes_service_type":                                          "service-type",
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
				"__meta_kubernetes_endpointslice_address_target_kind":                     "Pod",
				"__meta_kubernetes_endpointslice_address_target_name":                     "test-pod",
				"__meta_kubernetes_endpointslice_address_type":                            "foobar",
				"__meta_kubernetes_endpointslice_endpoint_conditions_ready":               "true",
				"__meta_kubernetes_endpointslice_endpoint_hostname":                       "foo.bar",
				"__meta_kubernetes_endpointslice_endpoint_topology_present_x":             "true",
				"__meta_kubernetes_endpointslice_endpoint_topology_x":                     "y",
				"__meta_kubernetes_endpointslice_label_kubernetes_io_service_name":        "test-svc",
				"__meta_kubernetes_endpointslice_labelpresent_kubernetes_io_service_name": "true",
				"__meta_kubernetes_endpointslice_name":                                    "test-eps",
				"__meta_kubernetes_endpointslice_port":                                    "8428",
				"__meta_kubernetes_endpointslice_port_name":                               "web",
				"__meta_kubernetes_endpointslice_port_protocol":                           "xabc",
				"__meta_kubernetes_namespace":                                             "default",
				"__meta_kubernetes_node_label_node_label":                                 "xyz",
				"__meta_kubernetes_node_labelpresent_node_label":                          "true",
				"__meta_kubernetes_node_name":                                             "test-node",
				"__meta_kubernetes_pod_container_image":                                   "test-image",
				"__meta_kubernetes_pod_container_name":                                    "metrics",
				"__meta_kubernetes_pod_container_port_name":                               "web",
				"__meta_kubernetes_pod_container_port_number":                             "8428",
				"__meta_kubernetes_pod_container_port_protocol":                           "sdc",
				"__meta_kubernetes_pod_host_ip":                                           "4.5.6.7",
				"__meta_kubernetes_pod_ip":                                                "192.168.15.1",
				"__meta_kubernetes_pod_name":                                              "test-pod",
				"__meta_kubernetes_pod_node_name":                                         "test-node",
				"__meta_kubernetes_pod_phase":                                             "abc",
				"__meta_kubernetes_pod_ready":                                             "unknown",
				"__meta_kubernetes_pod_uid":                                               "pod-uid",
				"__meta_kubernetes_service_cluster_ip":                                    "1.2.3.4",
				"__meta_kubernetes_service_name":                                          "test-svc",
				"__meta_kubernetes_service_type":                                          "service-type",
			}),
		})
	})
}
