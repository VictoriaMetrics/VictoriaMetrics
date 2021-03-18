package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseIngressListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		objectsByKey, _, err := parseIngressList(r)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(objectsByKey) != 0 {
			t.Fatalf("unexpected non-empty IngressList: %v", objectsByKey)
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
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseIngressList(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "351452"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}
	sortedLabelss := getSortedLabelss(objectsByKey)
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
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
