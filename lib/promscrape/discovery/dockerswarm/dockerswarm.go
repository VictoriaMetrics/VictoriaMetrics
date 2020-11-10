package dockerswarm

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDConfig represents docker swarm service discovery configuration
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config
type SDConfig struct {
	Host string `yaml:"host"`
	// TODO: add support for proxy_url
	TLSConfig *promauth.TLSConfig `yaml:"tls_config,omitempty"`
	Role      string              `yaml:"role"`
	Port      int                 `yaml:"port,omitempty"`
	// refresh_interval is obtained from `-promscrape.dockerswarmSDCheckInterval` command-line option
	BasicAuth       *promauth.BasicAuthConfig `yaml:"basic_auth,omitempty"`
	BearerToken     string                    `yaml:"bearer_token,omitempty"`
	BearerTokenFile string                    `yaml:"bearer_token_file,omitempty"`
}

// GetLabels returns dockerswarm labels according to sdc.
func GetLabels(sdc *SDConfig, baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Role {
	case "tasks":
		return getTasksLabels(cfg)
	case "services":
		return getServicesLabels(cfg)
	case "nodes":
		return getNodesLabels(cfg)
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `tasks`, `services` or `nodes`; skipping it", sdc.Role)
	}
}
