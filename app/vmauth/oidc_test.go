package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateIDToken(t *testing.T) {
	jt := newTokenTester(t)

	issuer := "https://idp.example.com"
	clientID := "my-client"
	nonce := "test-nonce"

	pm := &oidcProviderMetadata{
		Issuer: issuer,
		vp:     jt.NewVerifierPool(),
	}

	// valid token
	validToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err := validateIDToken(validToken, pm, clientID, nonce)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// invalid signature
	invalidSigToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, false)
	_, err = validateIDToken(invalidSigToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "signature verification failed") {
		t.Fatalf("expected signature verification error; got %v", err)
	}

	// expired token
	expiredToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(-time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(expiredToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error; got %v", err)
	}

	// wrong issuer
	wrongIssuerToken := jt.GenToken(map[string]any{
		"iss":   "https://evil.com",
		"aud":   clientID,
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongIssuerToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "issuer mismatch") {
		t.Fatalf("expected issuer mismatch error; got %v", err)
	}

	// wrong audience
	wrongAudToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   "wrong-client",
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongAudToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "audience mismatch") {
		t.Fatalf("expected audience mismatch error; got %v", err)
	}

	// wrong nonce
	wrongNonceToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": "wrong-nonce",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(wrongNonceToken, pm, clientID, nonce)
	if err == nil || !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("expected nonce mismatch error; got %v", err)
	}

	// client_id with regex metacharacters must be compared literally
	regexAudToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   "anything",
		"nonce": nonce,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(regexAudToken, pm, ".*", nonce)
	if err == nil || !strings.Contains(err.Error(), "audience mismatch") {
		t.Fatalf("expected audience mismatch for regex client_id; got %v", err)
	}

	// nonce with regex metacharacters must be compared literally
	regexNonceToken := jt.GenToken(map[string]any{
		"iss":   issuer,
		"aud":   clientID,
		"nonce": "anything",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, true)
	_, err = validateIDToken(regexNonceToken, pm, clientID, ".*")
	if err == nil || !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("expected nonce mismatch for regex nonce; got %v", err)
	}
}

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
