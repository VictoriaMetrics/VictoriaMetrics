package docker

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://github.com/moby/moby/blob/314759dc2f4745925d8dec6d15acc7761c6e5c92/docs/api/v1.41.yaml#L4024
type container struct {
	ID     string
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
	networkLabels, err := getNetworksLabelsByNetworkID(cfg)
	if err != nil {
		return nil, err
	}
	containers, err := getContainers(cfg)
	if err != nil {
		return nil, err
	}
	return addContainersLabels(containers, networkLabels, cfg.port, cfg.hostNetworkingHost), nil
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

func addContainersLabels(containers []container, networkLabels map[string]map[string]string, defaultPort int, hostNetworkingHost string) []map[string]string {
	var ms []map[string]string
	for i := range containers {
		c := &containers[i]
		if len(c.Names) == 0 {
			continue
		}
		for _, n := range c.NetworkSettings.Networks {
			var added bool
			for _, p := range c.Ports {
				if p.Type != "tcp" {
					continue
				}
				m := map[string]string{
					"__address__":                discoveryutils.JoinHostPort(n.IPAddress, p.PrivatePort),
					"__meta_docker_network_ip":   n.IPAddress,
					"__meta_docker_port_private": strconv.Itoa(p.PrivatePort),
				}
				if p.PublicPort > 0 {
					m["__meta_docker_port_public"] = strconv.Itoa(p.PublicPort)
					m["__meta_docker_port_public_ip"] = p.IP
				}
				addCommonLabels(m, c, networkLabels[n.NetworkID])
				ms = append(ms, m)
				added = true
			}
			if !added {
				// Use fallback port when no exposed ports are available or if all are non-TCP
				addr := hostNetworkingHost
				if c.HostConfig.NetworkMode != "host" {
					addr = discoveryutils.JoinHostPort(n.IPAddress, defaultPort)
				}
				m := map[string]string{
					"__address__":              addr,
					"__meta_docker_network_ip": n.IPAddress,
				}
				addCommonLabels(m, c, networkLabels[n.NetworkID])
				ms = append(ms, m)
			}
		}
	}
	return ms
}

func addCommonLabels(m map[string]string, c *container, networkLabels map[string]string) {
	m["__meta_docker_container_id"] = c.ID
	m["__meta_docker_container_name"] = c.Names[0]
	m["__meta_docker_container_network_mode"] = c.HostConfig.NetworkMode
	for k, v := range c.Labels {
		m["__meta_docker_container_label_"+discoveryutils.SanitizeLabelName(k)] = v
	}
	for k, v := range networkLabels {
		m[k] = v
	}
}
