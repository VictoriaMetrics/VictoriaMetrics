package docker

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// See https://github.com/moby/moby/blob/314759dc2f4745925d8dec6d15acc7761c6e5c92/docs/api/v1.41.yaml#L4024
type container struct {
	ID              string
	Names           []string
	Labels          map[string]string
	Ports           []containerPort
	HostConfig      containerHostConfig
	NetworkSettings containerNetworkSettings
}

type containerPort struct {
	IP          string
	PrivatePort int
	PublicPort  int
	Type        string
}

type containerHostConfig struct {
	NetworkMode string
}

type containerNetworkSettings struct {
	Networks map[string]containerNetwork
}

type containerNetwork struct {
	IPAddress string
	NetworkID string
}

func getContainersLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	networkLabels, err := getNetworksLabelsByNetworkID(cfg)
	if err != nil {
		return nil, err
	}
	containers, err := getContainers(cfg)
	if err != nil {
		return nil, err
	}
	return addContainersLabels(containers, networkLabels, cfg.port, cfg.hostNetworkingHost, cfg.matchFirstNetwork), nil
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

func addContainersLabels(containers []container, networkLabels map[string]*promutils.Labels, defaultPort int, hostNetworkingHost string, matchFirstNetwork bool) []*promutils.Labels {
	allContainers := make(map[string]container)
	for _, c := range containers {
		allContainers[c.ID] = c
	}

	var ms []*promutils.Labels
	for i := range containers {
		c := &containers[i]
		if len(c.Names) == 0 {
			continue
		}

		networks := c.NetworkSettings.Networks
		networkMode := c.HostConfig.NetworkMode
		if len(networks) == 0 {
			// Try to lookup shared networks
			for {
				if isContainer(networkMode) {
					tmpContainer, exists := allContainers[connectedContainer(networkMode)]
					if !exists {
						break
					}
					networks = tmpContainer.NetworkSettings.Networks
					networkMode = tmpContainer.HostConfig.NetworkMode
					if len(networks) > 0 {
						break
					}
				} else {
					break
				}
			}
		}

		if matchFirstNetwork && len(networks) > 1 {
			// Sort networks by name and take first network.
			keys := make([]string, 0, len(networks))
			for k := range networks {
				keys = append(keys, k)
			}
			if len(keys) > 0 {
				sort.Strings(keys)
				firstNetworkMode := keys[0]
				firstNetwork := networks[firstNetworkMode]
				networks = map[string]containerNetwork{firstNetworkMode: firstNetwork}
			}
		}
		for _, n := range networks {
			var added bool
			for _, p := range c.Ports {
				if p.Type != "tcp" {
					continue
				}
				m := promutils.NewLabels(16)
				m.Add("__address__", discoveryutils.JoinHostPort(n.IPAddress, p.PrivatePort))
				m.Add("__meta_docker_network_ip", n.IPAddress)
				m.Add("__meta_docker_port_private", strconv.Itoa(p.PrivatePort))
				if p.PublicPort > 0 {
					m.Add("__meta_docker_port_public", strconv.Itoa(p.PublicPort))
					m.Add("__meta_docker_port_public_ip", p.IP)
				}
				addCommonLabels(m, c, networkLabels[n.NetworkID])
				// Remove possible duplicate labels, which can appear after addCommonLabels() call
				m.RemoveDuplicates()
				ms = append(ms, m)
				added = true
			}
			if !added {
				// Use fallback port when no exposed ports are available or if all are non-TCP
				addr := hostNetworkingHost
				if c.HostConfig.NetworkMode != "host" {
					addr = discoveryutils.JoinHostPort(n.IPAddress, defaultPort)
				}
				m := promutils.NewLabels(16)
				m.Add("__address__", addr)
				m.Add("__meta_docker_network_ip", n.IPAddress)
				addCommonLabels(m, c, networkLabels[n.NetworkID])
				// Remove possible duplicate labels, which can appear after addCommonLabels() call
				m.RemoveDuplicates()
				ms = append(ms, m)
			}
		}
	}
	return ms
}

func addCommonLabels(m *promutils.Labels, c *container, networkLabels *promutils.Labels) {
	m.Add("__meta_docker_container_id", c.ID)
	m.Add("__meta_docker_container_name", c.Names[0])
	m.Add("__meta_docker_container_network_mode", c.HostConfig.NetworkMode)
	for k, v := range c.Labels {
		m.Add(discoveryutils.SanitizeLabelName("__meta_docker_container_label_"+k), v)
	}
	m.AddFrom(networkLabels)
}

func isContainer(val string) bool {
	_, ok := containerID(val)
	return ok
}

func containerID(val string) (idOrName string, ok bool) {
	k, v, hasSep := strings.Cut(val, ":")
	if !hasSep || k != "container" {
		return "", false
	}
	return v, true
}

func connectedContainer(val string) (idOrName string) {
	idOrName, _ = containerID(val)
	return idOrName

}
