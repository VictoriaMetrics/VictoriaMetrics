package datasource

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct {
	c                *http.Client
	authCfg          *promauth.Config
	datasourceURL    string
	appendTypePrefix bool
	lookBack         time.Duration
	queryStep        time.Duration

	dataSourceType     Type
	evaluationInterval time.Duration
	extraParams        url.Values
	disablePathAppend  bool
}

// Clone makes clone of VMStorage, shares http client.
func (s *VMStorage) Clone() *VMStorage {
	return &VMStorage{
		c:                 s.c,
		authCfg:           s.authCfg,
		datasourceURL:     s.datasourceURL,
		lookBack:          s.lookBack,
		queryStep:         s.queryStep,
		appendTypePrefix:  s.appendTypePrefix,
		dataSourceType:    s.dataSourceType,
		disablePathAppend: s.disablePathAppend,
	}
}

// ApplyParams - changes given querier params.
func (s *VMStorage) ApplyParams(params QuerierParams) *VMStorage {
	if params.DataSourceType != nil {
		s.dataSourceType = *params.DataSourceType
	}
	s.evaluationInterval = params.EvaluationInterval
	s.extraParams = params.QueryParams
	return s
}

// BuildWithParams - implements interface.
func (s *VMStorage) BuildWithParams(params QuerierParams) Querier {
	return s.Clone().ApplyParams(params)
}

// NewVMStorage is a constructor for VMStorage
func NewVMStorage(baseURL string, authCfg *promauth.Config, lookBack time.Duration, queryStep time.Duration, appendTypePrefix bool, c *http.Client, disablePathAppend bool) *VMStorage {
	return &VMStorage{
		c:                 c,
		authCfg:           authCfg,
		datasourceURL:     strings.TrimSuffix(baseURL, "/"),
		appendTypePrefix:  appendTypePrefix,
		lookBack:          lookBack,
		queryStep:         queryStep,
		dataSourceType:    NewPrometheusType(),
		disablePathAppend: disablePathAppend,
	}
}

// Query executes the given query and returns parsed response
func (s *VMStorage) Query(ctx context.Context, query string) ([]Metric, error) {
	req, err := s.newRequestPOST()
	if err != nil {
		return nil, err
	}

	ts := time.Now()
	switch s.dataSourceType.String() {
	case "prometheus":
		s.setPrometheusInstantReqParams(req, query, ts)
	case "graphite":
		s.setGraphiteReqParams(req, query, ts)
	default:
		return nil, fmt.Errorf("engine not found: %q", s.dataSourceType.name)
	}

	resp, err := s.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	parseFn := parsePrometheusResponse
	if s.dataSourceType.name != "prometheus" {
		parseFn = parseGraphiteResponse
	}
	return parseFn(req, resp)
}

// QueryRange executes the given query on the given time range.
// For Prometheus type see https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
// Graphite type isn't supported.
func (s *VMStorage) QueryRange(ctx context.Context, query string, start, end time.Time) ([]Metric, error) {
	if s.dataSourceType.name != "prometheus" {
		return nil, fmt.Errorf("%q is not supported for QueryRange", s.dataSourceType.name)
	}
	req, err := s.newRequestPOST()
	if err != nil {
		return nil, err
	}
	if start.IsZero() {
		return nil, fmt.Errorf("start param is missing")
	}
	if end.IsZero() {
		return nil, fmt.Errorf("end param is missing")
	}
	s.setPrometheusRangeReqParams(req, query, start, end)
	resp, err := s.do(ctx, req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return parsePrometheusResponse(req, resp)
}

func (s *VMStorage) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	resp, err := s.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s: %w", req.URL.Redacted(), err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected response code %d for %s. Response body %s", resp.StatusCode, req.URL.Redacted(), body)
	}
	return resp, nil
}

func (s *VMStorage) newRequestPOST() (*http.Request, error) {
	req, err := http.NewRequest("POST", s.datasourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authCfg != nil {
		if auth := s.authCfg.GetAuthHeader(); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}
	return req, nil
}
