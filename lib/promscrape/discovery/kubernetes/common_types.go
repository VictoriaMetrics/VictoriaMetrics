package kubernetes

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// ObjectMeta represents ObjectMeta from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta
type ObjectMeta struct {
	Name            string
	Namespace       string
	UID             string
	Labels          *promutil.Labels
	Annotations     *promutil.Labels
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

func (om *ObjectMeta) registerLabelsAndAnnotations(prefix string, m *promutil.Labels) {
	bb := bbPool.Get()
	b := bb.B
	for _, lb := range om.Labels.GetLabels() {
		b = appendThreeStrings(b[:0], prefix, "_label_", lb.Name)
		labelName := bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), lb.Value)

		b = appendThreeStrings(b[:0], prefix, "_labelpresent_", lb.Name)
		labelName = bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), "true")
	}
	for _, a := range om.Annotations.GetLabels() {
		b = appendThreeStrings(b[:0], prefix, "_annotation_", a.Name)
		labelName := bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), a.Value)

		b = appendThreeStrings(b[:0], prefix, "_annotationpresent_", a.Name)
		labelName = bytesutil.ToUnsafeString(b)
		m.Add(discoveryutil.SanitizeLabelName(labelName), "true")
	}
	bb.B = b
	bbPool.Put(bb)
}

var bbPool bytesutil.ByteBufferPool

func appendThreeStrings(dst []byte, a, b, c string) []byte {
	dst = append(dst, a...)
	dst = append(dst, b...)
	dst = append(dst, c...)
	return dst
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
