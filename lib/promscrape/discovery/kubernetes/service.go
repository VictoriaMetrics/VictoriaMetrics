package kubernetes

import (
	"encoding/json"
	"fmt"
)

// GetServicesLabels returns labels for k8s services obtained from the given cfg.
func GetServicesLabels(cfg *APIConfig) ([]map[string]string, error) {
	svcs, err := getServices(cfg)
	if err != nil {
		return nil, err
	}
	var ms []map[string]string
	for _, svc := range svcs {
		ms = svc.appendTargetLabels(ms)
	}
	return ms, nil
}

func getServices(cfg *APIConfig) ([]Service, error) {
	data, err := getAPIResponse(cfg, "service", "/api/v1/services")
	if err != nil {
		return nil, fmt.Errorf("cannot obtain services data from API server: %s", err)
	}
	sl, err := parseServiceList(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse services response from API server: %s", err)
	}
	return sl.Items, nil
}

// ServiceList is k8s service list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#servicelist-v1-core
type ServiceList struct {
	Items []Service
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

// parseServiceList parses ServiceList from data.
func parseServiceList(data []byte) (*ServiceList, error) {
	var sl ServiceList
	if err := json.Unmarshal(data, &sl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ServiceList from %q: %s", data, err)
	}
	return &sl, nil
}

// appendTargetLabels appends labels for each port of the given Service s to ms and returns the result.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#service
func (s *Service) appendTargetLabels(ms []map[string]string) []map[string]string {
	host := fmt.Sprintf("%s.%s.svc", s.Metadata.Name, s.Metadata.Namespace)
	for _, sp := range s.Spec.Ports {
		addr := joinHostPort(host, sp.Port)
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

func getService(svcs []Service, namespace, name string) *Service {
	for i := range svcs {
		svc := &svcs[i]
		if svc.Metadata.Name == name && svc.Metadata.Namespace == namespace {
			return svc
		}
	}
	return nil
}
