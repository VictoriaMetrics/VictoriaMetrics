package main

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSSOConfigGetSessionDuration(t *testing.T) {
	f := func(sessionDuration string, tokenExpiresAt time.Time, expectedDuration time.Duration) {
		t.Helper()
		s := fmt.Sprintf(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    session_duration: %s
`, sessionDuration)
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("cannot parse auth config: %s", err)
		}
		if err := normalizeSSOConfigs(ac.SSO); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := ac.SSO[0].OIDC.getSessionDuration(tokenExpiresAt).Truncate(time.Second)
		if got != expectedDuration {
			t.Fatalf("unexpected session duration; got %s; want %s", got, expectedDuration)
		}
	}

	// session duration not set (default 10m), token expiry is longer — use default
	f("", time.Now().Add(time.Hour), 10*time.Minute)

	// session duration not set (default 10m), token expiry is shorter — use token expiry
	f("", time.Now().Add(2*time.Minute+time.Second), 2*time.Minute)

	// session duration is less than token expiry — use session duration
	f("10m", time.Now().Add(time.Hour), 10*time.Minute)

	// token expiry is less than session duration — use token expiry
	f("1h", time.Now().Add(2*time.Minute+time.Second), 2*time.Minute)

	// token already expired — returns 0
	f("10m", time.Now().Add(-time.Minute), 0)

	// token already expired, no session duration set — returns 0
	f("", time.Now().Add(-time.Minute), 0)

	// session_duration explicitly null (default 10m), token expiry is longer — use default
	f("null", time.Now().Add(time.Hour), 10*time.Minute)
}

func TestSSOConfigGetCallbackURL(t *testing.T) {
	f := func(host string, insecure bool, expectedURL string) {
		t.Helper()
		s := fmt.Sprintf(`
sso:
- src_host: ".*"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    insecure: %v
`, insecure)
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("cannot parse auth config: %s", err)
		}
		if err := normalizeSSOConfigs(ac.SSO); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := ac.SSO[0].OIDC.getCallbackURL(host)
		if got != expectedURL {
			t.Fatalf("unexpected callback URL; got %q; want %q", got, expectedURL)
		}
	}

	// secure (default)
	f("example.com", false, "https://example.com/_vmauth/sso/callback")

	// insecure
	f("example.com", true, "http://example.com/_vmauth/sso/callback")

	// host with port, secure
	f("example.com:8427", false, "https://example.com:8427/_vmauth/sso/callback")

	// host with port, insecure
	f("localhost:8427", true, "http://localhost:8427/_vmauth/sso/callback")
}

func TestSSOConfigGetRedirectURL(t *testing.T) {
	f := func(defaultRedirectURL, redirect, expectedURL string) {
		t.Helper()
		s := fmt.Sprintf(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    default_redirect_url: %s
`, defaultRedirectURL)
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("cannot parse auth config: %s", err)
		}
		if err := normalizeSSOConfigs(ac.SSO); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := ac.SSO[0].OIDC.getRedirectURL(redirect)
		if got != expectedURL {
			t.Fatalf("unexpected redirect URL; got %q; want %q", got, expectedURL)
		}
	}

	// valid relative path
	f("", "/dashboard", "/dashboard")

	// valid relative path with query
	f("", "/dashboard?tab=1", "/dashboard?tab=1")

	// absolute URL — falls back to "/"
	f("", "https://evil.com", "/")

	// protocol-relative URL — falls back to "/"
	f("", "//evil.com", "/")

	// open redirect with backslash — falls back to "/"
	f("", "/\\evil.com", "/")

	// open redirect with dot segments — falls back to "/"
	f("", "/../evil.com", "/")

	// empty redirect — falls back to "/"
	f("", "", "/")

	// absolute URL with default_redirect_url set — uses default
	f("/home", "https://evil.com", "/home")

	// protocol-relative URL with default_redirect_url set — uses default
	f("/home", "//evil.com", "/home")

	// valid relative path with default_redirect_url set — uses the path
	f("/home", "/dashboard", "/dashboard")
}

func TestSSOConfigGetSSOConfigForHost(t *testing.T) {
	f := func(s string, host string, expectedFound bool) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("cannot parse auth config: %s", err)
		}
		if err := normalizeSSOConfigs(ac.SSO); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := getSSOConfigForHost(ac, host)
		if expectedFound && got == nil {
			t.Fatalf("expected SSO config for host %q, got nil", host)
		}
		if !expectedFound && got != nil {
			t.Fatalf("expected nil SSO config for host %q, got non-nil", host)
		}
	}

	// matching host
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "example.com", true)

	// non-matching host
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "other.com", false)

	// regex pattern matching
	f(`
sso:
- src_host: ".*\\.example\\.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "app.example.com", true)

	// regex pattern not matching
	f(`
sso:
- src_host: ".*\\.example\\.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "example.com", false)

	// multiple sso configs, second matches
	f(`
sso:
- src_host: "first.com"
  oidc:
    issuer: https://idp.first.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
- src_host: "second.com"
  oidc:
    issuer: https://idp.second.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "second.com", true)

	// partial match rejected due to anchoring
	f(`
sso:
- src_host: "example"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`, "example.com", false)

	// no sso configs
	f(`
users:
- username: foo
  password: bar
  url_prefix: http://foo.bar
`, "example.com", false)
}

func TestSSOConfigNormalizeSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("cannot parse auth config: %s", err)
		}
		if err := normalizeSSOConfigs(ac.SSO); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		for i, sso := range ac.SSO {
			if !slices.Contains(sso.OIDC.Scopes, "openid") {
				t.Fatalf("sso.%d: expected openid scope to be present; got %v", i, sso.OIDC.Scopes)
			}
		}
	}

	// minimal valid config
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`)

	// http issuer scheme
	f(`
sso:
- src_host: "localhost"
  oidc:
    issuer: http://localhost:8080
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
`)

	// custom scopes with openid already present
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    scopes: ["openid", "profile"]
`)

	// custom scopes without openid - should be added automatically
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    scopes: ["profile"]
`)

	// custom session duration
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    session_duration: "1h"
`)

	// multiple sso configs
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
- src_host: "other.com"
  oidc:
    issuer: https://idp.other.com
    client_id: other-client
    client_secret: other-secret
    cookie_secret: "abcdef0123456789"
`)
}

