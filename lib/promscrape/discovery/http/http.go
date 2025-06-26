package http

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.httpSDCheckInterval", time.Minute, "Interval for checking for changes in http endpoint service discovery. "+
	"This works only if http_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#http_sd_configs for details")

// SDConfig represents service discovery config for http.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config
type SDConfig struct {
	URL               string                     `yaml:"url"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

// GetLabels returns http service discovery labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	hts, err := getHTTPTargets(cfg)
	if err != nil {
		return nil, err
	}
	return addHTTPTargetLabels(hts, sdc.URL), nil
}

func addHTTPTargetLabels(src []httpGroupTarget, sourceURL string) []*promutil.Labels {
	ms := make([]*promutil.Labels, 0, len(src))
	for _, targetGroup := range src {
		labels := targetGroup.Labels
		for _, target := range targetGroup.Targets {
			m := promutil.NewLabels(2 + labels.Len())
			m.AddFrom(labels)
			m.Add("__address__", target)
			m.Add("__meta_url", sourceURL)
			// Remove possible duplicate labels, which can appear after AddFrom() call
			m.RemoveDuplicates()
			ms = append(ms, m)
		}
	}
	return ms
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}
