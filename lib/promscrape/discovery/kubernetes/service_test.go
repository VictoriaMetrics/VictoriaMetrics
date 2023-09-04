package kubernetes

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseServiceListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		r := bytes.NewBufferString(s)
		objectsByKey, _, err := parseServiceList(r)
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

func TestParseServiceListSuccess(t *testing.T) {
	data := `{
  "kind": "ServiceList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/services",
    "resourceVersion": "60485"
  },
  "items": [
    {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "selfLink": "/api/v1/namespaces/kube-system/services/kube-dns",
        "uid": "38a396f1-17fe-46c2-a5f4-3b225c18dcdf",
        "resourceVersion": "177",
        "creationTimestamp": "2020-03-16T20:44:26Z",
        "labels": {
          "k8s-app": "kube-dns",
          "kubernetes.io/cluster-service": "true",
          "kubernetes.io/name": "KubeDNS"
        },
        "annotations": {
          "prometheus.io/port": "9153",
          "prometheus.io/scrape": "true"
        }
      },
      "spec": {
        "ports": [
          {
            "name": "dns",
            "protocol": "UDP",
            "port": 53,
            "targetPort": 53
          },
          {
            "name": "dns-tcp",
            "protocol": "TCP",
            "port": 53,
            "targetPort": 53
          },
          {
            "name": "metrics",
            "protocol": "TCP",
            "port": 9153,
            "targetPort": 9153
          }
        ],
        "selector": {
          "k8s-app": "kube-dns"
        },
        "clusterIP": "10.96.0.10",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      },
      "status": {
        "loadBalancer": {
          
        }
      }
    }
  ]
}
`
	r := bytes.NewBufferString(data)
	objectsByKey, meta, err := parseServiceList(r)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "60485"
	if meta.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resource version; got %s; want %s", meta.ResourceVersion, expectedResourceVersion)
	}
	sortedLabelss := getSortedLabelss(objectsByKey)
	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:53",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "dns",
			"__meta_kubernetes_service_port_number":   "53",
			"__meta_kubernetes_service_port_protocol": "UDP",
			"__meta_kubernetes_service_cluster_ip":    "10.96.0.10",

			"__meta_kubernetes_service_label_k8s_app":                       "kube-dns",
			"__meta_kubernetes_service_label_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_label_kubernetes_io_name":            "KubeDNS",

			"__meta_kubernetes_service_labelpresent_k8s_app":                       "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_name":            "true",

			"__meta_kubernetes_service_annotation_prometheus_io_port":   "9153",
			"__meta_kubernetes_service_annotation_prometheus_io_scrape": "true",

			"__meta_kubernetes_service_annotationpresent_prometheus_io_port":   "true",
			"__meta_kubernetes_service_annotationpresent_prometheus_io_scrape": "true",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:53",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "dns-tcp",
			"__meta_kubernetes_service_port_number":   "53",
			"__meta_kubernetes_service_port_protocol": "TCP",
			"__meta_kubernetes_service_cluster_ip":    "10.96.0.10",

			"__meta_kubernetes_service_label_k8s_app":                       "kube-dns",
			"__meta_kubernetes_service_label_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_label_kubernetes_io_name":            "KubeDNS",

			"__meta_kubernetes_service_labelpresent_k8s_app":                       "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_name":            "true",

			"__meta_kubernetes_service_annotation_prometheus_io_port":   "9153",
			"__meta_kubernetes_service_annotation_prometheus_io_scrape": "true",

			"__meta_kubernetes_service_annotationpresent_prometheus_io_port":   "true",
			"__meta_kubernetes_service_annotationpresent_prometheus_io_scrape": "true",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:9153",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "metrics",
			"__meta_kubernetes_service_port_number":   "9153",
			"__meta_kubernetes_service_port_protocol": "TCP",
			"__meta_kubernetes_service_cluster_ip":    "10.96.0.10",

			"__meta_kubernetes_service_label_k8s_app":                       "kube-dns",
			"__meta_kubernetes_service_label_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_label_kubernetes_io_name":            "KubeDNS",

			"__meta_kubernetes_service_labelpresent_k8s_app":                       "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_cluster_service": "true",
			"__meta_kubernetes_service_labelpresent_kubernetes_io_name":            "true",

			"__meta_kubernetes_service_annotation_prometheus_io_port":   "9153",
			"__meta_kubernetes_service_annotation_prometheus_io_scrape": "true",

			"__meta_kubernetes_service_annotationpresent_prometheus_io_port":   "true",
			"__meta_kubernetes_service_annotationpresent_prometheus_io_scrape": "true",
		}),
	}
	if !areEqualLabelss(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
