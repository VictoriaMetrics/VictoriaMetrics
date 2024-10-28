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
	datasourceVLogs      datasourceType = "vlogs"
)

func toDatasourceType(s string) datasourceType {
	switch s {
	case string(datasourcePrometheus):
		return datasourcePrometheus
	case string(datasourceGraphite):
		return datasourceGraphite
	case string(datasourceVLogs):
		return datasourceVLogs
	default:
		logger.Panicf("BUG: unknown datasource type %q", s)
	}
	return ""
}

// Client is a datasource entity for reading data,
// supported clients are enumerated in datasourceType.
// WARN: when adding a new field, remember to check if Clone() method needs to be updated.
type Client struct {
	c                *http.Client
	authCfg          *promauth.Config
	datasourceURL    string
	appendTypePrefix bool
	queryStep        time.Duration
	dataSourceType   datasourceType
	// ApplyIntervalAsTimeFilter is only valid for vlogs datasource.
	// Set to true if there is no [timeFilter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) in the rule expression,
	// and we will add evaluation interval as an additional timeFilter when querying.
	applyIntervalAsTimeFilter bool

	// evaluationInterval will help setting request's `step` param,
	// or adding time filter for LogsQL expression.
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

// Clone clones shared http client and other configuration to the new client.
func (c *Client) Clone() *Client {
	ns := &Client{
		c:                c.c,
		authCfg:          c.authCfg,
		datasourceURL:    c.datasourceURL,
		appendTypePrefix: c.appendTypePrefix,
		queryStep:        c.queryStep,

		dataSourceType:     c.dataSourceType,
		evaluationInterval: c.evaluationInterval,

		// init map so it can be populated below
		extraParams: url.Values{},

		debug: c.debug,
	}
	if len(c.extraHeaders) > 0 {
		ns.extraHeaders = make([]keyValue, len(c.extraHeaders))
		copy(ns.extraHeaders, c.extraHeaders)
	}
	for k, v := range c.extraParams {
		ns.extraParams[k] = v
	}

	return ns
}

// ApplyParams - changes given querier params.
func (c *Client) ApplyParams(params QuerierParams) *Client {
	if params.DataSourceType != "" {
		c.dataSourceType = toDatasourceType(params.DataSourceType)
	}
	c.evaluationInterval = params.EvaluationInterval
	c.applyIntervalAsTimeFilter = params.ApplyIntervalAsTimeFilter
	if params.QueryParams != nil {
		if c.extraParams == nil {
			c.extraParams = url.Values{}
		}
		for k, vl := range params.QueryParams {
			// custom query params are prior to default ones
			if c.extraParams.Has(k) {
				c.extraParams.Del(k)
			}
			for _, v := range vl {
				// don't use .Set() instead of Del/Add since it is allowed
				// for GET params to be duplicated
				// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4908
				c.extraParams.Add(k, v)
			}
		}
	}
	if params.Headers != nil {
		for key, value := range params.Headers {
			kv := keyValue{key: key, value: value}
			c.extraHeaders = append(c.extraHeaders, kv)
		}
	}
	c.debug = params.Debug
	return c
}

// BuildWithParams - implements interface.
func (c *Client) BuildWithParams(params QuerierParams) Querier {
	return c.Clone().ApplyParams(params)
}

// NewPrometheusClient returns a new prometheus datasource client.
func NewPrometheusClient(baseURL string, authCfg *promauth.Config, appendTypePrefix bool, c *http.Client) *Client {
	return &Client{
		c:                c,
		authCfg:          authCfg,
		datasourceURL:    strings.TrimSuffix(baseURL, "/"),
		appendTypePrefix: appendTypePrefix,
		queryStep:        *queryStep,
		dataSourceType:   datasourcePrometheus,
		extraParams:      url.Values{},
	}
}

// Query executes the given query and returns parsed response
func (c *Client) Query(ctx context.Context, query string, ts time.Time) (Result, *http.Request, error) {
	req, err := c.newQueryRequest(ctx, query, ts)
	if err != nil {
		return Result{}, nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !netutil.IsTrivialNetworkError(err) {
			// Return unexpected error to the caller.
			return Result{}, nil, err
		}
		// Something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req, err = c.newQueryRequest(ctx, query, ts)
		if err != nil {
			return Result{}, nil, fmt.Errorf("second attempt: %w", err)
		}
		resp, err = c.do(req)
		if err != nil {
			return Result{}, nil, fmt.Errorf("second attempt: %w", err)
		}
	}

	// Process the received response.
	var parseFn func(req *http.Request, resp *http.Response) (Result, error)
	switch c.dataSourceType {
	case datasourcePrometheus:
		parseFn = parsePrometheusResponse
	case datasourceGraphite:
		parseFn = parseGraphiteResponse
	case datasourceVLogs:
		parseFn = parseVLogsResponse
	default:
		logger.Panicf("BUG: unsupported datasource type %q to parse query response", c.dataSourceType)
	}
	result, err := parseFn(req, resp)
	_ = resp.Body.Close()
	return result, req, err
}

// QueryRange executes the given query on the given time range.
// For Prometheus type see https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
// Graphite type isn't supported.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time) (res Result, err error) {
	if c.dataSourceType == datasourceGraphite {
		return res, fmt.Errorf("%q is not supported for QueryRange", c.dataSourceType)
	}
	// TODO: disable range query LogsQL with time filter now
	if c.dataSourceType == datasourceVLogs && !c.applyIntervalAsTimeFilter {
		return res, fmt.Errorf("range query is not supported for LogsQL expression %q because it contains time filter. Remove time filter from the expression and try again", query)
	}
	if start.IsZero() {
		return res, fmt.Errorf("start param is missing")
	}
	if end.IsZero() {
		return res, fmt.Errorf("end param is missing")
	}
	req, err := c.newQueryRangeRequest(ctx, query, start, end)
	if err != nil {
		return res, err
	}
	resp, err := c.do(req)
	if err != nil {
		if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) && !netutil.IsTrivialNetworkError(err) {
			// Return unexpected error to the caller.
			return res, err
		}
		// Something in the middle between client and datasource might be closing
		// the connection. So we do a one more attempt in hope request will succeed.
		req, err = c.newQueryRangeRequest(ctx, query, start, end)
		if err != nil {
			return res, fmt.Errorf("second attempt: %w", err)
		}
		resp, err = c.do(req)
		if err != nil {
			return res, fmt.Errorf("second attempt: %w", err)
		}
	}

	// Process the received response.
	var parseFn func(req *http.Request, resp *http.Response) (Result, error)
	switch c.dataSourceType {
	case datasourcePrometheus:
		parseFn = parsePrometheusResponse
	case datasourceVLogs:
		parseFn = parseVLogsResponse
	default:
		logger.Panicf("BUG: unsupported datasource type %q to parse query range response", c.dataSourceType)
	}
	res, err = parseFn(req, resp)
	_ = resp.Body.Close()
	return res, err
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	ru := req.URL.Redacted()
	if *showDatasourceURL {
		ru = req.URL.String()
	}
	if c.debug {
		logger.Infof("DEBUG datasource request: executing %s request with params %q", req.Method, ru)
	}
	resp, err := c.c.Do(req)
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

