package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func (s *Service) key() string {
	return s.Metadata.key()
}

func parseServiceList(r io.Reader) (map[string]object, ListMeta, error) {
	var sl ServiceList
	d := json.NewDecoder(r)
	if err := d.Decode(&sl); err != nil {
		return nil, sl.Metadata, fmt.Errorf("cannot unmarshal ServiceList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, s := range sl.Items {
		objectsByKey[s.key()] = s
	}
	return objectsByKey, sl.Metadata, nil
}

func parseService(data []byte) (object, error) {
	var s Service
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// ServiceList is k8s service list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicelist-v1-core
type ServiceList struct {
	Metadata ListMeta
	Items    []*Service
}

// Service is k8s service.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#service-v1-core
type Service struct {
	Metadata ObjectMeta
	Spec     ServiceSpec
}

// ServiceSpec is k8s service spec.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicespec-v1-core
type ServiceSpec struct {
	ClusterIP    string
	ExternalName string
	Type         string
	Ports        []ServicePort
}

// ServicePort is k8s service port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#serviceport-v1-core
type ServicePort struct {
	Name     string
	Protocol string
	Port     int
}

// getTargetLabels returns labels for each port of the given s.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#service
func (s *Service) getTargetLabels(gw *groupWatcher) []map[string]string {
	host := fmt.Sprintf("%s.%s.svc", s.Metadata.Name, s.Metadata.Namespace)
	var ms []map[string]string
	for _, sp := range s.Spec.Ports {
		addr := discoveryutils.JoinHostPort(host, sp.Port)
		m := map[string]string{
			"__address__":                             addr,
			"__meta_kubernetes_service_port_name":     sp.Name,
			"__meta_kubernetes_service_port_protocol": sp.Protocol,
		}
		s.appendCommonLabels(m)
		ms = append(ms, m)
	}
	return ms
}

func (s *Service) appendCommonLabels(m map[string]string) {
	m["__meta_kubernetes_namespace"] = s.Metadata.Namespace
	m["__meta_kubernetes_service_name"] = s.Metadata.Name
	m["__meta_kubernetes_service_type"] = s.Spec.Type
	if s.Spec.Type != "ExternalName" {
		m["__meta_kubernetes_service_cluster_ip"] = s.Spec.ClusterIP
	} else {
		m["__meta_kubernetes_service_external_name"] = s.Spec.ExternalName
	}
	s.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_service", m)
}
