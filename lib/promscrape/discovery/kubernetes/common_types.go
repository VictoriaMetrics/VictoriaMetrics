package kubernetes

import (
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// ObjectMeta represents ObjectMeta from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta
type ObjectMeta struct {
	Name            string
	Namespace       string
	UID             string
	Labels          discoveryutils.SortedLabels
	Annotations     discoveryutils.SortedLabels
	OwnerReferences []OwnerReference
}

func (om *ObjectMeta) registerLabelsAndAnnotations(prefix string, m map[string]string) {
	for _, lb := range om.Labels {
		ln := discoveryutils.SanitizeLabelName(lb.Name)
		m[prefix + "_label_" + ln] = lb.Value
		m[prefix + "_labelpresent_" + ln] = "true"
	}
	for _, a := range om.Annotations {
		an := discoveryutils.SanitizeLabelName(a.Name)
		m[prefix + "_annotation_" + an] = a.Value
		m[prefix + "_annotationpresent_" + an] = "true"
	}
}

// OwnerReference represents OwnerReferense from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#ownerreference-v1-meta
type OwnerReference struct {
	Name       string
	Controller bool
	Kind       string
}

// DaemonEndpoint represents DaemonEndpoint from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#daemonendpoint-v1-core
type DaemonEndpoint struct {
	Port int
}

func joinSelectors(role string, namespaces []string, selectors []Selector) string {
	var labelSelectors, fieldSelectors []string
	for _, ns := range namespaces {
		fieldSelectors = append(fieldSelectors, "metadata.namespace="+ns)
	}
	for _, s := range selectors {
		if s.Role != role {
			continue
		}
		if s.Label != "" {
			labelSelectors = append(labelSelectors, s.Label)
		}
		if s.Field != "" {
			fieldSelectors = append(fieldSelectors, s.Field)
		}
	}
	var args []string
	if len(labelSelectors) > 0 {
		args = append(args, "labelSelector="+url.QueryEscape(strings.Join(labelSelectors, ",")))
	}
	if len(fieldSelectors) > 0 {
		args = append(args, "fieldSelector="+url.QueryEscape(strings.Join(fieldSelectors, ",")))
	}
	return strings.Join(args, "&")
}
