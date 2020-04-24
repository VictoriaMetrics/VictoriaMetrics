package gce

import (
	"fmt"
)

// SDConfig represents service discovery config for gce.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config
type SDConfig struct {
	Project string `yaml:"project"`
	Zone    string `yaml:"zone"`
	Filter  string `yaml:"filter"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.gceSDCheckInterval` command-line option.
	Port         *int    `yaml:"port"`
	TagSeparator *string `yaml:"tag_separator"`
}

// GetLabels returns gce labels according to sdc.
func GetLabels(sdc *SDConfig) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %s", err)
	}
	ms, err := getInstancesLabels(cfg)
	if err != nil {
		return nil, fmt.Errorf("error when fetching instances data from GCE: %s", err)
	}
	return ms, nil
}
