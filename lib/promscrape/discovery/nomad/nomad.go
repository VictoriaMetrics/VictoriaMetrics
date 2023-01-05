package nomad

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig represents service discovery config for Nomad.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#nomad_sd_config
type SDConfig struct {
	Server     string           `yaml:"server,omitempty"`
	Token      *promauth.Secret `yaml:"token"`
	Datacenter string           `yaml:"datacenter"`
	Namespace  string           `yaml:"namespace,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.nomadSDCheckInterval` command-line option.
	Region            string                     `yaml:"region,omitempty"`
	Scheme            string                     `yaml:"scheme,omitempty"`
	Username          string                     `yaml:"username"`
	Password          *promauth.Secret           `yaml:"password"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	Services          []string                   `yaml:"services,omitempty"`
	Tags              []string                   `yaml:"tags,omitempty"`
	AllowStale        *bool                      `yaml:"allow_stale,omitempty"`
	TagSeparator      *string                    `yaml:"tag_separator,omitempty"`
}

// GetLabels returns Nomad labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutils.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ms := getServiceLabels(cfg)
	return ms, nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		// v can be nil if GetLabels wasn't called yet.
		cfg := v.(*apiConfig)
		cfg.mustStop()
	}
}
