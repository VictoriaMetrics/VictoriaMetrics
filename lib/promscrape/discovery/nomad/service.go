package nomad

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// getServiceLabels returns labels for Nomad services with given cfg.
func getServiceLabels(cfg *apiConfig) []*promutils.Labels {
	svcs := cfg.nomadWatcher.getServiceSnapshot()
	var ms []*promutils.Labels
	for _, s := range svcs {
		for i := range s {
			ms = s[i].appendTargetLabels(ms, cfg.tagSeparator)
		}
	}
	return ms
}

type ServiceList struct {
	Namespace string `json:"Namespace"`
	Services  []struct {
		ServiceName string   `json:"ServiceName"`
		Tags        []string `json:"Tags"`
	} `json:"Services"`
}

// Service is Nomad service.
// See https://developer.hashicorp.com/nomad/api-docs/services#list-services
type Service struct {
	ID          string   `json:"ID"`
	ServiceName string   `json:"ServiceName"`
	Namespace   string   `json:"Namespace"`
	NodeID      string   `json:"NodeID"`
	Datacenter  string   `json:"Datacenter"`
	JobID       string   `json:"JobID"`
	AllocID     string   `json:"AllocID"`
	Tags        []string `json:"Tags"`
	Address     string   `json:"Address"`
	Port        int      `json:"Port"`
}

func parseServices(data []byte) ([]Service, error) {
	var sns []Service
	if err := json.Unmarshal(data, &sns); err != nil {
		return nil, fmt.Errorf("cannot unmarshal Services from %q: %w", data, err)
	}
	return sns, nil
}

func (svc *Service) appendTargetLabels(ms []*promutils.Labels, tagSeparator string) []*promutils.Labels {
	addr := discoveryutils.JoinHostPort(svc.Address, svc.Port)
	m := promutils.NewLabels(16)
	m.Add("__address__", addr)
	m.Add("__meta_nomad_address", svc.Address)
	m.Add("__meta_nomad_dc", svc.Datacenter)
	m.Add("__meta_nomad_namespace", svc.Namespace)
	m.Add("__meta_nomad_node_id", svc.NodeID)
	m.Add("__meta_nomad_service", svc.ServiceName)
	m.Add("__meta_nomad_service_address", svc.Address)
	m.Add("__meta_nomad_service_alloc_id", svc.AllocID)
	m.Add("__meta_nomad_service_id", svc.ID)
	m.Add("__meta_nomad_service_job_id", svc.JobID)
	m.Add("__meta_nomad_service_port", strconv.Itoa(svc.Port))
	// We surround the separated list with the separator as well. This way regular expressions
	// in relabeling rules don't have to consider tag positions.
	m.Add("__meta_nomad_tags", tagSeparator+strings.Join(svc.Tags, tagSeparator)+tagSeparator)

	// Expose individual tags via __meta_nomad_tag_* labels, so users could move all the tags
	// into the discovered scrape target with the following relabeling rule in the way similar to kubernetes_sd_configs:
	//
	// - action: labelmap
	//   regex: __meta_nomad_tag_(.+)
	//
	// This solves https://stackoverflow.com/questions/44339461/relabeling-in-prometheus
	for _, tag := range svc.Tags {
		k := tag
		v := ""
		if n := strings.IndexByte(tag, '='); n >= 0 {
			k = tag[:n]
			v = tag[n+1:]
		}
		m.Add(discoveryutils.SanitizeLabelName("__meta_nomad_tag_"+k), v)
		m.Add(discoveryutils.SanitizeLabelName("__meta_nomad_tagpresent_"+k), "true")
	}

	ms = append(ms, m)
	return ms
}
