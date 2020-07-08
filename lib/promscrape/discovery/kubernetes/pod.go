package kubernetes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getPodsLabels returns labels for k8s pods obtained from the given cfg
func getPodsLabels(cfg *apiConfig) ([]map[string]string, error) {
	pods, err := getPods(cfg)
	if err != nil {
		return nil, err
	}
	var ms []map[string]string
	for _, p := range pods {
		ms = p.appendTargetLabels(ms)
	}
	return ms, nil
}

func getPods(cfg *apiConfig) ([]Pod, error) {
	if len(cfg.namespaces) == 0 {
		return getPodsByPath(cfg, "/api/v1/pods")
	}
	// Query /api/v1/namespaces/* for each namespace.
	// This fixes authorization issue at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/432
	cfgCopy := *cfg
	namespaces := cfgCopy.namespaces
	cfgCopy.namespaces = nil
	cfg = &cfgCopy
	var result []Pod
	for _, ns := range namespaces {
		path := fmt.Sprintf("/api/v1/namespaces/%s/pods", ns)
		pods, err := getPodsByPath(cfg, path)
		if err != nil {
			return nil, err
		}
		result = append(result, pods...)
	}
	return result, nil
}

func getPodsByPath(cfg *apiConfig, path string) ([]Pod, error) {
	data, err := getAPIResponse(cfg, "pod", path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain pods data from API server: %w", err)
	}
	pl, err := parsePodList(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pods response from API server: %w", err)
	}
	return pl.Items, nil
}

// PodList implements k8s pod list.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podlist-v1-core
type PodList struct {
	Items []Pod
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
	Phase      string
	PodIP      string
	HostIP     string
	Conditions []PodCondition
}

// PodCondition implements k8s pod condition.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#podcondition-v1-core
type PodCondition struct {
	Type   string
	Status string
}

// parsePodList parses PodList from data.
func parsePodList(data []byte) (*PodList, error) {
	var pl PodList
	if err := json.Unmarshal(data, &pl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal PodList from %q: %w", data, err)
	}
	return &pl, nil
}

// appendTargetLabels appends labels for each port of the given Pod p to ms and returns the result.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#pod
func (p *Pod) appendTargetLabels(ms []map[string]string) []map[string]string {
	if len(p.Status.PodIP) == 0 {
		// Skip pod without IP
		return ms
	}
	ms = appendPodLabels(ms, p, p.Spec.Containers, "false")
	ms = appendPodLabels(ms, p, p.Spec.InitContainers, "true")
	return ms
}

func appendPodLabels(ms []map[string]string, p *Pod, cs []Container, isInit string) []map[string]string {
	for _, c := range cs {
		for _, cp := range c.Ports {
			m := getPodLabels(p, c, &cp, isInit)
			ms = append(ms, m)
		}
		if len(c.Ports) == 0 {
			m := getPodLabels(p, c, nil, isInit)
			ms = append(ms, m)
		}
	}
	return ms
}

func getPodLabels(p *Pod, c Container, cp *ContainerPort, isInit string) map[string]string {
	addr := p.Status.PodIP
	if cp != nil {
		addr = discoveryutils.JoinHostPort(addr, cp.ContainerPort)
	}
	m := map[string]string{
		"__address__":                          addr,
		"__meta_kubernetes_pod_container_init": isInit,
	}
	p.appendCommonLabels(m)
	p.appendContainerLabels(m, c, cp)
	return m
}

func (p *Pod) appendContainerLabels(m map[string]string, c Container, cp *ContainerPort) {
	m["__meta_kubernetes_pod_container_name"] = c.Name
	if cp != nil {
		m["__meta_kubernetes_pod_container_port_name"] = cp.Name
		m["__meta_kubernetes_pod_container_port_number"] = strconv.Itoa(cp.ContainerPort)
		m["__meta_kubernetes_pod_container_port_protocol"] = cp.Protocol
	}
}

func (p *Pod) appendCommonLabels(m map[string]string) {
	m["__meta_kubernetes_pod_name"] = p.Metadata.Name
	m["__meta_kubernetes_pod_ip"] = p.Status.PodIP
	m["__meta_kubernetes_pod_ready"] = getPodReadyStatus(p.Status.Conditions)
	m["__meta_kubernetes_pod_phase"] = p.Status.Phase
	m["__meta_kubernetes_pod_node_name"] = p.Spec.NodeName
	m["__meta_kubernetes_pod_host_ip"] = p.Status.HostIP
	m["__meta_kubernetes_pod_uid"] = p.Metadata.UID
	m["__meta_kubernetes_namespace"] = p.Metadata.Namespace
	if pc := getPodController(p.Metadata.OwnerReferences); pc != nil {
		if pc.Kind != "" {
			m["__meta_kubernetes_pod_controller_kind"] = pc.Kind
		}
		if pc.Name != "" {
			m["__meta_kubernetes_pod_controller_name"] = pc.Name
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
			return strings.ToLower(c.Status)
		}
	}
	return "unknown"
}

func getPod(pods []Pod, namespace, name string) *Pod {
	for i := range pods {
		pod := &pods[i]
		if pod.Metadata.Name == name && pod.Metadata.Namespace == namespace {
			return pod
		}
	}
	return nil
}
