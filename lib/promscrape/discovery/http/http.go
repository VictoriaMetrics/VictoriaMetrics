package http

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig represents service discovery config for http.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config
type SDConfig struct {
	URL               string                     `yaml:"url"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          proxy.URL                  `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

// GetLabels returns http service discovery labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	hts, err := getHTTPTargets(cfg.client.GetAPIResponse, cfg.path)
	if err != nil {
		return nil, err
	}

	return addHTTPTargetLabels(hts), nil
}

func addHTTPTargetLabels(src []httpGroupTarget) []map[string]string {
	ms := make([]map[string]string, 0, len(src))
	for _, targetGroup := range src {
		labels := targetGroup.Labels
		for _, target := range targetGroup.Targets {
			m := make(map[string]string, len(labels))
			for k, v := range labels {
				m[k] = v
			}
			m["__address__"] = target
			ms = append(ms, m)
		}
	}
	return ms
}
