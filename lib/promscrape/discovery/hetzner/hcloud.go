package hetzner

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// getHCloudServerLabels returns labels for hcloud servers obtained from the given cfg
func getHCloudServerLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	networks, err := getHCloudNetworks(cfg)
	if err != nil {
		return nil, err
	}
	servers, err := getHCloudServers(cfg)
	if err != nil {
		return nil, err
	}
	var ms []*promutil.Labels
	for i := range servers {
		ms = appendHCloudTargetLabels(ms, &servers[i], networks, cfg.port)
	}
	return ms, nil
}

func appendHCloudTargetLabels(ms []*promutil.Labels, server *HCloudServer, networks []HCloudNetwork, port int) []*promutil.Labels {
	m := promutil.NewLabels(24)

	addr := discoveryutil.JoinHostPort(server.PublicNet.IPv4.IP, port)
	m.Add("__address__", addr)

	m.Add("__meta_hetzner_role", "hcloud")
	m.Add("__meta_hetzner_server_id", fmt.Sprintf("%d", server.ID))
	m.Add("__meta_hetzner_server_name", server.Name)
	m.Add("__meta_hetzner_datacenter", server.Datacenter.Name)
	m.Add("__meta_hetzner_public_ipv4", server.PublicNet.IPv4.IP)
	if _, n, _ := net.ParseCIDR(server.PublicNet.IPv6.IP); n != nil {
		m.Add("__meta_hetzner_public_ipv6_network", n.String())
	}
	m.Add("__meta_hetzner_server_status", server.Status)

	m.Add("__meta_hetzner_hcloud_datacenter_location", server.Datacenter.Location.Name)
	m.Add("__meta_hetzner_hcloud_datacenter_location_network_zone", server.Datacenter.Location.NetworkZone)
	m.Add("__meta_hetzner_hcloud_server_type", server.ServerType.Name)
	m.Add("__meta_hetzner_hcloud_cpu_cores", fmt.Sprintf("%d", server.ServerType.Cores))
	m.Add("__meta_hetzner_hcloud_cpu_type", server.ServerType.CPUType)
	m.Add("__meta_hetzner_hcloud_memory_size_gb", fmt.Sprintf("%d", int(server.ServerType.Memory)))
	m.Add("__meta_hetzner_hcloud_disk_size_gb", fmt.Sprintf("%d", server.ServerType.Disk))

	if server.Image != nil {
		m.Add("__meta_hetzner_hcloud_image_name", server.Image.Name)
		m.Add("__meta_hetzner_hcloud_image_description", server.Image.Description)
		m.Add("__meta_hetzner_hcloud_image_os_version", server.Image.OsVersion)
		m.Add("__meta_hetzner_hcloud_image_os_flavor", server.Image.OsFlavor)
	}

	for _, privateNet := range server.PrivateNet {
		networkID := privateNet.ID
		for _, network := range networks {
			if networkID == network.ID {
				labelName := discoveryutil.SanitizeLabelName("__meta_hetzner_hcloud_private_ipv4_" + network.Name)
				m.Add(labelName, privateNet.IP)
			}
		}
	}

	for labelKey, labelValue := range server.Labels {
		labelName := discoveryutil.SanitizeLabelName("__meta_hetzner_hcloud_labelpresent_" + labelKey)
		m.Add(labelName, "true")

		labelName = discoveryutil.SanitizeLabelName("__meta_hetzner_hcloud_label_" + labelKey)
		m.Add(labelName, labelValue)
	}

	ms = append(ms, m)
	return ms
}

// getHCloudNetworks returns hcloud networks obtained from the given cfg
func getHCloudNetworks(cfg *apiConfig) ([]HCloudNetwork, error) {
	// See https://docs.hetzner.cloud/#networks-get-all-networks
	var networks []HCloudNetwork
	page := 1
	for {
		path := fmt.Sprintf("/v1/networks?page=%d", page)
		data, err := cfg.client.GetAPIResponse(path)
		if err != nil {
			return nil, fmt.Errorf("cannot query hcloud api for networks: %w", err)
		}
		networksPage, nextPage, err := parseHCloudNetworksList(data)
		if err != nil {
			return nil, err
		}
		networks = append(networks, networksPage...)
		if nextPage <= page {
			break
		}
		page = nextPage
	}
	return networks, nil
}

