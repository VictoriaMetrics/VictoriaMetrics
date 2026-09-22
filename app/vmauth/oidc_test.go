package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOIDCHTTPClientRedirect(t *testing.T) {
	f := func(t *testing.T, handler http.HandlerFunc, expStatusCode int, expErrSubstring string) {
		t.Helper()

		srv := httptest.NewServer(handler)
		defer srv.Close()

		resp, err := oidcHTTPClient.Get(srv.URL + "/start")
		if expErrSubstring != "" {
			if err == nil {
				t.Fatalf("expecting error containing %q; got nil", expErrSubstring)
			}
			if !strings.Contains(err.Error(), expErrSubstring) {
				t.Fatalf("unexpected error; got\n%s\nwant substring\n%s", err.Error(), expErrSubstring)
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		resp.Body.Close()
		if resp.StatusCode != expStatusCode {
			t.Fatalf("unexpected status code; got %d; want %d", resp.StatusCode, expStatusCode)
		}
	}

	// same-host redirect must succeed
	f(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}, http.StatusOK, "")

	// cross-host redirect must be blocked
	var evilRequested bool
	evilSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilRequested = true
	}))
	defer evilSrv.Close()

	f(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evilSrv.URL+"/evil", http.StatusFound)
	}, 0, "is not allowed")
	if evilRequested {
		t.Fatalf("request must not reach the evil server")
	}

	// too many same-host redirects must be stopped
	f(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}, 0, "stopped after 10 redirects")
}
