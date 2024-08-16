package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

var validURLSchemes = []string{"http", "https", "socks5", "tls+socks5"}

func isURLSchemeValid(scheme string) bool {
	for _, vs := range validURLSchemes {
		if scheme == vs {
			return true
		}
	}
	return false
}

// URL implements YAML.Marshaler and yaml.Unmarshaler interfaces for url.URL.
type URL struct {
	URL *url.URL
}

// MustNewURL returns new URL for the given u.
func MustNewURL(u string) *URL {
	pu, err := url.Parse(u)
	if err != nil {
		logger.Panicf("BUG: cannot parse u=%q: %s", u, err)
	}
	return &URL{
		URL: pu,
	}
}

// GetURL return the underlying url.
func (u *URL) GetURL() *url.URL {
	if u == nil || u.URL == nil {
		return nil
	}
	return u.URL
}

// IsHTTPOrHTTPS returns true if u is http or https
func (u *URL) IsHTTPOrHTTPS() bool {
	pu := u.GetURL()
	if pu == nil {
		return false
	}
	scheme := u.URL.Scheme
	return scheme == "http" || scheme == "https"
}

// String returns string representation of u.
func (u *URL) String() string {
	pu := u.GetURL()
	if pu == nil {
		return ""
	}
	return pu.String()
}

// SetHeaders sets headers to req according to u and ac configs.
func (u *URL) SetHeaders(ac *promauth.Config, req *http.Request) error {
	ah, err := u.getAuthHeader(ac)
	if err != nil {
		return fmt.Errorf("cannot obtain Proxy-Authorization headers: %w", err)
	}
	if ah != "" {
		req.Header.Set("Proxy-Authorization", ah)
	}
	return ac.SetHeaders(req, false)
}

// getAuthHeader returns Proxy-Authorization auth header for the given u and ac.
func (u *URL) getAuthHeader(ac *promauth.Config) (string, error) {
	authHeader := ""
	if ac != nil {
		var err error
		authHeader, err = ac.GetAuthHeader()
		if err != nil {
			return "", err
		}
	}
	if u == nil || u.URL == nil {
		return authHeader, nil
	}
	pu := u.URL
	if pu.User != nil && len(pu.User.Username()) > 0 {
		userPasswordEncoded := base64.StdEncoding.EncodeToString([]byte(pu.User.String()))
		authHeader = "Basic " + userPasswordEncoded
	}
	return authHeader, nil
}

// MarshalYAML implements yaml.Marshaler interface.
func (u *URL) MarshalYAML() (any, error) {
	if u.URL == nil {
		return nil, nil
	}
	return u.URL.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (u *URL) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsedURL, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot parse proxy_url=%q as *url.URL: %w", s, err)
	}
	if !isURLSchemeValid(parsedURL.Scheme) {
		return fmt.Errorf("cannot parse proxy_url=%q unsupported scheme format=%q, valid schemes: %s", s, parsedURL.Scheme, validURLSchemes)
	}
	u.URL = parsedURL
	return nil
}

// UnmarshalJSON implements json.Unmarshaller interface.
// required to properly clone internal representation of url
func (u *URL) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsedURL, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("cannot parse proxy_url=%q as *url.URL: %w", s, err)
	}
	if !isURLSchemeValid(parsedURL.Scheme) {
		return fmt.Errorf("cannot parse proxy_url=%q unsupported scheme format=%q, valid schemes: %s", s, parsedURL.Scheme, validURLSchemes)
	}
	u.URL = parsedURL
	return nil
}

// MarshalJSON implements json.Marshal interface.
// required to properly clone internal representation of url
func (u *URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.URL.String())
}
