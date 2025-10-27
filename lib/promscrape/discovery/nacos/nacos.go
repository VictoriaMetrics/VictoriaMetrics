package nacos

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.nacosSDCheckInterval", 30*time.Second, "Interval for checking for changes in Nacos REST API. "+
	"This works only if nacos_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#nacos_sd_configs for details")

type SDConfig struct {
	Server            string                     `yaml:"server,omitempty"`
	Scheme            string                     `yaml:"scheme,omitempty"`
	Namespace         string                     `yaml:"namespace,omitempty"`
	Group             string                     `yaml:"group,omitempty"`
	Cluster           string                     `yaml:"cluster,omitempty"`
	Username          string                     `yaml:"username,omitempty"`
	Password          *promauth.Secret           `yaml:"password,omitempty"`
	Services          []string                   `yaml:"services,omitempty"`
	RefreshInterval   time.Duration              `yaml:"refresh_interval,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get Nacos config: %w", err)
	}
	return getServiceInstancesLabels(cfg), nil
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
