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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
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
	queryStep        time.Duration
	dataSourceType   datasourceType

	// evaluationInterval will help setting request's `step` param.
	evaluationInterval time.Duration
	// extraParams contains params to be attached to each HTTP request
	extraParams url.Values
	// extraHeaders are headers to be attached to each HTTP request
	extraHeaders []keyValue

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
	if params.QueryParams != nil {
		if s.extraParams == nil {
			s.extraParams = url.Values{}
		}
		for k, vl := range params.QueryParams {
			// custom query params are prior to default ones
			if s.extraParams.Has(k) {
				s.extraParams.Del(k)
			}
			for _, v := range vl {
				// don't use .Set() instead of Del/Add since it is allowed
				// for GET params to be duplicated
				// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4908
				s.extraParams.Add(k, v)
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
func NewVMStorage(baseURL string, authCfg *promauth.Config, queryStep time.Duration, appendTypePrefix bool, c *http.Client) *VMStorage {
	return &VMStorage{
		c:                c,
		authCfg:          authCfg,
		datasourceURL:    strings.TrimSuffix(baseURL, "/"),
		appendTypePrefix: appendTypePrefix,
		queryStep:        queryStep,
		dataSourceType:   datasourcePrometheus,
		extraParams:      url.Values{},
	}
}

// Query executes the given query and returns parsed response
func (s *VMStorage) Query(ctx context.Context, query string, ts time.Time) (Result, *http.Request, error) {
	req, err := s.newQueryRequest(ctx, query, ts)
	if err != nil {
		return Result{}, nil, err
	}
	resp, err := s.do(req)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !netutil.IsTrivialNetworkError(err) {
			// Return unexpected error to the caller.
			return Result{}, nil, err
		}
		// Something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req, err = s.newQueryRequest(ctx, query, ts)
		if err != nil {
			return Result{}, nil, fmt.Errorf("second attempt: %w", err)
		}
		resp, err = s.do(req)
		if err != nil {
			return Result{}, nil, fmt.Errorf("second attempt: %w", err)
		}
	}

	// Process the received response.
	parseFn := parsePrometheusResponse
	if s.dataSourceType != datasourcePrometheus {
		parseFn = parseGraphiteResponse
	}
	result, err := parseFn(req, resp)
	_ = resp.Body.Close()
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
	req, err := s.newQueryRangeRequest(ctx, query, start, end)
	if err != nil {
		return res, err
	}
	resp, err := s.do(req)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !netutil.IsTrivialNetworkError(err) {
			// Return unexpected error to the caller.
			return res, err
		}
		// Something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req, err = s.newQueryRangeRequest(ctx, query, start, end)
		if err != nil {
			return res, fmt.Errorf("second attempt: %w", err)
		}
		resp, err = s.do(req)
		if err != nil {
			return res, fmt.Errorf("second attempt: %w", err)
		}
	}

	// Process the received response.
	res, err = parsePrometheusResponse(req, resp)
	_ = resp.Body.Close()
	return res, err
}

func (s *VMStorage) do(req *http.Request) (*http.Response, error) {
	ru := req.URL.Redacted()
	if *showDatasourceURL {
		ru = req.URL.String()
	}
	if s.debug {
		logger.Infof("DEBUG datasource request: executing %s request with params %q", req.Method, ru)
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s: %w", ru, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unexpected response code %d for %s. Response body %s", resp.StatusCode, ru, body)
	}
	return resp, nil
}

func (s *VMStorage) newQueryRangeRequest(ctx context.Context, query string, start, end time.Time) (*http.Request, error) {
	req, err := s.newRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create query_range request to datasource %q: %w", s.datasourceURL, err)
	}
	s.setPrometheusRangeReqParams(req, query, start, end)
	return req, nil
}

func (s *VMStorage) newQueryRequest(ctx context.Context, query string, ts time.Time) (*http.Request, error) {
	req, err := s.newRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create query request to datasource %q: %w", s.datasourceURL, err)
	}
	switch s.dataSourceType {
	case "", datasourcePrometheus:
		s.setPrometheusInstantReqParams(req, query, ts)
	case datasourceGraphite:
		s.setGraphiteReqParams(req, query)
	default:
		logger.Panicf("BUG: engine not found: %q", s.dataSourceType)
	}
	return req, nil
}

func (s *VMStorage) newRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.datasourceURL, nil)
	if err != nil {
		logger.Panicf("BUG: unexpected error from http.NewRequest(%q): %s", s.datasourceURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.authCfg != nil {
		err = s.authCfg.SetHeaders(req, true)
		if err != nil {
			return nil, err
		}
	}
	for _, h := range s.extraHeaders {
		req.Header.Set(h.key, h.value)
	}
	return req, nil
}
