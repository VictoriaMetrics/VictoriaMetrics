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
