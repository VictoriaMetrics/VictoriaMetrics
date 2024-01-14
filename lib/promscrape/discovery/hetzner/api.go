package hetzner

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	role   string
	port   int
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hcc := sdc.HTTPClientConfig

	var apiServer string
	switch sdc.Role {
	case "robot":
		apiServer = "https://robot-ws.your-server.de"
		if hcc.BasicAuth == nil {
			return nil, fmt.Errorf("basic_auth must be set when role is `%q`", sdc.Role)
		}
	case "hcloud":
		apiServer = "https://api.hetzner.cloud/v1"
		if hcc.Authorization == nil {
			return nil, fmt.Errorf("authorization must be set when role is `%q`", sdc.Role)
		}
	default:
		return nil, fmt.Errorf("skipping unexpected role=%q; must be one of `robot` or `hcloud`", sdc.Role)
	}

	ac, err := hcc.NewConfig(baseDir)
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
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	cfg := &apiConfig{
		client: client,
		role:   sdc.Role,
		port:   port,
	}
	return cfg, nil
}
