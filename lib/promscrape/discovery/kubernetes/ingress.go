package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func (ig *Ingress) key() string {
	return ig.Metadata.key()
}

func parseIngressList(r io.Reader) (map[string]object, ListMeta, error) {
	var igl IngressList
	d := json.NewDecoder(r)
	if err := d.Decode(&igl); err != nil {
		return nil, igl.Metadata, fmt.Errorf("cannot unmarshal IngressList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, ig := range igl.Items {
		objectsByKey[ig.key()] = ig
	}
	return objectsByKey, igl.Metadata, nil
}

func parseIngress(data []byte) (object, error) {
	var ig Ingress
	if err := json.Unmarshal(data, &ig); err != nil {
		return nil, err
	}
	return &ig, nil
}

// IngressList represents ingress list in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#ingresslist-v1-networking-k8s-io
type IngressList struct {
	Metadata ListMeta
	Items    []*Ingress
}

// Ingress represents ingress in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#ingress-v1-networking-k8s-io
type Ingress struct {
	Metadata ObjectMeta
	Spec     IngressSpec
}

// IngressSpec represents ingress spec in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#ingressspec-v1-networking-k8s-io
type IngressSpec struct {
	TLS              []IngressTLS `json:"tls"`
	Rules            []IngressRule
	IngressClassName string
}

// IngressTLS represents ingress TLS spec in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#ingresstls-v1-networking-k8s-io
type IngressTLS struct {
	Hosts []string
}

// IngressRule represents ingress rule in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#ingressrule-v1-networking-k8s-io
type IngressRule struct {
	Host string
	HTTP HTTPIngressRuleValue `json:"http"`
}

// HTTPIngressRuleValue represents HTTP ingress rule value in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#httpingressrulevalue-v1-networking-k8s-io
type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath
}

// HTTPIngressPath represents HTTP ingress path in k8s.
//
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#httpingresspath-v1-networking-k8s-io
type HTTPIngressPath struct {
	Path string
}

// getTargetLabels returns labels for ig.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ingress
func (ig *Ingress) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, r := range ig.Spec.Rules {
		paths := getIngressRulePaths(r.HTTP.Paths)
		scheme := getSchemeForHost(r.Host, ig.Spec.TLS)
		for _, path := range paths {
			m := getLabelsForIngressPath(ig, scheme, r.Host, path)
			ms = append(ms, m)
		}
	}
	return ms
}

func getSchemeForHost(host string, tlss []IngressTLS) string {
	for _, tls := range tlss {
		for _, hostPattern := range tls.Hosts {
			if matchesHostPattern(hostPattern, host) {
				return "https"
			}
		}
	}
	return "http"
}

func matchesHostPattern(pattern, host string) bool {
	if pattern == host {
		return true
	}
	if !strings.HasPrefix(pattern, "*.") {
		return false
	}
	pattern = pattern[len("*."):]
	n := strings.IndexByte(host, '.')
	if n < 0 {
		return false
	}
	host = host[n+1:]
	return pattern == host
}

func getLabelsForIngressPath(ig *Ingress, scheme, host, path string) *promutil.Labels {
	m := promutil.GetLabels()
	m.Add("__address__", host)
	m.Add("__meta_kubernetes_namespace", ig.Metadata.Namespace)
	m.Add("__meta_kubernetes_ingress_name", ig.Metadata.Name)
	m.Add("__meta_kubernetes_ingress_scheme", scheme)
	m.Add("__meta_kubernetes_ingress_host", host)
	m.Add("__meta_kubernetes_ingress_path", path)
	m.Add("__meta_kubernetes_ingress_class_name", ig.Spec.IngressClassName)
	ig.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_ingress", m)
	return m
}

func getIngressRulePaths(paths []HTTPIngressPath) []string {
	if len(paths) == 0 {
		return []string{"/"}
	}
	var result []string
	for _, p := range paths {
		path := p.Path
		if path == "" {
			path = "/"
		}
		result = append(result, path)
	}
	return result
}
