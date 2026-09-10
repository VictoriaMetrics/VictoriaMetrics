package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SSOConfig holds the SSO configuration for a single host.
type SSOConfig struct {
	SrcHost       *Regex                `yaml:"src_host"`
	OpenIDConnect *SSOOIDCConnectConfig `yaml:"openid_connect"`
}

// SSOOIDCConnectConfig is the OpenID Connect configuration for SSO.
type SSOOIDCConnectConfig struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	// CookieSecret is used to sign the short-lived CSRF cookie set during the
	// authorization flow. Must be a random string; never shared with the IdP.
	CookieSecret string `yaml:"cookie_secret"`
	// CookieSecure controls the Secure flag on SSO cookies. Defaults to true.
	// Set to false only when vmauth is accessed over plain HTTP (e.g. local dev).
	// When vmauth runs behind an SSL-terminating proxy, keep this true — the
	// proxy speaks HTTPS to the browser even though vmauth sees plain HTTP.
	CookieSecure *bool `yaml:"cookie_secure,omitempty"`
	// RedirectURL is optional. Defaults to https://{host}/_vmauth/sso/callback.
	RedirectURL string `yaml:"redirect_url,omitempty"`
	// Scopes defaults to ["openid"] when not set.
	Scopes []string `yaml:"scopes,omitempty"`
}

// cookieSecure returns true unless CookieSecure is explicitly set to false.
func (c *SSOOIDCConnectConfig) cookieSecure() bool {
	return c.CookieSecure == nil || *c.CookieSecure
}

// validateSSOConfigs checks that all required fields are present in SSO configs.
func validateSSOConfigs(sso []*SSOConfig) error {
	for i, cfg := range sso {
		if cfg.SrcHost == nil {
			return fmt.Errorf("field sso.%d.src_host is required", i)
		}
		if cfg.OpenIDConnect == nil {
			return fmt.Errorf("field sso.%d.openid_connect is required", i)
		}
		oidc := cfg.OpenIDConnect
		if oidc.Issuer == "" {
			return fmt.Errorf("field sso.%d.openid_connect.issuer is required", i)
		}
		if oidc.ClientID == "" {
			return fmt.Errorf("field sso.%d.openid_connect.client_id is required", i)
		}
		if oidc.ClientSecret == "" {
			return fmt.Errorf("field sso.%d.openid_connect.client_secret is required", i)
		}
		if oidc.CookieSecret == "" {
			return fmt.Errorf("field sso.%d.openid_connect.cookie_secret is required", i)
		}
	}
	return nil
}

// getSSOConfigForHost returns the SSO host config for the given request host, or nil.
func getSSOConfigForHost(host string) (*SSOOIDCConnectConfig, *oidcProviderMetadata) {
	ac := authConfig.Load()
	if ac == nil || ac.SSO == nil {
		return nil, nil
	}
	for _, sso := range ac.SSO {
		if !sso.SrcHost.match(host) {
			continue
		}
		oidc := sso.OpenIDConnect
		return oidc, ac.oidcDP.getProviderMetadata(oidc.Issuer)
	}
	return nil, nil
}

const (
	ssoCookieName     = "_vmauth_sso"
	ssoCsrfCookieName = "_vmauth_sso_csrf"
	ssoCsrfCookieTTL  = 10 * time.Minute
)

// generateSSONonce generates a cryptographically random nonce for the SSO flow.
func generateSSONonce() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("cannot generate nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// signCSRFCookie returns a value for the CSRF cookie that carries both the nonce
// and the originalURL the user was trying to reach.
//
// Format: base64url(nonce ":" originalURL) "." base64url(HMAC-SHA256(cookieSecret, payload))
//
// The payload is base64url-encoded so that dots in the URL do not conflict with
// the "." separator between payload and signature.
func signCSRFCookie(nonce, originalURL, cookieSecret string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(nonce + ":" + originalURL))
	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyCSRFCookie verifies the CSRF cookie signature and returns the nonce and
// the original URL that was stored when the flow was initiated.
func verifyCSRFCookie(cookieValue, cookieSecret string) (nonce, originalURL string, err error) {
	dot := strings.LastIndexByte(cookieValue, '.')
	if dot < 0 {
		return "", "", fmt.Errorf("invalid CSRF cookie: missing separator")
	}
	payload, sig := cookieValue[:dot], cookieValue[dot+1:]

	mac := hmac.New(sha256.New, []byte(cookieSecret))
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", "", fmt.Errorf("CSRF cookie signature mismatch")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", "", fmt.Errorf("cannot decode CSRF cookie payload: %w", err)
	}
	// nonce is base64url (no colons), so the first ":" separates it from originalURL.
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid CSRF cookie payload format")
	}
	return parts[0], parts[1], nil
}

