package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

func (eps *Endpoints) key() string {
	return eps.Metadata.key()
}

func parseEndpointsList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	epsl := &EndpointsList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, epsl); err != nil {
			return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointsList: %w", err)
		}
	case contentTypeProtobuf:
		if err := epsl.unmarshalProtobuf(data); err != nil {
			return nil, epsl.Metadata, fmt.Errorf("cannot unmarshal EndpointsList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, eps := range epsl.Items {
		objectsByKey[eps.key()] = &eps
	}
	return objectsByKey, epsl.Metadata, nil
}

func parseEndpoints(data []byte, contentType string) (object, error) {
	eps := &Endpoints{}
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

// EndpointsList implements k8s endpoints list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointslist-v1-core
type EndpointsList struct {
	Metadata ListMeta
	Items    []Endpoints
}

// unmarshalProtobuf unmarshals EndpointsList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *EndpointsList) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointsList {
	//   optional ListMeta metadata = 1;
	//   repeated Endpoint items = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectMeta: %w", err)
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

func (r *EndpointsList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, pod := range r.Items {
		pod.marshalProtobuf(mm.AppendMessage(2))
	}
}

// Endpoints implements k8s endpoints.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpoints-v1-core
type Endpoints struct {
	Metadata ObjectMeta
	Subsets  []EndpointSubset
}

// unmarshalProtobuf unmarshals Endpoints according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (eps *Endpoints) unmarshalProtobuf(src []byte) (err error) {
	// message Endpoints {
	//   optional ObjectMeta metadata = 1;
	//   repeated EndpointSubset subsets = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Endpoints: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ObjectMeta")
			}
			m := &eps.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ObjectMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Subset")
			}
			eps.Subsets = slicesutil.SetLength(eps.Subsets, len(eps.Subsets)+1)
			s := &eps.Subsets[len(eps.Subsets)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Subset: %w", err)
			}
		}
	}
	return nil
}

func (eps *Endpoints) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	eps.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, subset := range eps.Subsets {
		subset.marshalProtobuf(mm.AppendMessage(2))
	}
}

// EndpointSubset implements k8s endpoint subset.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointsubset-v1-core
type EndpointSubset struct {
	Addresses         []EndpointAddress
	NotReadyAddresses []EndpointAddress
	Ports             []EndpointPort
}

// unmarshalProtobuf unmarshals EndpointSubset according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *EndpointSubset) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointSubset {
	//   repeated EndpointAddress addresses = 1;
	//   repeated EndpointAddress notReadyAddresses = 2;
	//   repeated EndpointPort ports = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointSubset: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Address")
			}
			r.Addresses = slicesutil.SetLength(r.Addresses, len(r.Addresses)+1)
			a := &r.Addresses[len(r.Addresses)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Address: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read NotReadyAddress")
			}
			r.NotReadyAddresses = slicesutil.SetLength(r.NotReadyAddresses, len(r.NotReadyAddresses)+1)
			a := &r.NotReadyAddresses[len(r.NotReadyAddresses)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal NotReadyAddress: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Port")
			}
			r.Ports = slicesutil.SetLength(r.Ports, len(r.Ports)+1)
			p := &r.Ports[len(r.Ports)-1]
			if err := p.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Port: %w", err)
			}
		}
	}
	return nil
}

func (r *EndpointSubset) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, addr := range r.Addresses {
		addr.marshalProtobuf(mm.AppendMessage(1))
	}
	for _, addr := range r.NotReadyAddresses {
		addr.marshalProtobuf(mm.AppendMessage(2))
	}
	for _, port := range r.Ports {
		port.marshalProtobuf(mm.AppendMessage(3))
	}
}

// EndpointAddress implements k8s endpoint address.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointaddress-v1-core
type EndpointAddress struct {
	Hostname  string
	IP        string
	NodeName  string
	TargetRef ObjectReference
}

// unmarshalProtobuf unmarshals EndpointAddress according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *EndpointAddress) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointAddress {
	//   optional string ip = 1;
	//   optional ObjectReference targetRef = 2;
	//   optional string hostname = 3;
	//   optional string nodeName = 4;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointAddress: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			ip, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read IP")
			}
			r.IP = strings.Clone(ip)
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read TargetRef")
			}
			ref := &r.TargetRef
			if err := ref.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal TargetRef: %w", err)
			}
		case 3:
			hostname, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Hostname")
			}
			r.Hostname = strings.Clone(hostname)
		case 4:
			nodeName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read NodeName")
			}
			r.NodeName = strings.Clone(nodeName)
		}
	}
	return nil
}

func (r *EndpointAddress) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.IP)
	r.TargetRef.marshalProtobuf(mm.AppendMessage(2))
	mm.AppendString(3, r.Hostname)
	mm.AppendString(4, r.NodeName)
}

// ObjectReference implements k8s object reference.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectreference-v1-core
type ObjectReference struct {
	Kind      string
	Name      string
	Namespace string
}

// unmarshalProtobuf unmarshals ObjectReference according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ObjectReference) unmarshalProtobuf(src []byte) (err error) {
	// message ObjectReference {
	//   optional string kind = 1;
	//   optional string namespace = 2;
	//   optional string name = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ObjectReference: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			kind, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Kind")
			}
			r.Kind = strings.Clone(kind)
		case 2:
			namespace, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Namespace")
			}
			r.Namespace = strings.Clone(namespace)
		case 3:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		}
	}
	return nil
}

func (r *ObjectReference) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Kind)
	mm.AppendString(2, r.Namespace)
	mm.AppendString(3, r.Name)
}

// EndpointPort implements k8s endpoint port.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#endpointport-v1-discovery-k8s-io
type EndpointPort struct {
	AppProtocol string
	Name        string
	Port        int
	Protocol    string
}

// unmarshalProtobuf unmarshals EndpointPort accordint to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *EndpointPort) unmarshalProtobuf(src []byte) (err error) {
	// message EndpointPort {
	//   optional string name = 1;
	//   optional int32 port = 2;
	//   optional string protocol = 3;
	//   optional string appProtocol = 4;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in EndpointPort: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 2:
			port, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read Port")
			}
			r.Port = int(port)
		case 3:
			protocol, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Protocol")
			}
			r.Protocol = strings.Clone(protocol)
		case 4:
			appProtocol, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read AppProtocol")
			}
			r.AppProtocol = strings.Clone(appProtocol)
		}
	}
	return nil
}

func (r *EndpointPort) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Name)
	mm.AppendInt32(2, int32(r.Port))
	mm.AppendString(3, r.Protocol)
	mm.AppendString(4, r.AppProtocol)
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
