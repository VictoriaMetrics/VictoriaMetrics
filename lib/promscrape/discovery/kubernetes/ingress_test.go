package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseIngressListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		nls, err := parseIngressList([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if nls != nil {
			t.Fatalf("unexpected non-nil IngressList: %v", nls)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
	f(`{"items":[{"metadata":{"labels":[1]}}]}`)
}

func TestParseIngressListSuccess(t *testing.T) {
	data := `
{
  "kind": "IngressList",
  "apiVersion": "extensions/v1beta1",
  "metadata": {
    "selfLink": "/apis/extensions/v1beta1/ingresses",
    "resourceVersion": "351452"
  },
  "items": [
    {
      "metadata": {
        "name": "test-ingress",
        "namespace": "default",
        "selfLink": "/apis/extensions/v1beta1/namespaces/default/ingresses/test-ingress",
        "uid": "6d3f38f9-de89-4bc9-b273-c8faf74e8a27",
        "resourceVersion": "351445",
        "generation": 1,
        "creationTimestamp": "2020-04-13T16:43:52Z",
        "annotations": {
          "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"networking.k8s.io/v1beta1\",\"kind\":\"Ingress\",\"metadata\":{\"annotations\":{},\"name\":\"test-ingress\",\"namespace\":\"default\"},\"spec\":{\"backend\":{\"serviceName\":\"testsvc\",\"servicePort\":80}}}\n"
        }
      },
      "spec": {
        "backend": {
          "serviceName": "testsvc",
          "servicePort": 80
        },
	"rules": [
	  {
            "host": "foobar"
          }
	]
      },
      "status": {
        "loadBalancer": {
          "ingress": [
            {
              "ip": "172.17.0.2"
            }
          ]
        }
      }
    }
  ]
}`
	igs, err := parseIngressList([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(igs.Items) != 1 {
		t.Fatalf("unexpected length of IngressList.Items; got %d; want %d", len(igs.Items), 1)
	}
	ig := igs.Items[0]

	// Check ig.appendTargetLabels()
	labelss := ig.appendTargetLabels(nil)
	var sortedLabelss [][]prompbmarshal.Label
	for _, labels := range labelss {
		sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__": "foobar",
			"__meta_kubernetes_ingress_annotation_kubectl_kubernetes_io_last_applied_configuration":        `{"apiVersion":"networking.k8s.io/v1beta1","kind":"Ingress","metadata":{"annotations":{},"name":"test-ingress","namespace":"default"},"spec":{"backend":{"serviceName":"testsvc","servicePort":80}}}` + "\n",
			"__meta_kubernetes_ingress_annotationpresent_kubectl_kubernetes_io_last_applied_configuration": "true",
			"__meta_kubernetes_ingress_host":   "foobar",
			"__meta_kubernetes_ingress_name":   "test-ingress",
			"__meta_kubernetes_ingress_path":   "/",
			"__meta_kubernetes_ingress_scheme": "http",
			"__meta_kubernetes_namespace":      "default",
		}),
	}
	if !reflect.DeepEqual(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
