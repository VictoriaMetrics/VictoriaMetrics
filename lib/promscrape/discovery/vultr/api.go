package vultr

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// apiConfig contains config for API server.
type apiConfig struct {
	c    *discoveryutils.Client
	port int

	listParams
}

// listParams is the query params of vultr ListInstance API.
type listParams struct {
	// paging params are not exposed to user, they will be filled
	// dynamically during request. See `getInstances`.
	// perPage int
	// cursor string

	// API query params for filtering.
	label           string
	mainIP          string
	region          string
	firewallGroupID string
	hostname        string
}

// getAPIConfig get or create API config from configMap.
func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
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

	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	c, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create client for %q: %w", apiServer, err)
	}
	cfg := &apiConfig{
		c:    c,
		port: port,
		listParams: listParams{
			label:           sdc.Label,
			mainIP:          sdc.MainIP,
			region:          sdc.Region,
			firewallGroupID: sdc.FirewallGroupID,
			hostname:        sdc.Hostname,
		},
	}
	return cfg, nil
}
