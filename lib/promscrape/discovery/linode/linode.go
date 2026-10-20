package linode

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.linodeSDCheckInterval", time.Minute, "Interval for checking for changes in Linode. "+
	"This works only if linode_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#linode_sd_configs for details")

// failuresTotal counts failed Linode SD refresh attempts.
// Analogous to Prometheus prometheus_sd_linode_failures_total.
var failuresTotal = metrics.NewCounter(`vm_promscrape_discovery_linode_failures_total`)

// SDConfig represents service discovery config for Linode.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#linode_sd_config
type SDConfig struct {
	Server            string                     `yaml:"server,omitempty"`
	Port              int                        `yaml:"port,omitempty"`
	TagSeparator      string                     `yaml:"tag_separator,omitempty"`
	Region            string                     `yaml:"region,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// refresh_interval is obtained from `-promscrape.linodeSDCheckInterval` command-line option.
}

// GetLabels returns Linode instance labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		failuresTotal.Inc()
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	instances, err := getInstances(cfg)
	if err != nil {
		failuresTotal.Inc()
		return nil, err
	}
	detailedIPs, err := getIPAddresses(cfg)
	if err != nil {
		failuresTotal.Inc()
		return nil, err
	}
	ipv6Ranges, err := getIPv6Ranges(cfg)
	if err != nil {
		failuresTotal.Inc()
		return nil, err
	}
	return addInstanceLabels(instances, detailedIPs, ipv6Ranges, cfg.port, cfg.tagSeparator), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}

// addInstanceLabels builds target labels from Linode API data.
// Label semantics match Prometheus linode_sd: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#linode_sd_config
func addInstanceLabels(instances []instance, detailedIPs []ipAddress, ipv6RangeList []ipv6Range, port int, tagSeparator string) []*promutil.Labels {
	ms := make([]*promutil.Labels, 0, len(instances))
	for _, inst := range instances {
		if len(inst.IPv4) == 0 {
			continue
		}

		var (
			privateIPv4, publicIPv4, publicIPv6             string
			privateIPv4RDNS, publicIPv4RDNS, publicIPv6RDNS string
			extraIPs, ipv6Ranges                            []string
		)

		for _, ip := range inst.IPv4 {
			for _, detailedIP := range detailedIPs {
				if detailedIP.Address != ip {
					continue
				}
				switch {
				case detailedIP.Public && publicIPv4 == "":
					publicIPv4 = detailedIP.Address
					if detailedIP.RDNS != "" && detailedIP.RDNS != "null" {
						publicIPv4RDNS = detailedIP.RDNS
					}
				case !detailedIP.Public && privateIPv4 == "":
					privateIPv4 = detailedIP.Address
					if detailedIP.RDNS != "" && detailedIP.RDNS != "null" {
						privateIPv4RDNS = detailedIP.RDNS
					}
				default:
					extraIPs = append(extraIPs, detailedIP.Address)
				}
			}
		}

		if inst.IPv6 != "" {
			slaac := strings.Split(inst.IPv6, "/")[0]
			for _, detailedIP := range detailedIPs {
				if detailedIP.Address != slaac {
					continue
				}
				publicIPv6 = detailedIP.Address
				if detailedIP.RDNS != "" && detailedIP.RDNS != "null" {
					publicIPv6RDNS = detailedIP.RDNS
				}
			}
			for _, r := range ipv6RangeList {
				if r.RouteTarget != slaac {
					continue
				}
				ipv6Ranges = append(ipv6Ranges, fmt.Sprintf("%s/%d", r.Range, r.Prefix))
			}
		}

		// Prefer public IPv4 for __address__ (Prometheus default). Fall back to private
		// when the instance has no public IPv4 so we never emit empty-host targets like ":80".
		addrHost := publicIPv4
		if addrHost == "" {
			addrHost = privateIPv4
		}
		if addrHost == "" {
			continue
		}

		backupsStatus := "disabled"
		if inst.Backups.Enabled {
			backupsStatus = "enabled"
		}

		m := promutil.NewLabels(28)
		m.Add("__address__", discoveryutil.JoinHostPort(addrHost, port))
		m.Add("__meta_linode_instance_id", strconv.Itoa(inst.ID))
		m.Add("__meta_linode_instance_label", inst.Label)
		m.Add("__meta_linode_image", inst.Image)
		m.Add("__meta_linode_private_ipv4", privateIPv4)
		m.Add("__meta_linode_public_ipv4", publicIPv4)
		m.Add("__meta_linode_public_ipv6", publicIPv6)
		m.Add("__meta_linode_private_ipv4_rdns", privateIPv4RDNS)
		m.Add("__meta_linode_public_ipv4_rdns", publicIPv4RDNS)
		m.Add("__meta_linode_public_ipv6_rdns", publicIPv6RDNS)
		m.Add("__meta_linode_region", inst.Region)
		m.Add("__meta_linode_type", inst.Type)
		m.Add("__meta_linode_status", inst.Status)
		m.Add("__meta_linode_group", inst.Group)
		m.Add("__meta_linode_gpus", strconv.Itoa(inst.Specs.GPUs))
		m.Add("__meta_linode_hypervisor", inst.Hypervisor)
		m.Add("__meta_linode_backups", backupsStatus)
		// Specs disk/memory/transfer are reported in MiB by the API; Prometheus converts with << 20.
		m.Add("__meta_linode_specs_disk_bytes", strconv.FormatInt(int64(inst.Specs.Disk)<<20, 10))
		m.Add("__meta_linode_specs_memory_bytes", strconv.FormatInt(int64(inst.Specs.Memory)<<20, 10))
		m.Add("__meta_linode_specs_vcpus", strconv.Itoa(inst.Specs.VCPUs))
		m.Add("__meta_linode_specs_transfer_bytes", strconv.FormatInt(int64(inst.Specs.Transfer)<<20, 10))

		if len(inst.Tags) > 0 {
			// Surround with separator so relabel regexes do not depend on tag position.
			tags := tagSeparator + strings.Join(inst.Tags, tagSeparator) + tagSeparator
			m.Add("__meta_linode_tags", tags)
		}
		if len(extraIPs) > 0 {
			ips := tagSeparator + strings.Join(extraIPs, tagSeparator) + tagSeparator
			m.Add("__meta_linode_extra_ips", ips)
		}
		if len(ipv6Ranges) > 0 {
			ranges := tagSeparator + strings.Join(ipv6Ranges, tagSeparator) + tagSeparator
			m.Add("__meta_linode_ipv6_ranges", ranges)
		}

		ms = append(ms, m)
	}
	return ms
}
