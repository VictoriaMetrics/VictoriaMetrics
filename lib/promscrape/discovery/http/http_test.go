package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAddHTTPTargetLabels(t *testing.T) {
	f := func(src []httpGroupTarget, labelssExpected []*promutil.Labels) {
		t.Helper()

		labelss := addHTTPTargetLabels(src, "http://foo.bar/baz?aaa=bb")
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// add ok
	src := []httpGroupTarget{
		{
			Targets: []string{"127.0.0.1:9100", "127.0.0.2:91001"},
			Labels:  promutil.NewLabelsFromMap(map[string]string{"__meta_kubernetes_pod": "pod-1", "__meta_consul_dc": "dc-2"}),
		},
	}
	labelssExpected := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.1:9100",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":           "127.0.0.2:91001",
			"__meta_kubernetes_pod": "pod-1",
			"__meta_consul_dc":      "dc-2",
			"__meta_url":            "http://foo.bar/baz?aaa=bb",
		}),
	}
	f(src, labelssExpected)
}

func TestSDConfigGetLabels(t *testing.T) {

	type apiResponse struct {
		statusCode int
		body       string
	}
	var currentResponse atomic.Pointer[apiResponse]

	// add initial non-empty response
	currentResponse.Store(&apiResponse{
		body:       `[{"targets":["10.0.0.2:9100"],"labels":{"job":"node"}}]`,
		statusCode: http.StatusOK,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := currentResponse.Load()
		w.WriteHeader(resp.statusCode)
		_, _ = w.Write([]byte(resp.body))
	}))
	defer srv.Close()

	sdc := &SDConfig{
		URL: srv.URL,
	}
	sdc.MustStart(".")
	defer sdc.MustStop()

	assertLabelss := func(expectedLabelss []*promutil.Labels) {
		t.Helper()
		got, err := sdc.GetLabels(".")
		if err != nil {
			t.Fatalf("unexpected GetLabels error: %s", err)
		}
		if len(got) == 0 && len(expectedLabelss) == 0 {
			return
		}
		discoveryutil.TestEqualLabelss(t, got, expectedLabelss)
	}

	// check initial state, it must be non-empty
	// it also inits apiConfig below
	assertLabelss([]*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "10.0.0.2:9100",
			"job":         "node",
			"__meta_url":  srv.URL,
		}),
	})

	updateAPIResponse := func(response apiResponse) {
		currentResponse.Store(&response)
		sdc.cfg.refreshTargetsIfNeeded()

	}

	// change response to empty
	updateAPIResponse(apiResponse{
		statusCode: http.StatusOK,
		body:       `[]`,
	})
	assertLabelss([]*promutil.Labels{})

	// change response to non-empty
	updateAPIResponse(apiResponse{
		statusCode: http.StatusOK,
		body:       `[{"targets":["10.0.0.1:9100"],"labels":{"job":"node"}},{"targets":["10.0.0.5:8429"],"labels":{"job":"vmagent"}}]`,
	})
	assertLabelss([]*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "10.0.0.1:9100",
			"job":         "node",
			"__meta_url":  srv.URL,
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "10.0.0.5:8429",
			"job":         "vmagent",
			"__meta_url":  srv.URL,
		}),
	})

	// change response to error
	updateAPIResponse(apiResponse{
		statusCode: http.StatusServiceUnavailable,
		body:       `Internal Server Error`,
	})
	_, err := sdc.GetLabels(".")
	if err == nil {
		t.Fatalf("unexpected empty error")
	}

	// transit back to correct api response
	updateAPIResponse(apiResponse{
		statusCode: http.StatusOK,
		body:       `[{"targets":["10.0.0.1:9100"],"labels":{"job":"node"}},{"targets":["10.0.0.5:8429"],"labels":{"job":"vmagent"}}]`,
	})
	assertLabelss([]*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "10.0.0.1:9100",
			"job":         "node",
			"__meta_url":  srv.URL,
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__": "10.0.0.5:8429",
			"job":         "vmagent",
			"__meta_url":  srv.URL,
		}),
	})

	// make sure that api response is properly cached
	before := sdc.cfg.targetLabels.Load()
	updateAPIResponse(apiResponse{statusCode: http.StatusOK,
		body: `[{"targets":["10.0.0.1:9100"],"labels":{"job":"node"}},{"targets":["10.0.0.5:8429"],"labels":{"job":"vmagent"}}]`})

	if sdc.cfg.targetLabels.Load() != before {
		t.Fatalf("expected identical response to be deduplicated")
	}
}
