package hetzner

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type robotServersList struct {
	Servers []RobotServerResponse
}

type RobotServerResponse struct {
	Server RobotServer `json:"server"`
}

// HcloudServer represents the structure of hetzner robot server data.
type RobotServer struct {
	ServerIP     string        `json:"server_ip"`
	ServerIPV6   string        `json:"server_ipv6_net"`
	ServerNumber int           `json:"server_number"`
	ServerName   string        `json:"server_name"`
	DC           string        `json:"dc"`
	Status       string        `json:"status"`
	Product      string        `json:"product"`
	Canceled     bool          `json:"cancelled"`
	Subnet       []RobotSubnet `json:"subnet"`
}

// HcloudServer represents the structure of hetzner robot subnet data.
type RobotSubnet struct {
	IP   string `json:"ip"`
	Mask string `json:"mask"`
}

func getRobotServerLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	servers, err := getRobotServers(cfg)
	if err != nil {
		return nil, err
	}
	var ms []*promutils.Labels
	for _, server := range servers.Servers {
		ms = server.appendTargetLabels(ms, cfg.port)
	}
	return ms, nil
}

// parseRobotServersList parses robotServersList from data.
func parseRobotServersList(data []byte) (*robotServersList, error) {
	var servers robotServersList
	err := json.Unmarshal(data, &servers.Servers)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal robotServersList from %q: %w", data, err)
	}
	return &servers, nil
}

func getRobotServers(cfg *apiConfig) (*robotServersList, error) {
	s, err := cfg.client.GetAPIResponse("/server")
	if err != nil {
		return nil, fmt.Errorf("cannot query hetzner robot api for servers: %w", err)
	}
	servers, err := parseRobotServersList(s)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func (server *RobotServerResponse) appendTargetLabels(ms []*promutils.Labels, port int) []*promutils.Labels {
	addr := discoveryutils.JoinHostPort(server.Server.ServerIP, port)
	m := promutils.NewLabels(16)
	m.Add("__address__", addr)
	m.Add("__meta_hetzner_server_id", fmt.Sprintf("%d", server.Server.ServerNumber))
	m.Add("__meta_hetzner_server_name", server.Server.ServerName)
	m.Add("__meta_hetzner_server_status", server.Server.Status)
	m.Add("__meta_hetzner_public_ipv4", server.Server.ServerIP)
	m.Add("__meta_hetzner_datacenter", strings.ToLower(server.Server.DC))
	m.Add("__meta_hetzner_robot_product", server.Server.Product)
	m.Add("__meta_hetzner_robot_cancelled", fmt.Sprintf("%t", server.Server.Canceled))

	for _, subnet := range server.Server.Subnet {
		ip := net.ParseIP(subnet.IP)
		if ip.To4() == nil {
			m.Add("__meta_hetzner_public_ipv6_network", fmt.Sprintf("%s/%s", subnet.IP, subnet.Mask))
			break
		}
	}

	ms = append(ms, m)
	return ms
}
