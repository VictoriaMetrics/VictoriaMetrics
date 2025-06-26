package kubernetes

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.kubernetesSDCheckInterval", 30*time.Second, "Interval for checking for changes in Kubernetes API server. "+
	"This works only if kubernetes_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs for details")

// SDConfig represents kubernetes-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
type SDConfig struct {
	APIServer string `yaml:"api_server,omitempty"`

	// Use role() function for accessing the Role field
	Role string `yaml:"role"`
	// The filepath to kube config.
	// If defined any cluster connection information from HTTPClientConfig is ignored.
	KubeConfigFile string `yaml:"kubeconfig_file,omitempty"`

	HTTPClientConfig promauth.HTTPClientConfig `yaml:",inline"`
	ProxyURL         *proxy.URL                `yaml:"proxy_url,omitempty"`
	Namespaces       Namespaces                `yaml:"namespaces,omitempty"`
	Selectors        []Selector                `yaml:"selectors,omitempty"`
	AttachMetadata   *AttachMetadata           `yaml:"attach_metadata,omitempty"`

	cfg      *apiConfig
	startErr error
}

func (sdc *SDConfig) role() string {
	if sdc.Role == "endpointslices" {
		// The endpointslices role isn't supported by Prometheus, but it is used by VictoriaMetrics operator.
		// Support it for backwards compatibility.
		return "endpointslice"
	}
	return sdc.Role
}

// AttachMetadata represents `attach_metadata` option at `kuberentes_sd_config`.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
type AttachMetadata struct {
	Node bool `yaml:"node"`
}

// Namespaces represents namespaces for SDConfig
type Namespaces struct {
	OwnNamespace bool     `yaml:"own_namespace"`
	Names        []string `yaml:"names"`
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
type ScrapeWorkConstructorFunc func(metaLabels *promutil.Labels) any

// GetScrapeWorkObjects returns ScrapeWork objects for the given sdc.
//
// This function must be called after MustStart call.
func (sdc *SDConfig) GetScrapeWorkObjects() ([]any, error) {
	if sdc.cfg == nil {
		return nil, sdc.startErr
	}
	return sdc.cfg.aw.getScrapeWorkObjects(), nil
}

// MustStart initializes sdc before its usage.
//
// swcFunc is used for constructing ScrapeWork objects from the given metadata.
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
