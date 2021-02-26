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

// StartWatchOnce returns init labels for the given sdc and baseDir.
// and starts watching for changes.
func StartWatchOnce(watchCfg *WatchConfig, setName string, sdc *SDConfig, baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(watchCfg, setName, sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot create API config: %w", err)
	}
	var ms []map[string]string
	cfg.watchOnce.Do(func() {
		ms = startWatcherByRole(watchCfg.Ctx, sdc.Role, cfg, watchCfg.SC)
	})
	return ms, nil
}
