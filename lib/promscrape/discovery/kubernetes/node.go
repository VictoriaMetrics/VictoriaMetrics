package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

// getNodesLabels returns labels for k8s nodes obtained from the given cfg
func (n *Node) key() string {
	return n.Metadata.key()
}

func parseNodeList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	nl := &NodeList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, nl); err != nil {
			return nil, nl.Metadata, fmt.Errorf("cannot unmarshal NodeList: %w", err)
		}
	case contentTypeProtobuf:
		if err := nl.unmarshalProtobuf(data); err != nil {
			return nil, nl.Metadata, fmt.Errorf("cannot unmarshal NodeList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, n := range nl.Items {
		objectsByKey[n.key()] = &n
	}
	return objectsByKey, nl.Metadata, nil
}

func parseNode(data []byte, contentType string) (object, error) {
	n := &Node{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, n); err != nil {
			return nil, err
		}
	case contentTypeProtobuf:
		if err := n.unmarshalProtobuf(data); err != nil {
			return nil, err
		}
	}
	return n, nil
}

// NodeList represents NodeList from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#nodelist-v1-core
type NodeList struct {
	Metadata ListMeta
	Items    []Node
}

// unmarshalProtobuf unmarshals NodeList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *NodeList) unmarshalProtobuf(src []byte) (err error) {
	// message NodeList {
	//   optional ListMeta metadata = 1;
	//   repeated Pod items = 2;
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

func (r *NodeList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, item := range r.Items {
		item.marshalProtobuf(mm.AppendMessage(2))
	}
}

// Node represents Node from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#node-v1-core
type Node struct {
	Metadata ObjectMeta
	Status   NodeStatus
	Spec     NodeSpec
}

// unmarshalProtobuf unmarshals Node according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (n *Node) unmarshalProtobuf(src []byte) (err error) {
	// message Node {
	//   optional ObjectMeta metadata = 1;
	//   repeated NodeSpec spec = 2;
	//   optional NodeStatus status = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Node: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ObjectMeta")
			}
			m := &n.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ObjectMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Spec")
			}
			s := &n.Spec
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Spec: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Status")
			}
			s := &n.Status
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Status: %w", err)
			}
		}
	}
	return nil
}

func (n *Node) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	n.Metadata.marshalProtobuf(mm.AppendMessage(1))
	n.Spec.marshalProtobuf(mm.AppendMessage(2))
	n.Status.marshalProtobuf(mm.AppendMessage(3))
}

// NodeStatus represents NodeStatus from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#nodestatus-v1-core
type NodeStatus struct {
	Addresses       []NodeAddress
	DaemonEndpoints NodeDaemonEndpoints
}

// unmarshalProtobuf unmarshals NodeStatus according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *NodeStatus) unmarshalProtobuf(src []byte) (err error) {
	// message NodeStatus {
	//   repeated NodeAddress addresses = 5;
	//   optional NodeDaemonEndpoints daemonEndpoints = 6;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in NodeStatus: %w", err)
		}
		switch fc.FieldNum {
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Address")
			}
			r.Addresses = slicesutil.SetLength(r.Addresses, len(r.Addresses)+1)
			a := &r.Addresses[len(r.Addresses)-1]
			if err := a.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Address: %w", err)
			}
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DaemonEndpoints")
			}
			e := &r.DaemonEndpoints
			if err := e.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal DaemonEndpoint: %w", err)
			}
		}
	}
	return nil
}

func (r *NodeStatus) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, addr := range r.Addresses {
		addr.marshalProtobuf(mm.AppendMessage(5))
	}
	r.DaemonEndpoints.marshalProtobuf(mm.AppendMessage(6))
}

// NodeSpec represents NodeSpec from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#nodespec-v1-core
type NodeSpec struct {
	ProviderID string
}

// unmarshalProtobuf unmarshals NodeSpec according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *NodeSpec) unmarshalProtobuf(src []byte) (err error) {
	// message NodeSpec {
	//   optional string providerID = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in NodeSpec: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			providerID, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read ProviderID")
			}
			r.ProviderID = strings.Clone(providerID)
		}
	}
	return nil
}

func (r *NodeSpec) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(3, r.ProviderID)
}

// NodeAddress represents NodeAddress from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#nodeaddress-v1-core
type NodeAddress struct {
	Type    string
	Address string
}

// unmarshalProtobuf unmarshals NodeAddress according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *NodeAddress) unmarshalProtobuf(src []byte) (err error) {
	// message NodeAddress {
	//   optional string type = 1;
	//   optional string address = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in NodeAddress: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			addrType, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Type")
			}
			r.Type = strings.Clone(addrType)
		case 2:
			addr, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Address")
			}
			r.Address = strings.Clone(addr)
		}
	}
	return nil
}

func (r *NodeAddress) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Type)
	mm.AppendString(2, r.Address)
}

// NodeDaemonEndpoints represents NodeDaemonEndpoints from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#nodedaemonendpoints-v1-core
type NodeDaemonEndpoints struct {
	KubeletEndpoint DaemonEndpoint
}

// unmarshalProtobuf unmarshals NodeDaemonEndpoints according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *NodeDaemonEndpoints) unmarshalProtobuf(src []byte) (err error) {
	// message NodeDaemonEndpoints {
	//   optional DaemonEndpoint kubeletEndpoint = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in NodeDaemonEndpoint: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read KubeletEndpoint")
			}
			e := &r.KubeletEndpoint
			if err := e.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal KubeletEndpoint: %w", err)
			}
		}
	}
	return nil
}

func (r *NodeDaemonEndpoints) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.KubeletEndpoint.marshalProtobuf(mm.AppendMessage(1))
}

// getTargetLabels returns labels for the given n.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#node
func (n *Node) getTargetLabels(_ *groupWatcher) []*promutil.Labels {
	addr := getNodeAddr(n.Status.Addresses)
	if len(addr) == 0 {
		// Skip node without address
		return nil
	}
	addr = discoveryutil.JoinHostPort(addr, n.Status.DaemonEndpoints.KubeletEndpoint.Port)
	m := promutil.GetLabels()
	m.Add("__address__", addr)
	m.Add("instance", n.Metadata.Name)
	m.Add("__meta_kubernetes_node_name", n.Metadata.Name)
	m.Add("__meta_kubernetes_node_provider_id", n.Spec.ProviderID)
	n.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_node", m)
	addrTypesUsed := make(map[string]bool, len(n.Status.Addresses))
	for _, a := range n.Status.Addresses {
		if addrTypesUsed[a.Type] {
			continue
		}
		addrTypesUsed[a.Type] = true
		m.Add(discoveryutil.SanitizeLabelName("__meta_kubernetes_node_address_"+a.Type), a.Address)
	}
	return []*promutil.Labels{m}
}

func getNodeAddr(nas []NodeAddress) string {
	if addr := getAddrByType(nas, "InternalIP"); len(addr) > 0 {
		return addr
	}
	if addr := getAddrByType(nas, "InternalDNS"); len(addr) > 0 {
		return addr
	}
	if addr := getAddrByType(nas, "ExternalIP"); len(addr) > 0 {
		return addr
	}
	if addr := getAddrByType(nas, "ExternalDNS"); len(addr) > 0 {
		return addr
	}
	if addr := getAddrByType(nas, "LegacyHostIP"); len(addr) > 0 {
		return addr
	}
	if addr := getAddrByType(nas, "Hostname"); len(addr) > 0 {
		return addr
	}
	return ""
}

func getAddrByType(nas []NodeAddress, typ string) string {
	for _, na := range nas {
		if na.Type == typ {
			return na.Address
		}
	}
	return ""
}
