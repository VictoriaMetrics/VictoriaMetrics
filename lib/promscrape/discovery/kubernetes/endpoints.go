package kubernetes

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getEndpointsLabels returns labels for k8s endpoints obtained from the given cfg.
func getEndpointsLabels(cfg *apiConfig) ([]map[string]string, error) {
	eps, err := getEndpoints(cfg)
	if err != nil {
		return nil, err
	}
	pods, err := getPods(cfg)
	if err != nil {
		return nil, err
	}
	svcs, err := getServices(cfg)
	if err != nil {
		return nil, err
	}
	var ms []map[string]string
	for _, ep := range eps {
		ms = ep.appendTargetLabels(ms, pods, svcs)
	}
	return ms, nil
}

func getEndpoints(cfg *apiConfig) ([]Endpoints, error) {
	if len(cfg.namespaces) == 0 {
		return getEndpointsByPath(cfg, "/api/v1/endpoints")
	}
	// Query /api/v1/namespaces/* for each namespace.
	// This fixes authorization issue at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/432
	cfgCopy := *cfg
	namespaces := cfgCopy.namespaces
	cfgCopy.namespaces = nil
	cfg = &cfgCopy
	var result []Endpoints
	for _, ns := range namespaces {
		path := fmt.Sprintf("/api/v1/namespaces/%s/endpoints", ns)
		eps, err := getEndpointsByPath(cfg, path)
		if err != nil {
			return nil, err
		}
		result = append(result, eps...)
	}
	return result, nil
}

func getEndpointsByPath(cfg *apiConfig, path string) ([]Endpoints, error) {
	data, err := getAPIResponse(cfg, "endpoints", path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain endpoints data from API server: %w", err)
	}
	epl, err := parseEndpointsList(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse endpoints response from API server: %w", err)
	}
	return epl.Items, nil
}

// EndpointsList implements k8s endpoints list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointslist-v1-core
type EndpointsList struct {
	Items []Endpoints
}

// Endpoints implements k8s endpoints.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpoints-v1-core
type Endpoints struct {
	Metadata ObjectMeta
	Subsets  []EndpointSubset
}

// EndpointSubset implements k8s endpoint subset.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointsubset-v1-core
type EndpointSubset struct {
	Addresses         []EndpointAddress
	NotReadyAddresses []EndpointAddress
	Ports             []EndpointPort
}

// EndpointAddress implements k8s endpoint address.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointaddress-v1-core
type EndpointAddress struct {
	Hostname  string
	IP        string
	NodeName  string
	TargetRef ObjectReference
}

// ObjectReference implements k8s object reference.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectreference-v1-core
type ObjectReference struct {
	Kind      string
	Name      string
	Namespace string
}

// EndpointPort implements k8s endpoint port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointport-v1beta1-discovery-k8s-io
type EndpointPort struct {
	AppProtocol string
	Name        string
	Port        int
	Protocol    string
}

// parseEndpointsList parses EndpointsList from data.
func parseEndpointsList(data []byte) (*EndpointsList, error) {
	var esl EndpointsList
	if err := json.Unmarshal(data, &esl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal EndpointsList from %q: %w", data, err)
	}
	return &esl, nil
}

// appendTargetLabels appends labels for each endpoint in eps to ms and returns the result.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#endpoints
func (eps *Endpoints) appendTargetLabels(ms []map[string]string, pods []Pod, svcs []Service) []map[string]string {
	svc := getService(svcs, eps.Metadata.Namespace, eps.Metadata.Name)
	podPortsSeen := make(map[*Pod][]int)
	for _, ess := range eps.Subsets {
		for _, epp := range ess.Ports {
			ms = appendEndpointLabelsForAddresses(ms, podPortsSeen, eps, ess.Addresses, epp, pods, svc, "true")
			ms = appendEndpointLabelsForAddresses(ms, podPortsSeen, eps, ess.NotReadyAddresses, epp, pods, svc, "false")
		}
	}

	// Append labels for skipped ports on seen pods.
	portSeen := func(port int, ports []int) bool {
		for _, p := range ports {
			if p == port {
				return true
			}
		}
		return false
	}
	for p, ports := range podPortsSeen {
		for _, c := range p.Spec.Containers {
			for _, cp := range c.Ports {
				if portSeen(cp.ContainerPort, ports) {
					continue
				}
				addr := discoveryutils.JoinHostPort(p.Status.PodIP, cp.ContainerPort)
				m := map[string]string{
					"__address__": addr,
				}
				p.appendCommonLabels(m)
				p.appendContainerLabels(m, c, &cp)
				ms = append(ms, m)
			}
		}
	}
	return ms
}

func appendEndpointLabelsForAddresses(ms []map[string]string, podPortsSeen map[*Pod][]int, eps *Endpoints, eas []EndpointAddress, epp EndpointPort,
	pods []Pod, svc *Service, ready string) []map[string]string {
	for _, ea := range eas {
		p := getPod(pods, ea.TargetRef.Namespace, ea.TargetRef.Name)
		m := getEndpointLabelsForAddressAndPort(podPortsSeen, eps, ea, epp, p, svc, ready)
		ms = append(ms, m)
	}
	return ms
}

func getEndpointLabelsForAddressAndPort(podPortsSeen map[*Pod][]int, eps *Endpoints, ea EndpointAddress, epp EndpointPort, p *Pod, svc *Service, ready string) map[string]string {
	m := getEndpointLabels(eps.Metadata, ea, epp, ready)
	if svc != nil {
		svc.appendCommonLabels(m)
	}
	if ea.TargetRef.Kind != "Pod" || p == nil {
		return m
	}
	p.appendCommonLabels(m)
	for _, c := range p.Spec.Containers {
		for _, cp := range c.Ports {
			if cp.ContainerPort == epp.Port {
				p.appendContainerLabels(m, c, &cp)
				podPortsSeen[p] = append(podPortsSeen[p], cp.ContainerPort)
				break
			}
		}
	}
	return m
}

func getEndpointLabels(om ObjectMeta, ea EndpointAddress, epp EndpointPort, ready string) map[string]string {
	addr := discoveryutils.JoinHostPort(ea.IP, epp.Port)
	m := map[string]string{
		"__address__":                      addr,
		"__meta_kubernetes_namespace":      om.Namespace,
		"__meta_kubernetes_endpoints_name": om.Name,

		"__meta_kubernetes_endpoint_ready":         ready,
		"__meta_kubernetes_endpoint_port_name":     epp.Name,
		"__meta_kubernetes_endpoint_port_protocol": epp.Protocol,
	}
	if ea.TargetRef.Kind != "" {
		m["__meta_kubernetes_endpoint_address_target_kind"] = ea.TargetRef.Kind
		m["__meta_kubernetes_endpoint_address_target_name"] = ea.TargetRef.Name
	}
	if ea.NodeName != "" {
		m["__meta_kubernetes_endpoint_node_name"] = ea.NodeName
	}
	if ea.Hostname != "" {
		m["__meta_kubernetes_endpoint_hostname"] = ea.Hostname
	}
	return m
}
