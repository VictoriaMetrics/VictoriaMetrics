package kubernetes

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// ObjectMeta represents ObjectMeta from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta
type ObjectMeta struct {
	Name            string
	Namespace       string
	UID             string
	Labels          SortedLabels
	Annotations     SortedLabels
	OwnerReferences []OwnerReference
}

func (om *ObjectMeta) registerLabelsAndAnnotations(prefix string, m map[string]string) {
	for _, lb := range om.Labels {
		ln := sanitizeLabelName(lb.Name)
		m[fmt.Sprintf("%s_label_%s", prefix, ln)] = lb.Value
		m[fmt.Sprintf("%s_labelpresent_%s", prefix, ln)] = "true"
	}
	for _, a := range om.Annotations {
		an := sanitizeLabelName(a.Name)
		m[fmt.Sprintf("%s_annotation_%s", prefix, an)] = a.Value
		m[fmt.Sprintf("%s_annotationpresent_%s", prefix, an)] = "true"
	}
}

// sanitizeLabelName replaces anything that doesn't match
// client_label.LabelNameRE with an underscore.
//
// This has been copied from Prometheus sources at util/strutil/strconv.go
func sanitizeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// SortedLabels represents sorted labels.
type SortedLabels []prompbmarshal.Label

// UnmarshalJSON unmarshals JSON from data.
func (sls *SortedLabels) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*sls = getSortedLabels(m)
	return nil
}

func getSortedLabels(m map[string]string) SortedLabels {
	a := make([]prompbmarshal.Label, 0, len(m))
	for k, v := range m {
		a = append(a, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].Name < a[j].Name
	})
	return a
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

func joinHostPort(host string, port int) string {
	portStr := strconv.Itoa(port)
	return net.JoinHostPort(host, portStr)
}

// APIConfig contains config for API server
type APIConfig struct {
	Server     string
	AuthConfig *promauth.Config
	Namespaces []string
	Selectors  []Selector
}

// Selector represents kubernetes selector.
//
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
// and https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
type Selector struct {
	Role  string `yaml:"role"`
	Label string `yaml:"label"`
	Field string `yaml:"field"`
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
