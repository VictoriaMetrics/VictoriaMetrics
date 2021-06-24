package docker

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// DockerSDConfig defines the `docker_sd` section for Docker based discovery
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config
type DockerSDConfig struct {
	Host    string   `yaml:"host"`
	Port    int      `yaml:"port,omitempty"`
	Filters []Filter `yaml:"filters,omitempty"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          proxy.URL                  `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// refresh_interval is obtained from `-promscrape.dockerSDCheckInterval` command-line option
}

// Filter is a filter, which can be passed to SDConfig.
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

// GetLabels returns docker labels according to sdc.
func (sdc *DockerSDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfigFromDockerSDConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	return getContainersLabels(cfg)
}

// MustStop stops further usage for sdc.
func (sdc *DockerSDConfig) MustStop() {
	configMap.Delete(sdc)
}
