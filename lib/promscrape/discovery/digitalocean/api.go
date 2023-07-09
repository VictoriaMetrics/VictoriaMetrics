package digitalocean

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	port   int
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}

	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = "https://api.digitalocean.com"
	}
	if !strings.Contains(apiServer, "://") {
		scheme := "http"
		if sdc.HTTPClientConfig.TLSConfig != nil {
			scheme = "https"
		}
		apiServer = scheme + "://" + apiServer
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	cfg := &apiConfig{
		client: client,
		port:   sdc.Port,
	}
	if cfg.port == 0 {
		cfg.port = 80
	}
	return cfg, nil
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

const dropletsAPIPath = "/v2/droplets"

func getDroplets(getAPIResponse func(string) ([]byte, error)) ([]droplet, error) {
	var droplets []droplet

	nextAPIURL := dropletsAPIPath
	for nextAPIURL != "" {
		data, err := getAPIResponse(nextAPIURL)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch data from digitalocean list api: %w", err)
		}
		apiResp, err := parseAPIResponse(data)
		if err != nil {
			return nil, err
		}
		droplets = append(droplets, apiResp.Droplets...)
		nextAPIURL, err = apiResp.nextURLPath()
		if err != nil {
			return nil, err
		}
	}
	return droplets, nil
}

func parseAPIResponse(data []byte) (*listDropletResponse, error) {
	var dps listDropletResponse
	if err := json.Unmarshal(data, &dps); err != nil {
		return nil, fmt.Errorf("failed parse digitalocean api response: %q, err: %w", data, err)
	}
	return &dps, nil
}
