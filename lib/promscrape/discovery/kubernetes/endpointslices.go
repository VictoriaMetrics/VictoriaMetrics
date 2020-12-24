package kubernetes

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getEndpointSlicesLabels returns labels for k8s endpointSlices obtained from the given cfg.
func getEndpointSlicesLabels(cfg *apiConfig) ([]map[string]string, error) {
	eps, err := getEndpointSlices(cfg)
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

// getEndpointSlices retrieves endpointSlice with given apiConfig
func getEndpointSlices(cfg *apiConfig) ([]EndpointSlice, error) {
	if len(cfg.namespaces) == 0 {
		return getEndpointSlicesByPath(cfg, "/apis/discovery.k8s.io/v1beta1/endpointslices")
	}
	// Query /api/v1/namespaces/* for each namespace.
	// This fixes authorization issue at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/432
	cfgCopy := *cfg
	namespaces := cfgCopy.namespaces
	cfgCopy.namespaces = nil
	cfg = &cfgCopy
	var result []EndpointSlice
	for _, ns := range namespaces {
		path := fmt.Sprintf("/apis/discovery.k8s.io/v1beta1/namespaces/%s/endpointslices", ns)
		eps, err := getEndpointSlicesByPath(cfg, path)
		if err != nil {
			return nil, err
		}
		result = append(result, eps...)
	}
	return result, nil
}

// getEndpointSlicesByPath retrieves endpointSlices from k8s api by given path
func getEndpointSlicesByPath(cfg *apiConfig, path string) ([]EndpointSlice, error) {
	data, err := getAPIResponse(cfg, "endpointslices", path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain endpointslices data from API server: %w", err)
	}
	epl, err := parseEndpointSlicesList(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse endpointslices response from API server: %w", err)
	}
	return epl.Items, nil

}

// parseEndpointsList parses EndpointSliceList from data.
func parseEndpointSlicesList(data []byte) (*EndpointSliceList, error) {
	var esl EndpointSliceList
	if err := json.Unmarshal(data, &esl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal EndpointSliceList from %q: %w", data, err)
	}

	return &esl, nil
}

// appendTargetLabels injects labels for endPointSlice to slice map
// follows TargetRef for enrich labels with pod and service metadata
func (eps *EndpointSlice) appendTargetLabels(ms []map[string]string, pods []Pod, svcs []Service) []map[string]string {
	svc := getService(svcs, eps.Metadata.Namespace, eps.Metadata.Name)
	podPortsSeen := make(map[*Pod][]int)
	for _, ess := range eps.Endpoints {
		pod := getPod(pods, ess.TargetRef.Namespace, ess.TargetRef.Name)
		for _, epp := range eps.Ports {
			for _, addr := range ess.Addresses {
				ms = append(ms, getEndpointSliceLabelsForAddressAndPort(podPortsSeen, addr, eps, ess, epp, pod, svc))
			}

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
				if svc != nil {
					svc.appendCommonLabels(m)
				}
				ms = append(ms, m)
			}
		}
	}
	return ms

}

// getEndpointSliceLabelsForAddressAndPort gets labels for endpointSlice
// from  address, Endpoint and EndpointPort
// enriches labels with TargetRef
// pod appended to seen Ports
// if TargetRef matches
func getEndpointSliceLabelsForAddressAndPort(podPortsSeen map[*Pod][]int, addr string, eps *EndpointSlice, ea Endpoint, epp EndpointPort, p *Pod, svc *Service) map[string]string {
	m := getEndpointSliceLabels(eps, addr, ea, epp)
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

// //getEndpointSliceLabels builds labels for given EndpointSlice
func getEndpointSliceLabels(eps *EndpointSlice, addr string, ea Endpoint, epp EndpointPort) map[string]string {

	addr = discoveryutils.JoinHostPort(addr, epp.Port)
	m := map[string]string{
		"__address__":                                               addr,
		"__meta_kubernetes_namespace":                               eps.Metadata.Namespace,
		"__meta_kubernetes_endpointslice_name":                      eps.Metadata.Name,
		"__meta_kubernetes_endpointslice_address_type":              eps.AddressType,
		"__meta_kubernetes_endpointslice_endpoint_conditions_ready": strconv.FormatBool(ea.Conditions.Ready),
		"__meta_kubernetes_endpointslice_port_name":                 epp.Name,
		"__meta_kubernetes_endpointslice_port_protocol":             epp.Protocol,
		"__meta_kubernetes_endpointslice_port":                      strconv.Itoa(epp.Port),
	}
	if epp.AppProtocol != "" {
		m["__meta_kubernetes_endpointslice_port_app_protocol"] = epp.AppProtocol
	}
	if ea.TargetRef.Kind != "" {
		m["__meta_kubernetes_endpointslice_address_target_kind"] = ea.TargetRef.Kind
		m["__meta_kubernetes_endpointslice_address_target_name"] = ea.TargetRef.Name
	}
	if ea.Hostname != "" {
		m["__meta_kubernetes_endpointslice_endpoint_hostname"] = ea.Hostname
	}
	for k, v := range ea.Topology {
		m["__meta_kubernetes_endpointslice_endpoint_topology_"+discoveryutils.SanitizeLabelName(k)] = v
		m["__meta_kubernetes_endpointslice_endpoint_topology_present_"+discoveryutils.SanitizeLabelName(k)] = "true"
	}
	return m
}

// EndpointSliceList - implements kubernetes endpoint slice list object,
// that groups service endpoints slices.
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointslice-v1beta1-discovery-k8s-io
type EndpointSliceList struct {
	Items []EndpointSlice
}

// EndpointSlice - implements kubernetes endpoint slice.
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointslice-v1beta1-discovery-k8s-io
type EndpointSlice struct {
	Metadata    ObjectMeta
	Endpoints   []Endpoint
	AddressType string
	Ports       []EndpointPort
}

// Endpoint implements kubernetes object endpoint for endpoint slice.
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpoint-v1beta1-discovery-k8s-io
type Endpoint struct {
	Addresses  []string
	Conditions EndpointConditions
	Hostname   string
	TargetRef  ObjectReference
	Topology   map[string]string
}

// EndpointConditions implements kubernetes endpoint condition.
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointconditions-v1beta1-discovery-k8s-io
type EndpointConditions struct {
	Ready bool
}
