package kubernetes

import (
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

func (om *ObjectMeta) key() string {
	return om.Namespace + "/" + om.Name
}

// ListMeta is a Kubernetes list metadata
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#listmeta-v1-meta
type ListMeta struct {
	ResourceVersion string
}

func (om *ObjectMeta) registerLabelsAndAnnotations(prefix string, m map[string]string) {
	for _, lb := range om.Labels {
		ln := discoveryutils.SanitizeLabelName(lb.Name)
		m[prefix+"_label_"+ln] = lb.Value
		m[prefix+"_labelpresent_"+ln] = "true"
	}
	for _, a := range om.Annotations {
		an := discoveryutils.SanitizeLabelName(a.Name)
		m[prefix+"_annotation_"+an] = a.Value
		m[prefix+"_annotationpresent_"+an] = "true"
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