func TestSSOConfigNormalizeFailure(t *testing.T) {
	f := func(s string, expErr string) {
		t.Helper()
		ac, err := parseAuthConfig([]byte(s))
		if err != nil {
			if expErr != err.Error() {
				t.Fatalf("unexpected error; got\n%q\nwant\n%q", err.Error(), expErr)
			}
			return
		}
		err = normalizeSSOConfigs(ac.SSO)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if expErr != err.Error() {
			t.Fatalf("unexpected error; got\n%q\nwant\n%q", err.Error(), expErr)
		}
	}

	// missing src_host
	f(`
sso:
- oidc:
    issuer: https://idp.example.com
`, `sso.0: src_host is required`)

	// missing oidc
	f(`
sso:
- src_host: "example.com"
`, `sso.0: oidc is required`)

	// missing issuer
	f(`
sso:
- src_host: "example.com"
  oidc:
    client_id: my-client
`, `sso.0: oidc.issuer is required`)

	// invalid issuer URL
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: "://bad"
`, `sso.0: oidc.issuer must be a valid URL`)

	// issuer with unsupported scheme
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: ftp://idp.example.com
`, `sso.0: oidc.issuer must have http or https scheme`)

	// missing client_id
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
`, `sso.0: oidc.client_id is required`)

	// missing client_secret
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
`, `sso.0: oidc.client_secret is required`)

	// cookie_secret too short
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "short"
`, `sso.0: oidc.cookie_secret must be at least 16 characters long`)

	// invalid session_duration
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    session_duration: "invalid"
`, `sso.0: oidc.session_duration: time: invalid duration "invalid"`)

	// negative session_duration
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
    session_duration: "-5m"
`, `sso.0: oidc.session_duration must not be negative`)

	// second sso config fails
	f(`
sso:
- src_host: "example.com"
  oidc:
    issuer: https://idp.example.com
    client_id: my-client
    client_secret: my-secret
    cookie_secret: "0123456789abcdef"
- src_host: "other.com"
  oidc:
    issuer: ftp://bad
`, `sso.1: oidc.issuer must have http or https scheme`)
}

func TestSignVerifyCSRFCookie(t *testing.T) {
	// round-trip sign and verify
	signed := signCSRFCookie("nonce123", "state456", "/dashboard", "secret0123456789")
	gotNonce, gotState, gotRedirectURL, err := verifyCSRFCookie(signed, "secret0123456789")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotNonce != "nonce123" {
		t.Fatalf("unexpected nonce; got %q; want %q", gotNonce, "nonce123")
	}
	if gotState != "state456" {
		t.Fatalf("unexpected state; got %q; want %q", gotState, "state456")
	}
	if gotRedirectURL != "/dashboard" {
		t.Fatalf("unexpected redirectURL; got %q; want %q", gotRedirectURL, "/dashboard")
	}

	// redirectURL with colons
	signed = signCSRFCookie("n", "s", "/path:with:colons", "secret0123456789")
	_, _, gotRedirectURL, err = verifyCSRFCookie(signed, "secret0123456789")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if gotRedirectURL != "/path:with:colons" {
		t.Fatalf("unexpected redirectURL; got %q; want %q", gotRedirectURL, "/path:with:colons")
	}

	// missing separator
	_, _, _, err = verifyCSRFCookie("noseparator", "secret")
	if err == nil || !strings.Contains(err.Error(), "missing separator") {
		t.Fatalf("expected missing separator error; got %v", err)
	}

	// wrong secret
	signed = signCSRFCookie("nonce", "state", "/path", "secret0123456789")
	_, _, _, err = verifyCSRFCookie(signed, "wrongsecret12345")
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("expected signature mismatch error; got %v", err)
	}

	// tampered payload — inject a symbol into valid signed cookie
	_, _, _, err = verifyCSRFCookie(signed[:10]+"X"+signed[11:], "secret0123456789")
	if err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("expected signature mismatch error; got %v", err)
	}

	// empty nonce panics
	assertPanic(t, "empty nonce", func() { signCSRFCookie("", "state", "/", "secret0123456789") })

	// empty state panics
	assertPanic(t, "empty state", func() { signCSRFCookie("nonce", "", "/", "secret0123456789") })

	// empty cookieSecret panics
	assertPanic(t, "empty cookieSecret in sign", func() { signCSRFCookie("nonce", "state", "/", "") })
	assertPanic(t, "empty cookieSecret in verify", func() { verifyCSRFCookie(signed, "") })
}


func assertPanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("%s: expected panic, got none", name)
		}
	}()
	fn()
}
