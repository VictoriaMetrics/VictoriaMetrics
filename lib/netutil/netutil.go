package netutil

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

type resolver interface {
	LookupSRV(ctx context.Context, service, proto, name string) (cname string, addrs []*net.SRV, err error)
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
}

// Resolver is default DNS resolver.
var Resolver resolver

func init() {
	Resolver = &net.Resolver{
		PreferGo:     true,
		StrictErrors: true,
	}
}

// IsTrivialNetworkError returns true if the err can be ignored during logging.
func IsTrivialNetworkError(err error) bool {
	// Suppress trivial network errors, which could occur at remote side.
	s := err.Error()
	if strings.Contains(s, "broken pipe") || strings.Contains(s, "reset by peer") {
		return true
	}
	return false
}

// DialMaybeSRV dials the given addr.
//
// The addr may be either the usual TCP address or srv+host form, where host is SRV addr.
// If the addr has srv+host form, then the host is resolved with SRV into randomly chosen TCP address for the connection.
func DialMaybeSRV(ctx context.Context, network, addr string) (net.Conn, error) {
	if strings.HasPrefix(addr, "srv+") {
		addr = strings.TrimPrefix(addr, "srv+")
		if n := strings.IndexByte(addr, ':'); n >= 0 {
			// Drop port, since it should be automatically resolved via DNS SRV lookup below.
			addr = addr[:n]
		}
		_, addrs, err := Resolver.LookupSRV(ctx, "", "", addr)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve SRV addr %s: %w", addr, err)
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("missing SRV records for %s", addr)
		}
		n := rand.Intn(len(addrs))
		addr = fmt.Sprintf("%s:%d", addrs[n].Target, addrs[n].Port)
	}
	return Dialer.DialContext(ctx, network, addr)
}

// Dialer is default network dialer.
var Dialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
	DualStack: TCP6Enabled(),
}

// IsErrMissingPort checks if the given error is due to a missing port in the address.
// It is expected to be used to validate error returned by net.SplitHostPort
// See https://github.com/golang/go/blob/ed08d2ad0928c0fc77cc2053863616ffb58c5aac/src/net/ipsock.go#L167
func IsErrMissingPort(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "missing port in address")
}

// NormalizeAddr normalizes the given addr by adding defaultPort if it is missing.
// It returns the normalized address in the form "host:port".
// It is expected that addr is in the form "host" or "host:port".
func NormalizeAddr(addr string, defaultPort int) (string, error) {
	if strings.Contains(addr, "/") {
		return "", fmt.Errorf("invalid address %q; expected format: host:port", addr)
	}

	_, _, err := net.SplitHostPort(addr)
	if IsErrMissingPort(err) {
		return fmt.Sprintf("%s:%d", addr, defaultPort), nil
	} else if err != nil {
		return "", fmt.Errorf("invalid address %q; expected format: host:port", addr)
	}
	return addr, nil
}
