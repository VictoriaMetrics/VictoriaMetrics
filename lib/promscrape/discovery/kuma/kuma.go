package kuma

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.kumaSDCheckInterval", 30*time.Second, "Interval for checking for changes in kuma service discovery. "+
	"This works only if kuma_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#kuma_sd_configs for details")

// SDConfig represents service discovery config for Kuma Service Mesh.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kuma_sd_config
type SDConfig struct {
	Server   string `yaml:"server"`
	ClientID string `yaml:"client_id,omitempty"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`

	// fetch_timeout isn't used, so it isn't defined.
	// FetchTimeout time.Duration `yaml:"fetch_timeout,omitempty"`

	// refresh_interval is obtained from `-promscrape.kumaSDCheckInterval` command-line option.
	// RefreshInterval time.Duration `yaml:"refresh_interval,omitempty"`
}

// GetLabels returns kuma service discovery labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config for kuma_sd: %w", err)
	}
	pLabels := cfg.labels.Load()
	return *pLabels, nil
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
