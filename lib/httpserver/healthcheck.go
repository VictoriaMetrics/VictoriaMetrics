package httpserver

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

var healthcheck = flag.Bool("healthcheck", false, "Whether to check the /health page of the running process and exit. "+
	"The exit code is non-zero if the check fails. This is useful for Docker HEALTHCHECK in images without curl or wget. "+
	"Pass the same command-line flags as the checked process has")

// CheckHealth checks the /health page at the given addrs and exits if -healthcheck command-line flag is set.
//
// The check succeeds if the /health page returns OK at least at a single address from addrs.
//
// It must be called after the logger initialization with the same addrs and opts as passed to Serve.
func CheckHealth(addrs []string, opts ServeOptions) {
	if !*healthcheck {
		return
	}
	if err := checkHealthAddrs(addrs, opts); err != nil {
		logger.Errorf("healthcheck failed: %s", err)
		os.Exit(1) // Docker HEALTHCHECK expects exit code 1 on failure
	}
	os.Exit(0)
}

func checkHealthAddrs(addrs []string, opts ServeOptions) error {
	var err error
	for idx, addr := range addrs {
		if err = checkHealth(addr, idx, opts); err == nil {
			return nil
		}
	}
	return err
}

func checkHealth(addr string, idx int, opts ServeOptions) error {
	// Dial connects to the local system if addr has an empty or unspecified host such as ":8428" or "0.0.0.0:8428".
	network, dialAddr := netutil.GetTCPNetwork(), addr
	if unixAddr, ok := strings.CutPrefix(addr, "unix:"); ok {
		network, dialAddr = "unix", unixAddr
	}
	useProxyProto := opts.UseProxyProtocol != nil && opts.UseProxyProtocol.GetOptionalArg(idx)

	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			c, err := (&net.Dialer{}).DialContext(ctx, network, dialAddr)
			if err != nil || !useProxyProto {
				return c, err
			}
			if _, err := c.Write([]byte("\r\n\r\n\x00\r\nQUIT\n\x20\x00\x00\x00")); err != nil {
				_ = c.Close()
				return nil, err
			}
			return c, nil
		},
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	hc := &http.Client{
		Transport: tr,
		Timeout:   5 * time.Second,
	}

	scheme := "http"
	if IsTLS(idx) {
		scheme = "https"
	}
	prefix := GetPathPrefix()
	if prefix == "" {
		prefix = "/"
	}
	// The url host is used only in the Host header, since the connection is established to dialAddr.
	resp, err := hc.Get(scheme + "://localhost" + prefix + "health")
	if err != nil {
		return err
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code: %d; want %d; response body: %q", resp.StatusCode, http.StatusOK, body)
	}
	return nil
}
