package ecs

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

type apiConfig struct {
	awsConfig          *awsapi.Config
	ecsEndpoint        string
	port               int
	clusters           []string
	requestConcurrency int
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
	awsCfg, err := awsapi.NewConfig(sdc.EC2Endpoint, stsEndpoint, sdc.Region, sdc.RoleARN, sdc.AccessKey, sdc.SecretKey.String(), "ecs")
	if err != nil {
		return nil, err
	}
	ecsEndpoint := sdc.Endpoint
	if ecsEndpoint == "" {
		ecsEndpoint = fmt.Sprintf("https://ecs.%s.amazonaws.com/", awsCfg.GetRegion())
	}
	return &apiConfig{
		awsConfig:          awsCfg,
		ecsEndpoint:        ecsEndpoint,
		port:               port,
		clusters:           sdc.Clusters,
		requestConcurrency: *ecsSDRequestConcurrency,
	}, nil
}
