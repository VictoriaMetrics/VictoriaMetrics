package docker

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client             *discoveryutil.Client
	port               int
	hostNetworkingHost string
	matchFirstNetwork  bool

	// filtersQueryArg contains escaped `filters` query arg to add to each request to Docker Swarm API.
	filtersQueryArg string
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hostNetworkingHost := sdc.HostNetworkingHost
	if hostNetworkingHost == "" {
		hostNetworkingHost = "localhost"
	}
	cfg := &apiConfig{
		port:               sdc.Port,
		hostNetworkingHost: hostNetworkingHost,
		matchFirstNetwork:  true,
		filtersQueryArg:    getFiltersQueryArg(sdc.Filters),
	}
	if sdc.MatchFirstNetwork != nil {
		cfg.matchFirstNetwork = *sdc.MatchFirstNetwork
	}
	if cfg.port == 0 {
		cfg.port = 80
	}
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutil.NewClient(sdc.Host, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", sdc.Host, err)
	}
	cfg.client = client
	return cfg, nil
}

func (cfg *apiConfig) getAPIResponse(path string) ([]byte, error) {
	if len(cfg.filtersQueryArg) > 0 {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		path += separator + "filters=" + cfg.filtersQueryArg
	}
	return cfg.client.GetAPIResponse(path)
}

func getFiltersQueryArg(filters []Filter) string {
	if len(filters) == 0 {
		return ""
	}
	m := make(map[string]map[string]bool)
	for _, f := range filters {
		x := m[f.Name]
		if x == nil {
			x = make(map[string]bool)
			m[f.Name] = x
		}
		for _, value := range f.Values {
			x[value] = true
		}
	}
	buf, err := json.Marshal(m)
	if err != nil {
		logger.Panicf("BUG: unexpected error in json.Marshal: %s", err)
	}
	return url.QueryEscape(string(buf))
}
