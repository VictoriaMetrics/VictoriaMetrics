package docker

import (
	"fmt"
)

// DockerSwarmSDConfig represents docker swarm service discovery configuration
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config
type DockerSwarmSDConfig struct {
	Role           string `yaml:"role"`
	DockerSDConfig `yaml:",inline"`
	// refresh_interval is obtained from `-promscrape.dockerswarmSDCheckInterval` command-line option
}

// GetLabels returns dockerswarm labels according to sdc.
func (sdc *DockerSwarmSDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfigFromDockerSwarmSDConfig(sdc, baseDir)
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

// MustStop stops further usage for sdc.
func (sdc *DockerSwarmSDConfig) MustStop() {
	configMap.Delete(sdc)
}
