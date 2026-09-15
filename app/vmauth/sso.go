package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// ssoConfig holds the SSO configuration for a single host.
type ssoConfig struct {
	SrcHost *Regex         `yaml:"src_host"`
	OIDC    *ssoOIDCConfig `yaml:"oidc"`
}

func (c *ssoConfig) validate() error {
	var res error
	if c.SrcHost == nil {
		res = errors.Join(res, fmt.Errorf("src_host is required"))
	}
	if c.OIDC == nil {
		res = errors.Join(res, fmt.Errorf("openid_connect is required"))
		return res
	}

	oidc := c.OIDC
	if oidc.Issuer == "" {
		res = errors.Join(res, fmt.Errorf("openid_connect.issuer is required"))
	}
	if oidc.ClientID == "" {
		res = errors.Join(res, fmt.Errorf("openid_connect.client_id is required"))
	}
	if oidc.ClientSecret == "" {
		res = errors.Join(res, fmt.Errorf("openid_connect.client_secret is required"))
	}
	if len(oidc.CookieSecret) < 16 {
		res = errors.Join(res, fmt.Errorf("openid_connect.cookie_secret must be at least 16 characters long"))
	}

	// openid scope MUST be present per
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthRequestValidation
	if !slices.Contains(oidc.Scopes, "openid") {
		oidc.Scopes = append([]string{"openid"}, oidc.Scopes...)
	}

	const defaultSessionDuration = 10 * time.Minute
	if oidc.SessionDuration == "" {
		oidc.sessionDuration = defaultSessionDuration
	} else {
		d, err := time.ParseDuration(oidc.SessionDuration)
		if err != nil {
			res = errors.Join(res, fmt.Errorf("openid_connect.session_duration: %w", err))
		} else if d < 0 {
			res = errors.Join(res, fmt.Errorf("openid_connect.session_duration must not be negative"))
		} else {
			oidc.sessionDuration = d
		}
	}

	return res
}

// validateSSOConfigs checks that all required fields are present in SSO configs.
func validateSSOConfigs(sso []*ssoConfig) error {
	for i, sso := range sso {
		if err := sso.validate(); err != nil {
			return fmt.Errorf("sso.%d: %w", i, err)
		}
	}
	return nil
}

