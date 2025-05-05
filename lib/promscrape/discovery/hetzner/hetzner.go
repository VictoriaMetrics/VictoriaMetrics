// SDConfig represents service discovery config for hetzner cloud and hetzner robot.
package hetzner

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.hetznerSDCheckInterval", time.Minute, "Interval for checking for changes in Hetzner API. "+
	"This works only if hetzner_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#hetzner_sd_configs for details")

// SDConfig represents service discovery config for Hetzner.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#hetzner_sd_config
type SDConfig struct {
	Role              string                     `yaml:"role"`
	Port              *int                       `yaml:"port,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
}

// GetLabels returns Hetzner target labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Role {
	case "robot":
		return getRobotServerLabels(cfg)
	case "hcloud":
		return getHCloudServerLabels(cfg)
	default:
		// The sdc.Role must be already verified by getAPIConfig().
		panic(fmt.Errorf("BUG: unexpected role=%q; must be one of `robot` or `hcloud`", sdc.Role))
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
