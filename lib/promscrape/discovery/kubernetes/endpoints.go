package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func (eps *Endpoints) key() string {
	return eps.Metadata.key()
}

func parseEndpointsList(r io.Reader) (map[string]object, ListMeta, error) {
	var epsl EndpointsList
	d := json.NewDecoder(r)
	if err := d.Decode(&epsl); err != nil {
		return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointsList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, eps := range epsl.Items {
		objectsByKey[eps.key()] = eps
	}
	return objectsByKey, epsl.Metadata, nil
}

func parseEndpoints(data []byte) (object, error) {
	var eps Endpoints
	if err := json.Unmarshal(data, &eps); err != nil {
		return nil, err
	}
	return &eps, nil
}

// EndpointsList implements k8s endpoints list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#endpointslist-v1-core
type EndpointsList struct {
	Metadata ListMeta
	Items    []*Endpoints
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
// See https://v1-21.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.21/#endpointport-v1-discovery-k8s-io
type EndpointPort struct {
	AppProtocol string
	Name        string
	Port        int
	Protocol    string
}

// getTargetLabels returns labels for each endpoint in eps.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#endpoints
func (eps *Endpoints) getTargetLabels(gw *groupWatcher) []map[string]string {
	var svc *Service
	if o := gw.getObjectByRoleLocked("service", eps.Metadata.Namespace, eps.Metadata.Name); o != nil {
		svc = o.(*Service)
	}
	podPortsSeen := make(map[*Pod][]int)
	var ms []map[string]string
	for _, ess := range eps.Subsets {
		for _, epp := range ess.Ports {
			ms = appendEndpointLabelsForAddresses(ms, gw, podPortsSeen, eps, ess.Addresses, epp, svc, "true")
			ms = appendEndpointLabelsForAddresses(ms, gw, podPortsSeen, eps, ess.NotReadyAddresses, epp, svc, "false")
		}
	}
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/#endpoints-kubernetes-io-over-capacity
	// and https://github.com/kubernetes/kubernetes/pull/99975
	switch eps.Metadata.Annotations.GetByName("endpoints.kubernetes.io/over-capacity") {
	case "truncated":
		logger.Warnf(`the number of targets for "role: endpoints" %q exceeds 1000 and has been truncated; please use "role: endpointslice" instead`, eps.Metadata.key())
	case "warning":
		logger.Warnf(`the number of targets for "role: endpoints" %q exceeds 1000 and will be truncated in the next k8s releases; please use "role: endpointslice" instead`, eps.Metadata.key())
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

func appendEndpointLabelsForAddresses(ms []map[string]string, gw *groupWatcher, podPortsSeen map[*Pod][]int, eps *Endpoints,
	eas []EndpointAddress, epp EndpointPort, svc *Service, ready string) []map[string]string {
	for _, ea := range eas {
		var p *Pod
		if ea.TargetRef.Name != "" {
			if o := gw.getObjectByRoleLocked("pod", ea.TargetRef.Namespace, ea.TargetRef.Name); o != nil {
				p = o.(*Pod)
			}
		}
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
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpoints", m)
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
