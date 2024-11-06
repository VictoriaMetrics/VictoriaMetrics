package apptest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// PrometheusQuerier contains methods available to Prometheus-like HTTP API for Querying
type PrometheusQuerier interface {
	PrometheusAPIV1Query(t *testing.T, query, time, step string, opts QueryOpts) *PrometheusAPIV1QueryResponse
	PrometheusAPIV1QueryRange(t *testing.T, query, start, end, step string, opts QueryOpts) *PrometheusAPIV1QueryResponse
	PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse
}

// PrometheusWriter contains methods available to Prometheus-like HTTP API for Writing new data
type PrometheusWriter interface {
	PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts)
}

// QueryOpts contains various params used for querying or ingesting data
type QueryOpts struct {
	Tenant  string
	Timeout string
}

// PrometheusAPIV1QueryResponse is an inmemory representation of the
// /prometheus/api/v1/query or /prometheus/api/v1/query_range response.
type PrometheusAPIV1QueryResponse struct {
	Status string
	Data   *QueryData
}

// NewPrometheusAPIV1QueryResponse is a test helper function that creates a new
// instance of PrometheusAPIV1QueryResponse by unmarshalling a json string.
func NewPrometheusAPIV1QueryResponse(t *testing.T, s string) *PrometheusAPIV1QueryResponse {
	t.Helper()

	res := &PrometheusAPIV1QueryResponse{}
	if err := json.Unmarshal([]byte(s), res); err != nil {
		t.Fatalf("could not unmarshal query response: %v", err)
	}
	return res
}

// QueryData holds the query result along with its type.
type QueryData struct {
	ResultType string
	Result     []*QueryResult
}

// QueryResult holds the metric name (in the form of label name-value
// collection) and its samples.
//
// Sample or Samples field is set for /prometheus/api/v1/query or
// /prometheus/api/v1/query_range response respectively.
type QueryResult struct {
	Metric  map[string]string
	Sample  *Sample   `json:"value"`
	Samples []*Sample `json:"values"`
}

// Sample is a timeseries value at a given timestamp.
type Sample struct {
	Timestamp int64
	Value     float64
}

// NewSample is a test helper function that creates a new sample out of time in
// RFC3339 format and a value.
func NewSample(t *testing.T, timeStr string, value float64) *Sample {
	parsedTime, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		t.Fatalf("could not parse RFC3339 time %q: %v", timeStr, err)
	}
	return &Sample{parsedTime.Unix(), value}
}

// UnmarshalJSON populates the sample fields from a JSON string.
func (s *Sample) UnmarshalJSON(b []byte) error {
	var (
		ts int64
		v  string
	)
	raw := []any{&ts, &v}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if got, want := len(raw), 2; got != want {
		return fmt.Errorf("unexpected number of fields: got %d, want %d (raw sample: %s)", got, want, string(b))
	}
	s.Timestamp = ts
	var err error
	s.Value, err = strconv.ParseFloat(v, 64)
	if err != nil {
		return fmt.Errorf("could not parse sample value %q: %w", v, err)
	}
	return nil
}

// PrometheusAPIV1SeriesResponse is an inmemory representation of the
// /prometheus/api/v1/series response.
type PrometheusAPIV1SeriesResponse struct {
	Status    string
	IsPartial bool
	Data      []map[string]string
}

// NewPrometheusAPIV1SeriesResponse is a test helper function that creates a new
// instance of PrometheusAPIV1SeriesResponse by unmarshalling a json string.
func NewPrometheusAPIV1SeriesResponse(t *testing.T, s string) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	res := &PrometheusAPIV1SeriesResponse{}
	if err := json.Unmarshal([]byte(s), res); err != nil {
		t.Fatalf("could not unmarshal series response: %v", err)
	}
	return res
}
