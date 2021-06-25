package dockerswarm

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://docs.docker.com/engine/api/v1.40/#tag/Network
type network struct {
	ID       string
	Name     string
	Scope    string
	Internal bool
	Ingress  bool
	Labels   map[string]string
}

func getNetworksLabelsByNetworkID(cfg *apiConfig) (map[string]map[string]string, error) {
	networks, err := getNetworks(cfg)
	if err != nil {
		return nil, err
	}
	return getNetworkLabelsByNetworkID(networks), nil
}

func getNetworks(cfg *apiConfig) ([]network, error) {
	resp, err := cfg.getAPIResponse("/networks")
	if err != nil {
		return nil, fmt.Errorf("cannot query dockerswarm api for networks: %w", err)
	}
	return parseNetworks(resp)
}

func parseNetworks(data []byte) ([]network, error) {
	var networks []network
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, fmt.Errorf("cannot parse networks: %w", err)
	}
	return networks, nil
}

func getNetworkLabelsByNetworkID(networks []network) map[string]map[string]string {
	ms := make(map[string]map[string]string)
	for _, network := range networks {
		m := map[string]string{
			"__meta_dockerswarm_network_id":       network.ID,
			"__meta_dockerswarm_network_name":     network.Name,
			"__meta_dockerswarm_network_internal": strconv.FormatBool(network.Internal),
			"__meta_dockerswarm_network_ingress":  strconv.FormatBool(network.Ingress),
			"__meta_dockerswarm_network_scope":    network.Scope,
		}
		for k, v := range network.Labels {
			m["__meta_dockerswarm_network_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		ms[network.ID] = m
	}
	return ms
}
