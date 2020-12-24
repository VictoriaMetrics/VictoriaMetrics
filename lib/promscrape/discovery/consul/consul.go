package consul

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
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
	ProxyURL     netutil.ProxyURL    `yaml:"proxy_url,omitempty"`
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
	ms := getServiceNodesLabels(cfg)
	return ms, nil
}
