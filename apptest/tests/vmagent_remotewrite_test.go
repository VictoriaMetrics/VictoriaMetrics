package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

// TestSingleVMAgentReloadConfigs verifies that vmagent reload new configurations on SIGHUP signal
func TestSingleVMAgentReloadConfigs(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartDefaultVmsingle()

	relabelingRules := `
- replacement: value1
  target_label: label1
  `
	relabelFilePath := fmt.Sprintf("%s/%s", t.TempDir(), "relabel_config.yaml")
	fs.MustWriteSync(relabelFilePath, []byte(relabelingRules))

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		`-remoteWrite.forcePromProto=true`,
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
		fmt.Sprintf(`-remoteWrite.url=http://%s/api/v1/write`, vmsingle.HTTPAddr()),
		fmt.Sprintf(`-remoteWrite.urlRelabelConfig=%s`, relabelFilePath),
	}, ``)

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmsingle.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmagent remote write`,
		Got: func() any {
			return vmsingle.PrometheusAPIV1Series(t, `{__name__="foo_bar"}`, apptest.QueryOpts{
				Start: "2022-05-10T00:00:00Z",
				End:   "2022-05-10T23:59:59Z",
			}).Sort()
		},
		Want: &apptest.PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   []map[string]string{{"__name__": "foo_bar", "label1": "value1"}},
		},
	})

	relabelingRules = `
- replacement: value2
  target_label: label1
  `

	fs.MustWriteSync(relabelFilePath, []byte(relabelingRules))

	vmagent.ReloadRelabelConfigs(t)

	vmagent.APIV1ImportPrometheus(t, []string{
		"bar_foo 1 1652169600001", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmsingle.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmagent remote write`,
		Got: func() any {
			return vmsingle.PrometheusAPIV1Series(t, `{__name__="bar_foo"}`, apptest.QueryOpts{
				Start: "2022-05-10T00:00:00Z",
				End:   "2022-05-10T23:59:59Z",
			}).Sort()
		},
		Want: &apptest.PrometheusAPIV1SeriesResponse{
			Status: "success",
			Data:   []map[string]string{{"__name__": "bar_foo", "label1": "value2"}},
		},
	})
}

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
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
	}, ``)

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmsingle.ForceFlush(t)

	tc.Assert(&apptest.AssertOptions{
		Msg: `unexpected metrics stored on vmagent remote write`,
		Got: func() any {
			return vmsingle.PrometheusAPIV1Series(t, `{__name__="foo_bar"}`, apptest.QueryOpts{
				Start: "2022-05-10T00:00:00Z",
				End:   "2022-05-10T23:59:59Z",
			}).Sort()
		},
		Want: &apptest.PrometheusAPIV1SeriesResponse{
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
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
	}, ``)

	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	tc.Assert(&apptest.AssertOptions{
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
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
	}, ``)

	// Send request encoded with `zstd`; it fails, gets repacked as `snappy`, and retries successfully.
	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// Send request encoded with `snappy` immediately; it succeeds without retries.
	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	tc.Assert(&apptest.AssertOptions{
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

func TestSingleVMAgentDropOnOverload(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	remoteWriteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remoteWriteSrv.Close()

	remoteWriteSrv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer remoteWriteSrv2.Close()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		fmt.Sprintf(`-remoteWrite.url=%s/api/v1/write`, remoteWriteSrv.URL),
		fmt.Sprintf(`-remoteWrite.url=%s/api/v1/write`, remoteWriteSrv2.URL),
		"-remoteWrite.disableOnDiskQueue=true",
		// use only 1 worker to get a full queue faster
		"-remoteWrite.queues=1",
		// fastqueue size is roughly memory.Allowed() / len(urls) / *maxRowsPerBlock / 100
		// Use very large maxRowsPerBlock to get fastqueue of minimal length(2).
		// See initRemoteWriteCtxs function in remotewrite.go for details.
		"-remoteWrite.maxRowsPerBlock=1000000000",
		"-remoteWrite.tmpDataPath=" + tc.Dir() + "/vmagent",
	}, ``)

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)

	waitFor := func(f func() bool) {
		t.Helper()
		for i := 0; i < retries; i++ {
			if f() {
				return
			}
			time.Sleep(period)
		}
		t.Fatalf("timed out waiting for retry #%d", retries)
	}

	// Real remote write URLs are hidden in metrics
	url1 := "1:secret-url"
	url2 := "2:secret-url"

	// Wait until first request got flushed to remote write server
	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return vmagent.RemoteWriteRequests(t, url1) == 1 && vmagent.RemoteWriteRequests(t, url2) == 1
		},
	)

	// Send 2 more requests, the first RW endpoint should receive everything, the second should add them to the queue
	// since worker is busy with the first request.
	for i := 0; i < 2; i++ {
		vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
			"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
		}, apptest.QueryOpts{})

		waitFor(
			func() bool {
				return vmagent.RemoteWriteRequests(t, url1) == 2+i && vmagent.RemoteWritePendingInmemoryBlocks(t, url2) == 1+i
			},
		)
	}

	// Send one more request.
	vmagent.APIV1ImportPrometheusNoWaitFlush(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	waitFor(
		func() bool {
			return vmagent.RemoteWriteRequests(t, url1) == 4 && vmagent.RemoteWriteSamplesDropped(t, url2) > 0
		},
	)
}
