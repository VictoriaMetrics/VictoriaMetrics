# SSO (OpenID Connect) for vmauth

## Requirements

* SSO should only support OpenID Connect (authorization code flow).
* Keep implementation as simple as possible.
* Avoid external dependencies — use only stdlib `net/http`, `crypto/hmac`, `encoding/json`, etc.
* SSO must be coupled to existing JWT logic: after OIDC callback, the id\_token is stored in a cookie and fed into the existing JWT user-matching pipeline on subsequent requests.
* SSO should be implemented as a standalone feature in `app/vmauth/sso.go`.
* Attempt SSO login only if:
  1. No existing credentials matched (bearer, basic, JWT), AND
  2. The request Host is listed in the `sso:` config section.
* Validate the OIDC callback (`state`, `code`, token exchange, id\_token signature) before setting the session cookie.
* Both cookie-based SSO sessions and all existing credentials (bearer, basic, JWT) work simultaneously — SSO is purely additive.

---

## Config

```yaml
# vmauth.yaml

sso:
  foo.com:
    openid_connect:
      issuer: https://accounts.google.com   # OIDC discovery base URL
      client_id: <client_id>
      client_secret: <client_secret>
      redirect_url: https://foo.com/_vmauth/sso/callback  # optional; derived from Host if omitted
      scopes: [openid, email, profile]                    # optional; default: [openid]

  bar.com:
    openid_connect:
      issuer: https://login.microsoftonline.com/<tenant>/v2.0
      client_id: <client_id>
      client_secret: <client_secret>
```

---

## Architecture

### New file: `app/vmauth/sso.go`

Responsibilities:
- Config structs (`SSOConfig`, `SSOHostConfig`, `OIDCConnectConfig`).
- OIDC discovery: fetch `{issuer}/.well-known/openid-configuration` → extract `authorization_endpoint`, `token_endpoint`, `jwks_uri`.
- `initiateSSOLogin(w, r, cfg)` — redirect browser to OIDC authorization URL with `state` + `nonce` (CSRF).
- `handleSSOCallback(w, r, cfg)` — validate `state`, exchange `code` for tokens, validate `id_token`, set signed session cookie, redirect to original URL.
- `ssoAuthTokenFromRequest(r)` — extract `id_token` from the session cookie and return it as an auth token string so that the existing JWT pipeline can match it to a configured user.

### Changes to `auth_config.go`

Add `SSO SSOConfig` field to `AuthConfig`:

```go
type AuthConfig struct {
    Users           []UserInfo  `yaml:"users"`
    UnauthorizedUser *UserInfo  `yaml:"unauthorized_user"`
    SSO             SSOConfig   `yaml:"sso"`        // NEW
}
```

`SSOConfig` is `map[string]*SSOHostConfig` (host → config). Parsed during `parseAuthConfig`; OIDC discovery is triggered at parse time (same pattern as `oidcDiscoverer`).

### Changes to `main.go`

**Callback handler** — registered before the main auth flow:
```
if r.URL.Path == "/_vmauth/sso/callback" {
    handleSSOCallback(w, r, ssoConfigForHost(r.Host))
    return
}
```

**Extended auth token extraction** — after `getAuthTokensFromRequest`:
```
if tok := ssoAuthTokenFromRequest(r); tok != "" {
    ats = append(ats, tok)
}
```
This feeds the cookie's `id_token` into the existing `getJWTUserInfo` call with zero duplication.

**SSO login page** — after all existing auth attempts fail and before returning 401:
```
if cfg := ssoConfigForHost(r.Host); cfg != nil {
    showSSOLoginPage(w, r, cfg)
    return
}
```

`showSSOLoginPage` computes the full OIDC authorization URL (including `state`, `client_id`, `redirect_uri`, `scope`) upfront and writes a minimal HTML page (200 OK) with a single `<a href="{authorizationURL}">Login with SSO</a>` button. The link points directly to the OIDC provider — no intermediate vmauth hop.

---

## OIDC Authorization Code Flow

