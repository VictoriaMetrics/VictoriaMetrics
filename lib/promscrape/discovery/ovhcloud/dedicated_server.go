package ovhcloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"path"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// dedicatedServer struct from API.
// IP addresses are fetched independently.
// See: https://eu.api.ovh.com/console/#/dedicated/server/%7BserviceName%7D~GET and getDedicatedServerDetails
type dedicatedServer struct {
	State           string `json:"state"`
	IPs             []netip.Addr
	CommercialRange string `json:"commercialRange"`
	LinkSpeed       int    `json:"linkSpeed"`
	Rack            string `json:"rack"`
	NoIntervention  bool   `json:"noIntervention"`
	Os              string `json:"os"`
	SupportLevel    string `json:"supportLevel"`
	ServerID        int64  `json:"serverId"`
	Reverse         string `json:"reverse"`
	Datacenter      string `json:"datacenter"`
	Name            string `json:"name"`
}

// getDedicatedServerLabels get labels for dedicated servers.
func getDedicatedServerLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	dedicatedServerList, err := getDedicatedServerList(cfg)
	if err != nil {
		return nil, err
	}

	// Attach properties to each VPS and compose vpsDetailedList
	dedicatedServerDetailList := make([]dedicatedServer, 0, len(dedicatedServerList))
	for _, dedicatedServerName := range dedicatedServerList {
		dedicatedServer, err := getDedicatedServerDetails(cfg, dedicatedServerName)
		if err != nil {
			logger.Errorf("getDedicatedServerDetails for %s failed, err: %v", dedicatedServerName, err)
			continue
		}
		dedicatedServerDetailList = append(dedicatedServerDetailList, *dedicatedServer)
	}

	ms := make([]*promutils.Labels, 0, len(dedicatedServerDetailList))
	for _, server := range dedicatedServerDetailList {
		// convert IPs into string and select default IP.
		var ipv4, ipv6 string
		for _, ip := range server.IPs {
			if ip.Is4() {
				ipv4 = ip.String()
			}
			if ip.Is6() {
				ipv6 = ip.String()
			}
		}
		defaultIP := ipv4
		if defaultIP == "" {
			defaultIP = ipv6
		}

		m := promutils.NewLabels(15)
		m.Add("__address__", defaultIP)
		m.Add("instance", server.Name)
		m.Add("__meta_ovhcloud_dedicated_server_state", server.State)
		m.Add("__meta_ovhcloud_dedicated_server_commercial_range", server.CommercialRange)
		m.Add("__meta_ovhcloud_dedicated_server_link_speed", fmt.Sprintf("%d", server.LinkSpeed))
		m.Add("__meta_ovhcloud_dedicated_server_rack", server.Rack)
		m.Add("__meta_ovhcloud_dedicated_server_no_intervention", strconv.FormatBool(server.NoIntervention))
		m.Add("__meta_ovhcloud_dedicated_server_os", server.Os)
		m.Add("__meta_ovhcloud_dedicated_server_support_level", server.SupportLevel)
		m.Add("__meta_ovhcloud_dedicated_server_server_id", fmt.Sprintf("%d", server.ServerID))
		m.Add("__meta_ovhcloud_dedicated_server_reverse", server.Reverse)
		m.Add("__meta_ovhcloud_dedicated_server_datacenter", server.Datacenter)
		m.Add("__meta_ovhcloud_dedicated_server_name", server.Name)
		m.Add("__meta_ovhcloud_dedicated_server_ipv4", ipv4)
		m.Add("__meta_ovhcloud_dedicated_server_ipv6", ipv6)

		ms = append(ms, m)
	}
	return ms, nil
}

// getVPSDetails get properties of a dedicated server.
// Also see: https://eu.api.ovh.com/console/#/dedicated/server/%7BserviceName%7D~GET
func getDedicatedServerDetails(cfg *apiConfig, dedicatedServerName string) (*dedicatedServer, error) {
	// get properties.
	reqPath := path.Join("/dedicated/server", url.QueryEscape(dedicatedServerName))
	resp, err := cfg.client.GetAPIResponseWithReqParams(reqPath, func(request *http.Request) {
		request.Header, _ = getAuthHeaders(cfg, request.Header, cfg.client.APIServer(), reqPath)
	})
	if err != nil {
		return nil, fmt.Errorf("request %s error: %v", reqPath, err)
	}

	var dedicatedServerDetails dedicatedServer
	if err = json.Unmarshal(resp, &dedicatedServerDetails); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s response: %v", reqPath, err)
	}

	// get IPs for this dedicated server.
	// e.g. ["139.99.154.111","2402:1f00:8100:401::bb6"]
	// Also see: https://eu.api.ovh.com/console/#/dedicated/server/%7BserviceName%7D/ips~GET
	reqPath = path.Join(reqPath, "ips")
	resp, err = cfg.client.GetAPIResponseWithReqParams(reqPath, func(request *http.Request) {
		request.Header, _ = getAuthHeaders(cfg, request.Header, cfg.client.APIServer(), reqPath)
	})
	if err != nil {
		return nil, fmt.Errorf("request %s error: %v", reqPath, err)
	}

	var ips []string
	if err = json.Unmarshal(resp, &ips); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s response: %v", reqPath, err)
	}

	// handle different IP formats
	parsedIPs, err := parseIPList(ips)
	if err != nil {
		return nil, err
	}
	dedicatedServerDetails.IPs = parsedIPs

	return &dedicatedServerDetails, nil
}

// getDedicatedServerList list available services.
// example: ["ns0000000.ip-00-00-000.eu"]
// Also see: https://eu.api.ovh.com/console/#/dedicated/server~GET
func getDedicatedServerList(cfg *apiConfig) ([]string, error) {
	var dedicatedServerList []string
	reqPath := "/dedicated/server"
	resp, err := cfg.client.GetAPIResponseWithReqParams(reqPath, func(request *http.Request) {
		request.Header, _ = getAuthHeaders(cfg, request.Header, cfg.client.APIServer(), reqPath)
	})
	if err != nil {
		return nil, fmt.Errorf("request %s error: %v", reqPath, err)
	}

	if err = json.Unmarshal(resp, &dedicatedServerList); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s response: %v", reqPath, err)
	}

	return dedicatedServerList, nil
}
