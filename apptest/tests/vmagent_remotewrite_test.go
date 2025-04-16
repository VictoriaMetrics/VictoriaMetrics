package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// TestSingleVMAgentZstdRemoteWrite verifies that vmagent can successfully perform
// a remote write to vmsingle using VM protocol (zstd).
func TestSingleVMAgentZstdRemoteWrite(t *testing.T) {
	testSingleVMAgentRemoteWrite(t, false)
}

// TestSingleVMAgentSnappyRemoteWrite verifies that vmagent can successfully perform
// a remote write to vmsingle using Prometheus protocol (snappy).
func TestSingleVMAgentSnappyRemoteWrite(t *testing.T) {
	testSingleVMAgentRemoteWrite(t, true)
}

func testSingleVMAgentRemoteWrite(t *testing.T, forcePromProto bool) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartDefaultVmsingle()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		fmt.Sprintf(`-remoteWrite.forcePromProto=%v`, forcePromProto),
		fmt.Sprintf(`-remoteWrite.url=http://%s/api/v1/write`, vmsingle.HTTPAddr()),
	}, ``)

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmsingle.ForceFlush(t)

	tc.Assert(&at.AssertOptions{
		Msg: `unexpected metrics stored on vmagent remote write`,
		Got: func() any {
			return vmsingle.PrometheusAPIV1Series(t, `{__name__="foo_bar"}`, at.QueryOpts{
				Start: "2022-05-10T00:00:00Z",
				End:   "2022-05-10T23:59:59Z",
			}).Sort()
		},
		Want: &at.PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   []map[string]string{{"__name__": "foo_bar"}},
		},
	})
}

// TestSingleVMAgentUnsupportedMediaTypeDropIfSnappy verifies that the remote write process:
// - Starts with Prometheus remote write protocol using `snappy`.
// - Does not retry `snappy`-encoded requests if they fail; instead, they are dropped.
func TestSingleVMAgentUnsupportedMediaTypeDropIfSnappy(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var remoteWriteContentEncodingsMux sync.Mutex
	var remoteWriteContentEncodings []string
	// remoteWriteSrv is a stub HTTP server simulate a remote write endpoint with the following behavior:
	// - Fail all requests with `415 Unsupported Media Type`.
	// - Records received `Content-Encoding` header.
	remoteWriteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteWriteContentEncodingsMux.Lock()
		remoteWriteContentEncodings = append(remoteWriteContentEncodings, r.Header.Get(`Content-Encoding`))
		remoteWriteContentEncodingsMux.Unlock()

		w.WriteHeader(http.StatusUnsupportedMediaType)
	}))
	defer remoteWriteSrv.Close()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		`-remoteWrite.forcePromProto=true`,
		fmt.Sprintf(`-remoteWrite.url=%s/api/v1/write`, remoteWriteSrv.URL),
	}, ``)

	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server; expected zstd`,
		Got: func() any {
			remoteWriteContentEncodingsMux.Lock()
			defer remoteWriteContentEncodingsMux.Unlock()

			return append([]string(nil), remoteWriteContentEncodings...)
		},
		Want: []string{`snappy`, `snappy`},
	})

	expectedRetriesCount := 0
	if actualRetriesCount := vmagent.RemoteWriteRequestsRetriesCountTotal(t); actualRetriesCount != expectedRetriesCount {
		t.Fatalf("unexpected number of retries; got %d, want %d", actualRetriesCount, expectedRetriesCount)
	}
	expectedPacketsDroppedTotal := 2
	if actualPacketsDroppedCount := vmagent.RemoteWritePacketsDroppedTotal(t); actualPacketsDroppedCount != expectedPacketsDroppedTotal {
		t.Fatalf("unexpected number of dropped packets; got %d, want %d", actualPacketsDroppedCount, expectedPacketsDroppedTotal)
	}
}

// TestSingleVMAgentDowngradeRemoteWriteProtocol verifies that the remote write process:
// - Starts with VictoriaMetrics remote write protocol using `zstd`.
// - Upon receiving `415 Unsupported Media Type`, downgrades to Prometheus remote write with `snappy`.
// - Re-packs and retries failed requests.
// - Sends all subsequent requests using `snappy`.
func TestSingleVMAgentDowngradeRemoteWriteProtocol(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var remoteWriteContentEncodings []string
	// remoteWriteSrv is a stub HTTP server that simulates a remote write endpoint with the following behavior:
	// - Rejects requests with `zstd` encoding by responding with `415 Unsupported Media Type`.
	// - Accepts requests with `snappy` encoding.
	// - Records the `Content-Encoding` header of incoming requests.
	remoteWriteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteWriteContentEncodings = append(remoteWriteContentEncodings, r.Header.Get(`Content-Encoding`))

		if r.Header.Get(`Content-Encoding`) == `zstd` {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			_, _ = w.Write([]byte(`zstd not supported`))

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer remoteWriteSrv.Close()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		fmt.Sprintf(`-remoteWrite.url=%s/api/v1/write`, remoteWriteSrv.URL),
	}, ``)

	// Send request encoded with `zstd`; it fails, gets repacked as `snappy`, and retries successfully.
	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// Send request encoded with `snappy` immediately; it succeeds without retries.
	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server`,
		Got: func() any {
			return remoteWriteContentEncodings
		},
		Want:       []string{`zstd`, `snappy`, `snappy`},
		DoNotRetry: true,
	})

	expectedRetriesCount := 1
	if actualRetriesCount := vmagent.RemoteWriteRequestsRetriesCountTotal(t); actualRetriesCount != expectedRetriesCount {
		t.Fatalf("unexpected number of retries; got %d, want %d", actualRetriesCount, expectedRetriesCount)
	}
	expectedPacketsDroppedTotal := 0
	if actualPacketsDroppedCount := vmagent.RemoteWritePacketsDroppedTotal(t); actualPacketsDroppedCount != expectedPacketsDroppedTotal {
		t.Fatalf("unexpected number of dropped packets; got %d, want %d", actualPacketsDroppedCount, expectedPacketsDroppedTotal)
	}
}
