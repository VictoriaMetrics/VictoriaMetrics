package apptest

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	pb "github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/golang/snappy"
)

// Vminsert holds the state of a vminsert app and provides vminsert-specific
// functions.
type Vminsert struct {
	*app
	*ServesMetrics

	httpListenAddr string
	cli            *Client
}

// StartVminsert starts an instance of vminsert with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVminsert(instance string, flags []string, cli *Client) (*Vminsert, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/vminsert", flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			httpListenAddrRE,
		},
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
		httpListenAddr: stderrExtracts[0],
		cli:            cli,
	}, nil
}

// PrometheusAPIV1Write is a test helper function that inserts a
// collection of records in Prometheus remote-write format by sending a HTTP
// POST request to /prometheus/api/v1/write vminsert endpoint.
func (app *Vminsert) PrometheusAPIV1Write(t *testing.T, records []pb.TimeSeries, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/write", app.httpListenAddr, opts.Tenant)
	wr := pb.WriteRequest{Timeseries: records}
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	app.cli.Post(t, url, "application/x-protobuf", data, http.StatusNoContent)
}

// PrometheusAPIV1ImportPrometheus is a test helper function that inserts a
// collection of records in Prometheus text exposition format for the given
// tenant by sending a HTTP POST request to
// /prometheus/api/v1/import/prometheus vminsert endpoint.
//
// See https://docs.victoriametrics.com/url-examples/#apiv1importprometheus
func (app *Vminsert) PrometheusAPIV1ImportPrometheus(t *testing.T, records []string, opts QueryOpts) {
	t.Helper()

	url := fmt.Sprintf("http://%s/insert/%s/prometheus/api/v1/import/prometheus", app.httpListenAddr, opts.Tenant)
	data := []byte(strings.Join(records, "\n"))
	app.cli.Post(t, url, "text/plain", data, http.StatusNoContent)
}

// String returns the string representation of the vminsert app state.
func (app *Vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}
