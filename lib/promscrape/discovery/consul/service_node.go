package consul

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// getServiceNodesLabels returns labels for Consul service nodes with given cfg.
func getServiceNodesLabels(cfg *apiConfig) []*promutil.Labels {
	sns := cfg.consulWatcher.getServiceNodesSnapshot()
	var ms []*promutil.Labels
	for svc, sn := range sns {
		for i := range sn {
			ms = sn[i].appendTargetLabels(ms, svc, cfg.tagSeparator)
		}
	}
	return ms
}

// ServiceNode is Consul service node.
//
// See https://www.consul.io/api/health.html#list-nodes-for-service
type ServiceNode struct {
	Service Service
	Node    Node
	Checks  []Check
}

// Service is Consul service.
//
// See https://www.consul.io/api/health.html#list-nodes-for-service
type Service struct {
	ID              string
	Service         string
	Address         string
	Namespace       string
	Partition       string
	Port            int
	Tags            []string
	Meta            map[string]string
	TaggedAddresses map[string]ServiceTaggedAddress
	Datacenter      string
}

// ServiceTaggedAddress is Consul service.
//
// See https://www.consul.io/api/health.html#list-nodes-for-service
type ServiceTaggedAddress struct {
	Address string
	Port    int
}

// Node is Consul node.
//
// See https://www.consul.io/api/health.html#list-nodes-for-service
type Node struct {
	Address         string
	Datacenter      string
	Node            string
	Meta            map[string]string
	TaggedAddresses map[string]string
}

// Check is Consul check.
//
// See https://www.consul.io/api/health.html#list-nodes-for-service
type Check struct {
	CheckID string
	Status  string
}

// ParseServiceNodes return parsed slice of ServiceNode by data.
func ParseServiceNodes(data []byte) ([]ServiceNode, error) {
	var sns []ServiceNode
	if err := json.Unmarshal(data, &sns); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ServiceNodes from %q: %w", data, err)
	}
	return sns, nil
}

func (sn *ServiceNode) appendTargetLabels(ms []*promutil.Labels, serviceName, tagSeparator string) []*promutil.Labels {
	var addr string
	if sn.Service.Address != "" {
		addr = discoveryutil.JoinHostPort(sn.Service.Address, sn.Service.Port)
	} else {
		addr = discoveryutil.JoinHostPort(sn.Node.Address, sn.Service.Port)
	}
	m := promutil.NewLabels(16)
	m.Add("__address__", addr)
	m.Add("__meta_consul_address", sn.Node.Address)
	m.Add("__meta_consul_dc", sn.Node.Datacenter)
	m.Add("__meta_consul_health", AggregatedStatus(sn.Checks))
	m.Add("__meta_consul_namespace", sn.Service.Namespace)
	m.Add("__meta_consul_partition", sn.Service.Partition)
	m.Add("__meta_consul_node", sn.Node.Node)
	m.Add("__meta_consul_service", serviceName)
	m.Add("__meta_consul_service_address", sn.Service.Address)
	m.Add("__meta_consul_service_id", sn.Service.ID)
	m.Add("__meta_consul_service_port", strconv.Itoa(sn.Service.Port))

	discoveryutil.AddTagsToLabels(m, sn.Service.Tags, "__meta_consul_", tagSeparator)

	for k, v := range sn.Node.Meta {
		m.Add(discoveryutil.SanitizeLabelName("__meta_consul_metadata_"+k), v)
	}
	for k, v := range sn.Service.Meta {
		m.Add(discoveryutil.SanitizeLabelName("__meta_consul_service_metadata_"+k), v)
	}
	for k, v := range sn.Node.TaggedAddresses {
		m.Add(discoveryutil.SanitizeLabelName("__meta_consul_tagged_address_"+k), v)
	}
	ms = append(ms, m)
	return ms
}

// AggregatedStatus returns aggregated status of service node checks.
func AggregatedStatus(checks []Check) string {
	// The code has been copy-pasted from HealthChecks.AggregatedStatus in Consul
	var passing, warning, critical, maintenance bool
	for _, check := range checks {
		id := check.CheckID
		if id == "_node_maintenance" || strings.HasPrefix(id, "_service_maintenance:") {
			maintenance = true
			continue
		}

		switch check.Status {
		case "passing":
			passing = true
		case "warning":
			warning = true
		case "critical":
			critical = true
		default:
			return ""
		}
	}
	switch {
	case maintenance:
		return "maintenance"
	case critical:
		return "critical"
	case warning:
		return "warning"
	case passing:
		return "passing"
	default:
		return "passing"
	}
}
