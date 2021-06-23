package docker

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://github.com/moby/moby/blob/master/docs/api/v1.41.yaml
type container struct {
	Id     string
	Names  []string
	Labels map[string]string
	Ports  []struct {
		IP          string
		PrivatePort int
		PublicPort  int
		Type        string
	}
	HostConfig struct {
		NetworkMode string
	}
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string
			NetworkID string
		}
	}
}

func getContainersLabels(cfg *apiConfig) ([]map[string]string, error) {
	networks, err := getNetworks(cfg)
	if err != nil {
		return nil, err
	}
	networkLabels := getNetworkLabels(networks, "__meta_docker_")

	containers, err := getContainers(cfg)
	if err != nil {
		return nil, err
	}
	return addContainersLabels(containers, networkLabels, cfg.port), nil
}

func getContainers(cfg *apiConfig) ([]container, error) {
	resp, err := cfg.getAPIResponse("/containers/json")
	if err != nil {
		return nil, fmt.Errorf("cannot query dockerd api for containers: %w", err)
	}
	return parseContainers(resp)
}

func parseContainers(data []byte) ([]container, error) {
	var containers []container
	if err := json.Unmarshal(data, &containers); err != nil {
		return nil, fmt.Errorf("cannot parse containers: %w", err)
	}
	return containers, nil
}

func addContainersLabels(containers []container, networkLabels map[string]map[string]string, defaultPort int) []map[string]string {
	var ms []map[string]string
	for _, c := range containers {
		commonLabels := map[string]string{
			"__meta_docker_container_id":           c.Id,
			"__meta_docker_container_name":         c.Names[0],
			"__meta_docker_container_network_mode": c.HostConfig.NetworkMode,
		}

		for k, v := range c.Labels {
			commonLabels["__meta_docker_container_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}

		for _, network := range c.NetworkSettings.Networks {
			var added bool

			for _, port := range c.Ports {
				if port.Type != "tcp" {
					continue
				}

				labels := map[string]string{
					"__meta_docker_network_ip":   network.IPAddress,
					"__meta_docker_port_private": strconv.FormatInt(int64(port.PrivatePort), 10),
				}

				if port.PublicPort > 0 {
					labels["__meta_docker_port_public"] = strconv.FormatInt(int64(port.PublicPort), 10)
					labels["__meta_docker_port_public_ip"] = port.IP
				}

				for k, v := range commonLabels {
					labels[k] = v
				}

				for k, v := range networkLabels[network.NetworkID] {
					labels[k] = v
				}

				labels["__address__"] = discoveryutils.JoinHostPort(network.IPAddress, port.PrivatePort)
				ms = append(ms, labels)

				added = true
			}

			if !added {
				// Use fallback port when no exposed ports are available or if all are non-TCP
				labels := map[string]string{
					"__meta_docker_network_ip": network.IPAddress,
				}

				for k, v := range commonLabels {
					labels[k] = v
				}

				for k, v := range networkLabels[network.NetworkID] {
					labels[k] = v
				}

				labels["__address__"] = discoveryutils.JoinHostPort(network.IPAddress, defaultPort)
				ms = append(ms, labels)
			}
		}
	}
	return ms
}
