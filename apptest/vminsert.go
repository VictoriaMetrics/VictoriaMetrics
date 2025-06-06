package apptest

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang/snappy"

	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// Vminsert holds the state of a vminsert app and provides vminsert-specific
// functions.
type Vminsert struct {
	*app
	*ServesMetrics

	httpListenAddr          string
	clusternativeListenAddr string
	graphiteListenAddr      string
	openTSDBListenAddr      string

	cli *Client
}

// storageNodes returns the storage node addresses passed to vminsert via
// -storageNode command line flag.
func storageNodes(flags []string) []string {
	for _, flag := range flags {
		if storageNodes, found := strings.CutPrefix(flag, "-storageNode="); found {
			return strings.Split(storageNodes, ",")
		}
	}
	return nil
}

// StartVminsert starts an instance of vminsert with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVminsert(instance string, flags []string, cli *Client) (*Vminsert, error) {
	extractREs := []*regexp.Regexp{
		httpListenAddrRE,
		vminsertClusterNativeAddrRE,
		graphiteListenAddrRE,
		openTSDBListenAddrRE,
	}
	// Add storateNode REs to block until vminsert establishes connections with
	// all storage nodes. The extracted values are unused.
	for _, sn := range storageNodes(flags) {
		logRecord := fmt.Sprintf("successfully dialed -storageNode=\"%s\"", sn)
		extractREs = append(extractREs, regexp.MustCompile(logRecord))
	}

	app, stderrExtracts, err := startApp(instance, "../../bin/vminsert", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":          "127.0.0.1:0",
			"-clusternativeListenAddr": "127.0.0.1:0",
			"-graphiteListenAddr":      ":0",
			"-opentsdbListenAddr":      "127.0.0.1:0",
		},
		extractREs: extractREs,
	})
	if err != nil {
		return nil, err
	}

	return &Vminsert{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[0]),
			cli:        cli,
		},
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
		graphiteListenAddr:      stderrExtracts[2],
		openTSDBListenAddr:      stderrExtracts[3],
		cli:                     cli,
	}, nil
}

// ClusternativeListenAddr returns the address at which the vminsert process is
// listening for connections from other vminsert apps.
func (app *Vminsert) ClusternativeListenAddr() string {
	return app.clusternativeListenAddr
}

// HTTPAddr returns the address at which the vminsert process is
// listening for incoming HTTP requests.
func (app *Vminsert) HTTPAddr() string {
	return app.httpListenAddr
}

// InfluxWrite is a test helper function that inserts a
// collection of records in Influx line format by sending a HTTP
// POST request to /influx/write vmsingle endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#influxwrite
func (app *Vminsert) InfluxWrite(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/influx/write", app.httpListenAddr, opts.getTenant())
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}

	data := []byte(strings.Join(records, "\n"))
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "text/plain", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// GraphiteWrite is a test helper function that sends a
// collection of records to graphiteListenAddr port.
//
// See https://docs.victoriametrics.com/victoriametrics/integrations/graphite/#ingesting
func (app *Vminsert) GraphiteWrite(t *testing.T, records []string, _ QueryOpts) {
	t.Helper()
	app.cli.Write(t, app.graphiteListenAddr, records)
}

// PrometheusAPIV1ImportCSV is a test helper function that inserts a collection
// of records in CSV format for the given tenant by sending an HTTP POST
// request to prometheus/api/v1/import/csv vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (app *Vminsert) PrometheusAPIV1ImportCSV(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/csv", app.httpListenAddr, opts.getTenant())
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "text/plain", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1ImportNative is a test helper function that inserts a collection
// of records in Native format for the given tenant by sending an HTTP POST
// request to prometheus/api/v1/import/native vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (app *Vminsert) PrometheusAPIV1ImportNative(t *testing.T, data []byte, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/native", app.httpListenAddr, opts.getTenant())
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	app.sendBlocking(t, 1, func() {
		_, statusCode := app.cli.Post(t, url, "text/plain", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// OpenTSDBAPIPut is a test helper function that inserts a collection of
// records in OpenTSDB format for the given tenant by sending an HTTP POST
// request to /opentsdb/api/put vminsert endpoint.
//
// See https://docs.victoriametrics.com/cluster-victoriametrics/#url-format
func (app *Vminsert) OpenTSDBAPIPut(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/opentsdb/api/put", app.openTSDBListenAddr, opts.getTenant())
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte("[" + strings.Join(records, ",") + "]")
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "application/json", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1Write is a test helper function that inserts a
// collection of records in Prometheus remote-write format by sending a HTTP
// POST request to /prometheus/api/v1/write vminsert endpoint.
func (app *Vminsert) PrometheusAPIV1Write(t *testing.T, records []pb.TimeSeries, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/write", app.httpListenAddr, opts.getTenant())
	wr := pb.WriteRequest{Timeseries: records}
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "application/x-protobuf", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// PrometheusAPIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to
// /prometheus/api/v1/import/prometheus vminsert endpoint.
//
// See https://docs.victoriametrics.com/victoriametrics/url-examples/#apiv1importprometheus
func (app *Vminsert) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/prometheus", app.httpListenAddr, opts.getTenant())
	uv := opts.asURLValues()
	uvs := uv.Encode()
	if len(uvs) > 0 {
		url += "?" + uvs
	}
	data := []byte(strings.Join(records, "\n"))
	app.sendBlocking(t, len(records), func() {
		_, statusCode := app.cli.Post(t, url, "text/plain", data)
		if statusCode != http.StatusNoContent {
			t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusNoContent)
		}
	})
}

// String returns the string representation of the vminsert app state.
func (app *Vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}

// sendBlocking sends the data to vmstorage by executing `send` function and
// waits until the data is actually sent.
//
// vminsert does not send the data immediately. It first puts the data into a
// buffer. Then a background goroutine takes the data from the buffer sends it
// to the vmstorage. This happens every 200ms.
//
// Waiting is implemented a retrieving the value of `vm_rpc_rows_sent_total`
// metric and checking whether it is equal or greater than the wanted value.
// If it is, then the data has been sent to vmstorage.
//
// Unreliable if the records are inserted concurrently.
// TODO(rtm0): Put sending and waiting into a critical section to make reliable?
func (app *Vminsert) sendBlocking(t *testing.T, numRecordsToSend int, send func()) {
	t.Helper()

	send()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)
	wantRowsSentCount := app.rpcRowsSentTotal(t) + numRecordsToSend
	for range retries {
		if app.rpcRowsSentTotal(t) >= wantRowsSentCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted rows to be sent to vmstorage")
}

// rpcRowsSentTotal retrieves the values of all vminsert
// `vm_rpc_rows_sent_total` metrics (there will be one for each vmstorage) and
// returns their integer sum.
func (app *Vminsert) rpcRowsSentTotal(t *testing.T) int {
	total := 0.0
	for _, v := range app.GetMetricsByPrefix(t, "vm_rpc_rows_sent_total") {
		total += v
	}
	return int(total)
}
