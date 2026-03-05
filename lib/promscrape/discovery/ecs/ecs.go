package ecs

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.ecsSDCheckInterval", time.Minute, "Interval for checking for changes in ECS. "+
	"This works only if ecs_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#ecs_sd_configs for details")

var ecsSDRequestConcurrency = flag.Int("promscrape.ecsSDRequestConcurrency", 20, "The number of concurrent requests for ECS service discovery. "+
	"This works only if ecs_sd_configs is configured in '-promscrape.config' file.")

// SDConfig represents service discovery config for ECS.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ecs_sd_config
type SDConfig struct {
	Region      string           `yaml:"region,omitempty"`
	Endpoint    string           `yaml:"endpoint,omitempty"`
	EC2Endpoint string           `yaml:"ec2_endpoint,omitempty"`
	STSEndpoint string           `yaml:"sts_endpoint,omitempty"`
	AccessKey   string           `yaml:"access_key,omitempty"`
	SecretKey   *promauth.Secret `yaml:"secret_key,omitempty"`
	RoleARN     string           `yaml:"role_arn,omitempty"`
	// Clusters is an optional list of cluster names or ARNs to discover.
	// When empty, all clusters in the region are used.
	Clusters []string `yaml:"clusters,omitempty"`
	Port     *int     `yaml:"port,omitempty"`
}

// GetLabels returns ECS labels according to sdc.
func (sdc *SDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *SDCheckInterval)
	defer cancel()
	return getInstancesLabels(ctx, cfg)
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}
