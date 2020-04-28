package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestParseServiceListFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		nls, err := parseServiceList([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if nls != nil {
			t.Fatalf("unexpected non-nil ServiceList: %v", nls)
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
	sls, err := parseServiceList([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sls.Items) != 1 {
		t.Fatalf("unexpected length of ServiceList.Items; got %d; want %d", len(sls.Items), 1)
	}
	service := sls.Items[0]
	meta := service.Metadata
	if meta.Name != "kube-dns" {
		t.Fatalf("unexpected ObjectMeta.Name; got %q; want %q", meta.Name, "kube-dns")
	}
	expectedLabels := discoveryutils.GetSortedLabels(map[string]string{
		"k8s-app":                       "kube-dns",
		"kubernetes.io/cluster-service": "true",
		"kubernetes.io/name":            "KubeDNS",
	})
	if !reflect.DeepEqual(meta.Labels, expectedLabels) {
		t.Fatalf("unexpected ObjectMeta.Labels\ngot\n%v\nwant\n%v", meta.Labels, expectedLabels)
	}
	expectedAnnotations := discoveryutils.GetSortedLabels(map[string]string{
		"prometheus.io/port":   "9153",
		"prometheus.io/scrape": "true",
	})
	if !reflect.DeepEqual(meta.Annotations, expectedAnnotations) {
		t.Fatalf("unexpected ObjectMeta.Annotations\ngot\n%v\nwant\n%v", meta.Annotations, expectedAnnotations)
	}
	spec := service.Spec
	expectedClusterIP := "10.96.0.10"
	if spec.ClusterIP != expectedClusterIP {
		t.Fatalf("unexpected clusterIP; got %q; want %q", spec.ClusterIP, expectedClusterIP)
	}
	if spec.Type != "ClusterIP" {
		t.Fatalf("unexpected type; got %q; want %q", spec.Type, "ClusterIP")
	}
	expectedPorts := []ServicePort{
		{
			Name:     "dns",
			Protocol: "UDP",
			Port:     53,
		},
		{
			Name:     "dns-tcp",
			Protocol: "TCP",
			Port:     53,
		},
		{
			Name:     "metrics",
			Protocol: "TCP",
			Port:     9153,
		},
	}
	if !reflect.DeepEqual(spec.Ports, expectedPorts) {
		t.Fatalf("unexpected ports\ngot\n%v\nwant\n%v", spec.Ports, expectedPorts)
	}

	// Check service.appendTargetLabels()
	labelss := service.appendTargetLabels(nil)
	var sortedLabelss [][]prompbmarshal.Label
	for _, labels := range labelss {
		sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
	}
	expectedLabelss := [][]prompbmarshal.Label{
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:53",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "dns",
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
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:53",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "dns-tcp",
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
		discoveryutils.GetSortedLabels(map[string]string{
			"__address__":                             "kube-dns.kube-system.svc:9153",
			"__meta_kubernetes_namespace":             "kube-system",
			"__meta_kubernetes_service_name":          "kube-dns",
			"__meta_kubernetes_service_type":          "ClusterIP",
			"__meta_kubernetes_service_port_name":     "metrics",
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
	if !reflect.DeepEqual(sortedLabelss, expectedLabelss) {
		t.Fatalf("unexpected labels:\ngot\n%v\nwant\n%v", sortedLabelss, expectedLabelss)
	}
}
