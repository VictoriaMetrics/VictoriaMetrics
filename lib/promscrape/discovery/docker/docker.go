package docker

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for docker targets refresh.
var SDCheckInterval = flag.Duration("promscrape.dockerSDCheckInterval", 30*time.Second, "Interval for checking for changes in docker. "+
	"This works only if docker_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#docker_sd_configs for details")

// SDConfig defines the `docker_sd` section for Docker based discovery
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config
type SDConfig struct {
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port,omitempty"`
	Filters            []Filter `yaml:"filters,omitempty"`
	HostNetworkingHost string   `yaml:"host_networking_host,omitempty"`
	// MatchFirstNetwork sorts container networks in ascending order by name
	// and uses first value
	//
	// Has default value of true
	// to align with prometheus service discovery behaviour
	MatchFirstNetwork *bool `yaml:"match_first_network"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// refresh_interval is obtained from `-promscrape.dockerSDCheckInterval` command-line option
}

// Filter is a filter, which can be passed to SDConfig.
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

// GetLabels returns docker labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	return getContainersLabels(cfg)
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}
