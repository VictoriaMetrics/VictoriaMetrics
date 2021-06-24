package docker

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

func getNetworksLabels(cfg *apiConfig, labelPrefix string) (map[string]map[string]string, error) {
	networks, err := getNetworks(cfg)
	if err != nil {
		return nil, err
	}
	return getNetworkLabelsGroupByNetworkID(networks, labelPrefix), nil
}

func getNetworks(cfg *apiConfig) ([]network, error) {
	resp, err := cfg.getAPIResponse("/networks")
	if err != nil {
		return nil, fmt.Errorf("cannot query docker/dockerswarm api for networks: %w", err)
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

func getNetworkLabelsGroupByNetworkID(networks []network, labelPrefix string) map[string]map[string]string {
	ms := make(map[string]map[string]string)
	for _, network := range networks {
		m := map[string]string{
			labelPrefix + "network_id":       network.ID,
			labelPrefix + "network_name":     network.Name,
			labelPrefix + "network_internal": strconv.FormatBool(network.Internal),
			labelPrefix + "network_ingress":  strconv.FormatBool(network.Ingress),
			labelPrefix + "network_scope":    network.Scope,
		}
		for k, v := range network.Labels {
			m[labelPrefix+"network_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		ms[network.ID] = m
	}
	return ms
}
