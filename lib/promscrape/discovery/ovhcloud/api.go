package ovhcloud

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// Endpoints
const (
	OvhEU        = "https://eu.api.ovh.com/1.0"
	OvhCA        = "https://ca.api.ovh.com/1.0"
	OvhUS        = "https://api.us.ovhcloud.com/1.0"
	KimsufiEU    = "https://eu.api.kimsufi.com/1.0"
	KimsufiCA    = "https://ca.api.kimsufi.com/1.0"
	SoyoustartEU = "https://eu.api.soyoustart.com/1.0"
	SoyoustartCA = "https://ca.api.soyoustart.com/1.0"
)

// Endpoints conveniently maps endpoints names to their URI for external configuration
var availableEndpoints = map[string]string{
	"ovh-eu":        OvhEU,
	"ovh-ca":        OvhCA,
	"ovh-us":        OvhUS,
	"kimsufi-eu":    KimsufiEU,
	"kimsufi-ca":    KimsufiCA,
	"soyoustart-eu": SoyoustartEU,
	"soyoustart-ca": SoyoustartCA,
}

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client

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

	var (
		apiServer string
		ok        bool
	)
	if apiServer, ok = availableEndpoints[sdc.Endpoint]; !ok {
		return nil, fmt.Errorf(
			"unsupported endpoint for ovhcloud sd: %s, see: https://docs.victoriametrics.com/sd_configs/#ovhcloud_sd_configs",
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

	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
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
