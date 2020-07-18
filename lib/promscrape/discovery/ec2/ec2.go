package ec2

import (
	"fmt"
)

// SDConfig represents service discovery config for ec2.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config
type SDConfig struct {
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Profile   string `yaml:"profile"`
	// TODO: add support for RoleARN
	// RoleARN   string `yaml:"role_arn"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.ec2SDCheckInterval` command-line option.
	Port    *int     `yaml:"port"`
	Filters []Filter `yaml:"filters"`
}

// Filter is ec2 filter.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
// and https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}

// GetLabels returns ec2 labels according to sdc.
func GetLabels(sdc *SDConfig) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ms, err := getInstancesLabels(cfg)
	if err != nil {
		return nil, fmt.Errorf("error when fetching instances data from EC2: %w", err)
	}
	return ms, nil
}