func (c *Client) newQueryRangeRequest(ctx context.Context, query string, start, end time.Time) (*http.Request, error) {
	req, err := c.newRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create query_range request to datasource %q: %w", c.datasourceURL, err)
	}
	switch c.dataSourceType {
	case datasourcePrometheus:
		c.setPrometheusRangeReqParams(req, query, start, end)
	case datasourceVLogs:
		c.setVLogsRangeReqParams(req, query, start, end)
	default:
		logger.Panicf("BUG: unsupported datasource type %q to create range query request", c.dataSourceType)
	}
	return req, nil
}

func (c *Client) newQueryRequest(ctx context.Context, query string, ts time.Time) (*http.Request, error) {
	req, err := c.newRequest(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot create query request to datasource %q: %w", c.datasourceURL, err)
	}
	switch c.dataSourceType {
	case datasourcePrometheus:
		c.setPrometheusInstantReqParams(req, query, ts)
	case datasourceGraphite:
		c.setGraphiteReqParams(req, query)
	case datasourceVLogs:
		c.setVLogsInstantReqParams(req, query, ts)
	default:
		logger.Panicf("BUG: unsupported datasource type %q to create query request", c.dataSourceType)
	}
	return req, nil
}

func (c *Client) newRequest(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.datasourceURL, nil)
	if err != nil {
		logger.Panicf("BUG: unexpected error from http.NewRequest(%q): %s", c.datasourceURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authCfg != nil {
		err = c.authCfg.SetHeaders(req, true)
		if err != nil {
			return nil, err
		}
	}
	for _, h := range c.extraHeaders {
		req.Header.Set(h.key, h.value)
	}
	return req, nil
}

// setReqParams adds query and other extra params for the request.
func (c *Client) setReqParams(r *http.Request, query string) {
	q := r.URL.Query()
	for k, vs := range c.extraParams {
		if q.Has(k) { // extraParams are prior to params in URL
			q.Del(k)
		}
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("query", query)
	r.URL.RawQuery = q.Encode()
}
