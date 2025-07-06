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

	"github.com/golang/snappy"

	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
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
	graphiteWriteAddr                  string
	openTSDBHTTPURL                    string
	prometheusAPIV1ImportPrometheusURL string
	prometheusAPIV1WriteURL            string

	// vmselect URLs.
	prometheusAPIV1ExportURL       string
	prometheusAPIV1ExportNativeURL string
	prometheusAPIV1QueryURL        string
	prometheusAPIV1QueryRangeURL   string
	prometheusAPIV1SeriesURL       string
}

// StartVmsingle starts an instance of vmsingle with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr).
func StartVmsingle(instance string, flags []string, cli *Client) (*Vmsingle, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/victoria-metrics", flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath":    fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":     "127.0.0.1:0",
			"-graphiteListenAddr": ":0",
			"-opentsdbListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			graphiteListenAddrRE,
			openTSDBListenAddrRE,
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
		graphiteWriteAddr:                  stderrExtracts[2],
		openTSDBHTTPURL:                    fmt.Sprintf("http://%s", stderrExtracts[3]),
		prometheusAPIV1ImportPrometheusURL: fmt.Sprintf("http://%s/prometheus/api/v1/import/prometheus", stderrExtracts[1]),
		prometheusAPIV1WriteURL:            fmt.Sprintf("http://%s/prometheus/api/v1/write", stderrExtracts[1]),
		prometheusAPIV1ExportURL:           fmt.Sprintf("http://%s/prometheus/api/v1/export", stderrExtracts[1]),
		prometheusAPIV1ExportNativeURL:     fmt.Sprintf("http://%s/prometheus/api/v1/export/native", stderrExtracts[1]),
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
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#influxwrite
func (app *Vmsingle) InfluxWrite(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	data := []byte(strings.Join(records, "\n"))

	url := app.influxLineWriteURL
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}

	_, statusCode := app.cli.Post(t, url, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// GraphiteWrite is a test helper function that sends a collection of records
// to graphiteListenAddr port.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting
func (app *Vmsingle) GraphiteWrite(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()
	app.cli.Write(t, app.graphiteWriteAddr, records)
}

// PrometheusAPIV1ImportCSV is a test helper function that inserts a collection
// of records in CSV format for the given tenant by sending an HTTP POST
// request to /api/v1/import/csv vmsingle endpoint.
//
// See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-import-csv-data
func (app *Vmsingle) PrometheusAPIV1ImportCSV(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/api/v1/import/csv", app.httpListenAddr)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	_, statusCode := app.cli.Post(t, url, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// PrometheusAPIV1ImportNative is a test helper function that inserts a collection
// of records in native format for the given tenant by sending an HTTP POST
// request to /api/v1/import/native vmsingle endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-native-format
func (app *Vmsingle) PrometheusAPIV1ImportNative(t *testing.T, data []byte, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/api/v1/import/native", app.httpListenAddr)
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	_, statusCode := app.cli.Post(t, url, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// OpenTSDBAPIPut is a test helper function that inserts a collection of
// records in OpenTSDB format for the given tenant by sending an HTTP POST
// request to /api/put vmsingle endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/opentsdb/#sending-data-via-http
func (app *Vmsingle) OpenTSDBAPIPut(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	// add extra label
	url := app.openTSDBHTTPURL + "/api/put"
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte("[" + strings.Join(records, ",") + "]")
	_, statusCode := app.cli.Post(t, url, "text/plain", data)
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
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1importprometheus
func (app *Vmsingle) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	// add extra label
	url := app.prometheusAPIV1ImportPrometheusURL
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}

	data := []byte(strings.Join(records, "\n"))
	_, statusCode := app.cli.Post(t, url, "text/plain", data)
	if statusCode != http.StatusNoContent {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
	}
}

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a HTTP POST request to
// /prometheus/api/v1/export vmsingle endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1export
func (app *Vmsingle) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) *PrometheusAPIV1QueryResponse {
	t.Helper()
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1ExportURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res)
}

// PrometheusAPIV1ExportNative is a test helper function that performs the export of
// raw samples in native binary format by sending an HTTP POST request to
// /prometheus/api/v1/export/native vmselect endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1exportnative
func (app *Vmsingle) PrometheusAPIV1ExportNative(t *testing.T, query string, opts QueryOpts) []byte {
	t.Helper()

	t.Helper()
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1ExportNativeURL, values)
	return []byte(res)
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a HTTP POST request to /prometheus/api/v1/query
// vmsingle endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query
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
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1query_range
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
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1series
func (app *Vmsingle) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) *PrometheusAPIV1SeriesResponse {
	t.Helper()

	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	res, _ := app.cli.PostForm(t, app.prometheusAPIV1SeriesURL, values)
	return NewPrometheusAPIV1SeriesResponse(t, res)
}

// GraphiteMetricsIndex sends a query to a /metrics/index.json
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#metrics-api
func (app *Vmsingle) GraphiteMetricsIndex(t *testing.T, _ QueryOpts) GraphiteMetricsIndexResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/metrics/index.json", app.httpListenAddr)
	res, statusCode := app.cli.Get(t, seriesURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var index GraphiteMetricsIndexResponse
	if err := json.Unmarshal([]byte(res), &index); err != nil {
		t.Fatalf("could not unmarshal metrics index response data:\n%s\n err: %v", res, err)
	}
	return index
}

// APIV1StatusMetricNamesStats sends a query to a /api/v1/status/metric_names_stats endpoint
// and returns the statistics response for given params.
//
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#track-ingested-metrics-usage
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
func (app *Vmsingle) SnapshotCreate(t *testing.T) *SnapshotCreateResponse {
	t.Helper()

	data, statusCode := app.cli.Post(t, app.SnapshotCreateURL(), "", nil)
	if got, want := statusCode, http.StatusOK; got != want {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", got, want, data)
	}

	var res SnapshotCreateResponse
	if err := json.Unmarshal([]byte(data), &res); err != nil {
		t.Fatalf("could not unmarshal snapshot create response: data=%q, err: %v", data, err)
	}

	return &res
}

// SnapshotCreateURL returns the URL for creating snapshots.
func (app *Vmsingle) SnapshotCreateURL() string {
	return fmt.Sprintf("http://%s/snapshot/create", app.httpListenAddr)
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
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
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-work-with-snapshots
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

// APIV1StatusTSDB sends a query to a /prometheus/api/v1/status/tsdb
// //
// See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#tsdb-stats
func (app *Vmsingle) APIV1StatusTSDB(t *testing.T, matchQuery string, date string, topN string, opts QueryOpts) TSDBStatusResponse {
	t.Helper()

	seriesURL := fmt.Sprintf("http://%s/prometheus/api/v1/status/tsdb", app.httpListenAddr)
	values := opts.asURLValues()
	addNonEmpty := func(name, value string) {
		if len(value) == 0 {
			return
		}
		values.Add(name, value)
	}
	addNonEmpty("match[]", matchQuery)
	addNonEmpty("topN", topN)
	addNonEmpty("date", date)

	res, statusCode := app.cli.PostForm(t, seriesURL, values)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d, resp text=%q", statusCode, http.StatusOK, res)
	}

	var status TSDBStatusResponse
	if err := json.Unmarshal([]byte(res), &status); err != nil {
		t.Fatalf("could not unmarshal tsdb status response data:\n%s\n err: %v", res, err)
	}
	status.Sort()
	return status
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
