package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
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

func TestBasicAuthMetrics(t *testing.T) {
	origUsername := *httpAuthUsername
	origPasswd := httpAuthPassword.Get()
	defer func() {
		if err := httpAuthPassword.Set(origPasswd); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		*httpAuthUsername = origUsername
	}()

	f := func(user, pass string, expCode int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth(user, pass)

		w := httptest.NewRecorder()
		CheckBasicAuth(w, req)

		res := w.Result()
		_ = res.Body.Close()
		if expCode != res.StatusCode {
			t.Fatalf("wanted status code: %d, got: %d\n", res.StatusCode, expCode)
		}
	}

	*httpAuthUsername = "test"
	if err := httpAuthPassword.Set("pass"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f("test", "pass", 200)
	f("test", "wrong", 401)
	f("wrong", "pass", 401)
	f("wrong", "wrong", 401)

	*httpAuthUsername = ""
	if err := httpAuthPassword.Set(""); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	f("test", "pass", 200)
	f("test", "wrong", 200)
	f("wrong", "pass", 200)
	f("wrong", "wrong", 200)
}

func TestAuthKeyMetrics(t *testing.T) {
	origUsername := *httpAuthUsername
	origPasswd := httpAuthPassword.Get()
	defer func() {
		if err := httpAuthPassword.Set(origPasswd); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		*httpAuthUsername = origUsername
	}()

	tstWithAuthKey := func(key string, expCode int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/metrics", strings.NewReader("authKey="+key))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;param=value")
		w := httptest.NewRecorder()

		p := &flagutil.Password{}
		if err := p.Set("rightKey"); err != nil {
			t.Fatalf("cannot set password: %s", err)
		}
		CheckAuthFlag(w, req, p)

		res := w.Result()
		defer res.Body.Close()
		if expCode != res.StatusCode {
			t.Fatalf("Unexpected status code: %d, Expected code is: %d\n", res.StatusCode, expCode)
		}
	}

	tstWithAuthKey("rightKey", 200)
	tstWithAuthKey("wrongKey", 401)

	tstWithOutAuthKey := func(user, pass string, expCode int) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth(user, pass)

		w := httptest.NewRecorder()
		p := &flagutil.Password{}
		if err := p.Set(""); err != nil {
			t.Fatalf("cannot set password: %s", err)
		}
		CheckAuthFlag(w, req, p)

		res := w.Result()
		_ = res.Body.Close()
		if expCode != res.StatusCode {
			t.Fatalf("wanted status code: %d, got: %d\n", res.StatusCode, expCode)
		}
	}

	*httpAuthUsername = "test"
	if err := httpAuthPassword.Set("pass"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	tstWithOutAuthKey("test", "pass", 200)
	tstWithOutAuthKey("test", "wrong", 401)
	tstWithOutAuthKey("wrong", "pass", 401)
	tstWithOutAuthKey("wrong", "wrong", 401)
}

func TestHandlerWrapper(t *testing.T) {
	const hstsHeader = "foo"
	const frameOptionsHeader = "bar"
	const cspHeader = "baz"

	*headerHSTS = hstsHeader
	*headerFrameOptions = frameOptionsHeader
	*headerCSP = cspHeader
	defer func() {
		*headerHSTS = ""
		*headerFrameOptions = ""
		*headerCSP = ""
	}()

	req, _ := http.NewRequest("GET", "/health", nil)

	srv := &server{s: &http.Server{}}
	w := &httptest.ResponseRecorder{}

	handlerWrapper(w, req, func(w http.ResponseWriter, r *http.Request) bool {
		return builtinRoutesHandler(srv, r, w, func(_ http.ResponseWriter, _ *http.Request) bool {
			return true
		})
	})

	h := w.Header()
	if got := h.Get("Strict-Transport-Security"); got != hstsHeader {
		t.Fatalf("unexpected HSTS header; got %q; want %q", got, hstsHeader)
	}
	if got := h.Get("X-Frame-Options"); got != frameOptionsHeader {
		t.Fatalf("unexpected X-Frame-Options header; got %q; want %q", got, frameOptionsHeader)
	}
	if got := h.Get("Content-Security-Policy"); got != cspHeader {
		t.Fatalf("unexpected CSP header; got %q; want %q", got, cspHeader)
	}
}

func TestServe(t *testing.T) {
	hndl := RequestHandler(func(w http.ResponseWriter, r *http.Request) bool {
		return true
	})
	addrs := make([]string, 0, 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hndl(w, r) }))
	ts.Close()
	addrs = append(addrs, ts.URL)
	dn, err := os.MkdirTemp("", "httpserver-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dn)
	addrs = append(addrs, "unix:"+filepath.Join(dn, "socket"))
	tr := http.DefaultTransport.(*http.Transport).Clone()
	unixtransportRegister(tr)
	unixClient := &http.Client{Transport: tr}

	// t.Log("addrs:", addrs)
	for _, addr := range addrs {
		scheme, _, _ := strings.Cut(addr, ":")
		t.Run(scheme, func(t *testing.T) {
			serveAddr := strings.TrimPrefix(addr, "http://")
			go func() {
				serve(serveAddr, hndl, 0, ServeOptions{})
				t.Log("returned")
			}()
			defer stop(serveAddr)
			time.Sleep(100 * time.Millisecond)
			client := http.DefaultClient
			if scheme == "unix" {
				client, addr = unixClient, "http+"+addr
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s (%s): %+v", req.Method, req.URL, addr, err)
			}
			t.Log(resp.Status)
		})
	}
}
