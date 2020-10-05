package openstack

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://docs.openstack.org/api-ref/compute/#list-hypervisors-details
type hypervisorDetail struct {
	Hypervisors []hypervisor `json:"hypervisors"`
	Links       []struct {
		HREF string `json:"href"`
		Rel  string `json:"rel,omitempty"`
	} `json:"hypervisors_links,omitempty"`
}

type hypervisor struct {
	HostIP   string `json:"host_ip"`
	ID       int    `json:"id"`
	Hostname string `json:"hypervisor_hostname"`
	Status   string `json:"status"`
	State    string `json:"state"`
	Type     string `json:"hypervisor_type"`
}

func parseHypervisorDetail(data []byte) (*hypervisorDetail, error) {
	var hvsd hypervisorDetail
	if err := json.Unmarshal(data, &hvsd); err != nil {
		return nil, fmt.Errorf("cannot parse hypervisorDetail: %w", err)
	}
	return &hvsd, nil
}

func (cfg *apiConfig) getHypervisors() ([]hypervisor, error) {
	creds, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	computeURL := *creds.computeURL
	computeURL.Path = path.Join(computeURL.Path, "os-hypervisors", "detail")
	nextLink := computeURL.String()
	var hvs []hypervisor
	for {
		resp, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, err
		}
		detail, err := parseHypervisorDetail(resp)
		if err != nil {
			return nil, err
		}
		hvs = append(hvs, detail.Hypervisors...)
		if len(detail.Links) == 0 {
			return hvs, nil
		}
		nextLink = detail.Links[0].HREF
	}
}

func addHypervisorLabels(hvs []hypervisor, port int) []map[string]string {
	var ms []map[string]string
	for _, hv := range hvs {
		addr := discoveryutils.JoinHostPort(hv.HostIP, port)
		m := map[string]string{
			"__address__":                          addr,
			"__meta_openstack_hypervisor_type":     hv.Type,
			"__meta_openstack_hypervisor_status":   hv.Status,
			"__meta_openstack_hypervisor_hostname": hv.Hostname,
			"__meta_openstack_hypervisor_state":    hv.State,
			"__meta_openstack_hypervisor_host_ip":  hv.HostIP,
			"__meta_openstack_hypervisor_id":       strconv.Itoa(hv.ID),
		}
		ms = append(ms, m)
	}
	return ms
}

func getHypervisorLabels(cfg *apiConfig) ([]map[string]string, error) {
	hvs, err := cfg.getHypervisors()
	if err != nil {
		return nil, fmt.Errorf("cannot get hypervisors: %w", err)
	}
	return addHypervisorLabels(hvs, cfg.port), nil
}
