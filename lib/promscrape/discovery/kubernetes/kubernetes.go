package kubernetes

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig represents kubernetes-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
type SDConfig struct {
	APIServer        string                    `yaml:"api_server,omitempty"`
	Role             string                    `yaml:"role"`
	HTTPClientConfig promauth.HTTPClientConfig `yaml:",inline"`
	ProxyURL         proxy.URL                 `yaml:"proxy_url,omitempty"`
	Namespaces       Namespaces                `yaml:"namespaces,omitempty"`
	Selectors        []Selector                `yaml:"selectors,omitempty"`

	cfg      *apiConfig
	startErr error
}

// Namespaces represents namespaces for SDConfig
type Namespaces struct {
	Names []string `yaml:"names"`
}

// Selector represents kubernetes selector.
//
// See https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
// and https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
type Selector struct {
	Role  string `yaml:"role"`
	Label string `yaml:"label"`
	Field string `yaml:"field"`
}

// ScrapeWorkConstructorFunc must construct ScrapeWork object for the given metaLabels.
type ScrapeWorkConstructorFunc func(metaLabels map[string]string) interface{}

// GetScrapeWorkObjects returns ScrapeWork objects for the given sdc.
//
// This function must be called after MustStart call.
func (sdc *SDConfig) GetScrapeWorkObjects() ([]interface{}, error) {
	if sdc.cfg == nil {
		return nil, sdc.startErr
	}
	return sdc.cfg.aw.getScrapeWorkObjects(), nil
}

// MustStart initializes sdc before its usage.
//
// swcFunc is used for constructing such objects.
func (sdc *SDConfig) MustStart(baseDir string, swcFunc ScrapeWorkConstructorFunc) {
	cfg, err := newAPIConfig(sdc, baseDir, swcFunc)
	if err != nil {
		sdc.startErr = fmt.Errorf("cannot create API config for kubernetes: %w", err)
		return
	}
	cfg.aw.mustStart()
	sdc.cfg = cfg
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	if sdc.cfg != nil {
		// sdc.cfg can be nil on MustStart error.
		sdc.cfg.aw.mustStop()
	}
}
