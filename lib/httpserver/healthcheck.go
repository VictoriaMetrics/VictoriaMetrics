package httpserver

import (
	"flag"
	"net"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
)

var healthcheck = flag.Bool("healthcheck", false, "Whether to check that the running process accepts connections at -httpListenAddr and exit. "+
	"The exit code is non-zero if the check fails. This is useful for Docker HEALTHCHECK in images without curl or wget. "+
	"Pass the same -httpListenAddr as the checked process has")

// CheckHealth checks that the running process accepts connections at the given addrs and exits if -healthcheck command-line flag is set.
//
// The check succeeds if at least a single address from addrs accepts connections.
//
// It must be called after the logger initialization with the same addrs as passed to Serve.
func CheckHealth(addrs []string) {
	if !*healthcheck {
		return
	}
	if err := checkHealthAddrs(addrs); err != nil {
		logger.Errorf("healthcheck failed: %s", err)
		os.Exit(1) // Docker HEALTHCHECK expects exit code 1 on failure
	}
	os.Exit(0)
}

func checkHealthAddrs(addrs []string) error {
	var err error
	for _, addr := range addrs {
		if err = checkHealth(addr); err == nil {
			return nil
		}
	}
	return err
}

func checkHealth(addr string) error {
	// Dial connects to the local system if addr has an empty or unspecified host such as ":8428" or "0.0.0.0:8428".
	network, dialAddr := netutil.GetTCPNetwork(), addr
	if unixAddr, ok := strings.CutPrefix(addr, "unix:"); ok {
		network, dialAddr = "unix", unixAddr
	}
	c, err := net.DialTimeout(network, dialAddr, 5*time.Second)
	if err != nil {
		return err
	}
	return c.Close()
}
