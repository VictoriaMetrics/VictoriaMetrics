package unixtransport

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// ParseURI parses the given `uri` into a network and address, suitable for use
// with functions like [net.Listen].
//
// The URI scheme is interpreted as the network, and the host (including port)
// as the address. For example, "tcp://:80" yields network "tcp" and address
// ":80", and "unix:///tmp/my.sock" yields network "unix" and address
// "/tmp/my.sock".
//
// If the URI doesn't have a scheme, "tcp://" is assumed by default, in an
// attempt to keep basic compatibility with common listen addresses like
// "localhost:8080" or ":9090".
func ParseURI(uri string) (network, address string, _ error) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", "", fmt.Errorf("empty URI")
	}

	if !strings.Contains(uri, "://") {
		uri = "tcp://" + uri
	}

	u, err := url.Parse(uri)
	if err != nil {
		return "", "", fmt.Errorf("parse URI: %w", err)
	}

	if u.Host == "" && u.Path != "" {
		u.Host = u.Path
	}

	if u.Host == "" {
		return "", "", fmt.Errorf("empty host in URI (%s)", uri)
	}

	return u.Scheme, u.Host, nil
}

// ListenURI is a convenience function that calls [ListenURIConfig] with a
// default [net.ListenConfig].
func ListenURI(ctx context.Context, uri string) (net.Listener, error) {
	return ListenURIConfig(ctx, uri, net.ListenConfig{})
}

// ListenURIConfig parses `uri` into a network and address using [ParseURI],
// then constructs a listener on that network and address, using the provided
// [net.ListenConfig].
//
// If the network is "unix" or "unixpacket", it first removes any existing
// socket file at the address, ignoring [os.IsNotExist] errors.
//
// The provided `ctx` is only used when resolving the listen address, it has no
// effect on the returned listener.
//
// For more precise control, use [ParseURI] and constructa listener yourself.
func ListenURIConfig(ctx context.Context, uri string, config net.ListenConfig) (net.Listener, error) {
	network, address, err := ParseURI(uri)
	if err != nil {
		return nil, err
	}

	if network == "unix" || network == "unixpacket" {
		if err := os.Remove(address); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove stale socket (%s): %w", address, err)
		}
	}

	listener, err := config.Listen(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	return listener, nil
}
