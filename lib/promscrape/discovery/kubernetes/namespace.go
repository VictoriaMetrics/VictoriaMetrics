package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func (ns *Namespace) key() string {
	// Namespaces don't have a namespace field, so we just use the name
	return "/" + ns.Metadata.Name
}

func parseNamespaceList(r io.Reader) (map[string]object, ListMeta, error) {
	var nsl NamespaceList
	d := json.NewDecoder(r)
	if err := d.Decode(&nsl); err != nil {
		return nil, nsl.Metadata, fmt.Errorf("cannot unmarshal NamespaceList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, ns := range nsl.Items {
		objectsByKey[ns.key()] = ns
	}
	return objectsByKey, nsl.Metadata, nil
}

func parseNamespace(data []byte) (object, error) {
	var ns Namespace
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, err
	}
	return &ns, nil
}

// NamespaceList represents NamespaceList from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#namespacelist-v1-core
type NamespaceList struct {
	Metadata ListMeta
	Items    []*Namespace
}

// Namespace represents Namespace from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#namespace-v1-core
type Namespace struct {
	Metadata ObjectMeta
	Spec     NamespaceSpec
	Status   NamespaceStatus
}

// NamespaceSpec represents NamespaceSpec from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#namespacespec-v1-core
type NamespaceSpec struct {
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage
	Finalizers []string
}

// NamespaceStatus represents NamespaceStatus from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#namespacestatus-v1-core
type NamespaceStatus struct {
	// Phase is the current lifecycle phase of the namespace
	Phase string
}

// getTargetLabels returns labels for the given namespace.
// Namespaces themselves are not scraped, so this returns nil.
// The namespace metadata is used to enrich labels for other resources.
func (ns *Namespace) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	return nil
}
