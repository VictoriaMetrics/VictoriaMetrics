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
	      "nodeName": "foobar",
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
			"__meta_kubernetes_endpoint_node_name":            "foobar",
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
