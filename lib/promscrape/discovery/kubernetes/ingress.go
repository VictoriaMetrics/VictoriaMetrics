package kubernetes

import (
	"encoding/json"
	"fmt"
)

// getIngressesLabels returns labels for k8s ingresses obtained from the given cfg.
func getIngressesLabels(cfg *APIConfig) ([]map[string]string, error) {
	data, err := getAPIResponse(cfg, "ingress", "/apis/extensions/v1beta1/ingresses")
	if err != nil {
		return nil, fmt.Errorf("cannot obtain ingresses data from API server: %s", err)
	}
	igl, err := parseIngressList(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse ingresses response from API server: %s", err)
	}
	var ms []map[string]string
	for _, ig := range igl.Items {
		ms = ig.appendTargetLabels(ms)
	}
	return ms, nil
}

// IngressList represents ingress list in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ingresslist-v1beta1-extensions
type IngressList struct {
	Items []Ingress
}

// Ingress represents ingress in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ingress-v1beta1-extensions
type Ingress struct {
	Metadata ObjectMeta
	Spec     IngressSpec
}

// IngressSpec represents ingress spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ingressspec-v1beta1-extensions
type IngressSpec struct {
	TLS   []IngressTLS `json:"tls"`
	Rules []IngressRule
}

// IngressTLS represents ingress TLS spec in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ingresstls-v1beta1-extensions
type IngressTLS struct {
	Hosts []string
}

// IngressRule represents ingress rule in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ingressrule-v1beta1-extensions
type IngressRule struct {
	Host string
	HTTP HTTPIngressRuleValue `json:"http"`
}

// HTTPIngressRuleValue represents HTTP ingress rule value in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#httpingressrulevalue-v1beta1-extensions
type HTTPIngressRuleValue struct {
	Paths []HTTPIngressPath
}

// HTTPIngressPath represents HTTP ingress path in k8s.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#httpingresspath-v1beta1-extensions
type HTTPIngressPath struct {
	Path string
}

// parseIngressList parses IngressList from data.
func parseIngressList(data []byte) (*IngressList, error) {
	var il IngressList
	if err := json.Unmarshal(data, &il); err != nil {
		return nil, fmt.Errorf("cannot unmarshal IngressList from %q: %s", data, err)
	}
	return &il, nil
}

// appendTargetLabels appends labels for Ingress ig to ms and returns the result.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ingress
func (ig *Ingress) appendTargetLabels(ms []map[string]string) []map[string]string {
	tlsHosts := make(map[string]bool)
	for _, tls := range ig.Spec.TLS {
		for _, host := range tls.Hosts {
			tlsHosts[host] = true
		}
	}
	for _, r := range ig.Spec.Rules {
		paths := getIngressRulePaths(r.HTTP.Paths)
		scheme := "http"
		if tlsHosts[r.Host] {
			scheme = "https"
		}
		for _, path := range paths {
			m := getLabelsForIngressPath(ig, scheme, r.Host, path)
			ms = append(ms, m)
		}
	}
	return ms
}

func getLabelsForIngressPath(ig *Ingress, scheme, host, path string) map[string]string {
	m := map[string]string{
		"__address__":                      host,
		"__meta_kubernetes_namespace":      ig.Metadata.Namespace,
		"__meta_kubernetes_ingress_name":   ig.Metadata.Name,
		"__meta_kubernetes_ingress_scheme": scheme,
		"__meta_kubernetes_ingress_host":   host,
		"__meta_kubernetes_ingress_path":   path,
	}
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
