package kubernetes

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// NodeList represents NodeList from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#nodelist-v1-core
type NodeList struct {
	Items    []Node
	Metadata listMetadata `json:"metadata"`
}

// Node represents Node from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#node-v1-core
type Node struct {
	Metadata ObjectMeta
	Status   NodeStatus
}

func (n Node) key() string {
	return n.Metadata.Name
}

// NodeStatus represents NodeStatus from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#nodestatus-v1-core
type NodeStatus struct {
	Addresses       []NodeAddress
	DaemonEndpoints NodeDaemonEndpoints
}

// NodeAddress represents NodeAddress from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#nodeaddress-v1-core
type NodeAddress struct {
	Type    string
	Address string
}

// NodeDaemonEndpoints represents NodeDaemonEndpoints from k8s API.
//
// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#nodedaemonendpoints-v1-core
type NodeDaemonEndpoints struct {
	KubeletEndpoint DaemonEndpoint
}

// parseNodeList parses NodeList from data.
func parseNodeList(data []byte) (*NodeList, error) {
	var nl NodeList
	if err := json.Unmarshal(data, &nl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal NodeList from %q: %w", data, err)
	}
	return &nl, nil
}

// appendTargetLabels appends labels for the given Node n to ms and returns the result.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#node
func (n *Node) appendTargetLabels(ms []map[string]string) []map[string]string {
	addr := getNodeAddr(n.Status.Addresses)
	if len(addr) == 0 {
		// Skip node without address
		return ms
	}
	addr = discoveryutils.JoinHostPort(addr, n.Status.DaemonEndpoints.KubeletEndpoint.Port)
	m := map[string]string{
		"__address__":                 addr,
		"instance":                    n.Metadata.Name,
		"__meta_kubernetes_node_name": n.Metadata.Name,
	}
	n.Metadata.registerLabelsAndAnnotations("__meta_kubernetes_node", m)
	addrTypesUsed := make(map[string]bool, len(n.Status.Addresses))
	for _, a := range n.Status.Addresses {
		if addrTypesUsed[a.Type] {
			continue
		}
		addrTypesUsed[a.Type] = true
		ln := discoveryutils.SanitizeLabelName(a.Type)
		m["__meta_kubernetes_node_address_"+ln] = a.Address
	}
	ms = append(ms, m)
	return ms
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

func processNode(cfg *apiConfig, n *Node, action string) {
	key := buildSyncKey("nodes", cfg.setName, n.key())
	switch action {
	case "ADDED", "MODIFIED":
		lbs := n.appendTargetLabels(nil)
		cfg.targetChan <- SyncEvent{
			Labels:           lbs,
			ConfigSectionSet: cfg.setName,
			Key:              key,
		}
	case "DELETED":
		cfg.targetChan <- SyncEvent{
			ConfigSectionSet: cfg.setName,
			Key:              key,
		}
	case "ERROR":
	default:
		logger.Warnf("unexpected action: %s", action)
	}
}
