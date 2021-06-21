package http

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	path   string
}

// httpGroupTarget respresent prometheus GroupTarget
// https://prometheus.io/docs/prometheus/latest/http_sd/
type httpGroupTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	parsedURL, err := url.Parse(sdc.URL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse http_sd URL: %w", err)
	}
	apiServer := fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	cfg := &apiConfig{
		client: client,
		path:   parsedURL.RequestURI(),
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

func getHTTPTargets(getAPIResponse func(path string) ([]byte, error), path string) ([]httpGroupTarget, error) {
	data, err := getAPIResponse(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read http_sd api response: %w", err)
	}
	return parseAPIResponse(data, path)
}

func parseAPIResponse(data []byte, path string) ([]httpGroupTarget, error) {
	var r []httpGroupTarget
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("cannot parse http_sd api response path: %s, err:  %w", path, err)
	}
	return r, nil
}