```
Browser                       vmauth                        OIDC Provider
  |                             |                                |
  |-- GET foo.com/app ---------->|                               |
  |                             |-- no credentials, host in SSO |
  |                             |   compute full authz URL with  |
  |                             |   state, client_id, scopes     |
  |<-- 200 HTML login page ------|                               |
  |   [Login with SSO] href=    |                               |
  |   https://provider/authorize?...                            |
  |                                                             |
  |-- (user clicks) GET /authorize?client_id=...&state=X ------>|
  |<-- 302 → foo.com/_vmauth/sso/callback?code=Y&state=X -------|
  |                             |                                |
  |-- GET /_vmauth/sso/callback?code=Y&state=X ->|              |
  |                             |-- validate state cookie        |
  |                             |-- POST /token {code} -------->|
  |                             |<-- {id_token, access_token} --|
  |                             |-- validate id_token JWT        |
  |                             |   (via OIDC JWKS)              |
  |                             |-- set _vmauth_sso cookie       |
  |<-- 302 → /app (original) ---|                               |
  |                             |                                |
  |-- GET foo.com/app (with cookie) ->|                         |
  |                             |-- extract id_token from cookie |
  |                             |-- JWT user matching (existing) |
  |                             |-- proxy to backend             |
  |<-- 200 OK ------------------|                               |
```

---

## Session Cookie

- Name: `_vmauth_sso`
- Value: raw OIDC `id_token` (already a signed JWT — no extra wrapping needed)
- Flags: `HttpOnly; Secure; SameSite=Lax; Path=/`
- Expiry: derived from `id_token` `exp` claim

No server-side session store is required. The `id_token` is self-contained and validated on every request via the existing JWT machinery (JWKS signature check, `exp` check).

---

## State / CSRF Protection

State is entirely self-contained in the `state` query parameter — no server-side storage, no per-instance secret. This works correctly when multiple vmauth instances are behind a load balancer.

- On SSO initiation: build `state = base64url(JSON{nonce, originalURL, issuedAt}) + "." + HMAC-SHA256(client_secret, payload)`.
  - `nonce` — 16 random bytes (replay protection).
  - `originalURL` — so the user is redirected back after login.
  - `issuedAt` — Unix timestamp; reject states older than 10 minutes on callback.
  - `client_secret` — already shared across all instances via the config file; no extra coordination needed.
- On callback: split `state` on `.`, re-compute HMAC over the payload using `client_secret`, compare in constant time, check `issuedAt` not expired.

---

## OIDC Discovery

Performed once at config load time (same as existing `oidcDiscoverer`):

```
GET {issuer}/.well-known/openid-configuration
→ parse authorization_endpoint, token_endpoint, jwks_uri
```

JWKS is fetched and cached by the existing `oidcDiscovererPool`. The `issuer` value in the SSO config is reused as the JWT `iss` claim for matching with an existing `UserInfo` that has `jwt.oidc.issuer` set — this is how SSO couples to existing users.

---

## Coupling SSO to Existing Users / JWT

The `id_token` returned by the OIDC provider is a JWT. vmauth stores it in a cookie and, on each request, presents it to the existing `getJWTUserInfo` pipeline. That pipeline already:
- Discovers JWKS from the issuer.
- Verifies the signature.
- Checks the `exp` claim.
- Matches `match_claims` patterns.

So a user in `vmauth.yaml` that would normally accept a JWT from that OIDC issuer will automatically accept SSO-authenticated sessions too — no new user-matching logic is needed.

Example: a JWT user config that SSO sessions will match:

```yaml
users:
  - name: sso-users
    jwt:
      oidc:
        issuer: https://accounts.google.com
      match_claims:
        hd: mycompany\.com   # only @mycompany.com Google accounts
    url_prefix: http://backend:8428
```

---

## Files Changed

| File | Change |
|---|---|
| `app/vmauth/sso.go` | New file — all SSO logic |
| `app/vmauth/auth_config.go` | Add `SSO SSOConfig` to `AuthConfig`; call OIDC discovery in `parseAuthConfig` |
| `app/vmauth/main.go` | Callback route, cookie token injection, SSO redirect fallback |

---

## What is NOT in scope

- OAuth2 implicit / device / client\_credentials flows.
- PKCE (not needed for confidential server-side clients, keeps it simple).
- Logout / RP-initiated logout endpoint.
- Token refresh (user re-authenticates when `id_token` expires).
- Storing sessions in an external store (Redis, DB).
