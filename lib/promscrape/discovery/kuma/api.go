package kuma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/metrics"
)

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client   *discoveryutil.Client
	clientID string
	apiPath  string

	// labels contains the latest discovered labels.
	labels atomic.Pointer[[]*promutil.Labels]

	cancel context.CancelFunc
	wg     sync.WaitGroup

	latestVersion string
	latestNonce   string

	fetchErrors *metrics.Counter
	parseErrors *metrics.Counter
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	apiServer, apiPath, err := getAPIServerPath(sdc.Server)
	if err != nil {
		return nil, fmt.Errorf("cannot parse server %q: %w", sdc.Server, err)
	}
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}

	clientID := sdc.ClientID
	if clientID == "" {
		clientID, _ = os.Hostname()
		if clientID == "" {
			clientID = "vmagent"
		}
	}

	cfg := &apiConfig{
		client:   client,
		clientID: clientID,
		apiPath:  apiPath,

		fetchErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_kuma_errors_total{type="fetch",url=%q}`, sdc.Server)),
		parseErrors: metrics.GetOrCreateCounter(fmt.Sprintf(`promscrape_discovery_kuma_errors_total{type="parse",url=%q}`, sdc.Server)),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg.cancel = cancel

	// Initialize targets synchronously and then start updating them in background.
	// The synchronous targets' update is needed for returning non-empty list of targets
	// just after the initialization.
	if err := cfg.updateTargetsLabels(ctx); err != nil {
		client.Stop()
		return nil, fmt.Errorf("cannot discover Kuma targets: %w", err)
	}
	cfg.wg.Add(1)
	go func() {
		defer cfg.wg.Done()
		cfg.runTargetsWatcher(ctx)
	}()

	return cfg, nil
}

func getAPIServerPath(serverURL string) (string, string, error) {
	if serverURL == "" {
		return "", "", fmt.Errorf("missing servier url")
	}
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	psu, err := url.Parse(serverURL)
	if err != nil {
		return "", "", fmt.Errorf("cannot parse server url=%q: %w", serverURL, err)
	}
	apiServer := fmt.Sprintf("%s://%s", psu.Scheme, psu.Host)
	apiPath := psu.Path
	if !strings.HasSuffix(apiPath, "/") {
		apiPath += "/"
	}
	apiPath += "v3/discovery:monitoringassignments"
	if psu.RawQuery != "" {
		apiPath += "?" + psu.RawQuery
	}
	return apiServer, apiPath, nil
}

func (cfg *apiConfig) runTargetsWatcher(ctx context.Context) {
	ticker := time.NewTicker(*SDCheckInterval)
	defer ticker.Stop()

	doneCh := ctx.Done()
	for {
		select {
		case <-ticker.C:
			if err := cfg.updateTargetsLabels(ctx); err != nil {
				logger.Errorf("there was an error when discovering Kuma targets, so preserving the previous targets; error: %s", err)
			}
		case <-doneCh:
			return
		}
	}
}

func (cfg *apiConfig) mustStop() {
	cfg.client.Stop()
	cfg.cancel()
	cfg.wg.Wait()
}

func (cfg *apiConfig) updateTargetsLabels(ctx context.Context) error {
	dReq := &discoveryRequest{
		VersionInfo: cfg.latestVersion,
		Node: discoveryRequestNode{
			ID: cfg.clientID,
		},
		TypeURL:       "type.googleapis.com/kuma.observability.v1.MonitoringAssignment",
		ResponseNonce: cfg.latestNonce,
	}
	requestBody, err := json.Marshal(dReq)
	if err != nil {
		logger.Panicf("BUG: cannot marshal Kuma discovery request: %s", err)
	}
	updateRequestFunc := func(req *http.Request) {
		req.Method = http.MethodPost
		req.Body = io.NopCloser(bytes.NewReader(requestBody))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
	}
	notModified := false
	inspectResponseFunc := func(resp *http.Response) {
		if resp.StatusCode == http.StatusNotModified {
			// Override status code, so GetBlockingAPIResponseWithParamsCtx() returns nil error.
			resp.StatusCode = http.StatusOK
			notModified = true
		}
	}
	data, err := cfg.client.GetAPIResponseWithParamsCtx(ctx, cfg.apiPath, updateRequestFunc, inspectResponseFunc)
	if err != nil {
		cfg.fetchErrors.Inc()
		return fmt.Errorf("error when reading Kuma discovery response: %w", err)
	}
	if notModified {
		// The targets weren't modified, so nothing to update.
		return nil
	}

	// Parse response
	labels, versionInfo, nonce, err := parseTargetsLabels(data)
	if err != nil {
		cfg.parseErrors.Inc()
		return fmt.Errorf("cannot parse Kuma discovery response received from %q: %w", cfg.client.APIServer(), err)
	}
	cfg.labels.Store(&labels)
	cfg.latestVersion = versionInfo
	cfg.latestNonce = nonce

	return nil
}

func parseTargetsLabels(data []byte) ([]*promutil.Labels, string, string, error) {
	var dResp discoveryResponse
	if err := json.Unmarshal(data, &dResp); err != nil {
		return nil, "", "", err
	}
	return dResp.getTargetsLabels(), dResp.VersionInfo, dResp.Nonce, nil
}

func (dr *discoveryResponse) getTargetsLabels() []*promutil.Labels {
	var ms []*promutil.Labels
	for _, r := range dr.Resources {
		for _, t := range r.Targets {
			m := promutil.NewLabels(8 + len(r.Labels) + len(t.Labels))

			m.Add("instance", t.Name)
			m.Add("__address__", t.Address)
			m.Add("__scheme__", t.Scheme)
			m.Add("__metrics_path__", t.MetricsPath)
			m.Add("__meta_kuma_dataplane", t.Name)
			m.Add("__meta_kuma_mesh", r.Mesh)
			m.Add("__meta_kuma_service", r.Service)

			addLabels(m, r.Labels)
			addLabels(m, t.Labels)
			// Remove possible duplicate labels after addLabels() calls above
			m.RemoveDuplicates()

			ms = append(ms, m)
		}
	}
	return ms
}

func addLabels(dst *promutil.Labels, src map[string]string) {
	bb := bbPool.Get()
	b := bb.B
	for k, v := range src {
		b = append(b[:0], "__meta_kuma_label_"...)
		b = append(b, discoveryutil.SanitizeLabelName(k)...)
		labelName := bytesutil.InternBytes(b)
		dst.Add(labelName, v)
	}
	bb.B = b
	bbPool.Put(bb)
}

var bbPool bytesutil.ByteBufferPool

// discoveryRequest represent xDS-requests for Kuma Service Mesh
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#envoy-v3-api-msg-service-discovery-v3-discoveryrequest
type discoveryRequest struct {
	VersionInfo   string               `json:"version_info"`
	Node          discoveryRequestNode `json:"node"`
	ResourceNames []string             `json:"resource_names"`
	TypeURL       string               `json:"type_url"`
	ResponseNonce string               `json:"response_nonce"`
}

type discoveryRequestNode struct {
	ID string `json:"id"`
}

// discoveryResponse represent xDS-requests for Kuma Service Mesh
// https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/discovery/v3/discovery.proto#envoy-v3-api-msg-service-discovery-v3-discoveryresponse
type discoveryResponse struct {
	VersionInfo string     `json:"version_info"`
	Resources   []resource `json:"resources"`
	Nonce       string     `json:"nonce"`
}

type resource struct {
	Mesh    string            `json:"mesh"`
	Service string            `json:"service"`
	Targets []target          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

type target struct {
	Name        string            `json:"name"`
	Scheme      string            `json:"scheme"`
	Address     string            `json:"address"`
	MetricsPath string            `json:"metrics_path"`
	Labels      map[string]string `json:"labels"`
}
