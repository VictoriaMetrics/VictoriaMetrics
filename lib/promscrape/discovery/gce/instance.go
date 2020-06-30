package gce

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// getInstancesLabels returns labels for gce instances obtained from the given cfg
func getInstancesLabels(cfg *apiConfig) []map[string]string {
	insts := getInstances(cfg)
	var ms []map[string]string
	for _, inst := range insts {
		ms = inst.appendTargetLabels(ms, cfg.project, cfg.tagSeparator, cfg.port)
	}
	return ms
}

func getInstances(cfg *apiConfig) []Instance {
	// Collect instances for each zone in parallel
	type result struct {
		zone  string
		insts []Instance
		err   error
	}
	ch := make(chan result, len(cfg.zones))
	for _, zone := range cfg.zones {
		go func(zone string) {
			insts, err := getInstancesForProjectAndZone(cfg.client, cfg.project, zone, cfg.filter)
			ch <- result{
				zone:  zone,
				insts: insts,
				err:   err,
			}
		}(zone)
	}
	var insts []Instance
	for range cfg.zones {
		r := <-ch
		if r.err != nil {
			logger.Errorf("cannot collect instances from zone %q: %s", r.zone, r.err)
			continue
		}
		insts = append(insts, r.insts...)
	}
	return insts
}

func getInstancesForProjectAndZone(client *http.Client, project, zone, filter string) ([]Instance, error) {
	// See https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
	instsURL := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/zones/%s/instances", project, zone)
	var insts []Instance
	pageToken := ""
	for {
		data, err := getAPIResponse(client, instsURL, filter, pageToken)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain instances: %w", err)
		}
		il, err := parseInstanceList(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse instance list from %q: %w", instsURL, err)
		}
		insts = append(insts, il.Items...)
		if len(il.NextPageToken) == 0 {
			return insts, nil
		}
		pageToken = il.NextPageToken
	}
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
		return nil, fmt.Errorf("cannot unmarshal InstanceList from %q: %w", data, err)
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
