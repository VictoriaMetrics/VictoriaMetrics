package apptest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/golang/snappy"
)

// Vmsingle holds the state of a vmsingle app and provides vmsingle-specific
// functions.
type Vmsingle struct {
	*app
	*ServesMetrics

	storageDataPath string
	httpListenAddr  string

	// vmstorage URLs.
	forceFlushURL string
	forceMergeURL string

	// vminsert URLs.
	influxLineWriteURL                 string
	prometheusAPIV1ImportPrometheusURL string
	prometheusAPIV1WriteURL            string

	// vmselect URLs.
	prometheusAPIV1ExportURL     string
	prometheusAPIV1QueryURL      string
	prometheusAPIV1QueryRangeURL string
	prometheusAPIV1SeriesURL     string
}

// StartVmsingle starts an instance of vmsingle with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr).
func StartVmsingle(instance string, flags []string, cli *Client) (*Vmsingle, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/victoria-metrics", flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":  "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
		},
	})
	if err != nil {
		return nil, err
	}

	return &Vmsingle{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],

		forceFlushURL: fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
		forceMergeURL: fmt.Sprintf("http://%s/internal/force_merge", stderrExtracts[1]),

		influxLineWriteURL:                 fmt.Sprintf("http://%s/influx/write", stderrExtracts[1]),
		prometheusAPIV1ImportPrometheusURL: fmt.Sprintf("http://%s/prometheus/api/v1/import/prometheus", stderrExtracts[1]),
		prometheusAPIV1WriteURL:            fmt.Sprintf("http://%s/prometheus/api/v1/write", stderrExtracts[1]),
		prometheusAPIV1ExportURL:           fmt.Sprintf("http://%s/prometheus/api/v1/export", stderrExtracts[1]),
		prometheusAPIV1QueryURL:            fmt.Sprintf("http://%s/prometheus/api/v1/query", stderrExtracts[1]),
		prometheusAPIV1QueryRangeURL:       fmt.Sprintf("http://%s/prometheus/api/v1/query_range", stderrExtracts[1]),
		prometheusAPIV1SeriesURL:           fmt.Sprintf("http://%s/prometheus/api/v1/series", stderrExtracts[1]),
	}, nil
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (app *Vmsingle) ForceFlush(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceFlushURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (app *Vmsingle) ForceMerge(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceMergeURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// InfluxWrite is a test helper function that inserts a
// collection of records in Influx line format by sending a HTTP
// POST request to /influx/write vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#influxwrite
func (app *Vmsingle) InfluxWrite(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))
	_, statusCode := app.cli.Post(t, app.influxLineWriteURL, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// PrometheusAPIV1Write is a test helper function that inserts a
// collection of records in Prometheus remote-write format by sending a HTTP
// POST request to /prometheus/api/v1/write vmsingle endpoint.
func (app *Vmsingle) PrometheusAPIV1Write(t *testing.T, records []pb.TimeSeries, _ QueryOpts) {
	t.Helper()

	wr := pb.WriteRequest{Timeseries: records}
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	_, statusCode := app.cli.Post(t, app.prometheusAPIV1WriteURL, "application/x-protobuf", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// PrometheusAPIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format by sending a HTTP
// POST request to /prometheus/api/v1/import/prometheus vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1importprometheus
func (app *Vmsingle) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))
	_, statusCode := app.cli.Post(t, app.prometheusAPIV1ImportPrometheusURL, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a HTTP POST request to
// /prometheus/api/v1/export vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1export
func (app *Vmsingle) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1ExportURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a HTTP POST request to /prometheus/api/v1/query
// vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query
func (app *Vmsingle) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("query", query)
	res, _ := app.cli.PostForm(t, app.prometheusAPIV1QueryURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a HTTP POST request to
// /prometheus/api/v1/query_range vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query_range
func (app *Vmsingle) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("query", query)

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1QueryRangeURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1Series sends a query to a /prometheus/api/v1/series endpoint
// and returns the list of time series that match the query.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1series
func (app *Vmsingle) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1SeriesURL, values)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// APIV1StatusMetricNamesStats sends a query to a /api/v1/status/metric_names_stats endpoint
// and returns the statistics response for given params.
//
// See https://docs.victoriametrics.com/#track-ingested-metrics-usage
func (app *Vmsingle) APIV1StatusMetricNamesStats(t *testing.T, limit, le, matchPattern string, opts QueryOpts) MetricNamesStatsResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("limit", limit)
	values.Add("le", le)
	values.Add("match_pattern", matchPattern)
	queryURL := fmt.Sprintf("http://%s/api/v1/status/metric_names_stats", app.httpListenAddr)

	res, statusCode := app.cli.PostForm(t, queryURL, values)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}
	var resp MetricNamesStatsResponse
	if err := json.Unmarshal([]byte(res), &resp); err != nil {
		t.Fatalf("could not unmarshal metric names stats response data:\n%s\n err: %v", res, err)
	}
	return resp
}

// APIV1AdminStatusMetricNamesStatsReset sends a query to a /api/v1/admin/status/metric_names_stats/reset endpoint
//
// See https://docs.victoriametrics.com/#Trackingestedmetricsusage
func (app *Vmsingle) APIV1AdminStatusMetricNamesStatsReset(t *testing.T, opts QueryOpts) {
	t.Helper()

	values := opts.asURLValues()
	queryURL := fmt.Sprintf("http://%s/api/v1/admin/status/metric_names_stats/reset", app.httpListenAddr)

	res, statusCode := app.cli.PostForm(t, queryURL, values)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusNoContent, res)
	}
}

