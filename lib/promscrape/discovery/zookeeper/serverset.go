package zookeeper

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type serversetEndpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type serversetMember struct {
	ServiceEndpoint     serversetEndpoint            `json:"serviceEndpoint"`
	AdditionalEndpoints map[string]serversetEndpoint `json:"additionalEndpoints"`
	Status              string                       `json:"status"`
	Shard               int                          `json:"shard"`
}

func getServersetLabels(cfg *apiConfig, paths []string) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, p := range paths {
		children, _, err := cfg.conn.Children(p)
		if err != nil {
			logger.Errorf("serverset_sd_config: cannot list children for path %q: %s", p, err)
			continue
		}
		for _, child := range children {
			childPath := path.Join(p, child)
			data, _, err := cfg.conn.Get(childPath)
			if err != nil {
				logger.Errorf("serverset_sd_config: cannot get data for path %q: %s", childPath, err)
				continue
			}
			if len(data) == 0 {
				continue
			}
			m, err := parseServersetMember(data, childPath)
			if err != nil {
				logger.Errorf("serverset_sd_config: cannot parse data for path %q: %s", childPath, err)
				continue
			}
			ms = append(ms, m)
		}
	}
	return ms
}

func parseServersetMember(data []byte, memberPath string) (*promutil.Labels, error) {
	var member serversetMember
	if err := json.Unmarshal(data, &member); err != nil {
		return nil, fmt.Errorf("cannot unmarshal JSON: %w", err)
	}

	addr := discoveryutil.JoinHostPort(member.ServiceEndpoint.Host, member.ServiceEndpoint.Port)

	// Count labels: __address__ + __meta_serverset_path + host + port + status + shard + additional*2
	labelCount := 6 + len(member.AdditionalEndpoints)*2
	m := promutil.NewLabels(labelCount)
	m.Add("__address__", addr)
	m.Add("__meta_serverset_path", memberPath)
	m.Add("__meta_serverset_endpoint_host", member.ServiceEndpoint.Host)
	m.Add("__meta_serverset_endpoint_port", strconv.Itoa(member.ServiceEndpoint.Port))
	m.Add("__meta_serverset_status", member.Status)
	m.Add("__meta_serverset_shard", strconv.Itoa(member.Shard))

	for name, ep := range member.AdditionalEndpoints {
		sanitizedName := discoveryutil.SanitizeLabelName(name)
		m.Add("__meta_serverset_endpoint_host_"+sanitizedName, ep.Host)
		m.Add("__meta_serverset_endpoint_port_"+sanitizedName, strconv.Itoa(ep.Port))
	}

	return m, nil
}
