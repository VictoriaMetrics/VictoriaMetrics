package consul

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

var (
	// SDCheckInterval - check interval for consul discovery.
	SDCheckInterval = flag.Duration("promscrape.consulSDCheckInterval", 30*time.Second, "Interval for checking for changes in consul. "+
		"This works only if `consul_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config for details")
	// duration for consul blocking request, maximum wait time is 10 min.
	// But fasthttp client has readTimeout for 1 min, so we use 50s timeout.
	// also consul adds random delay up to wait/16, so there is no need in jitter.
	// https://www.consul.io/api-docs/features/blocking
	watchTime = time.Second * 50
)

// SDConfig represents service discovery config for Consul.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config
type SDConfig struct {
	Server       string              `yaml:"server,omitempty"`
	Token        *string             `yaml:"token"`
	Datacenter   string              `yaml:"datacenter"`
	Scheme       string              `yaml:"scheme,omitempty"`
	Username     string              `yaml:"username"`
	Password     string              `yaml:"password"`
	TLSConfig    *promauth.TLSConfig `yaml:"tls_config,omitempty"`
	Services     []string            `yaml:"services,omitempty"`
	Tags         []string            `yaml:"tags,omitempty"`
	NodeMeta     map[string]string   `yaml:"node_meta,omitempty"`
	TagSeparator *string             `yaml:"tag_separator,omitempty"`
	AllowStale   bool                `yaml:"allow_stale,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.consulSDCheckInterval` command-line option.
}

// GetLabels returns Consul labels according to sdc.
func GetLabels(sdc *SDConfig, baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	sns := cfg.consulWatcher.getSNS()
	ms, err := addServiceNodesLabels(sns, cfg.tagSeparator)
	if err != nil {
		return nil, fmt.Errorf("error when fetching service nodes data from Consul: %w", err)
	}
	return ms, nil
}
