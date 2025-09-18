package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/metrics"
)

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client *discoveryutil.Client
	path   string

	fetchErrors *metrics.Counter
	parseErrors *metrics.Counter
}

// httpGroupTarget represent prometheus GroupTarget
// https://prometheus.io/docs/prometheus/latest/http_sd/
type httpGroupTarget struct {
	Targets []string         `json:"targets"`
	Labels  *promutil.Labels `json:"labels"`
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
	client, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	cfg := &apiConfig{
		client:      client,
		path:        parsedURL.RequestURI(),
		fetchErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_http_errors_total{type="fetch",url=%q}`, sdc.URL)),
		parseErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_http_errors_total{type="parse",url=%q}`, sdc.URL)),
	}
	return cfg, nil
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func getHTTPTargets(cfg *apiConfig) ([]httpGroupTarget, error) {
	data, err := cfg.client.GetAPIResponseWithReqParams(cfg.path, func(request *http.Request) {
		request.Header.Set("X-Prometheus-Refresh-Interval-Seconds", strconv.FormatFloat(SDCheckInterval.Seconds(), 'f', 0, 64))
		request.Header.Set("Accept", "application/json")
	})
	if err != nil {
		cfg.fetchErrors.Inc()
		return nil, fmt.Errorf("cannot read http_sd api response: %w", err)
	}
	tg, err := parseAPIResponse(data, cfg.path)
	if err != nil {
		cfg.parseErrors.Inc()
		return nil, err
	}
	return tg, nil
}

func parseAPIResponse(data []byte, path string) ([]httpGroupTarget, error) {
	var r []httpGroupTarget
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("cannot parse http_sd api response path=%q: %w", path, err)
	}
	return r, nil
}
