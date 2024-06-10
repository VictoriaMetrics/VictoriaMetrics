package vultr

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ListInstanceResponse is the response structure of Vultr ListInstance API.
type ListInstanceResponse struct {
	Instances []Instance `json:"instances"`
	Meta      *Meta      `json:"Meta"`
}

// Instance represents Vultr Instance (VPS).
// See: https://github.com/vultr/govultr/blob/5125e02e715ae6eb3ce854f0e7116c7ce545a710/instance.go#L81
type Instance struct {
	ID               string   `json:"id"`
	Os               string   `json:"os"`
	RAM              int      `json:"ram"`
	Disk             int      `json:"disk"`
	MainIP           string   `json:"main_ip"`
	VCPUCount        int      `json:"vcpu_count"`
	Region           string   `json:"region"`
	ServerStatus     string   `json:"server_status"`
	AllowedBandwidth int      `json:"allowed_bandwidth"`
	V6MainIP         string   `json:"v6_main_ip"`
	Hostname         string   `json:"hostname"`
	Label            string   `json:"label"`
	InternalIP       string   `json:"internal_ip"`
	OsID             int      `json:"os_id"`
	Features         []string `json:"features"`
	Plan             string   `json:"plan"`
	Tags             []string `json:"tags"`

	// The following fields are defined in the response but are not used during service discovery.
	//DefaultPassword string `json:"default_password,omitempty"`
	//DateCreated     string `json:"date_created"`
	//Status          string `json:"status"`
	//PowerStatus     string `json:"power_status"`
	//NetmaskV4       string `json:"netmask_v4"`
	//GatewayV4       string `json:"gateway_v4"`
	//V6Network       string `json:"v6_network"`
	//V6NetworkSize   int    `json:"v6_network_size"`
	//// Deprecated: Tag should no longer be used. Instead, use Tags.
	//Tag             string `json:"tag"`
	//KVM             string `json:"kvm"`
	//AppID           int    `json:"app_id"`
	//ImageID         string `json:"image_id"`
	//FirewallGroupID string `json:"firewall_group_id"`
	//UserScheme      string `json:"user_scheme"`
}

// Meta represents the available pagination information
type Meta struct {
	Total int `json:"total"`
	Links *Links
}

// Links represent the next/previous cursor in your pagination calls
type Links struct {
	Next string `json:"next"`
	Prev string `json:"prev"`
}

// getInstances retrieve instance from Vultr HTTP API.
func getInstances(cfg *apiConfig) ([]Instance, error) {
	var instances []Instance

	// prepare GET params
	params := url.Values{}
	params.Set("per_page", "100")
	params.Set("label", cfg.label)
	params.Set("main_ip", cfg.mainIP)
	params.Set("region", cfg.region)
	params.Set("firewall_group_id", cfg.firewallGroupID)
	params.Set("hostname", cfg.hostname)

	// send request to vultr API
	for {
		// See: https://www.vultr.com/api/#tag/instances/operation/list-instances
		path := fmt.Sprintf("/v2/instances?%s", params.Encode())
		resp, err := cfg.c.GetAPIResponse(path)
		if err != nil {
			logger.Errorf("get response from vultr failed, path:%s, err: %v", path, err)
			return nil, err
		}

		var listInstanceResp ListInstanceResponse
		if err = json.Unmarshal(resp, &listInstanceResp); err != nil {
			logger.Errorf("unmarshal response from vultr failed, err: %v", err)
			return nil, err
		}

		instances = append(instances, listInstanceResp.Instances...)

		if listInstanceResp.Meta != nil && listInstanceResp.Meta.Links != nil && listInstanceResp.Meta.Links.Next != "" {
			// if `next page` is available, set the cursor param and request again.
			params.Set("cursor", listInstanceResp.Meta.Links.Next)
		} else {
			// otherwise exit the loop
			break
		}
	}

	return instances, nil
}
