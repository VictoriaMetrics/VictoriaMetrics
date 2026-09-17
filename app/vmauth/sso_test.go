package main

import (
	"slices"
	"testing"
)

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
