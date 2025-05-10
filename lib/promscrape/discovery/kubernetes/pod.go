package kubernetes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/easyproto"
)

func (p *Pod) key() string {
	return p.Metadata.key()
}

func parsePodList(data []byte, contentType string) (map[string]object, ListMeta, error) {
	pl := &PodList{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, pl); err != nil {
			return nil, pl.Metadata, fmt.Errorf("cannot unmarshal PodList: %w", err)
		}
	case contentTypeProtobuf:
		if err := pl.unmarshalProtobuf(data); err != nil {
			return nil, pl.Metadata, fmt.Errorf("cannot unmarshal PodList: %w", err)
		}
	}
	objectsByKey := make(map[string]object)
	for _, p := range pl.Items {
		objectsByKey[p.key()] = &p
	}
	return objectsByKey, pl.Metadata, nil
}

func parsePod(data []byte, contentType string) (object, error) {
	p := &Pod{}
	switch contentType {
	case contentTypeJSON:
		if err := json.Unmarshal(data, p); err != nil {
			return nil, err
		}
	case contentTypeProtobuf:
		if err := p.unmarshalProtobuf(data); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// PodList implements k8s pod list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#podlist-v1-core
type PodList struct {
	Metadata ListMeta
	Items    []Pod
}

// unmarshalProtobuf unmarshals PodList according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *PodList) unmarshalProtobuf(src []byte) (err error) {
	// message PodList {
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

func (r *PodList) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	r.Metadata.marshalProtobuf(mm.AppendMessage(1))
	for _, item := range r.Items {
		item.marshalProtobuf(mm.AppendMessage(2))
	}
}

// Pod implements k8s pod.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#pod-v1-core
type Pod struct {
	Metadata ObjectMeta
	Spec     PodSpec
	Status   PodStatus
}

// unmarshalProtobuf unmarshals Pod according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (p *Pod) unmarshalProtobuf(src []byte) (err error) {
	// message Pod {
	//   optional ObjectMeta metadata = 1;
	//   optional PodSpec spec = 2;
	//   optional PodStatus status = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Pod: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ObjectMeta")
			}
			m := &p.Metadata
			if err := m.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ObjectMeta: %w", err)
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Spec")
			}
			s := &p.Spec
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Spec: %w", err)
			}
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Status")
			}
			s := &p.Status
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Status: %w", err)
			}
		}
	}
	return nil
}

func (p *Pod) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	p.Metadata.marshalProtobuf(mm.AppendMessage(1))
	p.Spec.marshalProtobuf(mm.AppendMessage(2))
	p.Status.marshalProtobuf(mm.AppendMessage(3))
}

// PodSpec implements k8s pod spec.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#podspec-v1-core
type PodSpec struct {
	NodeName       string
	Containers     []Container
	InitContainers []Container
}

// unmarshalProtobuf unmarshals PodSpec according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *PodSpec) unmarshalProtobuf(src []byte) (err error) {
	// message PodSpec {
	//   repeated Container containers = 2;
	//   optional string nodeName = 10;
	//   repeated Container initContainers = 20;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in PodSpec: %w", err)
		}
		switch fc.FieldNum {
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Container")
			}
			r.Containers = slicesutil.SetLength(r.Containers, len(r.Containers)+1)
			c := &r.Containers[len(r.Containers)-1]
			if err := c.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Container: %w", err)
			}
		case 10:
			nodeName, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read NodeName")
			}
			r.NodeName = strings.Clone(nodeName)
		case 20:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read InitContainer")
			}
			r.InitContainers = slicesutil.SetLength(r.InitContainers, len(r.InitContainers)+1)
			c := &r.InitContainers[len(r.InitContainers)-1]
			if err := c.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal InitContainer: %w", err)
			}
		}
	}
	return nil
}

func (r *PodSpec) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	for _, container := range r.Containers {
		container.marshalProtobuf(mm.AppendMessage(2))
	}
	mm.AppendString(10, r.NodeName)
	for _, container := range r.InitContainers {
		container.marshalProtobuf(mm.AppendMessage(20))
	}
}

// Container implements k8s container.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#container-v1-core
type Container struct {
	Name          string
	Image         string
	Ports         []ContainerPort
	RestartPolicy string
}

