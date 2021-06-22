package http

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/fasthttp"
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

func getHTTPTargets(cfg *apiConfig) ([]httpGroupTarget, error) {
	data, err := cfg.client.GetAPIResponseWithReqParams(cfg.path, func(request *fasthttp.Request) {
		request.Header.Set("X-Prometheus-Refresh-Interval-Seconds", strconv.FormatFloat(SDCheckInterval.Seconds(), 'f', 0, 64))
		request.Header.Set("Accept", "application/json")
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read http_sd api response: %w", err)
	}
	return parseAPIResponse(data, cfg.path)
}

func parseAPIResponse(data []byte, path string) ([]httpGroupTarget, error) {
	var r []httpGroupTarget
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("cannot parse http_sd api response path: %s, err:  %w", path, err)
	}
	return r, nil
}
