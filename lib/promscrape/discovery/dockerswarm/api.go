package dockerswarm

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
	client *discoveryutil.Client
	port   int

	// role is the type of objects to discover.
	//
	// filtersQueryArg is applied only to the given role - the rest of objects are queried without filters.
	role string

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
	cfg := &apiConfig{
		port:            sdc.Port,
		filtersQueryArg: getFiltersQueryArg(sdc.Filters),
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
	cfg.role = sdc.Role
	return cfg, nil
}

func (cfg *apiConfig) getAPIResponse(path, filtersQueryArg string) ([]byte, error) {
	if len(filtersQueryArg) > 0 {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		path += separator + "filters=" + cfg.filtersQueryArg
	}
	return cfg.client.GetAPIResponse(path)
}

// Encodes filters as `map[string][]string` and then marshals it to JSON.
// Reference: https://docs.docker.com/engine/api/v1.41/#tag/Task
func getFiltersQueryArg(filters []Filter) string {
	if len(filters) == 0 {
		return ""
	}

	m := make(map[string][]string)
	for _, f := range filters {
		m[f.Name] = f.Values
	}
	buf, err := json.Marshal(m)
	if err != nil {
		logger.Panicf("BUG: unexpected error in json.Marshal: %s", err)
	}
	return url.QueryEscape(string(buf))
}
