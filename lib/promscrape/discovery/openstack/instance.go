package openstack

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// See https://docs.openstack.org/api-ref/compute/#list-servers
type serversDetail struct {
	Servers []server `json:"servers"`
	Links   []link   `json:"servers_links,omitempty"`
}

type link struct {
	HREF string `json:"href"`
	Rel  string `json:"rel"`
}

type server struct {
	ID        string                     `json:"id"`
	TenantID  string                     `json:"tenant_id"`
	UserID    string                     `json:"user_id"`
	Name      string                     `json:"name"`
	HostID    string                     `json:"hostid"`
	Status    string                     `json:"status"`
	Addresses map[string][]serverAddress `json:"addresses"`
	Metadata  map[string]string          `json:"metadata,omitempty"`
	Flavor    serverFlavor               `json:"flavor"`
}

type serverAddress struct {
	Address string `json:"addr"`
	Version int    `json:"version"`
	Type    string `json:"OS-EXT-IPS:type"`
}

type serverFlavor struct {
	ID string `json:"id"`
}

func parseServersDetail(data []byte) (*serversDetail, error) {
	var srvd serversDetail
	if err := json.Unmarshal(data, &srvd); err != nil {
		return nil, fmt.Errorf("cannot parse serversDetail: %w", err)
	}
	return &srvd, nil
}

func addInstanceLabels(servers []server, port int) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, server := range servers {
		commonLabels := promutil.NewLabels(16)
		commonLabels.Add("__meta_openstack_instance_id", server.ID)
		commonLabels.Add("__meta_openstack_instance_status", server.Status)
		commonLabels.Add("__meta_openstack_instance_name", server.Name)
		commonLabels.Add("__meta_openstack_project_id", server.TenantID)
		commonLabels.Add("__meta_openstack_user_id", server.UserID)
		commonLabels.Add("__meta_openstack_instance_flavor", server.Flavor.ID)
		for k, v := range server.Metadata {
			commonLabels.Add(discoveryutil.SanitizeLabelName("__meta_openstack_tag_"+k), v)
		}
		// Traverse server.Addresses in alphabetical order of pool name
		// in order to return targets in deterministic order.
		sortedPools := make([]string, 0, len(server.Addresses))
		for pool := range server.Addresses {
			sortedPools = append(sortedPools, pool)
		}
		sort.Strings(sortedPools)
		for _, pool := range sortedPools {
			addresses := server.Addresses[pool]
			if len(addresses) == 0 {
				// skip pool with zero addresses
				continue
			}
			var publicIP string
			// its possible to have only one floating ip per pool
			for _, ip := range addresses {
				if ip.Type != "floating" {
					continue
				}
				publicIP = ip.Address
				break
			}
			for _, ip := range addresses {
				// fast return
				if len(ip.Address) == 0 || ip.Type == "floating" {
					continue
				}
				// copy labels
				m := promutil.NewLabels(20)
				m.AddFrom(commonLabels)
				m.Add("__meta_openstack_address_pool", pool)
				m.Add("__meta_openstack_private_ip", ip.Address)
				if len(publicIP) > 0 {
					m.Add("__meta_openstack_public_ip", publicIP)
				}
				m.Add("__address__", discoveryutil.JoinHostPort(ip.Address, port))
				ms = append(ms, m)
			}
		}
	}
	return ms
}

func (cfg *apiConfig) getServers() ([]server, error) {
	creds, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	computeURL := *creds.computeURL
	computeURL.Path = path.Join(computeURL.Path, "servers", "detail")
	q := computeURL.Query()
	q.Set("all_tenants", strconv.FormatBool(cfg.allTenants))
	computeURL.RawQuery = q.Encode()
	nextLink := computeURL.String()
	var servers []server
	for {
		resp, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, err
		}
		serversDetail, err := parseServersDetail(resp)
		if err != nil {
			return nil, err
		}
		servers = append(servers, serversDetail.Servers...)
		if len(serversDetail.Links) == 0 {
			return servers, nil
		}
		nextLink = serversDetail.Links[0].HREF
	}
}

func getInstancesLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	srv, err := cfg.getServers()
	if err != nil {
		return nil, err
	}
	return addInstanceLabels(srv, cfg.port), nil
}
