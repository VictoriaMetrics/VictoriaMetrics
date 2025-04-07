package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
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
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointslist-v1-core
type EndpointsList struct {
	Metadata ListMeta
	Items    []*Endpoints
}

// Endpoints implements k8s endpoints.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpoints-v1-core
type Endpoints struct {
	Metadata ObjectMeta
	Subsets  []EndpointSubset
}

// EndpointSubset implements k8s endpoint subset.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointsubset-v1-core
type EndpointSubset struct {
	Addresses         []EndpointAddress
	NotReadyAddresses []EndpointAddress
	Ports             []EndpointPort
}

// EndpointAddress implements k8s endpoint address.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointaddress-v1-core
type EndpointAddress struct {
	Hostname  string
	IP        string
	NodeName  string
	TargetRef ObjectReference
}

// ObjectReference implements k8s object reference.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectreference-v1-core
type ObjectReference struct {
	Kind      string
	Name      string
	Namespace string
}

// EndpointPort implements k8s endpoint port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointport-v1-discovery-k8s-io
type EndpointPort struct {
	AppProtocol string
	Name        string
	Port        int
	Protocol    string
}

// getTargetLabels returns labels for each endpoint in eps.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#endpoints
func (eps *Endpoints) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	var svc *Service
	if o := gw.getObjectByRoleLocked("service", eps.Metadata.Namespace, eps.Metadata.Name); o != nil {
		svc = o.(*Service)
	}
	podPortsSeen := make(map[*Pod][]int)
	var ms []*promutil.Labels
	for _, ess := range eps.Subsets {
		for _, epp := range ess.Ports {
			ms = appendEndpointLabelsForAddresses(ms, gw, podPortsSeen, eps, ess.Addresses, epp, svc, "true")
			ms = appendEndpointLabelsForAddresses(ms, gw, podPortsSeen, eps, ess.NotReadyAddresses, epp, svc, "false")
		}
	}
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/#endpoints-kubernetes-io-over-capacity
	// and https://github.com/kubernetes/kubernetes/pull/99975
	switch eps.Metadata.Annotations.Get("endpoints.kubernetes.io/over-capacity") {
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
	appendPodMetadata := func(p *Pod, c *Container, seen []int, isInit bool) {
		for _, cp := range c.Ports {
			if portSeen(cp.ContainerPort, seen) {
				continue
			}
			addr := discoveryutil.JoinHostPort(p.Status.PodIP, cp.ContainerPort)
			m := promutil.GetLabels()
			m.Add("__address__", addr)
			p.appendCommonLabels(m, gw)
			p.appendContainerLabels(m, c, &cp, isInit)

			// Prometheus sets endpoints_name and namespace labels for all endpoints
			// Even if port is not matching service port.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4154
			p.appendEndpointLabels(m, eps)
			if svc != nil {
				svc.appendCommonLabels(m)
			}
			// Remove possible duplicate labels, which can appear after appendCommonLabels() call
			m.RemoveDuplicates()
			ms = append(ms, m)
		}
	}
	for p, ports := range podPortsSeen {
		for _, c := range p.Spec.Containers {
			appendPodMetadata(p, &c, ports, false)
		}
		for _, c := range p.Spec.InitContainers {
			// Defines native sidecar https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/#what-are-sidecar-containers-in-1-28
			if c.RestartPolicy != "Always" {
				continue
			}
			appendPodMetadata(p, &c, ports, true)
		}
	}
	return ms
}

func appendEndpointLabelsForAddresses(ms []*promutil.Labels, gw *groupWatcher, podPortsSeen map[*Pod][]int, eps *Endpoints,
	eas []EndpointAddress, epp EndpointPort, svc *Service, ready string) []*promutil.Labels {
	for _, ea := range eas {
		var p *Pod
		if ea.TargetRef.Name != "" {
			if o := gw.getObjectByRoleLocked("pod", ea.TargetRef.Namespace, ea.TargetRef.Name); o != nil {
				p = o.(*Pod)
			}
		}
		m := getEndpointLabelsForAddressAndPort(gw, podPortsSeen, eps, ea, epp, p, svc, ready)
		// Remove possible duplicate labels, which can appear inside getEndpointLabelsForAddressAndPort()
		m.RemoveDuplicates()
		ms = append(ms, m)
	}
	return ms
}

func getEndpointLabelsForAddressAndPort(gw *groupWatcher, podPortsSeen map[*Pod][]int, eps *Endpoints, ea EndpointAddress, epp EndpointPort,
	p *Pod, svc *Service, ready string) *promutil.Labels {
	m := getEndpointLabels(eps.Metadata, ea, epp, ready)
	if svc != nil {
		svc.appendCommonLabels(m)
	}
	// See https://github.com/prometheus/prometheus/issues/10284
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpoints", m)
	if ea.TargetRef.Kind != "Pod" || p == nil {
		return m
	}
	p.appendCommonLabels(m, gw)
	// always add pod targetRef, even if epp port doesn't match container port
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2134
	if _, ok := podPortsSeen[p]; !ok {
		podPortsSeen[p] = []int{}
	}
	for _, c := range p.Spec.Containers {
		for _, cp := range c.Ports {
			if cp.ContainerPort == epp.Port {
				podPortsSeen[p] = append(podPortsSeen[p], cp.ContainerPort)
				p.appendContainerLabels(m, &c, &cp, false)
				break
			}
		}
	}
	for _, c := range p.Spec.InitContainers {
		// Defines native sidecar https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/#what-are-sidecar-containers-in-1-28
		if c.RestartPolicy != "Always" {
			continue
		}
		for _, cp := range c.Ports {
			if cp.ContainerPort == epp.Port {
				podPortsSeen[p] = append(podPortsSeen[p], cp.ContainerPort)
				p.appendContainerLabels(m, &c, &cp, true)
				break
			}
		}
	}
	return m
}

func getEndpointLabels(om ObjectMeta, ea EndpointAddress, epp EndpointPort, ready string) *promutil.Labels {
	addr := discoveryutil.JoinHostPort(ea.IP, epp.Port)
	m := promutil.GetLabels()
	m.Add("__address__", addr)
	m.Add("__meta_kubernetes_namespace", om.Namespace)
	m.Add("__meta_kubernetes_endpoints_name", om.Name)
	m.Add("__meta_kubernetes_endpoint_ready", ready)
	m.Add("__meta_kubernetes_endpoint_port_name", epp.Name)
	m.Add("__meta_kubernetes_endpoint_port_protocol", epp.Protocol)
	if ea.TargetRef.Kind != "" {
		m.Add("__meta_kubernetes_endpoint_address_target_kind", ea.TargetRef.Kind)
		m.Add("__meta_kubernetes_endpoint_address_target_name", ea.TargetRef.Name)
	}
	if ea.NodeName != "" {
		m.Add("__meta_kubernetes_endpoint_node_name", ea.NodeName)
	}
	if ea.Hostname != "" {
		m.Add("__meta_kubernetes_endpoint_hostname", ea.Hostname)
	}
	return m
}
