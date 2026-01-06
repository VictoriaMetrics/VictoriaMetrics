package kubernetes

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func (ns *Namespace) key() string {
	// Namespaces don't have a namespace field, so we just use the name
	return "/" + ns.Name
}

// Namespace represents Namespace from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#namespace-v1-core
type Namespace struct {
	ObjectMeta `json:"metadata"`
	Spec       NamespaceSpec
	Status     NamespaceStatus
}

// NamespaceSpec represents NamespaceSpec from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#namespacespec-v1-core
type NamespaceSpec struct {
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []string
}

// NamespaceStatus represents NamespaceStatus from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#namespacestatus-v1-core
type NamespaceStatus struct {
	// Phase is the current lifecycle phase of the namespace
	Phase string
}

// getTargetLabels returns labels for the given namespace.
// Namespaces themselves are not scraped, so this returns nil.
// The namespace metadata is used to enrich labels for other resources.
func (*Namespace) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	return nil
}
