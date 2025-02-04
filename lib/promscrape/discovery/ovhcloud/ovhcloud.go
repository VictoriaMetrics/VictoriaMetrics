package ovhcloud

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.ovhcloudSDCheckInterval", 30*time.Second, "Interval for checking for changes in OVH Cloud API. "+
	"This works only if ovhcloud_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/sd_configs/#ovhcloud_sd_configs for details")

// SDConfig is the configuration for OVH Cloud service discovery.
type SDConfig struct {
	Endpoint          string           `yaml:"endpoint"`
	ApplicationKey    string           `yaml:"application_key"`
	ApplicationSecret *promauth.Secret `yaml:"application_secret"`
	ConsumerKey       *promauth.Secret `yaml:"consumer_key"`
	Service           string           `yaml:"service"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

// GetLabels returns labels for OVH Cloud according to service discover config.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutils.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Service {
	case "dedicated_server":
		return getDedicatedServerLabels(cfg)
	case "vps":
		return getVPSLabels(cfg)
	default:
		return nil, fmt.Errorf("skipping unexpected service=%q; only `dedicated_server` and `vps` are supported for now", sdc.Service)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}
