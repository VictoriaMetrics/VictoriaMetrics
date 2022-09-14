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
		m[discoveryutils.SanitizeLabelName(prefix+"_label_"+lb.Name)] = lb.Value
		m[discoveryutils.SanitizeLabelName(prefix+"_labelpresent_"+lb.Name)] = "true"
	}
	for _, a := range om.Annotations {
		m[discoveryutils.SanitizeLabelName(prefix+"_annotation_"+a.Name)] = a.Value
		m[discoveryutils.SanitizeLabelName(prefix+"_annotationpresent_"+a.Name)] = "true"
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
