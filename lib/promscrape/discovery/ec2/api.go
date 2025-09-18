package ec2

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

type apiConfig struct {
	awsConfig       *awsapi.Config
	instanceFilters []awsapi.Filter
	azFilters       []awsapi.Filter
	port            int

	// A map from AZ name to AZ id.
	azMap     map[string]string
	azMapLock sync.Mutex
}

var configMap = discoveryutil.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	stsEndpoint := sdc.STSEndpoint
	if stsEndpoint == "" {
		stsEndpoint = sdc.Endpoint
	}
	awsCfg, err := awsapi.NewConfig(sdc.Endpoint, stsEndpoint, sdc.Region, sdc.RoleARN, sdc.AccessKey, sdc.SecretKey.String(), "ec2")
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		awsConfig:       awsCfg,
		instanceFilters: sdc.InstanceFilters,
		azFilters:       sdc.AZFilters,
		port:            port,
	}
	return cfg, nil
}
