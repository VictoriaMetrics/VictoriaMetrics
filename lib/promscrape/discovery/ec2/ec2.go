package ec2

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.ec2SDCheckInterval", time.Minute, "Interval for checking for changes in ec2. "+
	"This works only if ec2_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#ec2_sd_configs for details")

// SDConfig represents service discovery config for ec2.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config
type SDConfig struct {
	Region      string           `yaml:"region,omitempty"`
	Endpoint    string           `yaml:"endpoint,omitempty"`
	STSEndpoint string           `yaml:"sts_endpoint,omitempty"`
	AccessKey   string           `yaml:"access_key,omitempty"`
	SecretKey   *promauth.Secret `yaml:"secret_key,omitempty"`
	// TODO add support for Profile, not working atm
	// Profile string `yaml:"profile,omitempty"`
	RoleARN string `yaml:"role_arn,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.ec2SDCheckInterval` command-line option.
	Port            *int            `yaml:"port,omitempty"`
	InstanceFilters []awsapi.Filter `yaml:"filters,omitempty"`
	AZFilters       []awsapi.Filter `yaml:"az_filters,omitempty"`
}

// GetLabels returns ec2 labels according to sdc.
func (sdc *SDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
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
