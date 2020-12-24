package proxy

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/fasthttp"
)

// URL implements YAML.Marshaler and yaml.Unmarshaler interfaces for url.URL.
type URL struct {
	url *url.URL
}

// URL return the underlying url.
func (u *URL) URL() *url.URL {
	if u == nil || u.url == nil {
		return nil
	}
	return u.url
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

// NewDialFunc returns dial func for the given pu and tlsConfig.
func (u *URL) NewDialFunc(tlsConfig *tls.Config) (fasthttp.DialFunc, error) {
	if u == nil || u.url == nil {
		return defaultDialFunc, nil
	}
	pu := u.url
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return nil, fmt.Errorf("unknown scheme=%q for proxy_url=%q, must be http or https", pu.Scheme, pu)
	}
	var authHeader string
	if pu.User != nil && len(pu.User.Username()) > 0 {
		userPasswordEncoded := base64.StdEncoding.EncodeToString([]byte(pu.User.String()))
		authHeader = "Proxy-Authorization: Basic " + userPasswordEncoded + "\r\n"
	}
	dialFunc := func(addr string) (net.Conn, error) {
		proxyConn, err := defaultDialFunc(pu.Host)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to proxy %q: %w", pu, err)
		}
		if pu.Scheme == "https" {
			proxyConn = tls.Client(proxyConn, tlsConfig)
		}
		conn, err := sendConnectRequest(proxyConn, addr, authHeader)
		if err != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("error when sending CONNECT request to proxy %q: %w", pu, err)
		}
		return conn, nil
	}
	return dialFunc, nil
}

func defaultDialFunc(addr string) (net.Conn, error) {
	if netutil.TCP6Enabled() {
		return fasthttp.DialDualStack(addr)
	}
	return fasthttp.Dial(addr)
}

// sendConnectRequest sends CONNECT request to proxyConn for the given addr and authHeader and returns the established connection to dstAddr.
func sendConnectRequest(proxyConn net.Conn, dstAddr, authHeader string) (net.Conn, error) {
	req := "CONNECT " + dstAddr + " HTTP/1.1\r\nHost: " + dstAddr + "\r\n" + authHeader + "\r\n"
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
