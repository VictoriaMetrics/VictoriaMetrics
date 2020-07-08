package kubernetes

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDConfig represents kubernetes-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
type SDConfig struct {
	APIServer       string                    `yaml:"api_server"`
	Role            string                    `yaml:"role"`
	BasicAuth       *promauth.BasicAuthConfig `yaml:"basic_auth"`
	BearerToken     string                    `yaml:"bearer_token"`
	BearerTokenFile string                    `yaml:"bearer_token_file"`
	TLSConfig       *promauth.TLSConfig       `yaml:"tls_config"`
	Namespaces      Namespaces                `yaml:"namespaces"`
	Selectors       []Selector                `yaml:"selectors"`
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
func GetLabels(sdc *SDConfig, baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot create API config: %w", err)
	}
	switch sdc.Role {
	case "node":
		return getNodesLabels(cfg)
	case "service":
		return getServicesLabels(cfg)
	case "pod":
		return getPodsLabels(cfg)
	case "endpoints":
		return getEndpointsLabels(cfg)
	case "ingress":
		return getIngressesLabels(cfg)
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `node`, `service`, `pod`, `endpoints` or `ingress`; skipping it", sdc.Role)
	}
}
