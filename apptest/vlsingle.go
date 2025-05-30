package apptest

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

// Vlsingle holds the state of a vlsingle app and provides vlsingle-specific
// functions.
type Vlsingle struct {
	*app
	*ServesMetrics

	storageDataPath string
	httpListenAddr  string

	forceFlushURL string
	forceMergeURL string

	jaegerAPIServicesURL   string
	jaegerAPIOperationsURL string
	jaegerAPITracesURL     string
	jaegerAPITraceURL      string

	otlpTracesURL string
}

// StartVlsingle starts an instance of vlsingle with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr).
func StartVlsingle(instance string, flags []string, cli *Client) (*Vlsingle, error) {
	app, stderrExtracts, err := startApp(instance, "../../bin/victoria-logs", flags, &appOptions{
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

	return &Vlsingle{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],

		forceFlushURL: fmt.Sprintf("http://%s/internal/force_flush", stderrExtracts[1]),
		forceMergeURL: fmt.Sprintf("http://%s/internal/force_merge", stderrExtracts[1]),

		jaegerAPIServicesURL:   fmt.Sprintf("http://%s/select/jaeger/api/services", stderrExtracts[1]),
		jaegerAPIOperationsURL: fmt.Sprintf("http://%s/select/jaeger/api/services/%%s/operations", stderrExtracts[1]),
		jaegerAPITracesURL:     fmt.Sprintf("http://%s/select/jaeger/api/traces", stderrExtracts[1]),
		jaegerAPITraceURL:      fmt.Sprintf("http://%s/select/jaeger/api/traces/%%s", stderrExtracts[1]),

		otlpTracesURL: fmt.Sprintf("http://%s/insert/opentelemetry/v1/traces", stderrExtracts[1]),
	}, nil
}

// ForceFlush is a test helper function that forces the flushing of inserted
// data, so it becomes available for searching immediately.
func (app *Vlsingle) ForceFlush(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceFlushURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// ForceMerge is a test helper function that forces the merging of parts.
func (app *Vlsingle) ForceMerge(t *testing.T) {
	t.Helper()

	_, statusCode := app.cli.Get(t, app.forceMergeURL)
	if statusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", statusCode, http.StatusOK)
	}
}

// JaegerAPIServices is a test helper function that queries for service list
// by sending an HTTP GET request to /select/jaeger/api/services
// vlsingle endpoint.
func (app *Vlsingle) JaegerAPIServices(t *testing.T, opts QueryOpts) *JaegerAPIServicesResponse {
	t.Helper()

	res, _ := app.cli.Get(t, app.jaegerAPIServicesURL+"?"+opts.asURLValues().Encode())
	return NewJaegerAPIServicesResponse(t, res)
}

// JaegerAPIOperations is a test helper function that queries for operation list of a service
// by sending an HTTP GET request to /select/jaeger/api/services/<service_name>/operations
// vlsingle endpoint.
func (app *Vlsingle) JaegerAPIOperations(t *testing.T, serviceName string, opts QueryOpts) *JaegerAPIOperationsResponse {
	t.Helper()

	url := fmt.Sprintf(app.jaegerAPIOperationsURL, serviceName) + "?" + opts.asURLValues().Encode()
	res, _ := app.cli.Get(t, url)
	return NewJaegerAPIOperationsResponse(t, res)
}

// JaegerAPITraces is a test helper function that queries for traces with filter conditions
// by sending an HTTP GET request to /select/jaeger/api/traces vlsingle endpoint.
func (app *Vlsingle) JaegerAPITraces(t *testing.T, param JaegerQueryParam, opts QueryOpts) *JaegerAPITracesResponse {
	t.Helper()

	paramsEnc := "?"
	values := opts.asURLValues()
	if len(values) > 0 {
		paramsEnc += values.Encode() + "&"
	}
	uv := param.asURLValues()
	if len(uv) > 0 {
		paramsEnc += uv.Encode()
	}
	res, _ := app.cli.Get(t, app.jaegerAPITracesURL+paramsEnc)
	return NewJaegerAPITracesResponse(t, res)
}

// JaegerAPITrace is a test helper function that queries for a single trace with trace_id
// by sending an HTTP GET request to /select/jaeger/api/traces/<trace_id>
// vlsingle endpoint.
func (app *Vlsingle) JaegerAPITrace(t *testing.T, traceID string, opts QueryOpts) *JaegerAPITraceResponse {
	t.Helper()

	url := fmt.Sprintf(app.jaegerAPITraceURL, traceID)
	res, _ := app.cli.Get(t, url+"?"+opts.asURLValues().Encode())
	return NewJaegerAPITraceResponse(t, res)
}

// JaegerAPIDependencies is a test helper function that queries for the dependencies.
// This method is not implemented in Vlsingle and this test is no-op for now.
func (app *Vlsingle) JaegerAPIDependencies(_ *testing.T, _ QueryOpts) {}

// OTLPExportTraces is a test helper function that exports OTLP trace data
// by sending an HTTP POST request to /insert/opentelemetry/v1/traces
// vlsingle endpoint.
func (app *Vlsingle) OTLPExportTraces(t *testing.T, request *pb.ExportTraceServiceRequest, _ QueryOpts) {
	t.Helper()

	pbData := request.MarshalProtobuf(nil)
	_, code := app.cli.Post(t, app.otlpTracesURL, "application/x-protobuf", pbData)
	if code != 200 {
		t.Fatalf("got %d, expected 200", code)
	}
}

// HTTPAddr returns the address at which the vmstorage process is listening
// for http connections.
func (app *Vlsingle) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vmsingle app state.
func (app *Vlsingle) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr}...)
}