// SnapshotCreate creates a database snapshot by sending a query to the
// /snapshot/create endpoint.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotCreate(t *testing.T) *SnapshotCreateResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/create", app.httpListenAddr)
	data, statusCode := app.cli.Post(t, queryURL, "", nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotCreateResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// APIV1AdminTSDBSnapshot creates a database snapshot by sending a query to the
// /api/v1/admin/tsdb/snapshot endpoint.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot.
func (app *Vmsingle) APIV1AdminTSDBSnapshot(t *testing.T) *APIV1AdminTSDBSnapshotResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/api/v1/admin/tsdb/snapshot", app.httpListenAddr)
	data, statusCode := app.cli.Post(t, queryURL, "", nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res APIV1AdminTSDBSnapshotResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal prometheus snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotList lists existing database snapshots by sending a query to the
// /snapshot/list endpoint.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotList(t *testing.T) *SnapshotListResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/list", app.httpListenAddr)
	data, statusCode := app.cli.Get(t, queryURL)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotListResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot list response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDelete deletes a snapshot by sending a query to the
// /snapshot/delete endpoint.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotDelete(t *testing.T, snapshotName string) *SnapshotDeleteResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/delete?snapshot=%s", app.httpListenAddr, snapshotName)
	data, statusCode := app.cli.Delete(t, queryURL)
	wantStatusCodes := map[int]bool{
		http.StatusOK:                  true,
		http.StatusInternalServerError: true,
	}
	if !wantStatusCodes[statusCode] {
		t.Fatalf("unexpected status code: got %d, want %v, resp text=%q", statusCode, wantStatusCodes, data)
	}

	var res SnapshotDeleteResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotDeleteAll deletes all snapshots by sending a query to the
// /snapshot/delete_all endpoint.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotDeleteAll(t *testing.T) *SnapshotDeleteAllResponse {
	t.Helper()

	queryURL := fmt.Sprintf("http://%s/snapshot/delete_all", app.httpListenAddr)
	data, statusCode := app.cli.Get(t, queryURL)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotDeleteAllResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot delete all response: data=%q, err: %v", data, err)
	}

	return &res
}

// HTTPAddr returns the address at which the vmstorage process is listening
// for http connections.
func (app *Vmsingle) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vmsingle app state.
func (app *Vmsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
