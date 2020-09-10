package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func Test_parseEndpointSlicesListFail(t *testing.T) {
	f := func(data string) {
		eslList, err := parseEndpointSlicesList([]byte(data))
		if err == nil {
			t.Errorf("unexpected result, test must fail! data: %s", data)
		}
		if eslList != nil {
			t.Errorf("endpointSliceList must be nil, got: %v", eslList)
		}
	}

	f(``)
	f(`{"items": [1,2,3]`)
	f(`{"items": [
    {
      "metadata": {
        "name": "kubernetes"}]}`)

}

func Test_parseEndpointSlicesListSuccess(t *testing.T) {
	data := `{
  "kind": "EndpointSliceList",
  "apiVersion": "discovery.k8s.io/v1beta1",
  "metadata": {
    "selfLink": "/apis/discovery.k8s.io/v1beta1/endpointslices",
    "resourceVersion": "1177"
  },
  "items": [
    {
      "metadata": {
        "name": "kubernetes",
        "namespace": "default",
        "selfLink": "/apis/discovery.k8s.io/v1beta1/namespaces/default/endpointslices/kubernetes",
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
            "apiVersion": "discovery.k8s.io/v1beta1",
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
        "selfLink": "/apis/discovery.k8s.io/v1beta1/namespaces/kube-system/endpointslices/kube-dns-22mvb",
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
            "apiVersion": "discovery.k8s.io/v1beta1",
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
        },
        {
          "addresses": [
            "10.244.0.4"
          ],
          "conditions": {
            "ready": true
          },
          "targetRef": {
            "kind": "Pod",
            "namespace": "kube-system",
            "name": "coredns-66bff467f8-kpbhk",
            "uid": "db38d8b4-847a-4e82-874c-fe444fba2718",
            "resourceVersion": "576"
          },
          "topology": {
            "kubernetes.io/hostname": "kind-control-plane"
          }
        }
      ],
      "ports": [
        {
          "name": "dns-tcp",
          "protocol": "TCP",
          "port": 53
        },
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
	esl, err := parseEndpointSlicesList([]byte(data))
	if err != nil {
		t.Errorf("cannot parse data for EndpointSliceList: %v", err)
		return
	}
	if len(esl.Items) != 2 {
		t.Fatalf("expected 2 items at endpointSliceList, got: %d", len(esl.Items))
	}

	firstEsl := esl.Items[0]
	got := firstEsl.appendTargetLabels(nil, nil, nil)
	sortedLables := [][]prompbmarshal.Label{}
	for _, labels := range got {
		sortedLables = append(sortedLables, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabels := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "172.18.0.2:6443",
			"__meta_kubernetes_endpointslice_address_type":              "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready": "true",
			"__meta_kubernetes_endpointslice_name":                      "kubernetes",
			"__meta_kubernetes_endpointslice_port":                      "6443",
			"__meta_kubernetes_endpointslice_port_name":                 "https",
			"__meta_kubernetes_endpointslice_port_protocol":             "TCP",
			"__meta_kubernetes_namespace":                               "default",
		})}
	if !reflect.DeepEqual(sortedLables, expectedLabels) {
		t.Fatalf("unexpected labels,\ngot:\n%v,\nwant:\n%v", sortedLables, expectedLabels)
	}

}

func TestEndpointSlice_appendTargetLabels(t *testing.T) {
	type fields struct {
		Metadata    ObjectMeta
		Endpoints   []Endpoint
		AddressType string
		Ports       []EndpointPort
	}
	type args struct {
		ms   []map[string]string
		pods []Pod
		svcs []Service
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   [][]prompbmarshal.Label
	}{
		{
			name: "simple eps",
			args: args{},
			fields: fields{
				Metadata: ObjectMeta{
					Name:      "fake-esl",
					Namespace: "default",
				},
				AddressType: "ipv4",
				Endpoints: []Endpoint{
					{Addresses: []string{"127.0.0.1"},
						Hostname:   "node-1",
						Topology:   map[string]string{"kubernetes.topoligy.io/zone": "gce-1"},
						Conditions: EndpointConditions{Ready: true},
						TargetRef: ObjectReference{
							Kind:      "Pod",
							Namespace: "default",
							Name:      "main-pod",
						},
					},
				},
				Ports: []EndpointPort{
					{
						Name:        "http",
						Port:        8085,
						AppProtocol: "http",
						Protocol:    "tcp",
					},
				},
			},
			want: [][]prompbmarshal.Label{
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__": "127.0.0.1:8085",
					"__meta_kubernetes_endpointslice_address_target_kind":                                   "Pod",
					"__meta_kubernetes_endpointslice_address_target_name":                                   "main-pod",
					"__meta_kubernetes_endpointslice_address_type":                                          "ipv4",
					"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                             "true",
					"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_topoligy_io_zone":         "gce-1",
					"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_topoligy_io_zone": "true",
					"__meta_kubernetes_endpointslice_endpoint_hostname":                                     "node-1",
					"__meta_kubernetes_endpointslice_name":                                                  "fake-esl",
					"__meta_kubernetes_endpointslice_port":                                                  "8085",
					"__meta_kubernetes_endpointslice_port_app_protocol":                                     "http",
					"__meta_kubernetes_endpointslice_port_name":                                             "http",
					"__meta_kubernetes_endpointslice_port_protocol":                                         "tcp",
					"__meta_kubernetes_namespace":                                                           "default",
				}),
			},
		},
		{
			name: "eps with pods and services",
			args: args{
				pods: []Pod{
					{
						Metadata: ObjectMeta{
							UID:       "some-pod-uuid",
							Namespace: "monitoring",
							Name:      "main-pod",
							Labels: discoveryutils.GetSortedLabels(map[string]string{
								"pod-label-1": "pod-value-1",
								"pod-label-2": "pod-value-2",
							}),
							Annotations: discoveryutils.GetSortedLabels(map[string]string{
								"pod-annotations-1": "annotation-value-1",
							}),
						},
						Status: PodStatus{PodIP: "192.168.11.5", HostIP: "172.15.1.1"},
						Spec: PodSpec{NodeName: "node-2", Containers: []Container{
							{
								Name: "container-1",
								Ports: []ContainerPort{
									{
										ContainerPort: 8085,
										Protocol:      "tcp",
										Name:          "http",
									},
									{
										ContainerPort: 8011,
										Protocol:      "udp",
										Name:          "dns",
									},
								},
							},
						}},
					},
				},
				svcs: []Service{
					{
						Spec: ServiceSpec{Type: "ClusterIP", Ports: []ServicePort{
							{
								Name:     "http",
								Protocol: "tcp",
								Port:     8085,
							},
						}},
						Metadata: ObjectMeta{
							Name:      "custom-esl",
							Namespace: "monitoring",
							Labels: discoveryutils.GetSortedLabels(map[string]string{
								"service-label-1": "value-1",
								"service-label-2": "value-2",
							}),
						},
					},
				},
			},
			fields: fields{
				Metadata: ObjectMeta{
					Name:      "custom-esl",
					Namespace: "monitoring",
				},
				AddressType: "ipv4",
				Endpoints: []Endpoint{
					{Addresses: []string{"127.0.0.1"},
						Hostname:   "node-1",
						Topology:   map[string]string{"kubernetes.topoligy.io/zone": "gce-1"},
						Conditions: EndpointConditions{Ready: true},
						TargetRef: ObjectReference{
							Kind:      "Pod",
							Namespace: "monitoring",
							Name:      "main-pod",
						},
					},
				},
				Ports: []EndpointPort{
					{
						Name:        "http",
						Port:        8085,
						AppProtocol: "http",
						Protocol:    "tcp",
					},
				},
			},
			want: [][]prompbmarshal.Label{
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__": "127.0.0.1:8085",
					"__meta_kubernetes_endpointslice_address_target_kind":                                   "Pod",
					"__meta_kubernetes_endpointslice_address_target_name":                                   "main-pod",
					"__meta_kubernetes_endpointslice_address_type":                                          "ipv4",
					"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                             "true",
					"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_topoligy_io_zone":         "gce-1",
					"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_topoligy_io_zone": "true",
					"__meta_kubernetes_endpointslice_endpoint_hostname":                                     "node-1",
					"__meta_kubernetes_endpointslice_name":                                                  "custom-esl",
					"__meta_kubernetes_endpointslice_port":                                                  "8085",
					"__meta_kubernetes_endpointslice_port_app_protocol":                                     "http",
					"__meta_kubernetes_endpointslice_port_name":                                             "http",
					"__meta_kubernetes_endpointslice_port_protocol":                                         "tcp",
					"__meta_kubernetes_namespace":                                                           "monitoring",
					"__meta_kubernetes_pod_annotation_pod_annotations_1":                                    "annotation-value-1",
					"__meta_kubernetes_pod_annotationpresent_pod_annotations_1":                             "true",
					"__meta_kubernetes_pod_container_name":                                                  "container-1",
					"__meta_kubernetes_pod_container_port_name":                                             "http",
					"__meta_kubernetes_pod_container_port_number":                                           "8085",
					"__meta_kubernetes_pod_container_port_protocol":                                         "tcp",
					"__meta_kubernetes_pod_host_ip":                                                         "172.15.1.1",
					"__meta_kubernetes_pod_ip":                                                              "192.168.11.5",
					"__meta_kubernetes_pod_label_pod_label_1":                                               "pod-value-1",
					"__meta_kubernetes_pod_label_pod_label_2":                                               "pod-value-2",
					"__meta_kubernetes_pod_labelpresent_pod_label_1":                                        "true",
					"__meta_kubernetes_pod_labelpresent_pod_label_2":                                        "true",
					"__meta_kubernetes_pod_name":                                                            "main-pod",
					"__meta_kubernetes_pod_node_name":                                                       "node-2",
					"__meta_kubernetes_pod_phase":                                                           "",
					"__meta_kubernetes_pod_ready":                                                           "unknown",
					"__meta_kubernetes_pod_uid":                                                             "some-pod-uuid",
					"__meta_kubernetes_service_cluster_ip":                                                  "",
					"__meta_kubernetes_service_label_service_label_1":                                       "value-1",
					"__meta_kubernetes_service_label_service_label_2":                                       "value-2",
					"__meta_kubernetes_service_labelpresent_service_label_1":                                "true",
					"__meta_kubernetes_service_labelpresent_service_label_2":                                "true",
					"__meta_kubernetes_service_name":                                                        "custom-esl",
					"__meta_kubernetes_service_type":                                                        "ClusterIP",
				}),
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__":                 "192.168.11.5:8011",
					"__meta_kubernetes_namespace": "monitoring",
					"__meta_kubernetes_pod_annotation_pod_annotations_1":        "annotation-value-1",
					"__meta_kubernetes_pod_annotationpresent_pod_annotations_1": "true",
					"__meta_kubernetes_pod_container_name":                      "container-1",
					"__meta_kubernetes_pod_container_port_name":                 "dns",
					"__meta_kubernetes_pod_container_port_number":               "8011",
					"__meta_kubernetes_pod_container_port_protocol":             "udp",
					"__meta_kubernetes_pod_host_ip":                             "172.15.1.1",
					"__meta_kubernetes_pod_ip":                                  "192.168.11.5",
					"__meta_kubernetes_pod_label_pod_label_1":                   "pod-value-1",
					"__meta_kubernetes_pod_label_pod_label_2":                   "pod-value-2",
					"__meta_kubernetes_pod_labelpresent_pod_label_1":            "true",
					"__meta_kubernetes_pod_labelpresent_pod_label_2":            "true",
					"__meta_kubernetes_pod_name":                                "main-pod",
					"__meta_kubernetes_pod_node_name":                           "node-2",
					"__meta_kubernetes_pod_phase":                               "",
					"__meta_kubernetes_pod_ready":                               "unknown",
					"__meta_kubernetes_pod_uid":                                 "some-pod-uuid",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eps := &EndpointSlice{
				Metadata:    tt.fields.Metadata,
				Endpoints:   tt.fields.Endpoints,
				AddressType: tt.fields.AddressType,
				Ports:       tt.fields.Ports,
			}
			got := eps.appendTargetLabels(tt.args.ms, tt.args.pods, tt.args.svcs)
			var sortedLabelss [][]prompbmarshal.Label
			for _, labels := range got {
				sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
			}

			if !reflect.DeepEqual(sortedLabelss, tt.want) {
				t.Errorf("got unxpected labels: \ngot:\n %v, \nexpect:\n %v", sortedLabelss, tt.want)
			}
		})
	}
}