// unmarshalProtobuf unmarshals Container according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *Container) unmarshalProtobuf(src []byte) (err error) {
	// message Container {
	//   optional string name = 1;
	//   optional string image = 2;
	//   repeated ContainerPort ports = 6;
	//   optional string restartPolicy = 24;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in Container: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 2:
			image, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Image")
			}
			r.Image = strings.Clone(image)
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ContainerPort")
			}
			r.Ports = slicesutil.SetLength(r.Ports, len(r.Ports)+1)
			p := &r.Ports[len(r.Ports)-1]
			if err := p.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ContainerPort: %w", err)
			}
		case 24:
			restartPolicy, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read RestartPolicy")
			}
			r.RestartPolicy = strings.Clone(restartPolicy)
		}
	}
	return nil
}

func (r *Container) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Name)
	mm.AppendString(2, r.Image)
	for _, p := range r.Ports {
		p.marshalProtobuf(mm.AppendMessage(6))
	}
	mm.AppendString(24, r.RestartPolicy)
}

// ContainerPort implements k8s container port.
type ContainerPort struct {
	Name          string
	ContainerPort int
	Protocol      string
}

// unmarshalProtobuf unmarshals ContainerPort according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ContainerPort) unmarshalProtobuf(src []byte) (err error) {
	// message ContainerPort {
	//   optional string name = 1;
	//   optional int32 containerPort = 3;
	//   optional string protocol = 4;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ContainerState: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 3:
			containerPort, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read ContainerPort")
			}
			r.ContainerPort = int(containerPort)
		case 4:
			protocol, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Protocol")
			}
			r.Protocol = strings.Clone(protocol)
		}
	}
	return nil
}

func (r *ContainerPort) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Name)
	mm.AppendInt32(3, int32(r.ContainerPort))
	mm.AppendString(4, r.Protocol)
}

// PodStatus implements k8s pod status.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#podstatus-v1-core
type PodStatus struct {
	Phase                 string
	PodIP                 string
	HostIP                string
	Conditions            []PodCondition
	ContainerStatuses     []ContainerStatus
	InitContainerStatuses []ContainerStatus
}

// unmarshalProtobuf unmarshals PodStatus according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *PodStatus) unmarshalProtobuf(src []byte) (err error) {
	// message PodStatus {
	//   optional string phase = 1;
	//   repeated PodCondition conditions = 2;
	//   optional string hostIP = 5;
	//   optional string podIP = 6;
	//   repeated ContainerStatus containerStatuses = 8;
	//   repeated ContainerStatus initContainerStatuses = 10;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in PodState: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			phase, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Phase")
			}
			r.Phase = strings.Clone(phase)
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read PodCondition")
			}
			r.Conditions = slicesutil.SetLength(r.Conditions, len(r.Conditions)+1)
			c := &r.Conditions[len(r.Conditions)-1]
			if err := c.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal PodCondition: %w", err)
			}
		case 5:
			hostIP, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read HostIP")
			}
			r.HostIP = strings.Clone(hostIP)
		case 6:
			podIP, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read PodIP")
			}
			r.PodIP = strings.Clone(podIP)
		case 8:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ContainerStatus")
			}
			r.ContainerStatuses = slicesutil.SetLength(r.ContainerStatuses, len(r.ContainerStatuses)+1)
			s := &r.ContainerStatuses[len(r.ContainerStatuses)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ContainerStatus: %w", err)
			}
		case 10:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read InitContainerStatus")
			}
			r.InitContainerStatuses = slicesutil.SetLength(r.InitContainerStatuses, len(r.InitContainerStatuses)+1)
			s := &r.InitContainerStatuses[len(r.InitContainerStatuses)-1]
			if err := s.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal InitContainerStatus: %w", err)
			}
		}
	}
	return nil
}

func (r *PodStatus) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Phase)
	for _, condition := range r.Conditions {
		condition.marshalProtobuf(mm.AppendMessage(2))
	}
	mm.AppendString(5, r.HostIP)
	mm.AppendString(6, r.PodIP)
	for _, status := range r.ContainerStatuses {
		status.marshalProtobuf(mm.AppendMessage(8))
	}
	for _, status := range r.InitContainerStatuses {
		status.marshalProtobuf(mm.AppendMessage(10))
	}
}

// PodCondition implements k8s pod condition.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#podcondition-v1-core
type PodCondition struct {
	Type   string
	Status string
}

// unmarshalProtobuf unmarshals PodCondition according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *PodCondition) unmarshalProtobuf(src []byte) (err error) {
	// message PodCondition {
	//   optional string type = 1;
	//   optional string status = 2;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ContainerState: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			conditionType, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Type")
			}
			r.Type = strings.Clone(conditionType)
		case 2:
			status, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Status")
			}
			r.Status = strings.Clone(status)
		}
	}
	return nil
}

func (r *PodCondition) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Type)
	mm.AppendString(2, r.Status)
}

