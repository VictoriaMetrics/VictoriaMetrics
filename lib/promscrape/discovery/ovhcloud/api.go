package ovhcloud

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

// mapping for endpoint names to their URI for external configuration
var availableEndpoints = map[string]string{
	"ovh-eu":        "https://eu.api.ovh.com/1.0",
	"ovh-ca":        "https://ca.api.ovh.com/1.0",
	"ovh-us":        "https://api.us.ovhcloud.com/1.0",
	"kimsufi-eu":    "https://eu.api.kimsufi.com/1.0",
	"kimsufi-ca":    "https://ca.api.kimsufi.com/1.0",
	"soyoustart-eu": "https://eu.api.soyoustart.com/1.0",
	"soyoustart-ca": "https://ca.api.soyoustart.com/1.0",
}

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client *discoveryutil.Client

	applicationKey    string `yaml:"application_key"`
	applicationSecret string `yaml:"application_secret"`
	consumerKey       string `yaml:"consumer_key"`

	// internal fields, for ovh auth
	timeDelta atomic.Value
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	if sdc.Endpoint == "" {
		sdc.Endpoint = "ovh-eu"
	}

	apiServer, ok := availableEndpoints[sdc.Endpoint]
	if !ok {
		return nil, fmt.Errorf(
			"unsupported `endpoint` for ovhcloud sd: %s, see: https://docs.victoriametrics.com/victoriametrics/sd_configs/#ovhcloud_sd_configs",
			sdc.Endpoint,
		)
	}

	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	client, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}

	return &apiConfig{
		client: client,

		applicationKey:    sdc.ApplicationKey,
		applicationSecret: sdc.ApplicationSecret.String(),
		consumerKey:       sdc.ConsumerKey.String(),
	}, nil
}
