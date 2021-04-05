package dockerswarm

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig represents docker swarm service discovery configuration
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config
type SDConfig struct {
	Host    string   `yaml:"host"`
	Role    string   `yaml:"role"`
	Port    int      `yaml:"port,omitempty"`
	Filters []Filter `yaml:"filters,omitempty"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          proxy.URL                  `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// refresh_interval is obtained from `-promscrape.dockerswarmSDCheckInterval` command-line option
}

// Filter is a filter, which can be passed to SDConfig.
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

// GetLabels returns dockerswarm labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Role {
	case "tasks":
		return getTasksLabels(cfg)
	case "services":
		return getServicesLabels(cfg)
	case "nodes":
		return getNodesLabels(cfg)
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `tasks`, `services` or `nodes`; skipping it", sdc.Role)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
