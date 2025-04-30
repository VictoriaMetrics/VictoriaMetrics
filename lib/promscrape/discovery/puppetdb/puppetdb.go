package puppetdb

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.puppetdbSDCheckInterval", 30*time.Second, "Interval for checking for changes in PuppetDB API. "+
	"This works only if puppetdb_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#puppetdb_sd_configs for details")

// SDConfig is the configuration for PuppetDB based discovery.
type SDConfig struct {
	URL               string `yaml:"url"`
	Query             string `yaml:"query"`
	IncludeParameters bool   `yaml:"include_parameters"`
	Port              int    `yaml:"port"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

// GetLabels returns labels for PuppetDB according to service discover config.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}

	resources, err := getResourceList(cfg)
	if err != nil {
		return nil, err
	}

	return getResourceLabels(resources, cfg), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	_ = configMap.Delete(sdc)
}
