package kubernetes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

func (eps *EndpointSlice) key() string {
	return eps.Metadata.key()
}

func parseEndpointSliceList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	epsl := &EndpointSliceList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, epsl); err != nil {
			return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointSliceList: %w", err)
		}
	case contentTypeProtobuf:
		if err := epsl.unmarshalProtobuf(data); err != nil {
			return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointSliceList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, eps := range epsl.Items {
		objectsByKey[eps.key()] = &eps
	}
	return objectsByKey, epsl.Metadata, nil
}

func parseEndpointSlice(data []byte, contentType string) (object, error) {
	eps := &EndpointSlice{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, eps); err != nil {
			return nil, err
		}
	case contentTypeProtobuf:
		if err := eps.unmarshalProtobuf(data); err != nil {
			return nil, err
		}
	}
	return eps, nil
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
		m.Add("__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname", ea.NodeName)
		m.Add("__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname", "true")
	}
	if ea.Zone != "" {
		m.Add("__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_zone", ea.Zone)
		m.Add("__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_zone", "true")
	}
	return m
}

// EndpointSliceList - implements kubernetes endpoint slice list object, that groups service endpoints slices.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointslicelist-v1-discovery-k8s-io
type EndpointSliceList struct {
	Metadata ListMeta
	Items    []EndpointSlice
}

// unmarshalProtobuf unmarshals EndpointSliceList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *EndpointSliceList) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointSliceList {
	//   optional ListMeta metadata = 1;
	//   repeated EndpoinSlice items = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointSliceList: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ListMeta")
			}
			m := &r.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ListMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Items")
			}
			r.Items = slicesutil.SetLength(r.Items, len(r.Items)+1)
			s := &r.Items[len(r.Items)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Items: %w", err)
			}
		}
	}
	return nil
}

func (r *EndpointSliceList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, item := range r.Items {
		item.marshalProtobuf(mm.AppendMessage(2))
	}
}

// EndpointSlice - implements kubernetes endpoint slice.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointslice-v1-discovery-k8s-io
type EndpointSlice struct {
	Metadata    ObjectMeta
	Endpoints   []Endpoint
	AddressType string
	Ports       []EndpointPort
}

// unmarshalProtobuf unmarshals EndpointSlice according to spec
//
// See https://github.com/kubernetes/api/blob/master/discovery/v1/generated.proto
func (eps *EndpointSlice) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointSlice {
	//   optional ObjectMeta metadata = 1;
	//   repeated Endpoint endpoints = 2;
	//   repeated EndpointPort ports = 3;
	//   optional string addressType = 4;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointSlice: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Metadata")
			}
			m := &eps.Metadata
			if err = m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Metadata: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Endpoint")
			}
			eps.Endpoints = slicesutil.SetLength(eps.Endpoints, len(eps.Endpoints)+1)
			e := &eps.Endpoints[len(eps.Endpoints)-1]
			if err := e.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Endpoint: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read EndpointPort")
			}
			eps.Ports = slicesutil.SetLength(eps.Ports, len(eps.Ports)+1)
			p := &eps.Ports[len(eps.Ports)-1]
			if err := p.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal EndpointPort: %w", err)
			}
		case 4:
			addressType, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read addressType")
			}
			eps.AddressType = strings.Clone(addressType)
		}
	}
	return nil
}

func (eps *EndpointSlice) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	eps.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, ep := range eps.Endpoints {
		ep.marshalProtobuf(mm.AppendMessage(2))
	}
	for _, p := range eps.Ports {
		p.marshalProtobuf(mm.AppendMessage(3))
	}
	mm.AppendString(4, eps.AddressType)
}

// Endpoint implements kubernetes object endpoint for endpoint slice.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpoint-v1-discovery-k8s-io
type Endpoint struct {
	Addresses  []string
	Conditions EndpointConditions
	Hostname   string
	TargetRef  ObjectReference
	NodeName   string
	Zone       string
}

// unmarshalProtobuf unmarshals Endpoint according to spec
//
// See https://github.com/kubernetes/api/blob/master/discovery/v1/generated.proto
func (r *Endpoint) unmarshalProtobuf(src []byte) (err error) {
	// message Endpoint {
	//   repeated string addresses = 1;
	//   optional EndpointConditions conditions = 2;
	//   optional string hostname = 3;
	//   optional ObjectReference targetRef = 4;
	//   optional string nodeName = 6;
	//   optional string zone = 7;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Endpoint: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			address, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address")
			}
			r.Addresses = append(r.Addresses, address)
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Conditions")
			}
			c := &r.Conditions
			if err := c.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Conditions: %w", err)
			}
		case 3:
			hostname, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Hostname")
			}
			r.Hostname = strings.Clone(hostname)
		case 4:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read TargetRef")
			}
			t := &r.TargetRef
			if err := t.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal TargetRef: %w", err)
			}
		case 6:
			nodeName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read NodeName")
			}
			r.NodeName = strings.Clone(nodeName)
		case 7:
			zone, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Zone")
			}
			r.Zone = strings.Clone(zone)
		}
	}
	return nil
}

func (r *Endpoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, addr := range r.Addresses {
		mm.AppendString(1, addr)
	}
	r.Conditions.marshalProtobuf(mm.AppendMessage(2))
	mm.AppendString(3, r.Hostname)
	r.TargetRef.marshalProtobuf(mm.AppendMessage(4))
	mm.AppendString(6, r.NodeName)
	mm.AppendString(7, r.Zone)
}

// EndpointConditions implements kubernetes endpoint condition.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointconditions-v1-discovery-k8s-io
type EndpointConditions struct {
	Ready bool
}

// unmarshalProtobuf unmarshals EndpointConditions according to spec
//
// See https://github.com/kubernetes/api/blob/master/discovery/v1/generated.proto
func (r *EndpointConditions) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointConditions {
	//   optional bool ready = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointConditions: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ready, ok := fc.Bool()
			if !ok {
				return fmt.Errorf("cannot read Ready")
			}
			r.Ready = ready
		}
	}
	return nil
}

func (r *EndpointConditions) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendBool(1, r.Ready)
}
