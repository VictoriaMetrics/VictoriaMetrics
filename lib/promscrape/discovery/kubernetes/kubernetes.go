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
	APIServer       string                    `yaml:"api_server,omitempty"`
	Role            string                    `yaml:"role"`
	BasicAuth       *promauth.BasicAuthConfig `yaml:"basic_auth,omitempty"`
	BearerToken     string                    `yaml:"bearer_token,omitempty"`
	BearerTokenFile string                    `yaml:"bearer_token_file,omitempty"`
	ProxyURL        proxy.URL                 `yaml:"proxy_url,omitempty"`
	TLSConfig       *promauth.TLSConfig       `yaml:"tls_config,omitempty"`
	Namespaces      Namespaces                `yaml:"namespaces,omitempty"`
	Selectors       []Selector                `yaml:"selectors,omitempty"`
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

// GetLabels returns labels for the given sdc and baseDir.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot create API config: %w", err)
	}
	switch sdc.Role {
	case "node", "pod", "service", "endpoints", "endpointslices", "ingress":
		return cfg.aw.getLabelsForRole(sdc.Role), nil
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `node`, `pod`, `service`, `endpoints`, `endpointslices` or `ingress`; skipping it", sdc.Role)
	}
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
