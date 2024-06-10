package consulagent

import (
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// getServiceNodesLabels returns labels for Consul service nodes with given cfg.
func getServiceNodesLabels(cfg *apiConfig) []*promutils.Labels {
	sns := cfg.consulWatcher.getServiceNodesSnapshot()
	var ms []*promutils.Labels
	for svc, sn := range sns {
		for i := range sn {
			ms = appendTargetLabels(sn[i], ms, svc, cfg.tagSeparator, cfg.agent)
		}
	}
	return ms
}

func appendTargetLabels(sn consul.ServiceNode, ms []*promutils.Labels, serviceName, tagSeparator string, agent *consul.Agent) []*promutils.Labels {
	var addr string

	// If the service address is not empty it should be used instead of the node address
	// since the service may be registered remotely through a different node.
	if sn.Service.Address != "" {
		addr = discoveryutils.JoinHostPort(sn.Service.Address, sn.Service.Port)
	} else {
		addr = discoveryutils.JoinHostPort(agent.Member.Addr, sn.Service.Port)
	}

	m := promutils.NewLabels(16)
	m.Add("__address__", addr)
	m.Add("__meta_consulagent_address", agent.Member.Addr)
	m.Add("__meta_consulagent_dc", agent.Config.Datacenter)
	m.Add("__meta_consulagent_health", consul.AggregatedStatus(sn.Checks))
	m.Add("__meta_consulagent_namespace", sn.Service.Namespace)
	m.Add("__meta_consulagent_node", agent.Config.NodeName)
	m.Add("__meta_consulagent_service", serviceName)
	m.Add("__meta_consulagent_service_address", sn.Service.Address)
	m.Add("__meta_consulagent_service_id", sn.Service.ID)
	m.Add("__meta_consulagent_service_port", strconv.Itoa(sn.Service.Port))

	discoveryutils.AddTagsToLabels(m, sn.Service.Tags, "__meta_consulagent_", tagSeparator)

	for k, v := range agent.Meta {
		m.Add(discoveryutils.SanitizeLabelName("__meta_consulagent_metadata_"+k), v)
	}
	for k, v := range sn.Service.Meta {
		m.Add(discoveryutils.SanitizeLabelName("__meta_consulagent_service_metadata_"+k), v)
	}
	for k, v := range sn.Node.TaggedAddresses {
		m.Add(discoveryutils.SanitizeLabelName("__meta_consulagent_tagged_address_"+k), v)
	}
	for k, v := range sn.Service.TaggedAddresses {
		address := fmt.Sprintf("%s:%d", v.Address, v.Port)
		m.Add(discoveryutils.SanitizeLabelName("__meta_consulagent_tagged_address_"+k), address)
	}
	ms = append(ms, m)
	return ms
}
