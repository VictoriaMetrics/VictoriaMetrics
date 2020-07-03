package consul

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getServiceNodesLabels returns labels for Consul service nodes obtained from the given cfg
func getServiceNodesLabels(cfg *apiConfig) ([]map[string]string, error) {
	sns, err := getAllServiceNodes(cfg)
	if err != nil {
		return nil, err
	}
	var ms []map[string]string
	for _, sn := range sns {
		ms = sn.appendTargetLabels(ms, cfg.tagSeparator)
	}
	return ms, nil
}

func getAllServiceNodes(cfg *apiConfig) ([]ServiceNode, error) {
	// Obtain a list of services
	// See https://www.consul.io/api/catalog.html#list-services
	data, err := getAPIResponse(cfg, "/v1/catalog/services")
	if err != nil {
		return nil, fmt.Errorf("cannot obtain services: %w", err)
	}
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("cannot parse services response %q: %w", data, err)
	}
	serviceNames := make(map[string]bool)
	for serviceName, tags := range m {
		if !shouldCollectServiceByName(cfg.services, serviceName) {
			continue
		}
		if !shouldCollectServiceByTags(cfg.tags, tags) {
			continue
		}
		serviceNames[serviceName] = true
	}

	// Query all the serviceNames in parallel
	type response struct {
		sns []ServiceNode
		err error
	}
	responsesCh := make(chan response, len(serviceNames))
	for serviceName := range serviceNames {
		go func(serviceName string) {
			sns, err := getServiceNodes(cfg, serviceName)
			responsesCh <- response{
				sns: sns,
				err: err,
			}
		}(serviceName)
	}
	var sns []ServiceNode
	err = nil
	for i := 0; i < len(serviceNames); i++ {
		resp := <-responsesCh
		if resp.err != nil && err == nil {
			err = resp.err
		}
		sns = append(sns, resp.sns...)
	}
	if err != nil {
		return nil, err
	}
	return sns, nil
}

func shouldCollectServiceByName(filterServices []string, service string) bool {
	if len(filterServices) == 0 {
		return true
	}
	for _, filterService := range filterServices {
		if filterService == service {
			return true
		}
	}
	return false
}

func shouldCollectServiceByTags(filterTags, tags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	for _, filterTag := range filterTags {
		hasTag := false
		for _, tag := range tags {
			if tag == filterTag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}
	return true
}

func getServiceNodes(cfg *apiConfig, serviceName string) ([]ServiceNode, error) {
	// See https://www.consul.io/api/health.html#list-nodes-for-service
	path := fmt.Sprintf("/v1/health/service/%s", serviceName)
	// The /v1/health/service/:service endpoint supports background refresh caching,
	// which guarantees fresh results obtained from local Consul agent.
	// See https://www.consul.io/api-docs/health#list-nodes-for-service
	// and https://www.consul.io/api/features/caching for details.
	// Query cached results in order to reduce load on Consul cluster.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/574 .
	path += "?cached"
	var tagsArgs []string
	for _, tag := range cfg.tags {
		tagsArgs = append(tagsArgs, fmt.Sprintf("tag=%s", url.QueryEscape(tag)))
	}
	if len(tagsArgs) > 0 {
		path += "&" + strings.Join(tagsArgs, "&")
	}
	data, err := getAPIResponse(cfg, path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain instances for serviceName=%q: %w", serviceName, err)
	}
	return parseServiceNodes(data)
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
	ID      string
	Service string
	Address string
	Port    int
	Tags    []string
	Meta    map[string]string
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

func (sn *ServiceNode) appendTargetLabels(ms []map[string]string, tagSeparator string) []map[string]string {
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
		"__meta_consul_node":            sn.Node.Node,
		"__meta_consul_service":         sn.Service.Service,
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
