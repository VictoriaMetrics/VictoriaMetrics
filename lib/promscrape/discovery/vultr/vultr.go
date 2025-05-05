package vultr

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for Vultr targets refresh.
var SDCheckInterval = flag.Duration("promscrape.vultrSDCheckInterval", 30*time.Second, "Interval for checking for changes in Vultr. "+
	"This works only if vultr_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#vultr_sd_configs for details")

// SDConfig represents service discovery config for Vultr.
//
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

var configMap = discoveryutil.NewConfigMap()

// GetLabels returns Vultr instances' labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
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
	_ = configMap.Delete(sdc)
}

// getInstanceLabels returns labels for Vultr instances obtained from the given cfg
func getInstanceLabels(instances []Instance, port int) []*promutil.Labels {
	ms := make([]*promutil.Labels, 0, len(instances))

	for _, instance := range instances {
		m := promutil.NewLabels(18)
		m.Add("__address__", discoveryutil.JoinHostPort(instance.MainIP, port))
		m.Add("__meta_vultr_instance_allowed_bandwidth_gb", strconv.Itoa(instance.AllowedBandwidth))
		m.Add("__meta_vultr_instance_disk_gb", strconv.Itoa(instance.Disk))
		m.Add("__meta_vultr_instance_hostname", instance.Hostname)
		m.Add("__meta_vultr_instance_id", instance.ID)
		m.Add("__meta_vultr_instance_internal_ip", instance.InternalIP)
		m.Add("__meta_vultr_instance_label", instance.Label)
		m.Add("__meta_vultr_instance_main_ip", instance.MainIP)
		m.Add("__meta_vultr_instance_main_ipv6", instance.V6MainIP)
		m.Add("__meta_vultr_instance_os", instance.OS)
		m.Add("__meta_vultr_instance_os_id", strconv.Itoa(instance.OSID))
		m.Add("__meta_vultr_instance_plan", instance.Plan)
		m.Add("__meta_vultr_instance_region", instance.Region)
		m.Add("__meta_vultr_instance_ram_mb", strconv.Itoa(instance.RAM))
		m.Add("__meta_vultr_instance_server_status", instance.ServerStatus)
		m.Add("__meta_vultr_instance_vcpu_count", strconv.Itoa(instance.VCPUCount))

		if len(instance.Features) > 0 {
			m.Add("__meta_vultr_instance_features", joinStrings(instance.Features))
		}

		if len(instance.Tags) > 0 {
			m.Add("__meta_vultr_instance_tags", joinStrings(instance.Tags))
		}

		ms = append(ms, m)
	}

	return ms
}

func joinStrings(a []string) string {
	return "," + strings.Join(a, ",") + ","
}
