package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type apiConfig struct {
	client        *discoveryutil.Client
	path          string
	sourceURL     string
	checkInterval time.Duration

	fetchErrors *metrics.Counter
	parseErrors *metrics.Counter

	initOnce        sync.Once
	prevAPIResponse atomic.Pointer[[]byte]
	targetLabels    atomic.Pointer[targetLabelsResult]

	wg sync.WaitGroup
}

type targetLabelsResult struct {
	labels []*promutil.Labels
	err    error
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
		client:        client,
		path:          parsedURL.RequestURI(),
		sourceURL:     sdc.URL,
		checkInterval: max(*SDCheckInterval/2, time.Second),
		fetchErrors:   metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_http_errors_total{type="fetch",url=%q}`, sdc.URL)),
		parseErrors:   metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_http_errors_total{type="parse",url=%q}`, sdc.URL)),
	}
	cfg.wg.Go(func() {
		cfg.run()
	})
	return cfg, nil
}

func (cfg *apiConfig) init() {
	cfg.initOnce.Do(func() {
		cfg.refreshTargetsIfNeeded()
	})
}

func (cfg *apiConfig) run() {
	cfg.init()

	ticker := time.NewTicker(cfg.checkInterval)
	defer ticker.Stop()
	stopCh := cfg.client.Context().Done()
	for {
		select {
		case <-ticker.C:
			cfg.refreshTargetsIfNeeded()
		case <-stopCh:
			return
		}
	}
}

func (cfg *apiConfig) refreshTargetsIfNeeded() {
	apiResponse, err := cfg.getAPIResponseData()
	if err != nil {
		cfg.targetLabels.Store(&targetLabelsResult{err: err})
		cfg.prevAPIResponse.Store(nil)
		return
	}
	prevAPIResponse := cfg.prevAPIResponse.Load()
	if prevAPIResponse != nil && bytes.Equal(apiResponse, *prevAPIResponse) {
		return
	}
	hts, err := parseAPIResponse(apiResponse, cfg.path)
	if err != nil {
		cfg.prevAPIResponse.Store(nil)
		cfg.parseErrors.Inc()
		cfg.targetLabels.Store(&targetLabelsResult{err: err})
		return
	}
	newTargets := addHTTPTargetLabels(hts, cfg.sourceURL)
	cfg.targetLabels.Store(&targetLabelsResult{labels: newTargets})
	cfg.prevAPIResponse.Store(&apiResponse)
}

func (cfg *apiConfig) getAPIResponseData() ([]byte, error) {
	data, err := cfg.client.GetAPIResponseWithReqParams(cfg.path, func(request *http.Request) {
		request.Header.Set("X-Prometheus-Refresh-Interval-Seconds", strconv.FormatFloat(cfg.checkInterval.Seconds(), 'f', 0, 64))
		request.Header.Set("Accept", "application/json")
	})
	if err != nil {
		cfg.fetchErrors.Inc()
		return nil, fmt.Errorf("cannot read http_sd api response: %w", err)
	}
	return data, nil
}

func (cfg *apiConfig) getLabels() ([]*promutil.Labels, error) {
	cfg.init()

	tlr := cfg.targetLabels.Load()
	if tlr.err != nil {
		return nil, tlr.err
	}
	return tlr.labels, nil
}

func (cfg *apiConfig) mustStop() {
	cfg.client.Stop()
	cfg.wg.Wait()
}

func parseAPIResponse(data []byte, path string) ([]httpGroupTarget, error) {
	var r []httpGroupTarget
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("cannot parse http_sd api response path=%q: %w", path, err)
	}
	return r, nil
}
