package vultr

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ListInstanceResponse is the response structure of Vultr ListInstance API.
type ListInstanceResponse struct {
	Instances []Instance `json:"instances"`
	Meta      Meta       `json:"meta"`
}

// Instance represents Vultr Instance (VPS).
//
// See: https://github.com/vultr/govultr/blob/5125e02e715ae6eb3ce854f0e7116c7ce545a710/instance.go#L81
type Instance struct {
	ID               string   `json:"id"`
	OS               string   `json:"os"`
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
	OSID             int      `json:"os_id"`
	Features         []string `json:"features"`
	Plan             string   `json:"plan"`
	Tags             []string `json:"tags"`
}

// Meta represents the available pagination information
//
// See https://www.vultr.com/api/#section/Introduction/Meta-and-Pagination
type Meta struct {
	Links Links `json:"links"`
}

// Links represent the next/previous cursor in your pagination calls
type Links struct {
	Next string `json:"next"`
}

// getInstances retrieve instance from Vultr HTTP API.
func getInstances(cfg *apiConfig) ([]Instance, error) {
	var instances []Instance

	// prepare GET params
	queryParams := cfg.listQueryParams

	// send request to vultr API
	for {
		// See: https://www.vultr.com/api/#tag/instances/operation/list-instances
		path := "/v2/instances?" + queryParams + "&per_page=100"
		data, err := cfg.c.GetAPIResponse(path)
		if err != nil {
			return nil, fmt.Errorf("cannot get Vultr response from %q: %w", path, err)
		}

		var resp ListInstanceResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("cannot unmarshal ListInstanceResponse obtained from %q: %w; response=%q", path, err, data)
		}

		instances = append(instances, resp.Instances...)

		if resp.Meta.Links.Next == "" {
			break
		}

		// if `next page` is available, set the cursor param and request again.
		queryParams = cfg.listQueryParams + "&cursor=" + url.QueryEscape(resp.Meta.Links.Next)
	}

	return instances, nil
}
