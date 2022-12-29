package main

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/reverseproxy"
)

var (
	reverseProxy     *reverseproxy.ReverseProxy
	reverseProxyOnce sync.Once
)

func getReverseProxy() *reverseproxy.ReverseProxy {
	reverseProxyOnce.Do(initReverseProxy)
	return reverseProxy
}

// initReverseProxy must be called after flag.Parse(), since it uses command-line flags.
func initReverseProxy() {
	reverseProxy = reverseproxy.New()
}
