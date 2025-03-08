package tests

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleVMAgentBasicRemoteWrite(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmsingle := tc.MustStartDefaultVmsingle()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
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

func TestSingleVMAgentUpgradeRemoteWriteProtocol(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var remoteWriteContentEncodings []string
	// remoteWriteSrv is a stub HTTP server simulating a remote write endpoint with the following behavior:
	// - Fails `get_vm_proto_version` and force vmagent to switch to prometheus remote write protocol.
	// - Successfully accepts incoming write requests.
	// - Records received `Content-Encoding` header values.
	// - Responds with an `Accept-Encoding` header, indicating supported encodings (`zstd, snappy`).
	remoteWriteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// force proto remote write at start
		if r.URL.Query().Get(`get_vm_proto_version`) != `` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		remoteWriteContentEncodings = append(remoteWriteContentEncodings, r.Header.Get(`Content-Encoding`))

		w.Header().Set(`Accept-Encoding`, `zstd, snappy`)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remoteWriteSrv.Close()

	vmagent := tc.MustStartVmagent("vmagent", []string{
		`-remoteWrite.flushInterval=50ms`,
		fmt.Sprintf(`-remoteWrite.url=%s/api/v1/write`, remoteWriteSrv.URL),
	}, ``)

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// first one must be snappy
	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server; expected snappy`,
		Got: func() any {
			return remoteWriteContentEncodings[0]
		},
		Want:       `snappy`,
		DoNotRetry: true,
	})

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// second one must be zstd
	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server; expected zstd`,
		Got: func() any {
			return remoteWriteContentEncodings[1]
		},
		Want:       `zstd`,
		DoNotRetry: true,
	})
}

func TestSingleVMAgentDowngradeRemoteWriteProtocol(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var remoteWriteContentEncodings []string
	// remoteWriteSrv is a stub HTTP server simulating a remote write endpoint with the following behavior:
	// - Accept `get_vm_proto_version` and force vmagent to switch to vm remote write protocol.
	// - Fail requests with zstd content encoding.
	// - Accept requests with snappy content encoding.
	// - Records received `Content-Encoding` header values.
	remoteWriteSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// force proto remote write at start
		if r.URL.Query().Get(`get_vm_proto_version`) != `` {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`1`))
			return
		}

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

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// first one must be snappy
	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server; expected zstd`,
		Got: func() any {
			return remoteWriteContentEncodings[0]
		},
		Want:       `zstd`,
		DoNotRetry: true,
	})

	vmagent.APIV1ImportPrometheus(t, []string{
		"foo_bar 1 1652169600000", // 2022-05-10T08:00:00Z
	}, apptest.QueryOpts{})

	// second one must be zstd
	tc.Assert(&at.AssertOptions{
		Msg: `unexpected content encoding headers sent to remote write server; expected snappy`,
		Got: func() any {
			return remoteWriteContentEncodings[1]
		},
		Want:       `snappy`,
		DoNotRetry: true,
	})
}
