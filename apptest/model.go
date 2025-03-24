package apptest

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// PrometheusQuerier contains methods available to Prometheus-like HTTP API for Querying
type PrometheusQuerier interface {
	PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse
	PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse
	PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse
	PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse
}

// PrometheusWriter contains methods available to Prometheus-like HTTP API for Writing new data
type PrometheusWriter interface {
	PrometheusAPIV1Write(t *testing.T, records []pb.TimeSeries, opts QueryOpts)
	PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts)
}

// StorageFlusher defines a method that forces the flushing of data inserted
// into the storage, so it becomes available for searching immediately.
type StorageFlusher interface {
	ForceFlush(t *testing.T)
}

// StorageMerger defines a method that forces the merging of data inserted
// into the storage.
type StorageMerger interface {
	ForceMerge(t *testing.T)
}

// PrometheusWriteQuerier encompasses the methods for writing, flushing and
// querying the data.
type PrometheusWriteQuerier interface {
	PrometheusWriter
	PrometheusQuerier
	StorageFlusher
	StorageMerger
}

// QueryOpts contains various params used for querying or ingesting data
type QueryOpts struct {
	Tenant         string
	Timeout        string
	Start          string
	End            string
	Time           string
	Step           string
	ExtraFilters   []string
	ExtraLabels    []string
	Trace          string
	ReduceMemUsage string
}

func (qos *QueryOpts) asURLValues() url.Values {
	uv := make(url.Values)
	addNonEmpty := func(name string, values ...string) {
		for _, value := range values {
			if len(value) == 0 {
				continue
			}
			uv.Add(name, value)
		}
	}
	addNonEmpty("start", qos.Start)
	addNonEmpty("end", qos.End)
	addNonEmpty("time", qos.Time)
	addNonEmpty("step", qos.Step)
	addNonEmpty("timeout", qos.Timeout)
	addNonEmpty("extra_label", qos.ExtraLabels...)
	addNonEmpty("extra_filters", qos.ExtraFilters...)
	addNonEmpty("trace", qos.Trace)
	addNonEmpty("reduce_mem_usage", qos.ReduceMemUsage)

	return uv
}

// getTenant returns tenant with optional default value
func (qos *QueryOpts) getTenant() string {
	if qos.Tenant == "" {
		return "0"
	}
	return qos.Tenant
}

// PrometheusAPIV1QueryResponse is an inmemory representation of the
// /prometheus/api/v1/query or /prometheus/api/v1/query_range response.
type PrometheusAPIV1QueryResponse struct {
	Status    string
	Data      *QueryData
	ErrorType string
	Error     string
	IsPartial bool
}

// NewPrometheusAPIV1QueryResponse is a test helper function that creates a new
// instance of PrometheusAPIV1QueryResponse by unmarshalling a json string.
func NewPrometheusAPIV1QueryResponse(t *testing.T, s string) *PrometheusAPIV1QueryResponse {
	t.Helper()

	res := &PrometheusAPIV1QueryResponse{}
	if err := json.Unmarshal([]byte(s), res); err != nil {
		t.Fatalf("could not unmarshal query response data=\n%s\n: %v", string(s), err)
	}
	return res
}

// Sort performs data.Result sort by metric labels
func (pqr *PrometheusAPIV1QueryResponse) Sort() {
	if pqr.Data == nil {
		return
	}

	sort.Slice(pqr.Data.Result, func(i, j int) bool {
		leftS := make([]string, 0, len(pqr.Data.Result[i].Metric))
		rightS := make([]string, 0, len(pqr.Data.Result[j].Metric))
		for k, v := range pqr.Data.Result[i].Metric {
			leftS = append(leftS, fmt.Sprintf("%s=%s", k, v))
		}
		for k, v := range pqr.Data.Result[j].Metric {
			rightS = append(rightS, fmt.Sprintf("%s=%s", k, v))

		}
		sort.Strings(leftS)
		sort.Strings(rightS)
		return strings.Join(leftS, ",") < strings.Join(rightS, ",")
	})

	for _, result := range pqr.Data.Result {
		sort.Slice(result.Samples, func(i, j int) bool {
			a := result.Samples[i]
			b := result.Samples[j]
			if a.Timestamp != b.Timestamp {
				return a.Timestamp < b.Timestamp
			}

			// Put NaNs at the end of the slice.
			if math.IsNaN(a.Value) {
				return false
			}
			if math.IsNaN(b.Value) {
				return true
			}

			return a.Value < b.Value
		})
	}
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
	return &Sample{parsedTime.UnixMilli(), value}
}

// UnmarshalJSON populates the sample fields from a JSON string.
func (s *Sample) UnmarshalJSON(b []byte) error {
	var (
		ts float64
		v  string
	)
	raw := []any{&ts, &v}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if got, want := len(raw), 2; got != want {
		return fmt.Errorf("unexpected number of fields: got %d, want %d (raw sample: %s)", got, want, string(b))
	}
	s.Timestamp = int64(ts * 1000)
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
	Trace     *Trace
	ErrorType string
	Error     string
}

// NewPrometheusAPIV1SeriesResponse is a test helper function that creates a new
// instance of PrometheusAPIV1SeriesResponse by unmarshalling a json string.
func NewPrometheusAPIV1SeriesResponse(t *testing.T, s string) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	res := &PrometheusAPIV1SeriesResponse{}
	if err := json.Unmarshal([]byte(s), res); err != nil {
		t.Fatalf("could not unmarshal series response data:\n%s\n err: %v", string(s), err)
	}
	return res
}

// Sort sorts the response data.
func (r *PrometheusAPIV1SeriesResponse) Sort() *PrometheusAPIV1SeriesResponse {
	str := func(m map[string]string) string {
		s := []string{}
		for k, v := range m {
			s = append(s, k+v)
		}
		slices.Sort(s)
		return strings.Join(s, "")
	}

	slices.SortFunc(r.Data, func(a, b map[string]string) int {
		return strings.Compare(str(a), str(b))
	})

	return r
}

// Trace provides the description and the duration of some unit of work that has
// been performed during the request processing.
type Trace struct {
	DurationMsec float64 `json:"duration_msec"`
	Message      string
	Children     []*Trace
}

// String returns string representation of the trace.
func (t *Trace) String() string {
	return t.stringWithIndent("")
}

func (t *Trace) stringWithIndent(indent string) string {
	s := indent + fmt.Sprintf("{duration_msec: %.3f msg: %q", t.DurationMsec, t.Message)
	if len(t.Children) > 0 {
		s += " children: ["
		for _, c := range t.Children {
			s += "\n" + c.stringWithIndent(indent+" ")
		}
		s += "]"
	}
	return s + "}"
}

// Contains counts how many trace messages contain substring s.
func (t *Trace) Contains(s string) int {
	var times int
	if strings.Contains(t.Message, s) {
		times++
	}

	for _, c := range t.Children {
		times += c.Contains(s)
	}
	return times
}

// MetricNamesStatsResponse is an inmemory representation of the
// /api/v1/status/metric_names_stats API response
type MetricNamesStatsResponse struct {
	Records []MetricNamesStatsRecord
}

// MetricNamesStatsRecord is a record item for MetricNamesStatsResponse
type MetricNamesStatsRecord struct {
	MetricName         string
	QueryRequestsCount uint64
}