func parseHCloudNetworksList(data []byte) ([]HCloudNetwork, int, error) {
	var resp HCloudNetworksList
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, fmt.Errorf("cannot unmarshal HCloudNetworksList from %q: %w", data, err)
	}
	return resp.Networks, resp.Meta.Pagination.NextPage, nil
}

// HCloudNetworksList represents the hetzner cloud networks list.
//
// See https://docs.hetzner.cloud/#networks-get-all-networks
type HCloudNetworksList struct {
	Meta     HCloudMeta      `json:"meta"`
	Networks []HCloudNetwork `json:"networks"`
}

// HCloudNetwork represents the hetzner cloud network information.
//
// See https://docs.hetzner.cloud/#networks-get-all-networks
type HCloudNetwork struct {
	Name string `json:"name"`
	ID   int    `json:"id"`
}

// getHCloudServers returns hcloud servers obtained from the given cfg
func getHCloudServers(cfg *apiConfig) ([]HCloudServer, error) {
	// See https://docs.hetzner.cloud/#servers-get-all-servers
	var servers []HCloudServer
	page := 1
	for {
		path := fmt.Sprintf("/v1/servers?page=%d", page)
		data, err := cfg.client.GetAPIResponse(path)
		if err != nil {
			return nil, fmt.Errorf("cannot query hcloud api for servers: %w", err)
		}
		serversPage, nextPage, err := parseHCloudServerList(data)
		if err != nil {
			return nil, err
		}
		servers = append(servers, serversPage...)
		if nextPage <= page {
			break
		}
		page = nextPage
	}
	return servers, nil
}

func parseHCloudServerList(data []byte) ([]HCloudServer, int, error) {
	var resp HCloudServerList
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, fmt.Errorf("cannot unmarshal HCloudServerList from %q: %w", data, err)
	}
	return resp.Servers, resp.Meta.Pagination.NextPage, nil
}

// HCloudServerList represents a list of servers from Hetzner Cloud API.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudServerList struct {
	Meta    HCloudMeta     `json:"meta"`
	Servers []HCloudServer `json:"servers"`
}

// HCloudServer represents the structure of server data.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudServer struct {
	ID         int                `json:"id"`
	Name       string             `json:"name"`
	Status     string             `json:"status"`
	PublicNet  HCloudPublicNet    `json:"public_net"`
	PrivateNet []HCloudPrivateNet `json:"private_net"`
	ServerType HCloudServerType   `json:"server_type"`
	Datacenter HCloudDatacenter   `json:"datacenter"`
	Image      *HCloudImage       `json:"image"`
	Labels     map[string]string  `json:"labels"`
}

// HCloudServerType represents the server type information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudServerType struct {
	Name    string  `json:"name"`
	Cores   int     `json:"cores"`
	CPUType string  `json:"cpu_type"`
	Memory  float32 `json:"memory"`
	Disk    int     `json:"disk"`
}

// HCloudDatacenter represents the Hetzner datacenter.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudDatacenter struct {
	Name     string                   `json:"name"`
	Location HCloudDatacenterLocation `json:"location"`
}

// HCloudDatacenterLocation represents the datacenter information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudDatacenterLocation struct {
	Name        string `json:"name"`
	NetworkZone string `json:"network_zone"`
}

// HCloudPublicNet represents the public network information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudPublicNet struct {
	IPv4 HCloudIPv4 `json:"ipv4"`
	IPv6 HCloudIPv6 `json:"ipv6"`
}

// HCloudIPv4 represents the IPv4 information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudIPv4 struct {
	IP string `json:"ip"`
}

// HCloudIPv6 represents the IPv6 information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudIPv6 struct {
	IP string `json:"ip"`
}

// HCloudPrivateNet represents the private network information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudPrivateNet struct {
	ID int    `json:"network"`
	IP string `json:"ip"`
}

// HCloudImage represents the image information.
//
// See https://docs.hetzner.cloud/#servers-get-all-servers
type HCloudImage struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OsFlavor    string `json:"os_flavor"`
	OsVersion   string `json:"os_version"`
}

// HCloudMeta represents hetzner cloud meta-information.
//
// See https://docs.hetzner.cloud/#pagination
type HCloudMeta struct {
	Pagination HCloudPagination `json:"pagination"`
}

// HCloudPagination represents hetzner cloud pagination information.
//
// See https://docs.hetzner.cloud/#pagination
type HCloudPagination struct {
	NextPage int `json:"next_page"`
}
