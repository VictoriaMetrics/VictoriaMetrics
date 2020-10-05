package openstack

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://docs.openstack.org/api-ref/compute/#list-servers
type serversDetail struct {
	Servers []server `json:"servers"`
	Links   []struct {
		HREF string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"servers_links,omitempty"`
}

type server struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	HostID    string `json:"hostid"`
	Status    string `json:"status"`
	Addresses map[string][]struct {
		Address string `json:"addr"`
		Version int    `json:"version"`
		Type    string `json:"OS-EXT-IPS:type"`
	} `json:"addresses"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Flavor   struct {
		ID string `json:"id"`
	} `json:"flavor"`
}

func parseServersDetail(data []byte) (*serversDetail, error) {
	var srvd serversDetail
	if err := json.Unmarshal(data, &srvd); err != nil {
		return nil, fmt.Errorf("cannot parse serversDetail: %w", err)
	}
	return &srvd, nil
}

func addInstanceLabels(servers []server, port int) []map[string]string {
	var ms []map[string]string
	for _, server := range servers {
		m := map[string]string{
			"__meta_openstack_instance_id":     server.ID,
			"__meta_openstack_instance_status": server.Status,
			"__meta_openstack_instance_name":   server.Name,
			"__meta_openstack_project_id":      server.TenantID,
			"__meta_openstack_user_id":         server.UserID,
			"__meta_openstack_instance_flavor": server.Flavor.ID,
		}
		for k, v := range server.Metadata {
			m["__meta_openstack_tag_"+discoveryutils.SanitizeLabelName(k)] = v
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
				lbls := make(map[string]string, len(m))
				for k, v := range m {
					lbls[k] = v
				}
				lbls["__meta_openstack_address_pool"] = pool
				lbls["__meta_openstack_private_ip"] = ip.Address
				if len(publicIP) > 0 {
					lbls["__meta_openstack_public_ip"] = publicIP
				}
				lbls["__address__"] = discoveryutils.JoinHostPort(ip.Address, port)
				ms = append(ms, lbls)
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
	// by default, query fetches data from all tenants
	if !cfg.allTenants {
		q := computeURL.Query()
		q.Set("all_tenants", "false")
		computeURL.RawQuery = q.Encode()
	}
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

func getInstancesLabels(cfg *apiConfig) ([]map[string]string, error) {
	srv, err := cfg.getServers()
	if err != nil {
		return nil, err
	}
	return addInstanceLabels(srv, cfg.port), nil
}
