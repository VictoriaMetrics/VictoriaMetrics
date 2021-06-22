package consul

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getServiceNodesLabels returns labels for Consul service nodes with given cfg.
func getServiceNodesLabels(cfg *apiConfig) []map[string]string {
	sns := cfg.consulWatcher.getServiceNodesSnapshot()
	var ms []map[string]string
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
	ID        string
	Service   string
	Address   string
	Namespace string
	Port      int
	Tags      []string
	Meta      map[string]string
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

func parseServiceNodes(data []byte) ([]ServiceNode, error) {
	var sns []ServiceNode
	if err := json.Unmarshal(data, &sns); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ServiceNodes from %q: %w", data, err)
	}
	return sns, nil
}

func (sn *ServiceNode) appendTargetLabels(ms []map[string]string, serviceName, tagSeparator string) []map[string]string {
	var addr string
	if sn.Service.Address != "" {
		addr = discoveryutils.JoinHostPort(sn.Service.Address, sn.Service.Port)
	} else {
		addr = discoveryutils.JoinHostPort(sn.Node.Address, sn.Service.Port)
	}
	m := map[string]string{
		"__address__":                   addr,
		"__meta_consul_address":         sn.Node.Address,
		"__meta_consul_dc":              sn.Node.Datacenter,
		"__meta_consul_health":          aggregatedStatus(sn.Checks),
		"__meta_consul_namespace":       sn.Service.Namespace,
		"__meta_consul_node":            sn.Node.Node,
		"__meta_consul_service":         serviceName,
		"__meta_consul_service_address": sn.Service.Address,
		"__meta_consul_service_id":      sn.Service.ID,
		"__meta_consul_service_port":    strconv.Itoa(sn.Service.Port),
	}
	// We surround the separated list with the separator as well. This way regular expressions
	// in relabeling rules don't have to consider tag positions.
	m["__meta_consul_tags"] = tagSeparator + strings.Join(sn.Service.Tags, tagSeparator) + tagSeparator

	for k, v := range sn.Node.Meta {
		key := discoveryutils.SanitizeLabelName(k)
		m["__meta_consul_metadata_"+key] = v
	}
	for k, v := range sn.Service.Meta {
		key := discoveryutils.SanitizeLabelName(k)
		m["__meta_consul_service_metadata_"+key] = v
	}
	for k, v := range sn.Node.TaggedAddresses {
		key := discoveryutils.SanitizeLabelName(k)
		m["__meta_consul_tagged_address_"+key] = v
	}
	ms = append(ms, m)
	return ms
}

func aggregatedStatus(checks []Check) string {
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
