package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseEndpointsListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		els, err := parseEndpointsList([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if els != nil {
			t.Fatalf("unexpected non-nil EnpointsList: %v", els)
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
        "creationTimestamp": "2020-03-16T20:44:25Z"
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
	els, err := parseEndpointsList([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(els.Items) != 1 {
		t.Fatalf("unexpected length of EndpointsList.Items; got %d; want %d", len(els.Items), 1)
	}
	endpoint := els.Items[0]

	// Check endpoint.appendTargetLabels()
	labelss := endpoint.appendTargetLabels(nil, nil, nil)
	var sortedLabelss [][]prompbmarshal.Label
	for _, labels := range labelss {
		sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "172.17.0.2:8443",
			"__meta_kubernetes_endpoint_address_target_kind": "Pod",
			"__meta_kubernetes_endpoint_address_target_name": "coredns-6955765f44-lnp6t",
			"__meta_kubernetes_endpoint_hostname":            "aaa.bbb",
			"__meta_kubernetes_endpoint_node_name":           "foobar",
			"__meta_kubernetes_endpoint_port_name":           "https",
			"__meta_kubernetes_endpoint_port_protocol":       "TCP",
			"__meta_kubernetes_endpoint_ready":               "true",
			"__meta_kubernetes_endpoints_name":               "kubernetes",
			"__meta_kubernetes_namespace":                    "default",
		}),
	}
	if !reflect.DeepEqual(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
