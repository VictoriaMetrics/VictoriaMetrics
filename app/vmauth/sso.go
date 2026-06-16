package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SSOConfig maps hostname to its SSO configuration.
type SSOConfig map[string]*SSOHostConfig

// SSOHostConfig holds the SSO configuration for a single host.
type SSOHostConfig struct {
	OpenIDConnect *OIDCConnectConfig `yaml:"openid_connect"`
}

// OIDCConnectConfig is the OpenID Connect configuration for SSO.
type OIDCConnectConfig struct {
	Issuer       string `yaml:"issuer"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	// RedirectURL is optional. Defaults to https://{host}/_vmauth/sso/callback.
	RedirectURL string `yaml:"redirect_url,omitempty"`
	// Scopes defaults to ["openid"] when not set.
	Scopes []string `yaml:"scopes,omitempty"`

	// filled from OIDC discovery at init time
	authEndpoint  string
	tokenEndpoint string
}

// validateSSOConfigs checks that all required fields are present in SSO configs.
func validateSSOConfigs(sso SSOConfig) error {
	for host, cfg := range sso {
		if cfg.OpenIDConnect == nil {
			return fmt.Errorf("missing openid_connect config for sso host %q", host)
		}
		oidc := cfg.OpenIDConnect
		if oidc.Issuer == "" {
			return fmt.Errorf("missing issuer in openid_connect config for sso host %q", host)
		}
		if oidc.ClientID == "" {
			return fmt.Errorf("missing client_id in openid_connect config for sso host %q", host)
		}
		if oidc.ClientSecret == "" {
			return fmt.Errorf("missing client_secret in openid_connect config for sso host %q", host)
		}
	}
	return nil
}

// ssoConfigForHost returns the SSO host config for the given request host, or nil.
func ssoConfigForHost(host string) *SSOHostConfig {
	// Strip port, e.g. "foo.com:8427" -> "foo.com"
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	ac := authConfig.Load()
	if ac == nil || ac.SSO == nil {
		log.Println(21)
		return nil
	}
	ssoh := ac.SSO[host]
	if ssoh == nil || ssoh.OpenIDConnect == nil {
		log.Println(22)
		return nil
	}

	oidcCfg := ac.oidcDP.openIDConfig(ssoh.OpenIDConnect.Issuer)
	if oidcCfg == nil {
		log.Println(24)
		return nil
	}
	ssoh.OpenIDConnect.authEndpoint = oidcCfg.AuthorizationEndpoint
	ssoh.OpenIDConnect.tokenEndpoint = oidcCfg.TokenEndpoint

	log.Println(25)
	return ssoh
}

// ssoStatePayload is the CSRF state payload embedded in the OIDC state parameter.
type ssoStatePayload struct {
	Nonce       string `json:"n"`
	OriginalURL string `json:"u"`
	IssuedAt    int64  `json:"t"`
}

const (
	ssoStateTTL   = 10 * time.Minute
	ssoCookieName = "_vmauth_sso"
)

// buildSSOState builds a signed, self-contained state value safe to use across
// multiple vmauth instances behind a load balancer.
//
// Format: base64url(JSON(payload)) "." base64url(HMAC-SHA256(clientSecret, payload))
func buildSSOState(originalURL, clientSecret string) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("cannot generate nonce: %w", err)
	}
	p := ssoStatePayload{
		Nonce:       base64.RawURLEncoding.EncodeToString(nonce),
		OriginalURL: originalURL,
		IssuedAt:    time.Now().Unix(),
	}
	payloadJSON, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)
	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(payloadEnc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payloadEnc + "." + sig, nil
}

// verifySSOState verifies the state signature and expiry, returning the original URL.
func verifySSOState(state, clientSecret string) (string, error) {
	dot := strings.LastIndexByte(state, '.')
	if dot < 0 {
		return "", fmt.Errorf("invalid state: missing separator")
	}
	payloadEnc := state[:dot]
	sig := state[dot+1:]

	mac := hmac.New(sha256.New, []byte(clientSecret))
	mac.Write([]byte(payloadEnc))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid state signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadEnc)
	if err != nil {
		return "", fmt.Errorf("cannot decode state payload: %w", err)
	}
	var p ssoStatePayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return "", fmt.Errorf("cannot unmarshal state payload: %w", err)
	}
	if time.Since(time.Unix(p.IssuedAt, 0)) > ssoStateTTL {
		return "", fmt.Errorf("state expired")
	}
	return p.OriginalURL, nil
}

var ssoLoginPageTmpl = template.Must(template.New("sso_login").Parse(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Login</title></head>
<body>
<p><a href="{{.}}">Login with SSO</a></p>
</body>
</html>`))

// showSSOLoginPage renders a minimal HTML page with a single "Login with SSO"
// button pointing directly to the OIDC provider's authorization endpoint.
func showSSOLoginPage(w http.ResponseWriter, r *http.Request, cfg *SSOHostConfig) {
	oidc := cfg.OpenIDConnect
	if oidc == nil || oidc.authEndpoint == "" {
		http.Error(w, "SSO not properly configured for this host", http.StatusInternalServerError)
		return
	}

	state, err := buildSSOState(r.RequestURI, oidc.ClientSecret)
	if err != nil {
		logger.Errorf("SSO: cannot build state: %s", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

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
	params.Set("state", state)
	authURL := oidc.authEndpoint + "?" + params.Encode()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := ssoLoginPageTmpl.Execute(w, authURL); err != nil {
		logger.Errorf("SSO: cannot render login page: %s", err)
	}
}

// handleSSOCallback handles the OIDC authorization code callback at /_vmauth/sso/callback.
func handleSSOCallback(w http.ResponseWriter, r *http.Request) {
	cfg := ssoConfigForHost(r.Host)
	if cfg == nil || cfg.OpenIDConnect == nil {
		http.Error(w, "SSO not configured for this host", http.StatusBadRequest)
		return
	}
	oidc := cfg.OpenIDConnect

	q := r.URL.Query()

	state := q.Get("state")
	if state == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}
	originalURL, err := verifySSOState(state, oidc.ClientSecret)
	if err != nil {
		logger.Warnf("SSO callback: invalid state from %s: %s", r.RemoteAddr, err)
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	code := q.Get("code")
	if code == "" {
		http.Error(w, "missing code parameter", http.StatusBadRequest)
		return
	}

	idToken, err := exchangeCodeForIDToken(r.Context(), oidc, code, ssoRedirectURL(r, oidc))
	if err != nil {
		logger.Warnf("SSO callback: token exchange failed: %s", err)
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     ssoCookieName,
		Value:    idToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})

	if originalURL == "" {
		originalURL = "/"
	}
	http.Redirect(w, r, originalURL, http.StatusFound)
}

type tokenResponse struct {
	IDToken string `json:"id_token"`
}

// exchangeCodeForIDToken exchanges the OIDC authorization code for an id_token.
func exchangeCodeForIDToken(ctx context.Context, oidc *OIDCConnectConfig, code, redirectURL string) (string, error) {
	params := url.Values{}
	params.Set("grant_type", "authorization_code")
	params.Set("code", code)
	params.Set("redirect_uri", redirectURL)
	params.Set("client_id", oidc.ClientID)
	params.Set("client_secret", oidc.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oidc.tokenEndpoint, strings.NewReader(params.Encode()))
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
func ssoRedirectURL(r *http.Request, oidc *OIDCConnectConfig) string {
	if oidc.RedirectURL != "" {
		return oidc.RedirectURL
	}
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return scheme + "://" + r.Host + "/_vmauth/sso/callback"
}
