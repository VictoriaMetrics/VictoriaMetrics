package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/fasthttp"
)

// URL implements YAML.Marshaler and yaml.Unmarshaler interfaces for url.URL.
type URL struct {
	url *url.URL
}

// MustNewURL returns new URL for the given u.
func MustNewURL(u string) URL {
	pu, err := url.Parse(u)
	if err != nil {
		logger.Panicf("BUG: cannot parse u=%q: %s", u, err)
	}
	return URL{
		url: pu,
	}
}

// URL return the underlying url.
func (u *URL) URL() *url.URL {
	if u == nil || u.url == nil {
		return nil
	}
	return u.url
}

// String returns string representation of u.
func (u *URL) String() string {
	pu := u.URL()
	if pu == nil {
		return ""
	}
	return pu.String()
}

// MarshalYAML implements yaml.Marshaler interface.
func (u *URL) MarshalYAML() (interface{}, error) {
	if u.url == nil {
		return nil, nil
	}
	return u.url.String(), nil
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
	u.url = parsedURL
	return nil
}

// NewDialFunc returns dial func for the given u and ac.
func (u *URL) NewDialFunc(ac *promauth.Config) (fasthttp.DialFunc, error) {
	if u == nil || u.url == nil {
		return defaultDialFunc, nil
	}
	pu := u.url
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return nil, fmt.Errorf("unknown scheme=%q for proxy_url=%q, must be http or https", pu.Scheme, pu.Redacted())
	}
	isTLS := pu.Scheme == "https"
	proxyAddr := addMissingPort(pu.Host, isTLS)
	var authHeader string
	if ac != nil {
		authHeader = ac.Authorization
	}
	if pu.User != nil && len(pu.User.Username()) > 0 {
		userPasswordEncoded := base64.StdEncoding.EncodeToString([]byte(pu.User.String()))
		authHeader = "Basic " + userPasswordEncoded
	}
	if authHeader != "" {
		authHeader = "Proxy-Authorization: " + authHeader + "\r\n"
	}
	var tlsCfg *tls.Config
	if isTLS {
		tlsCfg = ac.NewTLSConfig()
		if !tlsCfg.InsecureSkipVerify && tlsCfg.ServerName == "" {
			tlsCfg.ServerName = tlsServerName(proxyAddr)
		}
	}
	dialFunc := func(addr string) (net.Conn, error) {
		proxyConn, err := defaultDialFunc(proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to proxy %q: %w", pu.Redacted(), err)
		}
		if isTLS {
			proxyConn = tls.Client(proxyConn, tlsCfg)
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
		return nil, fmt.Errorf("unexpected status code received: %d; want: 200", statusCode)
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
