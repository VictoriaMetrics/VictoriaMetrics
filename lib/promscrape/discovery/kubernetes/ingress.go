package kubernetes

import (
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// Ingress represents ingress in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#ingress-v1-networking-k8s-io
type Ingress struct {
	ObjectMeta `json:"metadata"`
	Spec       IngressSpec
}

// IngressSpec represents ingress spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#ingressspec-v1-networking-k8s-io
type IngressSpec struct {
	TLS              []IngressTLS `json:"tls"`
	Rules            []IngressRule
	IngressClassName string
}

// IngressTLS represents ingress TLS spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#ingresstls-v1-networking-k8s-io
type IngressTLS struct {
	Hosts []string
}

// IngressRule represents ingress rule in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#ingressrule-v1-networking-k8s-io
type IngressRule struct {
	Host string
	HTTP HTTPIngressRuleValue `json:"http"`
}

// HTTPIngressRuleValue represents HTTP ingress rule value in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#httpingressrulevalue-v1-networking-k8s-io
type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath
}

// HTTPIngressPath represents HTTP ingress path in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#httpingresspath-v1-networking-k8s-io
type HTTPIngressPath struct {
	Path string
}

// getTargetLabels returns labels for ig.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ingress
func (ig *Ingress) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, r := range ig.Spec.Rules {
		paths := getIngressRulePaths(r.HTTP.Paths)
		scheme := getSchemeForHost(r.Host, ig.Spec.TLS)
		for _, path := range paths {
			m := getLabelsForIngressPath(ig, gw, scheme, r.Host, path)
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

func getLabelsForIngressPath(ig *Ingress, gw *groupWatcher, scheme, host, path string) *promutil.Labels {
	m := promutil.GetLabels()
	m.Add("__address__", host)
	m.Add("__meta_kubernetes_namespace", ig.Namespace)
	m.Add("__meta_kubernetes_ingress_name", ig.Name)
	m.Add("__meta_kubernetes_ingress_scheme", scheme)
	m.Add("__meta_kubernetes_ingress_host", host)
	m.Add("__meta_kubernetes_ingress_path", path)
	m.Add("__meta_kubernetes_ingress_class_name", ig.Spec.IngressClassName)
	if gw.attachNamespaceMetadata {
		o := gw.getObjectByRoleLocked("namespace", "", ig.Namespace)
		if o != nil {
			ns := o.(*Namespace)
			ns.registerLabelsAndAnnotations("__meta_kubernetes_namespace", m)
		}
	}
	ig.registerLabelsAndAnnotations("__meta_kubernetes_ingress", m)
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
