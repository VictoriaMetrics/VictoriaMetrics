package datasource

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct {
	c                *http.Client
	datasourceURL    string
	basicAuthUser    string
	basicAuthPass    string
	appendTypePrefix bool
	lookBack         time.Duration
	queryStep        time.Duration
	roundDigits      string

	dataSourceType     Type
	evaluationInterval time.Duration
	extraLabels        []string
}

// Clone makes clone of VMStorage, shares http client.
func (s *VMStorage) Clone() *VMStorage {
	return &VMStorage{
		c:                s.c,
		datasourceURL:    s.datasourceURL,
		basicAuthUser:    s.basicAuthUser,
		basicAuthPass:    s.basicAuthPass,
		lookBack:         s.lookBack,
		queryStep:        s.queryStep,
		appendTypePrefix: s.appendTypePrefix,
		dataSourceType:   s.dataSourceType,
	}
}

// ApplyParams - changes given querier params.
func (s *VMStorage) ApplyParams(params QuerierParams) *VMStorage {
	if params.DataSourceType != nil {
		s.dataSourceType = *params.DataSourceType
	}
	s.evaluationInterval = params.EvaluationInterval
	for k, v := range params.ExtraLabels {
		s.extraLabels = append(s.extraLabels, fmt.Sprintf("%s=%s", k, v))
	}
	return s
}

// BuildWithParams - implements interface.
func (s *VMStorage) BuildWithParams(params QuerierParams) Querier {
	return s.Clone().ApplyParams(params)
}

// NewVMStorage is a constructor for VMStorage
func NewVMStorage(baseURL, basicAuthUser, basicAuthPass string, lookBack time.Duration, queryStep time.Duration, appendTypePrefix bool, c *http.Client) *VMStorage {
	return &VMStorage{
		c:                c,
		basicAuthUser:    basicAuthUser,
		basicAuthPass:    basicAuthPass,
		datasourceURL:    strings.TrimSuffix(baseURL, "/"),
		appendTypePrefix: appendTypePrefix,
		lookBack:         lookBack,
		queryStep:        queryStep,
		dataSourceType:   NewPrometheusType(),
	}
}

// Query executes the given query and returns parsed response
func (s *VMStorage) Query(ctx context.Context, query string) ([]Metric, error) {
	req, err := s.newRequestPOST()
	if err != nil {
		return nil, err
	}

	ts := time.Now()
	switch s.dataSourceType.name {
	case "", prometheusType:
		s.setPrometheusInstantReqParams(req, query, ts)
	case graphiteType:
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
	if s.dataSourceType.name != prometheusType {
		parseFn = parseGraphiteResponse
	}
	return parseFn(req, resp)
}

// QueryRange executes the given query on the given time range.
// For Prometheus type see https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
// Graphite type isn't supported.
func (s *VMStorage) QueryRange(ctx context.Context, query string, start, end time.Time) ([]Metric, error) {
	if s.dataSourceType.name != prometheusType {
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
		return nil, fmt.Errorf("error getting response from %s: %w", req.URL, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected response code %d for %s. Response body %s", resp.StatusCode, req.URL, body)
	}
	return resp, nil
}

func (s *VMStorage) newRequestPOST() (*http.Request, error) {
	req, err := http.NewRequest("POST", s.datasourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if s.basicAuthPass != "" {
		req.SetBasicAuth(s.basicAuthUser, s.basicAuthPass)
	}
	return req, nil
}
