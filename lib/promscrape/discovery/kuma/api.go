package kuma

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/metrics"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	path   string

	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.Mutex // protects targets
	targets       []kumaTarget
	latestVersion string
	latestNonce   string

	fetchErrors *metrics.Counter
	parseErrors *metrics.Counter
}

const (
	discoveryNode      = "victoria-metrics"
	xdsApiVersion      = "v3"
	xdsRequestType     = "discovery"
	xdsResourceType    = "monitoringassignments"
	xdsResourceTypeUrl = "type.googleapis.com/kuma.observability.v1.MonitoringAssignment"
)

var waitTime = flag.Duration("promscrape.kuma.waitTime", 0, "Wait time used by Kuma service discovery. Default value is used if not set")

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	parsedURL, err := url.Parse(sdc.Server)
	if err != nil {
		return nil, fmt.Errorf("cannot parse kuma_sd server URL: %w", err)
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

	apiPath := path.Join(
		parsedURL.RequestURI(),
		xdsApiVersion,
		xdsRequestType+":"+xdsResourceType,
	)

	cfg := &apiConfig{
		client:      client,
		path:        apiPath,
		fetchErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_kuma_errors_total{type="fetch",url=%q}`, sdc.Server)),
		parseErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_kuma_errors_total{type="parse",url=%q}`, sdc.Server)),
	}

	// initialize targets synchronously and start updating them in background
	cfg.startWatcher()

	return cfg, nil
}

func (cfg *apiConfig) startWatcher() func() {
	ctx, cancel := context.WithCancel(context.Background())
	cfg.cancel = cancel

	// blocking initial targets update
	if err := cfg.updateTargets(ctx); err != nil {
		logger.Errorf("there were errors when discovering kuma targets, so preserving the previous targets. error: %v", err)
	}

	// start updating targets with a long polling in background
	cfg.wg.Add(1)
	go func() {
		ticker := time.NewTicker(*SDCheckInterval)
		defer cfg.wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// we are constantly waiting for targets updates in long polling requests
				err := cfg.updateTargets(ctx)
				if err != nil {
					logger.Errorf("there were errors when discovering kuma targets, so preserving the previous targets. error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel
}

func (cfg *apiConfig) stopWatcher() {
	cfg.cancel()
	cfg.wg.Wait()
}

func (cfg *apiConfig) getTargets() ([]kumaTarget, error) {
	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	return cfg.targets, nil
}

func (cfg *apiConfig) updateTargets(ctx context.Context) error {
	requestBody, err := json.Marshal(discoveryRequest{
		VersionInfo:   cfg.latestVersion,
		Node:          discoveryRequestNode{Id: discoveryNode},
		TypeUrl:       xdsResourceTypeUrl,
		ResponseNonce: cfg.latestNonce,
	})
	if err != nil {
		return fmt.Errorf("cannot marshal request body for kuma_sd api: %w", err)
	}

	var statusCode int
	data, err := cfg.client.GetBlockingAPIResponseWithParamsCtx(
		ctx,
		cfg.path,
		func(request *http.Request) {
			request.Method = http.MethodPost
			request.Body = io.NopCloser(bytes.NewReader(requestBody))

			// set max duration for long polling request
			query := request.URL.Query()
			query.Add("fetch-timeout", cfg.getWaitTime().String())
			request.URL.RawQuery = query.Encode()

			request.Header.Set("Accept", "application/json")
			request.Header.Set("Content-Type", "application/json")
		},
		func(response *http.Response) {
			statusCode = response.StatusCode
		},
	)

	if statusCode == http.StatusNotModified {
		return nil
	}
	if err != nil {
		cfg.fetchErrors.Inc()
		return fmt.Errorf("cannot read kuma_sd api response: %w", err)
	}

	response, err := parseDiscoveryResponse(data)
	if err != nil {
		cfg.parseErrors.Inc()
		return fmt.Errorf("cannot parse kuma_sd api response: %w", err)
	}

	cfg.mu.Lock()
	defer cfg.mu.Unlock()
	cfg.targets = parseKumaTargets(response)
	cfg.latestVersion = response.VersionInfo
	cfg.latestNonce = response.Nonce

	return nil
}

func (cfg *apiConfig) getWaitTime() time.Duration {
	d := discoveryutils.BlockingClientReadTimeout
	// Reduce wait time to avoid timeouts (request execution time should be less than the read timeout)
	d -= d / 8
	if *waitTime > time.Second && *waitTime < d {
		d = *waitTime
	}
	return d
}

func (cfg *apiConfig) mustStop() {
	cfg.stopWatcher()
	cfg.client.Stop()
}
