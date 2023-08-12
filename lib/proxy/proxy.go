package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/fasthttp"
	"golang.org/x/net/proxy"
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
func (u *URL) SetHeaders(ac *promauth.Config, req *http.Request) {
	ah := u.getAuthHeader(ac)
	if ah != "" {
		req.Header.Set("Proxy-Authorization", ah)
	}
	ac.SetHeaders(req, false)
}

// SetFasthttpHeaders sets headers to req according to u and ac configs.
func (u *URL) SetFasthttpHeaders(ac *promauth.Config, req *fasthttp.Request) {
	ah := u.getAuthHeader(ac)
	if ah != "" {
		req.Header.Set("Proxy-Authorization", ah)
	}
	ac.SetFasthttpHeaders(req, false)
}

// getAuthHeader returns Proxy-Authorization auth header for the given u and ac.
func (u *URL) getAuthHeader(ac *promauth.Config) string {
	authHeader := ""
	if ac != nil {
		authHeader = ac.GetAuthHeader()
	}
	if u == nil || u.URL == nil {
		return authHeader
	}
	pu := u.URL
	if pu.User != nil && len(pu.User.Username()) > 0 {
		userPasswordEncoded := base64.StdEncoding.EncodeToString([]byte(pu.User.String()))
		authHeader = "Basic " + userPasswordEncoded
	}
	return authHeader
}

// MarshalYAML implements yaml.Marshaler interface.
func (u *URL) MarshalYAML() (interface{}, error) {
	if u.URL == nil {
		return nil, nil
	}
	return u.URL.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
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

// NewDialFunc returns dial func for the given u and ac.
func (u *URL) NewDialFunc(ac *promauth.Config) (fasthttp.DialFunc, error) {
	if u == nil || u.URL == nil {
		return defaultDialFunc, nil
	}
	pu := u.URL
	if !isURLSchemeValid(pu.Scheme) {
		return nil, fmt.Errorf("unknown scheme=%q for proxy_url=%q, must be in %s", pu.Scheme, pu.Redacted(), validURLSchemes)
	}
	isTLS := (pu.Scheme == "https" || pu.Scheme == "tls+socks5")
	proxyAddr := addMissingPort(pu.Host, isTLS)
	var tlsCfg *tls.Config
	if isTLS {
		tlsCfg = ac.NewTLSConfig()
		if !tlsCfg.InsecureSkipVerify && tlsCfg.ServerName == "" {
			tlsCfg.ServerName = tlsServerName(proxyAddr)
		}
	}
	if pu.Scheme == "socks5" || pu.Scheme == "tls+socks5" {
		return socks5DialFunc(proxyAddr, pu, tlsCfg)
	}
	dialFunc := func(addr string) (net.Conn, error) {
		proxyConn, err := defaultDialFunc(proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to proxy %q: %w", pu.Redacted(), err)
		}
		if isTLS {
			proxyConn = tls.Client(proxyConn, tlsCfg)
		}
		authHeader := u.getAuthHeader(ac)
		if authHeader != "" {
			authHeader = "Proxy-Authorization: " + authHeader + "\r\n"
			authHeader += ac.HeadersNoAuthString()
		}
		conn, err := sendConnectRequest(proxyConn, proxyAddr, addr, authHeader)
		if err != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("error when sending CONNECT request to proxy %q: %w", pu.Redacted(), err)
		}
		return conn, nil
	}
	return dialFunc, nil
}

func socks5DialFunc(proxyAddr string, pu *url.URL, tlsCfg *tls.Config) (fasthttp.DialFunc, error) {
	var sac *proxy.Auth
	if pu.User != nil {
		username := pu.User.Username()
		password, _ := pu.User.Password()
		sac = &proxy.Auth{
			User:     username,
			Password: password,
		}
	}
	network := netutil.GetTCPNetwork()
	var dialer proxy.Dialer = proxy.Direct
	if tlsCfg != nil {
		dialer = &tls.Dialer{
			Config: tlsCfg,
		}
	}
	d, err := proxy.SOCKS5(network, proxyAddr, sac, dialer)
	if err != nil {
		return nil, fmt.Errorf("cannot create socks5 proxy for url: %s, err: %w", pu.Redacted(), err)
	}
	dialFunc := func(addr string) (net.Conn, error) {
		return d.Dial(network, addr)
	}
	return dialFunc, nil
}

func addMissingPort(addr string, isTLS bool) string {
	if strings.IndexByte(addr, ':') >= 0 {
		return addr
	}
	port := "80"
	if isTLS {
		port = "443"
	}
	return addr + ":" + port
}

func tlsServerName(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func defaultDialFunc(addr string) (net.Conn, error) {
	network := netutil.GetTCPNetwork()
	// Do not use fasthttp.Dial because of https://github.com/VictoriaMetrics/VictoriaMetrics/issues/987
	return net.DialTimeout(network, addr, 5*time.Second)
}

// sendConnectRequest sends CONNECT request to proxyConn for the given addr and authHeader and returns the established connection to dstAddr.
func sendConnectRequest(proxyConn net.Conn, proxyAddr, dstAddr, authHeader string) (net.Conn, error) {
	req := "CONNECT " + dstAddr + " HTTP/1.1\r\nHost: " + proxyAddr + "\r\n" + authHeader + "\r\n"
	if _, err := proxyConn.Write([]byte(req)); err != nil {
		return nil, fmt.Errorf("cannot send CONNECT request for dstAddr=%q: %w", dstAddr, err)
	}
	var res fasthttp.Response
	res.SkipBody = true
	conn := &bufferedReaderConn{
		br:   bufio.NewReader(proxyConn),
		Conn: proxyConn,
	}
	if err := res.Read(conn.br); err != nil {
		return nil, fmt.Errorf("cannot read CONNECT response for dstAddr=%q: %w", dstAddr, err)
	}
	if statusCode := res.Header.StatusCode(); statusCode != 200 {
		return nil, fmt.Errorf("unexpected status code received: %d; want: 200; response body: %q", statusCode, res.Body())
	}
	return conn, nil
}

type bufferedReaderConn struct {
	net.Conn
	br *bufio.Reader
}

func (brc *bufferedReaderConn) Read(p []byte) (int, error) {
	return brc.br.Read(p)
}
