package marathon

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

// apiConfig contains config for API server.
type apiConfig struct {
	cs []*discoveryutil.Client
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
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	cs := make([]*discoveryutil.Client, 0, len(sdc.Servers))
	for i := range sdc.Servers {
		c, e := discoveryutil.NewClient(sdc.Servers[i], ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
		if e != nil {
			return nil, fmt.Errorf("cannot create client for %q: %w", sdc.Servers[i], e)
		}
		cs = append(cs, c)
	}

	cfg := &apiConfig{
		cs: cs,
	}
	return cfg, nil
}
