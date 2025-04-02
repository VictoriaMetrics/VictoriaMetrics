package openstack

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// See https://docs.openstack.org/api-ref/compute/#list-hypervisors-details
type hypervisorDetail struct {
	Hypervisors []hypervisor     `json:"hypervisors"`
	Links       []hypervisorLink `json:"hypervisors_links,omitempty"`
}

type hypervisorLink struct {
	HREF string `json:"href"`
	Rel  string `json:"rel,omitempty"`
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

func addHypervisorLabels(hvs []hypervisor, port int) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, hv := range hvs {
		addr := discoveryutil.JoinHostPort(hv.HostIP, port)
		m := promutil.NewLabels(8)
		m.Add("__address__", addr)
		m.Add("__meta_openstack_hypervisor_type", hv.Type)
		m.Add("__meta_openstack_hypervisor_status", hv.Status)
		m.Add("__meta_openstack_hypervisor_hostname", hv.Hostname)
		m.Add("__meta_openstack_hypervisor_state", hv.State)
		m.Add("__meta_openstack_hypervisor_host_ip", hv.HostIP)
		m.Add("__meta_openstack_hypervisor_id", strconv.Itoa(hv.ID))
		ms = append(ms, m)
	}
	return ms
}

func getHypervisorLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	hvs, err := cfg.getHypervisors()
	if err != nil {
		return nil, fmt.Errorf("cannot get hypervisors: %w", err)
	}
	return addHypervisorLabels(hvs, cfg.port), nil
}
