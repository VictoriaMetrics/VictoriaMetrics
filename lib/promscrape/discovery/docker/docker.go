package docker

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig defines the `docker_sd` section for Docker based discovery
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config
type SDConfig struct {
	Host    string   `yaml:"host"`
	Port    int      `yaml:"port,omitempty"`
	Filters []Filter `yaml:"filters,omitempty"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          proxy.URL                  `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// TODO: refresh_interval is obtained from `-promscrape.dockerswarmSDCheckInterval` command-line option
}

// Filter is a filter, which can be passed to SDConfig.
// TODO: extract to common pkg
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

// GetLabels returns docker labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	return getContainersLabels(cfg)
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
