package netutil

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/fasthttp"
)

// ProxyURL implements marshal interfaces for url.URL.
type ProxyURL struct {
	url *url.URL
}

// URL returns *url.URL.
func (pu ProxyURL) URL() *url.URL {
	return pu.url
}

// String implements String interface.
func (pu ProxyURL) String() string {
	if pu.url == nil {
		return ""
	}
	return pu.url.String()
}

// MarshalYAML implements yaml.Marshaler interface.
func (pu ProxyURL) MarshalYAML() (interface{}, error) {
	if pu.url == nil {
		return nil, nil
	}
	return pu.url.String(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (pu *ProxyURL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsedURL, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("failed parse proxy_url=%q as *url.URL, err=%w", s, err)
	}
	pu.url = parsedURL
	return nil
}

// GetProxyDialFunc returns dial proxy func for the given proxy url.
// currently only http based proxy is supported.
func GetProxyDialFunc(proxyURL *url.URL) (fasthttp.DialFunc, error) {
	if strings.HasPrefix(proxyURL.Scheme, "http") {
		return httpProxy(proxyURL.Host, MakeBasicAuthHeader(nil, proxyURL)), nil
	}
	return nil, fmt.Errorf("unknown scheme=%q for proxy_url: %q, must be http or https", proxyURL.Scheme, proxyURL)
}

func httpProxy(proxyAddr string, auth []byte) fasthttp.DialFunc {
	return func(addr string) (net.Conn, error) {
		var (
			conn net.Conn
			err  error
		)
		if TCP6Enabled() {
			conn, err = fasthttp.DialDualStack(proxyAddr)
		} else {
			conn, err = fasthttp.Dial(proxyAddr)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot connect to the proxy=%q,err=%w", proxyAddr, err)
		}
		if err := MakeProxyConnectCall(conn, []byte(addr), auth); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return conn, nil
	}
}

// MakeBasicAuthHeader encodes and writes basic auth http header from url into given dst and returns it.
func MakeBasicAuthHeader(dst []byte, url *url.URL) []byte {
	if url == nil || url.User == nil {
		return dst
	}
	if len(url.User.Username()) > 0 {
		dst = append(dst, "Proxy-Authorization: Basic "...)
		dst = append(dst, base64.StdEncoding.EncodeToString([]byte(url.User.String()))...)
	}
	return dst
}

// MakeProxyConnectCall execute CONNECT method to proxy with given destination address.
func MakeProxyConnectCall(conn net.Conn, dstAddr, auth []byte) error {
	conReq := make([]byte, 0, 10)
	conReq = append(conReq, []byte("CONNECT ")...)
	conReq = append(conReq, dstAddr...)
	conReq = append(conReq, []byte(" HTTP/1.1\r\n")...)
	if len(auth) > 0 {
		conReq = append(conReq, auth...)
		conReq = append(conReq, []byte("\r\n")...)
	}
	conReq = append(conReq, []byte("\r\n")...)

	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)
	res.SkipBody = true
	if _, err := conn.Write(conReq); err != nil {
		return err
	}
	if err := res.Read(bufio.NewReader(conn)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("cannot read CONNECT response from proxy, err=%w", err)
	}
	if res.Header.StatusCode() != 200 {
		_ = conn.Close()
		return fmt.Errorf("unexpected proxy response status code, want: 200, get: %d", res.Header.StatusCode())
	}
	return nil
}
