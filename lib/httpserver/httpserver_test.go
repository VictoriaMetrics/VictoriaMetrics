package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetQuotedRemoteAddr(t *testing.T) {
	f := func(remoteAddr, xForwardedFor, expectedAddr string) {
		t.Helper()

		req := &http.Request{
			RemoteAddr: remoteAddr,
		}
		if xForwardedFor != "" {
			req.Header = map[string][]string{
				"X-Forwarded-For": {xForwardedFor},
			}
		}
		addr := GetQuotedRemoteAddr(req)
		if addr != expectedAddr {
			t.Fatalf("unexpected remote addr;\ngot\n%s\nwant\n%s", addr, expectedAddr)
		}

		// Verify that the addr can be unmarshaled as JSON string
		var s string
		if err := json.Unmarshal([]byte(addr), &s); err != nil {
			t.Fatalf("cannot unmarshal addr: %s", err)
		}
	}

	f("1.2.3.4", "", `"1.2.3.4"`)
	f("1.2.3.4", "foo.bar", `"1.2.3.4, X-Forwarded-For: foo.bar"`)
	f("1.2\n\"3.4", "foo\nb\"ar", `"1.2\n\"3.4, X-Forwarded-For: foo\nb\"ar"`)
}

func TestHandlerWrapper(t *testing.T) {
	*headerHSTS = "foo"
	*headerFrameOptions = "bar"
	*headerCSP = "baz"
	defer func() {
		*headerHSTS = ""
		*headerFrameOptions = ""
		*headerCSP = ""
	}()

	req, _ := http.NewRequest("GET", "/health", nil)

	srv := &server{s: &http.Server{}}
	w := &httptest.ResponseRecorder{}
	handlerWrapper(srv, w, req, func(_ http.ResponseWriter, _ *http.Request) bool {
		return true
	})

	if w.Header().Get("Strict-Transport-Security") != "foo" {
		t.Errorf("HSTS header not set")
	}
	if w.Header().Get("X-Frame-Options") != "bar" {
		t.Errorf("X-Frame-Options header not set")
	}
	if w.Header().Get("Content-Security-Policy") != "baz" {
		t.Errorf("CSP header not set")
	}
}
