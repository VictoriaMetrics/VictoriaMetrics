package dockerswarm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	port   int

	// filtersQueryArg contains escaped `filters` query arg to add to each request to Docker Swarm API.
	filtersQueryArg string
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	cfg := &apiConfig{
		port:            sdc.Port,
		filtersQueryArg: getFiltersQueryArg(sdc.Filters),
	}
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutils.NewClient(sdc.Host, ac, sdc.ProxyURL, proxyAC)
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
