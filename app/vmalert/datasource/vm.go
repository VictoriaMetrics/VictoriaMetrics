package datasource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

type datasourceType string

const (
	datasourcePrometheus datasourceType = "prometheus"
	datasourceGraphite   datasourceType = "graphite"
)

func toDatasourceType(s string) datasourceType {
	if s == string(datasourceGraphite) {
		return datasourceGraphite
	}
	return datasourcePrometheus
}

// VMStorage represents vmstorage entity with ability to read and write metrics
// WARN: when adding a new field, remember to update Clone() method.
type VMStorage struct {
	c                *http.Client
	authCfg          *promauth.Config
	datasourceURL    string
	appendTypePrefix bool
	lookBack         time.Duration
	evalOffset       *time.Duration
	queryStep        time.Duration

	dataSourceType     datasourceType
	evaluationInterval time.Duration
	extraParams        url.Values
	extraHeaders       []keyValue

	// whether to print additional log messages
	// for each sent request
	debug bool
}

type keyValue struct {
	key   string
	value string
}

// Clone makes clone of VMStorage, shares http client.
func (s *VMStorage) Clone() *VMStorage {
	ns := &VMStorage{
		c:                s.c,
		authCfg:          s.authCfg,
		datasourceURL:    s.datasourceURL,
		appendTypePrefix: s.appendTypePrefix,
		lookBack:         s.lookBack,
		queryStep:        s.queryStep,

		dataSourceType:     s.dataSourceType,
		evaluationInterval: s.evaluationInterval,

		// init map so it can be populated below
		extraParams: url.Values{},

		debug: s.debug,
	}
	if len(s.extraHeaders) > 0 {
		ns.extraHeaders = make([]keyValue, len(s.extraHeaders))
		copy(ns.extraHeaders, s.extraHeaders)
	}
	for k, v := range s.extraParams {
		ns.extraParams[k] = v
	}

	return ns
}

// ApplyParams - changes given querier params.
func (s *VMStorage) ApplyParams(params QuerierParams) *VMStorage {
	s.dataSourceType = toDatasourceType(params.DataSourceType)
	s.evaluationInterval = params.EvaluationInterval
	s.evalOffset = params.EvalOffset
	if params.QueryParams != nil {
		if s.extraParams == nil {
			s.extraParams = url.Values{}
		}
		for k, vl := range params.QueryParams {
			for _, v := range vl { // custom query params are prior to default ones
				s.extraParams.Set(k, v)
			}
		}
	}
	if params.Headers != nil {
		for key, value := range params.Headers {
			kv := keyValue{key: key, value: value}
			s.extraHeaders = append(s.extraHeaders, kv)
		}
	}
	s.debug = params.Debug
	return s
}

// BuildWithParams - implements interface.
func (s *VMStorage) BuildWithParams(params QuerierParams) Querier {
	return s.Clone().ApplyParams(params)
}

// NewVMStorage is a constructor for VMStorage
func NewVMStorage(baseURL string, authCfg *promauth.Config, lookBack time.Duration, queryStep time.Duration, appendTypePrefix bool, c *http.Client) *VMStorage {
	return &VMStorage{
		c:                c,
		authCfg:          authCfg,
		datasourceURL:    strings.TrimSuffix(baseURL, "/"),
		appendTypePrefix: appendTypePrefix,
		lookBack:         lookBack,
		queryStep:        queryStep,
		dataSourceType:   datasourcePrometheus,
		extraParams:      url.Values{},
	}
}

// Query executes the given query and returns parsed response
func (s *VMStorage) Query(ctx context.Context, query string, ts time.Time) (Result, *http.Request, error) {
	req := s.newQueryRequest(query, ts)
	resp, err := s.do(ctx, req)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		// something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req = s.newQueryRequest(query, ts)
		resp, err = s.do(ctx, req)
	}
	if err != nil {
		return Result{}, req, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	parseFn := parsePrometheusResponse
	if s.dataSourceType != datasourcePrometheus {
		parseFn = parseGraphiteResponse
	}
	result, err := parseFn(req, resp)
	return result, req, err
}

// QueryRange executes the given query on the given time range.
// For Prometheus type see https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
// Graphite type isn't supported.
func (s *VMStorage) QueryRange(ctx context.Context, query string, start, end time.Time) (res Result, err error) {
	if s.dataSourceType != datasourcePrometheus {
		return res, fmt.Errorf("%q is not supported for QueryRange", s.dataSourceType)
	}
	if start.IsZero() {
		return res, fmt.Errorf("start param is missing")
	}
	if end.IsZero() {
		return res, fmt.Errorf("end param is missing")
	}
	req := s.newQueryRangeRequest(query, start, end)
	resp, err := s.do(ctx, req)
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		// something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req = s.newQueryRangeRequest(query, start, end)
		resp, err = s.do(ctx, req)
	}
	if err != nil {
		return res, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return parsePrometheusResponse(req, resp)
}

func (s *VMStorage) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if s.debug {
		logger.Infof("DEBUG datasource request: executing %s request with params %q", req.Method, req.URL.RawQuery)
	}
	resp, err := s.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s: %w", req.URL.Redacted(), err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected response code %d for %s. Response body %s", resp.StatusCode, req.URL.Redacted(), body)
	}
	return resp, nil
}

func (s *VMStorage) newQueryRangeRequest(query string, start, end time.Time) *http.Request {
	req := s.newRequest()
	s.setPrometheusRangeReqParams(req, query, start, end)
	return req
}

func (s *VMStorage) newQueryRequest(query string, ts time.Time) *http.Request {
	req := s.newRequest()
	switch s.dataSourceType {
	case "", datasourcePrometheus:
		s.setPrometheusInstantReqParams(req, query, ts)
	case datasourceGraphite:
		s.setGraphiteReqParams(req, query, ts)
	default:
		logger.Panicf("BUG: engine not found: %q", s.dataSourceType)
	}
	return req
}

func (s *VMStorage) newRequest() *http.Request {
	req, err := http.NewRequest(http.MethodPost, s.datasourceURL, nil)
	if err != nil {
		logger.Panicf("BUG: unexpected error from http.NewRequest(%q): %s", s.datasourceURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authCfg != nil {
		s.authCfg.SetHeaders(req, true)
	}
	for _, h := range s.extraHeaders {
		req.Header.Set(h.key, h.value)
	}
	return req
}
