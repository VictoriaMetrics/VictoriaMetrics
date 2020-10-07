package dockerswarm

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDConfig represents docker swarm service discovery configuration
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config
type SDConfig struct {
	Host            string                    `yaml:"host"`
	Role            string                    `yaml:"role"`
	Port            int                       `yaml:"port"`
	TLSConfig       *promauth.TLSConfig       `yaml:"tls_config"`
	BasicAuth       *promauth.BasicAuthConfig `yaml:"basic_auth"`
	BearerToken     string                    `yaml:"bearer_token"`
	BearerTokenFile string                    `yaml:"bearer_token_file"`
}

// joinLabels adds labels to destination from source with given key from destination matching given value.
func joinLabels(source []map[string]string, destination map[string]string, key, value string) map[string]string {
	for _, sourceLabels := range source {
		if sourceLabels[key] == value {
			for k, v := range sourceLabels {
				destination[k] = v
			}
			return destination
		}
	}
	return destination
}

// GetLabels returns gce labels according to sdc.
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
