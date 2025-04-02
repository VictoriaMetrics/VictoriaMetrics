package vultr

import (
	"fmt"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

// apiConfig contains config for API server.
type apiConfig struct {
	c    *discoveryutil.Client
	port int

	listQueryParams string
}

// getAPIConfig get or create API config from configMap.
func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

// newAPIConfig create API Config.
func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	port := sdc.Port
	if port == 0 {
		port = 80
	}

	// See: https://www.vultr.com/api/
	apiServer := "https://api.vultr.com"

	if sdc.HTTPClientConfig.BearerToken == nil {
		return nil, fmt.Errorf("missing `bearer_token` option")
	}

	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	c, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create client for %q: %w", apiServer, err)
	}

	// Prepare additional query params for list instance API.
	// See https://www.vultr.com/api/#tag/instances/operation/list-instances
	var qp url.Values
	if sdc.Label != "" {
		qp.Set("label", sdc.Label)
	}
	if sdc.MainIP != "" {
		qp.Set("main_ip", sdc.MainIP)
	}
	if sdc.Region != "" {
		qp.Set("region", sdc.Region)
	}
	if sdc.FirewallGroupID != "" {
		qp.Set("firewall_group_id", sdc.FirewallGroupID)
	}
	if sdc.Hostname != "" {
		qp.Set("hostname", sdc.Hostname)
	}

	cfg := &apiConfig{
		c:    c,
		port: port,

		listQueryParams: qp.Encode(),
	}
	return cfg, nil
}