// ContainerStatus implements k8s container status.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#containerstatus-v1-core
type ContainerStatus struct {
	Name        string
	ContainerID string
	State       ContainerState
}

// unmarshalProtobuf unmarshals ContainerStatus according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ContainerStatus) unmarshalProtobuf(src []byte) (err error) {
	// message ContainerStatus {
	//   optional string name = 1;
	//   optional ContainerState state = 2;
	//   optional string containerID = 8;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ContainerState: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			name, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read Name")
			}
			r.Name = strings.Clone(name)
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read ContainerState")
			}
			t := &r.State
			if err := t.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal ContainerState: %w", err)
			}
		case 8:
			containerID, ok := fc.String()
			if !ok {
				return fmt.Errorf("cannot read ContainerID")
			}
			r.ContainerID = strings.Clone(containerID)
		}
	}
	return nil
}

func (r *ContainerStatus) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, r.Name)
	r.State.marshalProtobuf(mm.AppendMessage(2))
	mm.AppendString(8, r.ContainerID)
}

// ContainerState implements k8s container state.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#containerstatus-v1-core
type ContainerState struct {
	Terminated *ContainerStateTerminated
}

// unmarshalProtobuf unmarshals ContainerState according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ContainerState) unmarshalProtobuf(src []byte) (err error) {
	// message ContainerState {
	//   optional ContainerStateTerminated terminated = 3;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ContainerState: %w", err)
		}
		switch fc.FieldNum {
		case 3:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Terminated")
			}
			t := r.Terminated
			if err := t.unmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal Terminated: %w", err)
			}
		}
	}
	return nil
}

func (r *ContainerState) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	if r.Terminated != nil {
		r.Terminated.marshalProtobuf(mm.AppendMessage(3))
	}
}

// ContainerStateTerminated implements k8s terminated container state.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#containerstatus-v1-core
type ContainerStateTerminated struct {
	ExitCode int
}

// unmarshalProtobuf unmarshals ContainerStateTerminated according to spec
//
// See https://github.com/kubernetes/api/blob/master/core/v1/generated.proto
func (r *ContainerStateTerminated) unmarshalProtobuf(src []byte) (err error) {
	// message ContainerStateTerminated {
	//   optional int32 exitCode = 1;
	// }
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("cannot read next field in ContainerStateTerminated: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			exitCode, ok := fc.Int32()
			if !ok {
				return fmt.Errorf("cannot read ExitCode")
			}
			r.ExitCode = int(exitCode)
		}
	}
	return nil
}

func (r *ContainerStateTerminated) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendInt32(1, int32(r.ExitCode))
}

func getContainerID(p *Pod, containerName string, isInit bool) string {
	cs := p.getContainerStatus(containerName, isInit)
	if cs == nil {
		return ""
	}
	return cs.ContainerID
}

func isContainerTerminated(p *Pod, containerName string, isInit bool) bool {
	cs := p.getContainerStatus(containerName, isInit)
	if cs == nil {
		return false
	}
	return cs.State.Terminated != nil
}

func (p *Pod) getContainerStatus(containerName string, isInit bool) *ContainerStatus {
	css := p.Status.ContainerStatuses
	if isInit {
		css = p.Status.InitContainerStatuses
	}
	for i := range css {
		cs := &css[i]
		if cs.Name == containerName {
			return cs
		}
	}
	return nil
}

// getTargetLabels returns labels for each port of the given p.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#pod
func (p *Pod) getTargetLabels(gw *groupWatcher) []*promutil.Labels {
	if len(p.Status.PodIP) == 0 {
		// Skip pod without IP, since such pods cannot be scraped.
		return nil
	}
	if isPodPhaseFinished(p.Status.Phase) {
		// Skip already stopped pod, since it cannot be scraped.
		return nil
	}

	var ms []*promutil.Labels
	ms = appendPodLabels(ms, gw, p, p.Spec.Containers, false)
	ms = appendPodLabels(ms, gw, p, p.Spec.InitContainers, true)
	return ms
}

func isPodPhaseFinished(phase string) bool {
	// See https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase
	return phase == "Succeeded" || phase == "Failed"

}
func appendPodLabels(ms []*promutil.Labels, gw *groupWatcher, p *Pod, cs []Container, isInit bool) []*promutil.Labels {
	for _, c := range cs {
		if isContainerTerminated(p, c.Name, isInit) {
			// Skip terminated containers
			continue
		}
		for _, cp := range c.Ports {
			ms = appendPodLabelsInternal(ms, gw, p, &c, &cp, isInit)
		}
		if len(c.Ports) == 0 {
			ms = appendPodLabelsInternal(ms, gw, p, &c, nil, isInit)
		}
	}
	return ms
}

