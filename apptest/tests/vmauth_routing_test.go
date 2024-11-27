package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestVMAuthRouting(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	var proxiedRequestsCount int
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		proxiedRequestsCount++
	}))
	defer backend.Close()

	authConfig := fmt.Sprintf(`
unauthorized_user:
   url_prefix: %s
  `, backend.URL)

	const (
		// it's not possible to use random ports
		// it makes test flaky
		listenPortNoBuiltin   = "50128"
		listenPortWithBuiltin = "50127"
	)

	vmauthFlags := []string{
		fmt.Sprintf("-httpListenAddr=127.0.0.1:%s,127.0.0.1:%s", listenPortWithBuiltin, listenPortNoBuiltin),
		"-httpListenAddr.disableBuiltinRoutes=false,true",
		"-flagsAuthKey=protected",
	}
	vmauth := tc.MustStartVmauth("vmauth",
		vmauthFlags,
		authConfig)

	var hc http.Client
	makeGetRequestExpectCode := func(targetURL string, expectCode int) {
		t.Helper()
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			t.Fatalf("cannot build http.Request for target=%q: %s", targetURL, err)
		}
		resp, err := hc.Do(req)
		if err != nil {
			t.Fatalf("unexpected http request error: %s", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != expectCode {
			responseText, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("cannot read response body: %s", err)
			}
			t.Fatalf("unexpected http response code: %d, want: %d, response text: %s", resp.StatusCode, expectCode, responseText)
		}
	}
	// built-in http server must reject request, since it protected with authKey
	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortWithBuiltin), http.StatusUnauthorized)
	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortNoBuiltin), http.StatusOK)
	if proxiedRequestsCount != 1 {
		t.Fatalf("expected to have 1 proxied request, got: %d", proxiedRequestsCount)
	}
	// reload config and ensure that it no longer proxy requests to the backend
	vmauth.UpdateConfiguration(t, "")
	makeGetRequestExpectCode(fmt.Sprintf("http://127.0.0.1:%s/flags", listenPortWithBuiltin), http.StatusUnauthorized)
	if proxiedRequestsCount != 1 {
		t.Fatalf("expected to have 1 proxied request, got: %d", proxiedRequestsCount)
	}
}
