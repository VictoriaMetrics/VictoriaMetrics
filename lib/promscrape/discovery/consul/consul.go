package consul

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDConfig represents service discovery config for Consul.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config
type SDConfig struct {
	Server       string              `yaml:"server"`
	Token        *string             `yaml:"token"`
	Datacenter   string              `yaml:"datacenter"`
	Scheme       string              `yaml:"scheme"`
	Username     string              `yaml:"username"`
	Password     string              `yaml:"password"`
	TLSConfig    *promauth.TLSConfig `yaml:"tls_config"`
	Services     []string            `yaml:"services"`
	Tags         []string            `yaml:"tags"`
	NodeMeta     map[string]string   `yaml:"node_meta"`
	TagSeparator *string             `yaml:"tag_separator"`
	AllowStale   bool                `yaml:"allow_stale"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.consulSDCheckInterval` command-line option.
}

// GetLabels returns Consul labels according to sdc.
func GetLabels(sdc *SDConfig, baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ms, err := getServiceNodesLabels(cfg)
	if err != nil {
		return nil, fmt.Errorf("error when fetching service nodes data from Consul: %w", err)
	}
	return ms, nil
}
