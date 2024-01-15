package hetzner

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// HcloudServerList represents a list of servers from Hetzner Cloud API.
type HcloudServerList struct {
	Servers []HcloudServer `json:"servers"`
}

// HcloudServer represents the structure of server data.
type HcloudServer struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	PublicNet  PublicNet         `json:"public_net,omitempty"`
	PrivateNet []PrivateNet      `json:"private_net,omitempty"`
	ServerType ServerType        `json:"server_type"`
	Datacenter Datacenter        `json:"datacenter"`
	Image      Image             `json:"image"`
	Labels     map[string]string `json:"labels"`
}

// Datacenter represents the Hetzner datacenter.
type Datacenter struct {
	Name     string             `json:"name"`
	Location DatacenterLocation `json:"location"`
}

// DatacenterLocation represents the datacenter information.
type DatacenterLocation struct {
	Name        string `json:"name"`
	NetworkZone string `json:"network_zone"`
}

// Image represents the image information.
type Image struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OsFlavor    string `json:"os_flavor"`
	OsVersion   string `json:"os_version"`
}

// PublicNet represents the public network information.
type PublicNet struct {
	IPv4 IPv4 `json:"ipv4"`
	IPv6 IPv6 `json:"ipv6"`
}

// PrivateNet represents the private network information.
type PrivateNet struct {
	ID int    `json:"network"`
	IP string `json:"ip"`
}

// IPv4 represents the IPv4 information.
type IPv4 struct {
	IP string `json:"ip"`
}

// IPv6 represents the IPv6 information.
type IPv6 struct {
	IP string `json:"ip"`
}

// ServerType represents the server type information.
type ServerType struct {
	Name    string  `json:"name"`
	Cores   int     `json:"cores"`
	CPUType string  `json:"cpu_type"`
	Memory  float32 `json:"memory"`
	Disk    int     `json:"disk"`
}

// HcloudNetwork represents the hetzner cloud network information.
type HcloudNetwork struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// HcloudNetworksList represents the hetzner cloud networks list.
type HcloudNetworksList struct {
	Networks []HcloudNetwork `json:"networks"`
}

// getHcloudServerLabels returns labels for hcloud servers obtained from the given cfg
func getHcloudServerLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	networks, err := getHcloudNetworks(cfg)
	if err != nil {
		return nil, err
	}
	servers, err := getServers(cfg)
	if err != nil {
		return nil, err
	}
	var ms []*promutils.Labels
	for _, server := range servers.Servers {
		ms = server.appendTargetLabels(ms, cfg.port, networks)
	}
	return ms, nil
}

// getHcloudNetworks returns hcloud networks obtained from the given cfg
func getHcloudNetworks(cfg *apiConfig) (*HcloudNetworksList, error) {
	n, err := cfg.client.GetAPIResponse("/networks")
	if err != nil {
		return nil, fmt.Errorf("cannot query hcloud api for networks: %w", err)
	}
	networks, err := parseHcloudNetworksList(n)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal HcloudServerList from %q: %w", n, err)
	}
	return networks, nil
}

// getServers returns hcloud servers obtained from the given cfg
func getServers(cfg *apiConfig) (*HcloudServerList, error) {
	s, err := cfg.client.GetAPIResponse("/servers")
	if err != nil {
		return nil, fmt.Errorf("cannot query hcloud api for servers: %w", err)
	}
	servers, err := parseHcloudServerList(s)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

// parseHcloudNetworks parses HcloudNetworksList from data.
func parseHcloudNetworksList(data []byte) (*HcloudNetworksList, error) {
	var networks HcloudNetworksList
	err := json.Unmarshal(data, &networks)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal HcloudNetworksList from %q: %w", data, err)
	}
	return &networks, nil
}

// parseHcloudServerList parses HcloudServerList from data.
func parseHcloudServerList(data []byte) (*HcloudServerList, error) {
	var servers HcloudServerList
	err := json.Unmarshal(data, &servers)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal HcloudServerList from %q: %w", data, err)
	}
	return &servers, nil
}

func (server *HcloudServer) appendTargetLabels(ms []*promutils.Labels, port int, networks *HcloudNetworksList) []*promutils.Labels {
	addr := discoveryutils.JoinHostPort(server.PublicNet.IPv4.IP, port)
	m := promutils.NewLabels(24)
	m.Add("__address__", addr)
	m.Add("__meta_hetzner_server_id", fmt.Sprintf("%d", server.ID))
	m.Add("__meta_hetzner_server_name", server.Name)
	m.Add("__meta_hetzner_server_status", server.Status)
	m.Add("__meta_hetzner_public_ipv4", server.PublicNet.IPv4.IP)
	m.Add("__meta_hetzner_public_ipv6_network", server.PublicNet.IPv6.IP)
	m.Add("__meta_hetzner_datacenter", server.Datacenter.Name)
	m.Add("__meta_hetzner_hcloud_image_name", server.Image.Name)
	m.Add("__meta_hetzner_hcloud_image_description", server.Image.Description)
	m.Add("__meta_hetzner_hcloud_image_os_flavor", server.Image.OsFlavor)
	m.Add("__meta_hetzner_hcloud_image_os_version", server.Image.OsVersion)
	m.Add("__meta_hetzner_hcloud_datacenter_location", server.Datacenter.Location.Name)
	m.Add("__meta_hetzner_hcloud_datacenter_location_network_zone", server.Datacenter.Location.NetworkZone)
	m.Add("__meta_hetzner_hcloud_server_type", server.ServerType.Name)
	m.Add("__meta_hetzner_hcloud_cpu_cores", fmt.Sprintf("%d", server.ServerType.Cores))
	m.Add("__meta_hetzner_hcloud_cpu_type", server.ServerType.CPUType)
	m.Add("__meta_hetzner_hcloud_memory_size_gb", fmt.Sprintf("%d", int(server.ServerType.Memory)))
	m.Add("__meta_hetzner_hcloud_disk_size_gb", fmt.Sprintf("%d", server.ServerType.Disk))

	for _, privateNet := range server.PrivateNet {
		for _, network := range networks.Networks {
			if privateNet.ID == network.ID {
				m.Add(discoveryutils.SanitizeLabelName("__meta_hetzner_hcloud_private_ipv4_"+network.Name), privateNet.IP)
			}
		}
	}
	for labelKey, labelValue := range server.Labels {
		m.Add(discoveryutils.SanitizeLabelName("__meta_hetzner_hcloud_label_"+labelKey), labelValue)
		m.Add(discoveryutils.SanitizeLabelName("__meta_hetzner_hcloud_labelpresent_"+labelKey), fmt.Sprintf("%t", true))

	}
	ms = append(ms, m)
	return ms
}
