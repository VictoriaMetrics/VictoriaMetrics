package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
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
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "172.18.0.2:6443",
			"__meta_kubernetes_endpointslice_address_type":              "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready": "true",
			"__meta_kubernetes_endpointslice_name":                      "kubernetes",
			"__meta_kubernetes_endpointslice_port":                      "6443",
			"__meta_kubernetes_endpointslice_port_name":                 "https",
			"__meta_kubernetes_endpointslice_port_protocol":             "TCP",
			"__meta_kubernetes_namespace":                               "default",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.3:53",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-z8czk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "53",
			"__meta_kubernetes_endpointslice_port_name":                                        "dns-tcp",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "TCP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.3:9153",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-z8czk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "9153",
			"__meta_kubernetes_endpointslice_port_name":                                        "metrics",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "TCP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.3:53",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-z8czk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "53",
			"__meta_kubernetes_endpointslice_port_name":                                        "dns",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "UDP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.4:53",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-kpbhk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "53",
			"__meta_kubernetes_endpointslice_port_name":                                        "dns-tcp",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "TCP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.4:9153",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-kpbhk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "9153",
			"__meta_kubernetes_endpointslice_port_name":                                        "metrics",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "TCP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "10.244.0.4:53",
			"__meta_kubernetes_endpointslice_address_target_kind":                              "Pod",
			"__meta_kubernetes_endpointslice_address_target_name":                              "coredns-66bff467f8-kpbhk",
			"__meta_kubernetes_endpointslice_address_type":                                     "IPv4",
			"__meta_kubernetes_endpointslice_endpoint_conditions_ready":                        "true",
			"__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname":         "kind-control-plane",
			"__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname": "true",
			"__meta_kubernetes_endpointslice_name":                                             "kube-dns-22mvb",
			"__meta_kubernetes_endpointslice_port":                                             "53",
			"__meta_kubernetes_endpointslice_port_name":                                        "dns",
			"__meta_kubernetes_endpointslice_port_protocol":                                    "UDP",
			"__meta_kubernetes_namespace":                                                      "kube-system",
		}),
	}
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels,\ngot:\n%v,\nwant:\n%v", sortedLabelss, expectedLabelss)
	}

}
