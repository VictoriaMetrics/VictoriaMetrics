package ovhcloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"path"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// vpsModel struct from API.
// See: https://eu.api.ovh.com/console/#/vps/%7BserviceName%7D~GET and getVPSDetails
type vpsModel struct {
	MaximumAdditionalIP int      `json:"maximumAdditionnalIp"`
	Offer               string   `json:"offer"`
	Datacenter          []string `json:"datacenter"`
	Vcore               int      `json:"vcore"`
	Version             string   `json:"version"`
	Name                string   `json:"name"`
	Disk                int      `json:"disk"`
	Memory              int      `json:"memory"`
}

// virtualPrivateServer struct from API.
// IP addresses are fetched independently.
type virtualPrivateServer struct {
	IPs         []netip.Addr
	Zone        string   `json:"zone"`
	Model       vpsModel `json:"model"`
	DisplayName string   `json:"displayName"`
	Cluster     string   `json:"cluster"`
	State       string   `json:"state"`
	Name        string   `json:"name"`
	NetbootMode string   `json:"netbootMode"`
	MemoryLimit int      `json:"memoryLimit"`
	OfferType   string   `json:"offerType"`
	Vcore       int      `json:"vcore"`

	// The following fields are defined in the response but are not used during service discovery.
	//Keymap             []string `json:"keymap"`
	//MonitoringIPBlocks []string `json:"monitoringIpBlocks"`
}

// getVPSLabels get labels for VPS.
func getVPSLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	vpsList, err := getVPSList(cfg)
	if err != nil {
		return nil, err
	}

	// Attach properties to each VPS and compose vpsDetailedList
	vpsDetailedList := make([]virtualPrivateServer, 0, len(vpsList))
	for _, vpsName := range vpsList {
		vpsDetailed, err := getVPSDetails(cfg, vpsName)
		if err != nil {
			logger.Errorf("getVPSDetails for %s failed, err: %v", vpsName, err)
			continue
		}
		vpsDetailedList = append(vpsDetailedList, *vpsDetailed)
	}

	ms := make([]*promutil.Labels, 0, len(vpsDetailedList))
	for _, server := range vpsDetailedList {
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

		m := promutil.NewLabels(21)
		m.Add("__address__", defaultIP)
		m.Add("instance", server.Name)
		m.Add("__meta_ovhcloud_vps_offer", server.Model.Offer)
		m.Add("__meta_ovhcloud_vps_datacenter", fmt.Sprintf("%+v", server.Model.Datacenter))
		m.Add("__meta_ovhcloud_vps_model_vcore", fmt.Sprintf("%d", server.Model.Vcore))
		m.Add("__meta_ovhcloud_vps_maximum_additional_ip", fmt.Sprintf("%d", server.Model.MaximumAdditionalIP))
		m.Add("__meta_ovhcloud_vps_version", server.Model.Version)
		m.Add("__meta_ovhcloud_vps_model_name", server.Model.Name)
		m.Add("__meta_ovhcloud_vps_disk", fmt.Sprintf("%d", server.Model.Disk))
		m.Add("__meta_ovhcloud_vps_memory", fmt.Sprintf("%d", server.Model.Memory))
		m.Add("__meta_ovhcloud_vps_zone", server.Zone)
		m.Add("__meta_ovhcloud_vps_display_name", server.DisplayName)
		m.Add("__meta_ovhcloud_vps_cluster", server.Cluster)
		m.Add("__meta_ovhcloud_vps_state", server.State)
		m.Add("__meta_ovhcloud_vps_name", server.Name)
		m.Add("__meta_ovhcloud_vps_netboot_mode", server.NetbootMode)
		m.Add("__meta_ovhcloud_vps_memory_limit", fmt.Sprintf("%d", server.MemoryLimit))
		m.Add("__meta_ovhcloud_vps_offer_type", server.OfferType)
		m.Add("__meta_ovhcloud_vps_vcore", fmt.Sprintf("%d", server.Vcore))
		m.Add("__meta_ovhcloud_vps_ipv4", ipv4)
		m.Add("__meta_ovhcloud_vps_ipv6", ipv6)

		ms = append(ms, m)
	}
	return ms, nil
}

// getVPSDetails get properties of a VPS.
// Also see: https://eu.api.ovh.com/console/#/vps/%7BserviceName%7D~GET
func getVPSDetails(cfg *apiConfig, vpsName string) (*virtualPrivateServer, error) {
	// get properties.
	reqPath := path.Join("/vps", url.QueryEscape(vpsName))
	resp, err := cfg.client.GetAPIResponseWithReqParams(reqPath, func(request *http.Request) {
		request.Header, _ = getAuthHeaders(cfg, request.Header, cfg.client.APIServer(), reqPath)
	})
	if err != nil {
		return nil, fmt.Errorf("request %s error: %v", reqPath, err)
	}

	var vpsDetails virtualPrivateServer
	if err = json.Unmarshal(resp, &vpsDetails); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s response: %v", reqPath, err)
	}

	// get IPs for this vps.
	// e.g. ["139.99.154.111","2402:1f00:8100:401::bb6"]
	// Also see: https://eu.api.ovh.com/console/#/vps/%7BserviceName%7D/ips~GET
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

	// attach to details
	vpsDetails.IPs = parsedIPs

	return &vpsDetails, nil
}

// getVPSList list available services.
// example: ["vps-000e0e00.vps.ovh.ca", "vps-000e0e01.vps.ovh.ca"]
// Also see: https://eu.api.ovh.com/console/#/vps~GET
func getVPSList(cfg *apiConfig) ([]string, error) {
	reqPath := "/vps"
	resp, err := cfg.client.GetAPIResponseWithReqParams(reqPath, func(request *http.Request) {
		request.Header, _ = getAuthHeaders(cfg, request.Header, cfg.client.APIServer(), reqPath)
	})
	if err != nil {
		return nil, fmt.Errorf("request %s error: %v", reqPath, err)
	}

	var vpsList []string
	if err = json.Unmarshal(resp, &vpsList); err != nil {
		return nil, fmt.Errorf("cannot unmarshal %s response: %v", reqPath, err)
	}

	return vpsList, nil
}
