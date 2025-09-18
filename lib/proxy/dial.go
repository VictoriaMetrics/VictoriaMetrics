package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// NewDialFunc returns dial func for the given u and ac.
// Dial uses HTTP CONNECT for http and https targets https://en.wikipedia.org/wiki/HTTP_tunnel#HTTP_CONNECT_method
// And socks5 for socks5 targets  https://en.wikipedia.org/wiki/SOCKS
// supports authorization and tls configuration
func (u *URL) NewDialFunc(ac *promauth.Config) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	pu := u.URL
	if !isURLSchemeValid(pu.Scheme) {
		return nil, fmt.Errorf("unknown scheme=%q for proxy_url=%q, must be in %s", pu.Scheme, pu.Redacted(), validURLSchemes)
	}
	isTLS := (pu.Scheme == "https" || pu.Scheme == "tls+socks5")
	proxyAddr := addMissingPort(pu.Host, isTLS)

	var tlsCfg *tls.Config
	if isTLS {
		var err error
		tlsCfg, err = ac.GetTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("cannot initialize tls config: %w", err)
		}
		if !tlsCfg.InsecureSkipVerify && tlsCfg.ServerName == "" {
			tlsCfg.ServerName = tlsServerName(proxyAddr)
		}
	}
	if pu.Scheme == "socks5" || pu.Scheme == "tls+socks5" {
		return socks5DialFunc(proxyAddr, pu, tlsCfg)
	}
	dialFunc := func(ctx context.Context, network, addr string) (net.Conn, error) {
		proxyConn, err := netutil.DialMaybeSRV(ctx, network, proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to proxy %q: %w", pu.Redacted(), err)
		}
		if isTLS {
			proxyConn = tls.Client(proxyConn, tlsCfg)
		}
		hdr := ac.GetHTTPHeadersNoAuth()
		authHeader, err := u.getAuthHeader(ac)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain Proxy-Authorization header: %w", err)
		}
		if len(authHeader) > 0 {
			if hdr == nil {
				hdr = make(http.Header)
			}
			hdr.Add("Proxy-Authorization", authHeader)
		}

		conn, err := sendConnectRequest(proxyConn, proxyAddr, addr, hdr)
		if err != nil {
			_ = proxyConn.Close()
			return nil, fmt.Errorf("error when sending CONNECT request to proxy %q: %w", pu.Redacted(), err)
		}
		return conn, nil
	}
	return dialFunc, nil
}

func socks5DialFunc(proxyAddr string, pu *url.URL, tlsCfg *tls.Config) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
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
	dialFunc := func(_ context.Context, _, addr string) (net.Conn, error) {
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

// sendConnectRequest sends CONNECT request to proxyConn for the given addr and headers and returns the established connection to dstAddr.
func sendConnectRequest(proxyConn net.Conn, proxyAddr, dstAddr string, hdr http.Header) (net.Conn, error) {
	r := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: dstAddr},
		Host:   proxyAddr,
		Header: hdr,
	}

	if err := r.Write(proxyConn); err != nil {
		return nil, fmt.Errorf("cannot send CONNECT request for dstAddr=%q: %w", dstAddr, err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), r)
	if err != nil {
		return nil, fmt.Errorf("cannot read CONNECT response for dstAddr=%q: %w", dstAddr, err)
	}

	if statusCode := resp.StatusCode; statusCode != 200 {
		return nil, fmt.Errorf("unexpected status code received: %d; want: 200; response body: %q", statusCode, resp.Status)
	}
	return proxyConn, nil
}
