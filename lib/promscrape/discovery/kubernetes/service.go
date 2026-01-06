package kubernetes

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// Service is k8s service.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#service-v1-core
type Service struct {
	ObjectMeta `json:"metadata"`
	Spec       ServiceSpec
}

// ServiceSpec is k8s service spec.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#servicespec-v1-core
type ServiceSpec struct {
	ClusterIP    string
	ExternalName string
	Type         string
	Ports        []ServicePort
}

// ServicePort is k8s service port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#serviceport-v1-core
type ServicePort struct {
	Name     string
	Protocol string
	Port     int
}

// getTargetLabels returns labels for each port of the given s.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#service
func (s *Service) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	host := fmt.Sprintf("%s.%s.svc", s.Name, s.Namespace)
	var ms []*promutil.Labels
	for _, sp := range s.Spec.Ports {
		addr := discoveryutil.JoinHostPort(host, sp.Port)
		m := promutil.GetLabels()
		m.Add("__address__", addr)
		m.Add("__meta_kubernetes_service_port_name", sp.Name)
		m.Add("__meta_kubernetes_service_port_number", strconv.Itoa(sp.Port))
		m.Add("__meta_kubernetes_service_port_protocol", sp.Protocol)
		s.appendCommonLabels(m, gw)
		ms = append(ms, m)
	}
	return ms
}

func (s *Service) appendCommonLabels(m *promutil.Labels, gw *groupWatcher) {
	m.Add("__meta_kubernetes_namespace", s.Namespace)
	m.Add("__meta_kubernetes_service_name", s.Name)
	m.Add("__meta_kubernetes_service_type", s.Spec.Type)
	if s.Spec.Type != "ExternalName" {
		m.Add("__meta_kubernetes_service_cluster_ip", s.Spec.ClusterIP)
	} else {
		m.Add("__meta_kubernetes_service_external_name", s.Spec.ExternalName)
	}
	if gw.attachNamespaceMetadata {
		o := gw.getObjectByRoleLocked("namespace", "", s.Namespace)
		if o != nil {
			ns := o.(*Namespace)
			ns.registerLabelsAndAnnotations("__meta_kubernetes_namespace", m)
		}
	}
	s.registerLabelsAndAnnotations("__meta_kubernetes_service", m)
}