func appendPodLabelsInternal(ms []*promutil.Labels, gw *groupWatcher, p *Pod, c *Container, cp *ContainerPort, isInit bool) []*promutil.Labels {
	addr := p.Status.PodIP
	if cp != nil {
		addr = discoveryutil.JoinHostPort(addr, cp.ContainerPort)
	} else if discoveryutil.IsIPv6Host(addr) {
		addr = discoveryutil.EscapeIPv6Host(addr)
	}
	m := promutil.GetLabels()
	m.Add("__address__", addr)

	containerID := getContainerID(p, c.Name, isInit)
	if containerID != "" {
		m.Add("__meta_kubernetes_pod_container_id", containerID)
	}

	p.appendCommonLabels(m, gw)
	p.appendContainerLabels(m, c, cp, isInit)
	return append(ms, m)
}

func (p *Pod) appendContainerLabels(m *promutil.Labels, c *Container, cp *ContainerPort, isInit bool) {
	m.Add("__meta_kubernetes_pod_container_image", c.Image)
	m.Add("__meta_kubernetes_pod_container_name", c.Name)
	isInitStr := "false"
	if isInit {
		isInitStr = "true"
	}
	m.Add("__meta_kubernetes_pod_container_init", isInitStr)
	if cp != nil {
		m.Add("__meta_kubernetes_pod_container_port_name", cp.Name)
		m.Add("__meta_kubernetes_pod_container_port_number", bytesutil.Itoa(cp.ContainerPort))
		m.Add("__meta_kubernetes_pod_container_port_protocol", cp.Protocol)
	}
}

func (p *Pod) appendEndpointLabels(m *promutil.Labels, eps *Endpoints) {
	m.Add("__meta_kubernetes_endpoints_name", eps.Metadata.Name)
	m.Add("__meta_kubernetes_endpoint_address_target_kind", "Pod")
	m.Add("__meta_kubernetes_endpoint_address_target_name", p.Metadata.Name)
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpoints", m)
}

func (p *Pod) appendEndpointSliceLabels(m *promutil.Labels, eps *EndpointSlice) {
	m.Add("__meta_kubernetes_endpointslice_name", eps.Metadata.Name)
	m.Add("__meta_kubernetes_endpointslice_address_target_kind", "Pod")
	m.Add("__meta_kubernetes_endpointslice_address_target_name", p.Metadata.Name)
	m.Add("__meta_kubernetes_endpointslice_address_type", eps.AddressType)
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpointslice", m)
}

func (p *Pod) appendCommonLabels(m *promutil.Labels, gw *groupWatcher) {
	if gw.attachNodeMetadata {
		m.Add("__meta_kubernetes_node_name", p.Spec.NodeName)
		o := gw.getObjectByRoleLocked("node", p.Metadata.Namespace, p.Spec.NodeName)
		if o != nil {
			n := o.(*Node)
			n.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_node", m)
		}
	}
	m.Add("__meta_kubernetes_pod_name", p.Metadata.Name)
	m.Add("__meta_kubernetes_pod_ip", p.Status.PodIP)
	m.Add("__meta_kubernetes_pod_ready", getPodReadyStatus(p.Status.Conditions))
	m.Add("__meta_kubernetes_pod_phase", p.Status.Phase)
	m.Add("__meta_kubernetes_pod_node_name", p.Spec.NodeName)
	m.Add("__meta_kubernetes_pod_host_ip", p.Status.HostIP)
	m.Add("__meta_kubernetes_pod_uid", p.Metadata.UID)
	m.Add("__meta_kubernetes_namespace", p.Metadata.Namespace)
	if pc := getPodController(p.Metadata.OwnerReferences); pc != nil {
		if pc.Kind != "" {
			m.Add("__meta_kubernetes_pod_controller_kind", pc.Kind)
		}
		if pc.Name != "" {
			m.Add("__meta_kubernetes_pod_controller_name", pc.Name)
		}
	}
	p.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_pod", m)
}

func getPodController(ors []OwnerReference) *OwnerReference {
	for _, or := range ors {
		if or.Controller {
			return &or
		}
	}
	return nil
}

func getPodReadyStatus(conds []PodCondition) string {
	for _, c := range conds {
		if c.Type == "Ready" {
			return toLowerConverter.Transform(c.Status)
		}
	}
	return "unknown"
}

var toLowerConverter = bytesutil.NewFastStringTransformer(strings.ToLower)