// processSSOLogin renders a minimal HTML page with a single "Login with SSO"
// button pointing directly to the OIDC provider's authorization endpoint.
func processSSOLogin(w http.ResponseWriter, r *http.Request) bool {
	oidc, pm := getSSOConfigForHost(r.Host)
	if oidc == nil {
		return false
	}
	if pm == nil {
		http.Error(w, "OIDC discovery not yet complete, try again shortly", http.StatusServiceUnavailable)
		return true
	}

	// Nonce usage follows recommendation from:
	// https://openid.net/specs/openid-connect-core-1_0.html#NonceNotes
	nonce, err := generateSSONonce()
	if err != nil {
		logger.Errorf("SSO: cannot generate nonce: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return true
	}

	originalURL := r.URL.RequestURI()
	// Reject absolute and protocol-relative URLs to prevent open redirect.
	if !strings.HasPrefix(originalURL, "/") || strings.HasPrefix(originalURL, "//") || strings.HasPrefix(originalURL, "/\\") {
		originalURL = "/"
	}

	// Store nonce + originalURL in the CSRF cookie. The raw nonce never leaves
	// the browser; only its SHA256 hash is sent to the IdP.
	http.SetCookie(w, &http.Cookie{
		Name:     ssoCsrfCookieName,
		Value:    signCSRFCookie(nonce, originalURL, oidc.CookieSecret),
		Path:     "/_vmauth/sso/",
		HttpOnly: true,
		Secure:   oidc.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ssoCsrfCookieTTL.Seconds()),
	})

	h := sha256.Sum256([]byte(nonce))
	nonceHash := base64.RawURLEncoding.EncodeToString(h[:])

	redirectURL := ssoRedirectURL(r, oidc)
	scopes := oidc.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid"}
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", oidc.ClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("nonce", nonceHash)
	authURL := pm.AuthorizationEndpoint + "?" + params.Encode()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	WriteSSOLoginPage(w, authURL)
	return true
}

// processSSOCallback handles the OIDC authorization code callback at /_vmauth/sso/callback.
func processSSOCallback(w http.ResponseWriter, r *http.Request) {
	oidc, pm := getSSOConfigForHost(r.Host)
	if oidc == nil {
		http.Error(w, "SSO not configured for this host", http.StatusNotFound)
		return
	}
	if pm == nil {
		http.Error(w, "OIDC discovery not yet complete, try again shortly", http.StatusServiceUnavailable)
		return
	}

	// Verify the CSRF cookie — it carries the nonce and the originalURL, and
	// binds this callback to the browser session that initiated the flow.
	csrfCookie, err := r.Cookie(ssoCsrfCookieName)
	if err != nil {
		http.Error(w, "missing CSRF cookie", http.StatusBadRequest)
		return
	}
	nonce, originalURL, err := verifyCSRFCookie(csrfCookie.Value, oidc.CookieSecret)
	if err != nil {
		logger.Warnf("SSO callback: invalid CSRF cookie from %s: %s", r.RemoteAddr, err)
		http.Error(w, "invalid CSRF cookie", http.StatusBadRequest)
		return
	}
	// Consume the CSRF cookie — it is single-use.
	http.SetCookie(w, &http.Cookie{
		Name:   ssoCsrfCookieName,
		Path:   "/_vmauth/sso/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code parameter", http.StatusBadRequest)
		return
	}

	idToken, err := exchangeCodeForIDToken(r.Context(), pm.TokenEndpoint, oidc, code, ssoRedirectURL(r, oidc))
	if err != nil {
		logger.Warnf("SSO callback: token exchange failed: %s", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	// OIDC Core §3.1.3.7: verify id_token nonce == SHA256(nonce from CSRF cookie)
	// to prevent id_token replay attacks.
	h := sha256.Sum256([]byte(nonce))
	nonceHash := base64.RawURLEncoding.EncodeToString(h[:])
	if err := verifyIDTokenNonce(idToken, nonceHash); err != nil {
		logger.Warnf("SSO callback: id_token nonce mismatch from %s: %s", r.RemoteAddr, err)
		http.Error(w, "invalid id_token nonce", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ssoCookieName,
		Value:    idToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   oidc.cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})

	if originalURL == "" {
		originalURL = "/"
	}
	http.Redirect(w, r, originalURL, http.StatusFound)
}

// verifyIDTokenNonce decodes the id_token JWT payload and checks that the nonce
// claim matches the expected value. The signature is validated separately by the
// existing JWT pipeline; this check only protects against id_token replay attacks
// (OIDC Core §3.1.3.7).
func verifyIDTokenNonce(idToken, expectedNonce string) error {
	// JWT format: header.payload.signature — all base64url encoded.
	parts := strings.SplitN(idToken, ".", 3)
	if len(parts) != 3 {
		return fmt.Errorf("id_token is not a valid JWT")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("cannot decode id_token payload: %w", err)
	}
	var claims struct {
		Nonce string `json:"nonce"`
	}
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return fmt.Errorf("cannot unmarshal id_token claims: %w", err)
	}
	if claims.Nonce != expectedNonce {
		return fmt.Errorf("nonce mismatch: got %q, want %q", claims.Nonce, expectedNonce)
	}
	return nil
}

type tokenResponse struct {
	IDToken string `json:"id_token"`
}

// exchangeCodeForIDToken exchanges the OIDC authorization code for an id_token.
func exchangeCodeForIDToken(ctx context.Context, tokenEndpoint string, oidc *SSOOIDCConnectConfig, code, redirectURL string) (string, error) {
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

	body, err := io.ReadAll(resp.Body)
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

// handleSSOLogout clears the SSO session cookie and redirects to the root.
func handleSSOLogout(w http.ResponseWriter, r *http.Request) {
	oidc, _ := getSSOConfigForHost(r.Host)
	secure := oidc != nil && oidc.cookieSecure()
	http.SetCookie(w, &http.Cookie{
		Name:     ssoCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

// ssoAuthTokenFromRequest extracts the SSO session cookie and returns it as
// a Bearer auth token string compatible with the existing JWT pipeline.
func ssoAuthTokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(ssoCookieName)
	if err != nil || c.Value == "" {
		return ""
	}
	return "http_auth:Bearer " + c.Value
}

// ssoRedirectURL returns the OIDC redirect URL for the current request.
func ssoRedirectURL(r *http.Request, oidc *SSOOIDCConnectConfig) string {
	if oidc.RedirectURL != "" {
		return oidc.RedirectURL
	}
	scheme := "http"
	if oidc.cookieSecure() {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/_vmauth/sso/callback"
}
