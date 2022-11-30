package openstack

import (
	"encoding/json"
	"fmt"
	"path"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
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

func addHypervisorLabels(hvs []hypervisor, port int) []*promutils.Labels {
	var ms []*promutils.Labels
	for _, hv := range hvs {
		addr := discoveryutils.JoinHostPort(hv.HostIP, port)
		m := promutils.NewLabels(8)
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

func getHypervisorLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	hvs, err := cfg.getHypervisors()
	if err != nil {
		return nil, fmt.Errorf("cannot get hypervisors: %w", err)
	}
	return addHypervisorLabels(hvs, cfg.port), nil
}
