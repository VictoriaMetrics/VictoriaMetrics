package ec2

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.ec2SDCheckInterval", time.Minute, "Interval for checking for changes in ec2. "+
	"This works only if ec2_sd_configs is configured in '-promscrape.config' file. "+
	"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config for details")

// SDConfig represents service discovery config for ec2.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config
type SDConfig struct {
	Region    string           `yaml:"region,omitempty"`
	Endpoint  string           `yaml:"endpoint,omitempty"`
	AccessKey string           `yaml:"access_key,omitempty"`
	SecretKey *promauth.Secret `yaml:"secret_key,omitempty"`
	// TODO add support for Profile, not working atm
	Profile string `yaml:"profile,omitempty"`
	RoleARN string `yaml:"role_arn,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.ec2SDCheckInterval` command-line option.
	Port    *int     `yaml:"port,omitempty"`
	Filters []Filter `yaml:"filters,omitempty"`
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
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
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

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