// ssoOIDCConfig is the OpenID Connect configuration for SSO.
type ssoOIDCConfig struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`

	// CookieSecret is used to sign the short-lived CSRF cookie set during the
	// authorization flow. Must be at least 16 characters; never shared with the IdP.
	// Generate with: openssl rand -base64 32
	CookieSecret string `yaml:"cookie_secret"`

	// CookieSecure controls the Secure flag on SSO cookies. Defaults to true.
	// Set to false only when vmauth is accessed over plain HTTP (e.g. local dev).
	// When vmauth runs behind an SSL-terminating proxy, keep this true — the
	// proxy speaks HTTPS to the browser even though vmauth sees plain HTTP.
	CookieSecure *bool `yaml:"cookie_secure,omitempty"`

	// Scopes defaults to ["openid"] when not set.
	Scopes []string `yaml:"scopes,omitempty"`

	// SessionDuration caps the SSO session cookie lifetime.
	// The cookie MaxAge is the minimum of this value and the id_token's exp claim.
	// Defaults to 10m when not set. Parsed via time.ParseDuration, e.g. "10m", "1h".
	SessionDuration string `yaml:"session_duration,omitempty"`
	sessionDuration time.Duration

	// DefaultRedirectURL is the URL users are sent to after SSO login when the
	// original request URL fails open-redirect validation (e.g. absolute or
	// protocol-relative path). Defaults to "/".
	DefaultRedirectURL string `yaml:"default_redirect_url,omitempty"`

	pm atomic.Pointer[oidcProviderMetadata]
}

// cookieSecure returns true unless CookieSecure is explicitly set to false.
func (c *ssoOIDCConfig) cookieSecure() bool {
	return c.CookieSecure == nil || *c.CookieSecure
}

// getSessionDuration returns the SSO session duration as the minimum of the
// configured sessionDuration and the token's remaining lifetime.
// If sessionDuration is 0, the token expiry is used as-is.
func (c *ssoOIDCConfig) getSessionDuration(tokenExpiresAt time.Time) time.Duration {
	ttl := time.Until(tokenExpiresAt)
	if c.sessionDuration > 0 && c.sessionDuration < ttl {
		ttl = c.sessionDuration
	}
	if ttl < 0 {
		ttl = 0
	}
	return ttl
}

// getCallbackURL returns the OIDC redirect URL for the current request.
// The host must be already validated by sso.src_host regexp
func (c *ssoOIDCConfig) getCallbackURL(host string) string {
	scheme := "http"
	if c.cookieSecure() {
		scheme = "https"
	}
	return scheme + "://" + host + getPathWithPrefix("/_vmauth/sso/callback")
}

var (
	// Used to check final redirects are not susceptible to open redirects.
	// Matches //, /\ and both of these with whitespace in between (eg / / or / \).
	// Copy-pasted from oauth2-proxy
	// https://github.com/oauth2-proxy/oauth2-proxy/blob/6420aae79003dfb47885018856dd524342367dfc/pkg/app/redirect/validator.go#L16
	invalidRedirectRegex = regexp.MustCompile(`[/\\](?:[\s\v]*|\.{1,2})[/\\]`)
)

// getRedirectURL sanitizes the redirect URL to prevent open redirect attacks.
// Returns DefaultRedirectURL (or "/") if the URL is not a safe relative path.
func (c *ssoOIDCConfig) getRedirectURL(redirect string) string {
	// Copy-pated from oauth2-proxy
	// https://github.com/oauth2-proxy/oauth2-proxy/blob/6420aae79003dfb47885018856dd524342367dfc/pkg/app/redirect/validator.go#L47
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") && !invalidRedirectRegex.MatchString(redirect) {
		return getPathWithPrefix(redirect)
	}
	if c.DefaultRedirectURL != "" {
		return getPathWithPrefix(c.DefaultRedirectURL)
	}
	return getPathWithPrefix("/")
}

// getSSOConfigForHost returns the SSO host config for the given request host, or nil.
func getSSOConfigForHost(host string) *ssoOIDCConfig {
	ac := authConfig.Load()
	if ac == nil || ac.SSO == nil {
		return nil
	}
	for _, sso := range ac.SSO {
		if sso.SrcHost.match(host) {
			return sso.OIDC
		}
	}
	return nil
}

const (
	ssoCookieName     = "_vmauth_sso"
	ssoCsrfCookieName = "_vmauth_sso_csrf"
	ssoCsrfCookieTTL  = 10 * time.Minute
)

var ssoLogger = logger.WithThrottler("sso", 5*time.Second)

// getPathWithPrefix prepends -http.pathPrefix to the given path.
// The browser sees the full external URL (including the prefix), so cookie
// paths and redirect URIs must include it.
func getPathWithPrefix(p string) string {
	return strings.TrimSuffix(httpserver.GetPathPrefix(), "/") + p
}

// generateRandomString generates a cryptographically random base64url-encoded string of the given byte length.
func generateRandomString(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cannot generate random string: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// signCSRFCookie returns a value for the CSRF cookie that carries the nonce,
// state, and the redirectURL the user was trying to reach.
//
// Format: base64url(nonce ":" state ":" redirectURL) "." base64url(HMAC-SHA256(cookieSecret, payload))
//
// The payload is base64url-encoded so that dots in the URL do not conflict with
// the "." separator between payload and signature.
// nonce and state are independent base64url strings (no colons), so the first
// two ":" delimiters are unambiguous.
func signCSRFCookie(nonce, state, redirectURL, cookieSecret string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(nonce + ":" + state + ":" + redirectURL))
	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyCSRFCookie verifies the CSRF cookie signature and returns the nonce,
// state, and the original URL that were stored when the flow was initiated.
func verifyCSRFCookie(cookieValue, cookieSecret string) (nonce, state, redirectURL string, err error) {
	dot := strings.LastIndexByte(cookieValue, '.')
	if dot < 0 {
		return "", "", "", fmt.Errorf("invalid CSRF cookie: missing separator")
	}
	payload, sig := cookieValue[:dot], cookieValue[dot+1:]

	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", "", "", fmt.Errorf("CSRF cookie signature mismatch")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", "", fmt.Errorf("cannot decode CSRF cookie payload: %w", err)
	}
	// nonce and state are base64url (no colons), so the first two ":"
	// separate them from each other and from redirectURL.
	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid CSRF cookie payload format")
	}
	return parts[0], parts[1], parts[2], nil
}

// setSSONoCacheHeaders sets Content-Type and no-cache headers on SSO responses.
// Login and callback pages must not be cached because they contain CSRF tokens
// and auth state that are valid for a single flow.
//
// No-cache headers follow oauth2-proxy convention:
// https://github.com/oauth2-proxy/oauth2-proxy/blob/33c2eb92dea78204f7a18bc2dfdbccc220f39257/oauthproxy.go#L1089
func setSSONoCacheHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-store, must-revalidate, max-age=0")
	h.Set("Expires", "Thu, 01 Jan 1970 00:00:00 GMT")
	h.Set("X-Accel-Expires", "0")
}

// processSSOLogin renders a minimal HTML page with a single "Login with SSO"
// button pointing directly to the OIDC provider's authorization endpoint.
// Only GET and HEAD requests are redirected to the IdP; other methods receive
// a 401 so that the caller's request body is not silently discarded.
//
// If the request already carries auth tokens (e.g. from an SSO cookie) but
// no user config matched, the page shows an "Access Denied" hint above the
// login button so the user knows their identity was recognized but not authorized.
func processSSOLogin(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	oidc := getSSOConfigForHost(r.Host)
	if oidc == nil {
		return false
	}
	redirectURL := oidc.getRedirectURL(r.URL.RequestURI())

	pm := oidc.pm.Load()
	if pm == nil {
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusServiceUnavailable)
		WriteSSOErrorPage(w, "Identity Provider is not available, try again later", "", redirectURL)
		return true
	}

	// Nonce binds the id_token to this session (replay protection).
	// https://openid.net/specs/openid-connect-core-1_0.html#NonceNotes
	nonce, err := generateRandomString(32)
	if err != nil {
		ssoLogger.Errorf("generate nonce failed: %s", err)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusInternalServerError)
		WriteSSOErrorPage(w, "Internal Server Error", "", redirectURL)
		return true
	}

	// State binds the authorization response to this request (CSRF protection).
	// State should differ from nonce as per OIDC best practices.
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthRequest
	state, err := generateRandomString(32)
	if err != nil {
		ssoLogger.Errorf("generate state failed: %s", err)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusInternalServerError)
		WriteSSOErrorPage(w, "Internal Server Error", "", redirectURL)
		return true
	}

	// Store nonce, state, and redirectURL in the CSRF cookie. The raw nonce
	// never leaves the browser; only its SHA256 hash is sent to the IdP.
	// The state is sent as-is to the IdP and verified on callback.
	http.SetCookie(w, &http.Cookie{
		Name:     ssoCsrfCookieName,
		Value:    signCSRFCookie(nonce, state, redirectURL, oidc.CookieSecret),
		Path:     getPathWithPrefix("/_vmauth/sso/"),
		HttpOnly: true,
		Secure:   oidc.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ssoCsrfCookieTTL.Seconds()),
	})

	h := sha256.Sum256([]byte(nonce))
	nonceHash := base64.RawURLEncoding.EncodeToString(h[:])

	callbackURL := oidc.getCallbackURL(r.Host)
	scopes := oidc.Scopes

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", oidc.ClientID)
	params.Set("redirect_uri", callbackURL)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("nonce", nonceHash)
	params.Set("state", state)
	authURL := pm.AuthorizationEndpoint + "?" + params.Encode()

	setSSONoCacheHeaders(w)
	if len(getAuthTokensFromRequest(r)) > 0 {
		// The user authenticated but no user config matched — authorization failure.
		w.WriteHeader(http.StatusForbidden)
		WriteSSOLoginPage(w, authURL, "Access Denied")
		return true
	}

	// No credentials at all — return 401 so programmatic clients (curl,
	// Grafana, scripts) can distinguish "not authenticated" from a successful
	// response.
	w.WriteHeader(http.StatusUnauthorized)
	WriteSSOLoginPage(w, authURL, "")
	return true
}

// processSSOCallback handles the OIDC authorization code callback at /_vmauth/sso/callback.
func processSSOCallback(w http.ResponseWriter, r *http.Request) {
	oidc := getSSOConfigForHost(r.Host)
	if oidc == nil {
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		WriteSSOErrorPage(w, "SSO not configured for this host", ``, "/")
		return
	}
	pm := oidc.pm.Load()
	if pm == nil {
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusServiceUnavailable)
		WriteSSOErrorPage(w, "Identity Provider is not available, try again later", "", "/")
		return
	}

	// Verify the CSRF cookie — it carries the nonce and the redirectURL, and
	// binds this callback to the browser session that initiated the flow.
	csrfCookie, err := r.Cookie(ssoCsrfCookieName)
	if err != nil {
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		WriteSSOErrorPage(w, "Missing CSRF Cookie", "", "/")
		return
	}
	// Clear the CSRF cookie immediately so it cannot be replayed.
	http.SetCookie(w, &http.Cookie{
		Name:     ssoCsrfCookieName,
		Path:     getPathWithPrefix("/_vmauth/sso/"),
		HttpOnly: true,
		Secure:   oidc.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	nonce, state, redirectURL, err := verifyCSRFCookie(csrfCookie.Value, oidc.CookieSecret)
	redirectURL = oidc.getRedirectURL(redirectURL)
	if err != nil {
		ssoLogger.Warnf("SSO callback: invalid CSRF cookie from %s: %s", r.RemoteAddr, err)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		WriteSSOErrorPage(w, "Invalid CSRF Cookie", "", "/")
		return
	}

	// Verify the state parameter matches the value we sent — this binds the
	// callback to the specific authorization request (CSRF protection).
	if returnedState := r.URL.Query().Get("state"); returnedState != state {
		ssoLogger.Warnf("SSO callback: state mismatch from %s", r.RemoteAddr)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		WriteSSOErrorPage(w, "Invalid state parameter", "", redirectURL)
		return
	}

	// Handle Authentication Error Response per
	// https://openid.net/specs/openid-connect-core-1_0.html#AuthResponseValidation
	if errCode := r.URL.Query().Get("error"); errCode != "" {
		errDescription := r.URL.Query().Get("error_description")
		ssoLogger.Warnf("SSO callback: IdP returned error %q (%s) for %s", errCode, errDescription, r.RemoteAddr)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		WriteSSOErrorPage(w, errCode, errDescription, redirectURL)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		WriteSSOErrorPage(w, "Missing code parameter", "", redirectURL)
		return
	}

	idToken, err := exchangeCodeForIDToken(r.Context(), pm.TokenEndpoint, oidc, code, oidc.getCallbackURL(r.Host))
	if err != nil {
		ssoLogger.Warnf("SSO callback: token exchange failed: %s", err)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusBadRequest)
		WriteSSOErrorPage(w, "Token exchange failed", "", redirectURL)
		return
	}

	// Compute nonceHash from the raw nonce stored in the cookie.
	// Only the hash was sent to the IdP as the nonce claim in the id_token.
	h := sha256.Sum256([]byte(nonce))
	nonceHash := base64.RawURLEncoding.EncodeToString(h[:])

	expiresAt, err := validateIDToken(idToken, pm, oidc.ClientID, nonceHash)
	if err != nil {
		ssoLogger.Warnf("SSO callback: id_token verification failed from %s: %s", r.RemoteAddr, err)
		setSSONoCacheHeaders(w)
		w.WriteHeader(http.StatusUnauthorized)
		WriteSSOErrorPage(w, "Token verification failed", "", redirectURL)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ssoCookieName,
		Value:    idToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   oidc.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(oidc.getSessionDuration(expiresAt).Seconds()),
	})

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// validateIDToken parses and validates the id_token JWT:
// verifies the signature using the provider's public keys, checks expiry, issuer,
// audience (client_id), and confirms the nonce claim matches expectedNonce to prevent replay attacks.
// Returns the token expiration time on success.
// See https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation
func validateIDToken(idToken string, pm *oidcProviderMetadata, clientID, expectedNonce string) (time.Time, error) {
	tkn := getToken()
	if err := tkn.Parse(idToken, false); err != nil {
		return time.Time{}, fmt.Errorf("cannot parse id_token: %w", err)
	}
	defer putToken(tkn)

	if err := pm.vp.Verify(tkn); err != nil {
		return time.Time{}, fmt.Errorf("signature verification failed: %w", err)
	}
	if tkn.IsExpired(time.Now()) {
		return time.Time{}, fmt.Errorf("id_token is expired")
	}
	if tkn.Issuer() != pm.Issuer {
		return time.Time{}, fmt.Errorf("issuer mismatch: got %q, want %q", tkn.Issuer(), pm.Issuer)
	}
	// The aud claim MUST contain the client_id per OIDC Core.
	// Verifying it prevents accepting tokens issued for a different client of the same IdP.
	// See step 3 in
	// https://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation
	audClaim, err := jwt.NewClaim("aud", clientID)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot build aud claim: %w", err)
	}
	if !tkn.MatchClaims([]*jwt.Claim{audClaim}) {
		return time.Time{}, fmt.Errorf("audience mismatch: token not issued for client_id %q", clientID)
	}
	nonceClaim, err := jwt.NewClaim("nonce", expectedNonce)
	if err != nil {
		return time.Time{}, fmt.Errorf("cannot build nonce claim: %w", err)
	}
	if !tkn.MatchClaims([]*jwt.Claim{nonceClaim}) {
		return time.Time{}, fmt.Errorf("nonce mismatch")
	}
	return tkn.ExpiresAt(), nil
}

type tokenResponse struct {
	IDToken string `json:"id_token"`
}

// exchangeCodeForIDToken exchanges the OIDC authorization code for an id_token.
func exchangeCodeForIDToken(ctx context.Context, tokenEndpoint string, oidc *ssoOIDCConfig, code, redirectURL string) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	params.Set("redirect_uri", redirectURL)
	params.Set("client_id", oidc.ClientID)
	params.Set("client_secret", oidc.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("cannot create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("cannot read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, body)
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("cannot unmarshal token response: %w", err)
	}
	if tr.IDToken == "" {
		return "", fmt.Errorf("token response missing id_token")
	}
	return tr.IDToken, nil
}

// ssoAuthTokenFromRequest extracts the SSO session cookie and returns it as
// a Bearer auth token string compatible with the existing JWT pipeline.
func getSSOAuthTokensFromRequest(r *http.Request) []string {
	ac := authConfig.Load()
	if ac.SSO == nil {
		return nil
	}

	c, err := r.Cookie(ssoCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	return []string{"http_auth:Bearer " + c.Value}
}
