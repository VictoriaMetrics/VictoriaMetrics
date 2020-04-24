package gce

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getInstancesLabels returns labels for gce instances obtained from the given cfg
func getInstancesLabels(cfg *apiConfig) ([]map[string]string, error) {
	insts, err := getInstances(cfg)
	if err != nil {
		return nil, err
	}
	var ms []map[string]string
	for _, inst := range insts {
		ms = inst.appendTargetLabels(ms, cfg.project, cfg.tagSeparator, cfg.port)
	}
	return ms, nil
}

func getInstances(cfg *apiConfig) ([]Instance, error) {
	var result []Instance
	pageToken := ""
	for {
		insts, nextPageToken, err := getInstancesPage(cfg, pageToken)
		if err != nil {
			return nil, err
		}
		result = append(result, insts...)
		if len(nextPageToken) == 0 {
			return result, nil
		}
		pageToken = nextPageToken
	}
}

func getInstancesPage(cfg *apiConfig, pageToken string) ([]Instance, string, error) {
	apiURL := cfg.apiURL
	if len(pageToken) > 0 {
		// See https://cloud.google.com/compute/docs/reference/rest/v1/instances/list about pageToken
		prefix := "?"
		if strings.Contains(apiURL, "?") {
			prefix = "&"
		}
		apiURL += fmt.Sprintf("%spageToken=%s", prefix, url.QueryEscape(pageToken))
	}
	resp, err := cfg.client.Get(apiURL)
	if err != nil {
		return nil, "", fmt.Errorf("cannot obtain instances data from API server: %s", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("cannot read instances data from API server: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code when reading instances data from API server; got %d; want %d; response body: %q",
			resp.StatusCode, http.StatusOK, data)
	}
	il, err := parseInstanceList(data)
	if err != nil {
		return nil, "", fmt.Errorf("cannot parse instances response from API server: %s", err)
	}
	return il.Items, il.NextPageToken, nil
}

// InstanceList is response to https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type InstanceList struct {
	Items         []Instance
	NextPageToken string
}

// Instance is instance from https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type Instance struct {
	ID                string `json:"id"`
	Name              string
	Status            string
	MachineType       string
	Zone              string
	NetworkInterfaces []NetworkInterface
	Tags              TagList
	Metadata          MetadataList
	Labels            discoveryutils.SortedLabels
}

// NetworkInterface is network interface from https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type NetworkInterface struct {
	Network       string
	Subnetwork    string
	NetworkIP     string
	AccessConfigs []AccessConfig
}

// AccessConfig is access config from https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type AccessConfig struct {
	Type  string
	NatIP string
}

// TagList is tag list from https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type TagList struct {
	Items []string
}

// MetadataList is metadataList from https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
type MetadataList struct {
	Items []MetadataEntry
}

// MetadataEntry is a single entry from metadata
type MetadataEntry struct {
	Key   string
	Value string
}

// parseInstanceList parses InstanceList from data.
func parseInstanceList(data []byte) (*InstanceList, error) {
	var il InstanceList
	if err := json.Unmarshal(data, &il); err != nil {
		return nil, fmt.Errorf("cannot unmarshal InstanceList from %q: %s", data, err)
	}
	return &il, nil
}

func (inst *Instance) appendTargetLabels(ms []map[string]string, project, tagSeparator string, port int) []map[string]string {
	if len(inst.NetworkInterfaces) == 0 {
		return ms
	}
	iface := inst.NetworkInterfaces[0]
	addr := discoveryutils.JoinHostPort(iface.NetworkIP, port)
	m := map[string]string{
		"__address__":                addr,
		"__meta_gce_instance_id":     inst.ID,
		"__meta_gce_instance_status": inst.Status,
		"__meta_gce_instance_name":   inst.Name,
		"__meta_gce_machine_type":    inst.MachineType,
		"__meta_gce_network":         iface.Network,
		"__meta_gce_private_ip":      iface.NetworkIP,
		"__meta_gce_project":         project,
		"__meta_gce_subnetwork":      iface.Subnetwork,
		"__meta_gce_zone":            inst.Zone,
	}
	if len(inst.Tags.Items) > 0 {
		// We surround the separated list with the separator as well. This way regular expressions
		// in relabeling rules don't have to consider tag positions.
		m["__meta_gce_tags"] = tagSeparator + strings.Join(inst.Tags.Items, tagSeparator) + tagSeparator
	}
	for _, item := range inst.Metadata.Items {
		key := discoveryutils.SanitizeLabelName(item.Key)
		m["__meta_gce_metadata_"+key] = item.Value
	}
	for _, label := range inst.Labels {
		name := discoveryutils.SanitizeLabelName(label.Name)
		m["__meta_gce_label_"+name] = label.Value
	}
	if len(iface.AccessConfigs) > 0 {
		ac := iface.AccessConfigs[0]
		if ac.Type == "ONE_TO_ONE_NAT" {
			m["__meta_gce_public_ip"] = ac.NatIP
		}
	}
	ms = append(ms, m)
	return ms
}
