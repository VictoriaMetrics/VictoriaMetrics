package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func (eps *EndpointSlice) key() string {
	return eps.Metadata.key()
}

func parseEndpointSliceList(r io.Reader) (map[string]object, ListMeta, error) {
	var epsl EndpointSliceList
	d := json.NewDecoder(r)
	if err := d.Decode(&epsl); err != nil {
		return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointSliceList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, eps := range epsl.Items {
		objectsByKey[eps.key()] = eps
	}
	return objectsByKey, epsl.Metadata, nil
}

func parseEndpointSlice(data []byte) (object, error) {
	var eps EndpointSlice
	if err := json.Unmarshal(data, &eps); err != nil {
		return nil, err
	}
	return &eps, nil
}

// getTargetLabels returns labels for eps.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#endpointslices
func (eps *EndpointSlice) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	// The associated service name is stored in kubernetes.io/service-name label.
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/#kubernetesioservice-name
	svcName := eps.Metadata.Labels.Get("kubernetes.io/service-name")
	var svc *Service
	if o := gw.getObjectByRoleLocked("service", eps.Metadata.Namespace, svcName); o != nil {
		svc = o.(*Service)
	}
	podPortsSeen := make(map[*Pod][]int)
	var ms []*promutil.Labels
	for _, ess := range eps.Endpoints {
		var p *Pod
		if o := gw.getObjectByRoleLocked("pod", ess.TargetRef.Namespace, ess.TargetRef.Name); o != nil {
			p = o.(*Pod)
		}
		for _, epp := range eps.Ports {
			for _, addr := range ess.Addresses {
				m := getEndpointSliceLabelsForAddressAndPort(gw, podPortsSeen, addr, eps, ess, epp, p, svc)
				// Remove possible duplicate labels, which can appear after getEndpointSliceLabelsForAddressAndPort() call
				m.RemoveDuplicates()
				ms = append(ms, m)
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
			p.appendEndpointSliceLabels(m, eps)
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

// getEndpointSliceLabelsForAddressAndPort gets labels for endpointSlice
// from  address, Endpoint and EndpointPort
// enriches labels with TargetRef
// p appended to seen Ports
// if TargetRef matches
func getEndpointSliceLabelsForAddressAndPort(gw *groupWatcher, podPortsSeen map[*Pod][]int, addr string, eps *EndpointSlice, ea Endpoint, epp EndpointPort,
	p *Pod, svc *Service) *promutil.Labels {
	m := getEndpointSliceLabels(eps, addr, ea, epp)
	if svc != nil {
		svc.appendCommonLabels(m)
	}
	// See https://github.com/prometheus/prometheus/issues/10284
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpointslice", m)
	if ea.TargetRef.Kind != "Pod" || p == nil {
		return m
	}
	// always add pod targetRef, even if epp port doesn't match container port.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2134
	if _, ok := podPortsSeen[p]; !ok {
		podPortsSeen[p] = []int{}
	}
	p.appendCommonLabels(m, gw)
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

// //getEndpointSliceLabels builds labels for given EndpointSlice
func getEndpointSliceLabels(eps *EndpointSlice, addr string, ea Endpoint, epp EndpointPort) *promutil.Labels {
	addr = discoveryutil.JoinHostPort(addr, epp.Port)
	m := promutil.GetLabels()
	m.Add("__address__", addr)
	m.Add("__meta_kubernetes_namespace", eps.Metadata.Namespace)
	m.Add("__meta_kubernetes_endpointslice_name", eps.Metadata.Name)
	m.Add("__meta_kubernetes_endpointslice_address_type", eps.AddressType)
	m.Add("__meta_kubernetes_endpointslice_endpoint_conditions_ready", strconv.FormatBool(ea.Conditions.Ready))
	m.Add("__meta_kubernetes_endpointslice_endpoint_conditions_serving", strconv.FormatBool(ea.Conditions.Serving))
	m.Add("__meta_kubernetes_endpointslice_endpoint_conditions_terminating", strconv.FormatBool(ea.Conditions.Terminating))
	m.Add("__meta_kubernetes_endpointslice_port_name", epp.Name)
	m.Add("__meta_kubernetes_endpointslice_port_protocol", epp.Protocol)
	m.Add("__meta_kubernetes_endpointslice_port", strconv.Itoa(epp.Port))
	if epp.AppProtocol != "" {
		m.Add("__meta_kubernetes_endpointslice_port_app_protocol", epp.AppProtocol)
	}
	if ea.TargetRef.Kind != "" {
		m.Add("__meta_kubernetes_endpointslice_address_target_kind", ea.TargetRef.Kind)
		m.Add("__meta_kubernetes_endpointslice_address_target_name", ea.TargetRef.Name)
	}
	if ea.Hostname != "" {
		m.Add("__meta_kubernetes_endpointslice_endpoint_hostname", ea.Hostname)
	}
	if ea.NodeName != "" {
		m.Add("__meta_kubernetes_endpointslice_endpoint_node_name", ea.NodeName)
	}
	for k, v := range ea.Topology {
		m.Add(discoveryutil.SanitizeLabelName("__meta_kubernetes_endpointslice_endpoint_topology_"+k), v)
		m.Add(discoveryutil.SanitizeLabelName("__meta_kubernetes_endpointslice_endpoint_topology_present_"+k), "true")
	}
	return m
}

// EndpointSliceList - implements kubernetes endpoint slice list object, that groups service endpoints slices.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointslicelist-v1-discovery-k8s-io
type EndpointSliceList struct {
	Metadata ListMeta
	Items    []*EndpointSlice
}

// EndpointSlice - implements kubernetes endpoint slice.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointslice-v1-discovery-k8s-io
type EndpointSlice struct {
	Metadata    ObjectMeta
	Endpoints   []Endpoint
	AddressType string
	Ports       []EndpointPort
}

// Endpoint implements kubernetes object endpoint for endpoint slice.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpoint-v1-discovery-k8s-io
type Endpoint struct {
	Addresses  []string
	Conditions EndpointConditions
	Hostname   string
	TargetRef  ObjectReference
	Topology   map[string]string
	NodeName   string
}

// EndpointConditions implements kubernetes endpoint condition.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#endpointconditions-v1-discovery-k8s-io
type EndpointConditions struct {
	Ready       bool
	Serving     bool
	Terminating bool
}
