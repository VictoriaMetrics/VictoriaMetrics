package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type response struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Labels map[string]string `json:"metric"`
			TV     [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (r response) metrics() ([]Metric, error) {
	var ms []Metric
	var m Metric
	var f float64
	var err error
	for i, res := range r.Data.Result {
		f, err = strconv.ParseFloat(res.TV[1].(string), 64)
		if err != nil {
			return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, res.TV[1], err)
		}
		m.Labels = nil
		for k, v := range r.Data.Result[i].Labels {
			m.AddLabel(k, v)
		}
		m.Timestamp = int64(res.TV[0].(float64))
		m.Value = f
		ms = append(ms, m)
	}
	return ms, nil
}

type graphiteResponse []graphiteResponseTarget

type graphiteResponseTarget struct {
	Target     string            `json:"target"`
	Tags       map[string]string `json:"tags"`
	DataPoints [][2]float64      `json:"datapoints"`
}

func (r graphiteResponse) metrics() []Metric {
	var ms []Metric
	for _, res := range r {
		if len(res.DataPoints) < 1 {
			continue
		}
		var m Metric
		// add only last value to the result.
		last := res.DataPoints[len(res.DataPoints)-1]
		m.Value = last[0]
		m.Timestamp = int64(last[1])
		for k, v := range res.Tags {
			m.AddLabel(k, v)
		}
		ms = append(ms, m)
	}
	return ms
}

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct {
	c                *http.Client
	datasourceURL    string
	basicAuthUser    string
	basicAuthPass    string
	appendTypePrefix bool
	lookBack         time.Duration
	queryStep        time.Duration
}

const queryPath = "/api/v1/query"
const graphitePath = "/render"

const prometheusPrefix = "/prometheus"
const graphitePrefix = "/graphite"

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
	}
}

// Query reads metrics from datasource by given query and type
func (s *VMStorage) Query(ctx context.Context, query string, dataSourceType Type) ([]Metric, error) {
	switch dataSourceType.name {
	case "", prometheusType:
		return s.queryDataSource(ctx, query, s.setPrometheusReqParams, parsePrometheusResponse)
	case graphiteType:
		return s.queryDataSource(ctx, query, s.setGraphiteReqParams, parseGraphiteResponse)
	default:
		return nil, fmt.Errorf("engine not found: %q", dataSourceType)
	}
}

func (s *VMStorage) queryDataSource(
	ctx context.Context,
	query string,
	setReqParams func(r *http.Request, query string),
	processResponse func(r *http.Request, resp *http.Response,
	) ([]Metric, error)) ([]Metric, error) {
	req, err := http.NewRequest("POST", s.datasourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if s.basicAuthPass != "" {
		req.SetBasicAuth(s.basicAuthUser, s.basicAuthPass)
	}
	setReqParams(req, query)
	resp, err := s.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s: %w", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("datasource returns unexpected response code %d for %s. Response body %s", resp.StatusCode, req.URL, body)
	}
	return processResponse(req, resp)
}

func (s *VMStorage) setPrometheusReqParams(r *http.Request, query string) {
	if s.appendTypePrefix {
		r.URL.Path += prometheusPrefix
	}
	r.URL.Path += queryPath
	q := r.URL.Query()
	q.Set("query", query)
	if s.lookBack > 0 {
		lookBack := time.Now().Add(-s.lookBack)
		q.Set("time", fmt.Sprintf("%d", lookBack.Unix()))
	}
	if s.queryStep > 0 {
		q.Set("step", s.queryStep.String())
	}
	r.URL.RawQuery = q.Encode()
}

func (s *VMStorage) setGraphiteReqParams(r *http.Request, query string) {
	if s.appendTypePrefix {
		r.URL.Path += graphitePrefix
	}
	r.URL.Path += graphitePath
	q := r.URL.Query()
	q.Set("format", "json")
	q.Set("target", query)
	from := "-5min"
	if s.lookBack > 0 {
		lookBack := time.Now().Add(-s.lookBack)
		from = strconv.FormatInt(lookBack.Unix(), 10)
	}
	q.Set("from", from)
	q.Set("until", "now")
	r.URL.RawQuery = q.Encode()
}

const (
	statusSuccess, statusError, rtVector = "success", "error", "vector"
)

func parsePrometheusResponse(req *http.Request, resp *http.Response) ([]Metric, error) {
	r := &response{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing prometheus metrics for %s: %w", req.URL, err)
	}
	if r.Status == statusError {
		return nil, fmt.Errorf("response error, query: %s, errorType: %s, error: %s", req.URL, r.ErrorType, r.Error)
	}
	if r.Status != statusSuccess {
		return nil, fmt.Errorf("unknown status: %s, Expected success or error ", r.Status)
	}
	if r.Data.ResultType != rtVector {
		return nil, fmt.Errorf("unknown result type:%s. Expected vector", r.Data.ResultType)
	}
	return r.metrics()
}

func parseGraphiteResponse(req *http.Request, resp *http.Response) ([]Metric, error) {
	r := &graphiteResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing graphite metrics for %s: %w", req.URL, err)
	}
	return r.metrics(), nil
}
