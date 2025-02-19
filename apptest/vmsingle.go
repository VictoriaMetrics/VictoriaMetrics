package apptest

import (
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

		forceFlushURL:                      fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
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
		t.Fatalf("failed to force flush data")
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
		t.Fatalf("failed to write influx data, status code: %d", statusCode)
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
		t.Fatalf("failed to write prometheus data, status code: %d", statusCode)
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
		t.Fatalf("failed to import prometheus data, status code: %d", statusCode)
	}
}

// PrometheusAPIV1Export is a test helper function that performs the export of
// raw samples in JSON line format by sending a HTTP POST request to
// /prometheus/api/v1/export vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1export
func (app *Vmsingle) PrometheusAPIV1Export(t *testing.T, query string, opts QueryOpts) (*PrometheusAPIV1QueryResponse, int) {
	t.Helper()
	values := opts.asURLValues()
	values.Add("match[]", query)
	values.Add("format", "promapi")

	res, statusCode := app.cli.PostForm(t, app.prometheusAPIV1ExportURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res), statusCode
}

// PrometheusAPIV1Query is a test helper function that performs PromQL/MetricsQL
// instant query by sending a HTTP POST request to /prometheus/api/v1/query
// vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query
func (app *Vmsingle) PrometheusAPIV1Query(t *testing.T, query string, opts QueryOpts) (*PrometheusAPIV1QueryResponse, int) {
	t.Helper()

	values := opts.asURLValues()
	values.Add("query", query)
	res, statusCode := app.cli.PostForm(t, app.prometheusAPIV1QueryURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res), statusCode
}

// PrometheusAPIV1QueryRange is a test helper function that performs
// PromQL/MetricsQL range query by sending a HTTP POST request to
// /prometheus/api/v1/query_range vmsingle endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1query_range
func (app *Vmsingle) PrometheusAPIV1QueryRange(t *testing.T, query string, opts QueryOpts) (*PrometheusAPIV1QueryResponse, int) {
	t.Helper()

	values := opts.asURLValues()
	values.Add("query", query)

	res, statusCode := app.cli.PostForm(t, app.prometheusAPIV1QueryRangeURL, values)
	return NewPrometheusAPIV1QueryResponse(t, res), statusCode
}

// PrometheusAPIV1Series sends a query to a /prometheus/api/v1/series endpoint
// and returns the list of time series that match the query.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1series
func (app *Vmsingle) PrometheusAPIV1Series(t *testing.T, matchQuery string, opts QueryOpts) (*PrometheusAPIV1SeriesResponse, int) {
	t.Helper()

	values := opts.asURLValues()
	values.Add("match[]", matchQuery)

	res, statusCode := app.cli.PostForm(t, app.prometheusAPIV1SeriesURL, values)
	return NewPrometheusAPIV1SeriesResponse(t, res), statusCode
}

// String returns the string representation of the vmsingle app state.
func (app *Vmsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
