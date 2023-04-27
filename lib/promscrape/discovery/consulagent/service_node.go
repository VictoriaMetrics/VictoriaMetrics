package consulagent

import (
	"fmt"
	"strconv"
	"strings"

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
	const metaPrefix = "__meta_consulagent_"

	var addr string
	if sn.Service.Address != "" {
		addr = discoveryutils.JoinHostPort(sn.Service.Address, sn.Service.Port)
	} else {
		addr = discoveryutils.JoinHostPort(agent.Member.Addr, sn.Service.Port)
	}

	m := promutils.NewLabels(16)
	m.Add("__address__", addr)
	m.Add(metaPrefix+"address", agent.Member.Addr)
	m.Add(metaPrefix+"dc", agent.Config.Datacenter)
	m.Add(metaPrefix+"health", consul.AggregatedStatus(sn.Checks))
	m.Add(metaPrefix+"namespace", sn.Service.Namespace)
	m.Add(metaPrefix+"node", agent.Config.NodeName)
	m.Add(metaPrefix+"service", serviceName)
	m.Add(metaPrefix+"service_address", sn.Service.Address)
	m.Add(metaPrefix+"service_id", sn.Service.ID)
	m.Add(metaPrefix+"service_port", strconv.Itoa(sn.Service.Port))
	// We surround the separated list with the separator as well. This way regular expressions
	// in relabeling rules don't have to consider tag positions.
	m.Add(metaPrefix+"tags", tagSeparator+strings.Join(sn.Service.Tags, tagSeparator)+tagSeparator)

	// Expose individual tags via __meta_consul_tag_* labels, so users could move all the tags
	// into the discovered scrape target with the following relabeling rule in the way similar to kubernetes_sd_configs:
	//
	// - action: labelmap
	//   regex: __meta_consulagent_tag_(.+)
	//
	// This solves https://stackoverflow.com/questions/44339461/relabeling-in-prometheus
	for _, tag := range sn.Service.Tags {
		k := tag
		v := ""
		if n := strings.IndexByte(tag, '='); n >= 0 {
			k = tag[:n]
			v = tag[n+1:]
		}
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"tag_"+k), v)
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"tagpresent_"+k), "true")
	}

	for k, v := range agent.Meta {
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"metadata_"+k), v)
	}
	for k, v := range sn.Service.Meta {
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"service_metadata_"+k), v)
	}
	for k, v := range sn.Node.TaggedAddresses {
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"tagged_address_"+k), v)
	}
	for k, v := range sn.Service.TaggedAddresses {
		address := fmt.Sprintf("%s:%d", v.Address, v.Port)
		m.Add(discoveryutils.SanitizeLabelName(metaPrefix+"tagged_address_"+k), address)
	}
	ms = append(ms, m)
	return ms
}
