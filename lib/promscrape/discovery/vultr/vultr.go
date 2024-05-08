package vultr

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

const (
	separator = ","
)

// SDCheckInterval defines interval for docker targets refresh.
var SDCheckInterval = flag.Duration("promscrape.vultrSDCheckInterval", 30*time.Second, "Interval for checking for changes in Vultr. "+
	"This works only if vultr_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/sd_configs.html#vultr_sd_configs for details")

// SDConfig represents service discovery config for Vultr.
// See: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#vultr_sd_config
// Additional query params are supported, while Prometheus only supports `Port` and HTTP auth.
type SDConfig struct {
	// API query params for filtering. All of them are optional.
	// See: https://www.vultr.com/api/#tag/instances/operation/list-instances
	Label           string `yaml:"label,omitempty"`
	MainIP          string `yaml:"main_ip,omitempty"`
	Region          string `yaml:"region,omitempty"`
	FirewallGroupID string `yaml:"firewall_group_id,omitempty"`
	Hostname        string `yaml:"hostname,omitempty"`

	// The port to scrape metrics from. Default 80.
	Port int `yaml:"port"`

	// General HTTP / Auth configs.
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`

	// refresh_interval is obtained from `-promscrape.vultrSDCheckInterval` command-line option.
}

var configMap = discoveryutils.NewConfigMap()

// GetLabels returns gce labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutils.Labels, error) {
	ac, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	instances, err := getInstances(ac)
	if err != nil {
		return nil, err
	}
	return getInstanceLabels(instances, ac.port), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}

// getInstanceLabels returns labels for vultr instances obtained from the given cfg
func getInstanceLabels(instances []Instance, port int) []*promutils.Labels {
	ms := make([]*promutils.Labels, 0, len(instances))

	for _, instance := range instances {
		m := promutils.NewLabels(18)
		m.Add("__address__", discoveryutils.JoinHostPort(instance.MainIP, port))
		m.Add("__meta_vultr_instance_id", instance.ID)
		m.Add("__meta_vultr_instance_label", instance.Label)
		m.Add("__meta_vultr_instance_os", instance.Os)
		m.Add("__meta_vultr_instance_os_id", strconv.Itoa(instance.OsID))
		m.Add("__meta_vultr_instance_region", instance.Region)
		m.Add("__meta_vultr_instance_plan", instance.Plan)
		m.Add("__meta_vultr_instance_main_ip", instance.MainIP)
		m.Add("__meta_vultr_instance_internal_ip", instance.InternalIP)
		m.Add("__meta_vultr_instance_main_ipv6", instance.V6MainIP)
		m.Add("__meta_vultr_instance_hostname", instance.Hostname)
		m.Add("__meta_vultr_instance_server_status", instance.ServerStatus)
		m.Add("__meta_vultr_instance_vcpu_count", strconv.Itoa(instance.VCPUCount))
		m.Add("__meta_vultr_instance_ram_mb", strconv.Itoa(instance.RAM))
		m.Add("__meta_vultr_instance_allowed_bandwidth_gb", strconv.Itoa(instance.AllowedBandwidth))
		m.Add("__meta_vultr_instance_disk_gb", strconv.Itoa(instance.Disk))

		// We surround the separated list with the separator as well. This way regular expressions
		// in relabeling rules don't have to consider feature positions.
		if len(instance.Features) > 0 {
			features := separator + strings.Join(instance.Features, separator) + separator
			m.Add("__meta_vultr_instance_features", features)
		}

		if len(instance.Tags) > 0 {
			tags := separator + strings.Join(instance.Tags, separator) + separator
			m.Add("__meta_vultr_instance_tags", tags)
		}
		ms = append(ms, m)
	}
	return ms
}
