package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func (p *Pod) key() string {
	return p.Metadata.key()
}

func parsePodList(r io.Reader) (map[string]object, ListMeta, error) {
	var pl PodList
	d := json.NewDecoder(r)
	if err := d.Decode(&pl); err != nil {
		return nil, pl.Metadata, fmt.Errorf("cannot unmarshal PodList: %w", err)
	}
	objectsByKey := make(map[string]object)
	for _, p := range pl.Items {
		objectsByKey[p.key()] = p
	}
	return objectsByKey, pl.Metadata, nil
}

func parsePod(data []byte) (object, error) {
	var p Pod
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// PodList implements k8s pod list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podlist-v1-core
type PodList struct {
	Metadata ListMeta
	Items    []*Pod
}

// Pod implements k8s pod.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#pod-v1-core
type Pod struct {
	Metadata ObjectMeta
	Spec     PodSpec
	Status   PodStatus
}

// PodSpec implements k8s pod spec.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podspec-v1-core
type PodSpec struct {
	NodeName       string
	Containers     []Container
	InitContainers []Container
}

// Container implements k8s container.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#container-v1-core
type Container struct {
	Name  string
	Image string
	Ports []ContainerPort
}

// ContainerPort implements k8s container port.
type ContainerPort struct {
	Name          string
	ContainerPort int
	Protocol      string
}

// PodStatus implements k8s pod status.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podstatus-v1-core
type PodStatus struct {
	Phase                 string
	PodIP                 string
	HostIP                string
	Conditions            []PodCondition
	ContainerStatuses     []ContainerStatus
	InitContainerStatuses []ContainerStatus
}

// PodCondition implements k8s pod condition.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podcondition-v1-core
type PodCondition struct {
	Type   string
	Status string
}

// ContainerStatus implements k8s container status.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#containerstatus-v1-core
type ContainerStatus struct {
	Name        string
	ContainerID string
}

func getContainerID(p *Pod, containerName string, isInit bool) string {
	css := p.Status.ContainerStatuses
	if isInit {
		css = p.Status.InitContainerStatuses
	}
	for _, cs := range css {
		if cs.Name == containerName {
			return cs.ContainerID
		}
	}
	return ""
}

// getTargetLabels returns labels for each port of the given p.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#pod
func (p *Pod) getTargetLabels(gw *groupWatcher) []*promutils.Labels {
	if len(p.Status.PodIP) == 0 {
		// Skip pod without IP
		return nil
	}
	var ms []*promutils.Labels
	ms = appendPodLabels(ms, gw, p, p.Spec.Containers, false)
	ms = appendPodLabels(ms, gw, p, p.Spec.InitContainers, true)
	return ms
}

func appendPodLabels(ms []*promutils.Labels, gw *groupWatcher, p *Pod, cs []Container, isInit bool) []*promutils.Labels {
	for _, c := range cs {
		for _, cp := range c.Ports {
			ms = appendPodLabelsInternal(ms, gw, p, c, &cp, isInit)
		}
		if len(c.Ports) == 0 {
			ms = appendPodLabelsInternal(ms, gw, p, c, nil, isInit)
		}
	}
	return ms
}

func appendPodLabelsInternal(ms []*promutils.Labels, gw *groupWatcher, p *Pod, c Container, cp *ContainerPort, isInit bool) []*promutils.Labels {
	addr := p.Status.PodIP
	if cp != nil {
		addr = discoveryutils.JoinHostPort(addr, cp.ContainerPort)
	}
	m := promutils.GetLabels()
	m.Add("__address__", addr)
	isInitStr := "false"
	if isInit {
		isInitStr = "true"
	}
	m.Add("__meta_kubernetes_pod_container_init", isInitStr)

	containerID := getContainerID(p, c.Name, isInit)
	if containerID != "" {
		m.Add("__meta_kubernetes_pod_container_id", containerID)
	}

	p.appendCommonLabels(m, gw)
	p.appendContainerLabels(m, c, cp)
	return append(ms, m)
}

func (p *Pod) appendContainerLabels(m *promutils.Labels, c Container, cp *ContainerPort) {
	m.Add("__meta_kubernetes_pod_container_image", c.Image)
	m.Add("__meta_kubernetes_pod_container_name", c.Name)
	if cp != nil {
		m.Add("__meta_kubernetes_pod_container_port_name", cp.Name)
		m.Add("__meta_kubernetes_pod_container_port_number", bytesutil.Itoa(cp.ContainerPort))
		m.Add("__meta_kubernetes_pod_container_port_protocol", cp.Protocol)
	}
}

func (p *Pod) appendEndpointLabels(m *promutils.Labels, eps *Endpoints) {
	m.Add("__meta_kubernetes_endpoints_name", eps.Metadata.Name)
	m.Add("__meta_kubernetes_endpoint_address_target_kind", "Pod")
	m.Add("__meta_kubernetes_endpoint_address_target_name", p.Metadata.Name)
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpoints", m)
}

func (p *Pod) appendEndpointSliceLabels(m *promutils.Labels, eps *EndpointSlice) {
	m.Add("__meta_kubernetes_endpointslice_name", eps.Metadata.Name)
	m.Add("__meta_kubernetes_endpointslice_address_target_kind", "Pod")
	m.Add("__meta_kubernetes_endpointslice_address_target_name", p.Metadata.Name)
	m.Add("__meta_kubernetes_endpointslice_address_type", eps.AddressType)
	eps.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_endpointslice", m)
}

func (p *Pod) appendCommonLabels(m *promutils.Labels, gw *groupWatcher) {
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
