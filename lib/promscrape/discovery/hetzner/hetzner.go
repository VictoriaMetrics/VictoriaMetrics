// SDConfig represents service discovery config for hetzner cloud and hetzner robot.
package hetzner

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
) //

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.hetznerSDCheckInterval", time.Minute, "Interval for checking for changes in hetzner. "+
	"This works only if hetzner_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/sd_configs.html#hetzner_sd_configs for details")

// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#hetzner_sd_config
type SDConfig struct {
	Role              string                     `yaml:"role,omitempty"`
	Port              *int                       `yaml:"port,omitempty"`
	Token             *promauth.Secret           `yaml:"token"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
}

// GetLabels returns hcloud or hetzner robot labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutils.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Role {
	case "robot":
		return getRobotServerLabels(cfg)
	case "hcloud":
		return getHcloudServerLabels(cfg)
	default:
		return nil, fmt.Errorf("skipping unexpected role=%q; must be one of `robot` or `hcloud`", sdc.Role)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
