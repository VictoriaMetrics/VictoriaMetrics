package hetzner

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func getRobotServerLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	servers, err := getRobotServers(cfg)
	if err != nil {
		return nil, err
	}
	var ms []*promutil.Labels
	for i := range servers {
		ms = appendRobotTargetLabels(ms, &servers[i], cfg.port)
	}
	return ms, nil
}

func appendRobotTargetLabels(ms []*promutil.Labels, server *RobotServer, port int) []*promutil.Labels {
	m := promutil.NewLabels(16)

	addr := discoveryutil.JoinHostPort(server.ServerIP, port)
	m.Add("__address__", addr)

	m.Add("__meta_hetzner_role", "robot")
	m.Add("__meta_hetzner_server_id", fmt.Sprintf("%d", server.ServerNumber))
	m.Add("__meta_hetzner_server_name", server.ServerName)
	m.Add("__meta_hetzner_datacenter", strings.ToLower(server.DC))
	m.Add("__meta_hetzner_public_ipv4", server.ServerIP)
	for _, subnet := range server.Subnet {
		ip := net.ParseIP(subnet.IP)
		if ip.To4() == nil {
			m.Add("__meta_hetzner_public_ipv6_network", fmt.Sprintf("%s/%s", subnet.IP, subnet.Mask))
			break
		}
	}
	m.Add("__meta_hetzner_server_status", server.Status)

	m.Add("__meta_hetzner_robot_product", server.Product)
	m.Add("__meta_hetzner_robot_cancelled", fmt.Sprintf("%t", server.Canceled))

	ms = append(ms, m)
	return ms
}

func getRobotServers(cfg *apiConfig) ([]RobotServer, error) {
	// See https://robot.hetzner.com/doc/webservice/en.html#server
	data, err := cfg.client.GetAPIResponse("/server")
	if err != nil {
		return nil, fmt.Errorf("cannot query hetzner robot api for servers: %w", err)
	}
	servers, err := parseRobotServers(data)
	if err != nil {
		return nil, err
	}
	return servers, nil
}

func parseRobotServers(data []byte) ([]RobotServer, error) {
	var serverEntries []RobotServerEntry
	err := json.Unmarshal(data, &serverEntries)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal RobotServer list from %q: %w", data, err)
	}
	servers := make([]RobotServer, len(serverEntries))
	for i := range serverEntries {
		servers[i] = serverEntries[i].Server
	}
	return servers, nil
}

// RobotServerEntry represents a single server entry in hetzner robot server response.
//
// See https://robot.hetzner.com/doc/webservice/en.html#server
type RobotServerEntry struct {
	Server RobotServer `json:"server"`
}

// RobotServer represents the structure of hetzner robot server data.
//
// See https://robot.hetzner.com/doc/webservice/en.html#server
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

// RobotSubnet represents the structure of hetzner robot subnet data.
//
// See https://robot.hetzner.com/doc/webservice/en.html#server
type RobotSubnet struct {
	IP   string `json:"ip"`
	Mask string `json:"mask"`
}
