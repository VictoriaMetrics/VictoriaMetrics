package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/reverseproxy"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr       = flag.String("httpListenAddr", ":8427", "TCP address to listen for http connections")
	reloadAuthKey        = flag.String("reloadAuthKey", "", "Auth key for /-/reload http endpoint. It must be passed as authKey=...")
	logInvalidAuthTokens = flag.Bool("logInvalidAuthTokens", false, "Whether to log requests with invalid auth tokens. "+
		`Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page`)
	maxProxiedConnections = flag.Int("maxProxiedConnections", 100, "The maximum number of connections that can be proxied by vmauth to backends")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	pushmetrics.Init()

	logger.Infof("starting vmauth at %q...", *httpListenAddr)
	startTime := time.Now()
	initAuthConfig()
	go httpserver.Serve(*httpListenAddr, requestHandler)
	logger.Infof("started vmauth in %.3f seconds", time.Since(startTime).Seconds())

	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)

	startTime = time.Now()
	logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())
	stopAuthConfig()
	logger.Infof("successfully stopped vmauth in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/-/reload":
		authKey := r.FormValue("authKey")
		if authKey != *reloadAuthKey {
			httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -reloadAuthKey command line flag", authKey)
			return true
		}
		configReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	}
	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "missing `Authorization` request header", http.StatusUnauthorized)
		return true
	}
	if strings.HasPrefix(authToken, "Token ") {
		// Handle InfluxDB's proprietary token authentication scheme as a bearer token authentication
		// See https://docs.influxdata.com/influxdb/v2.0/api/
		authToken = strings.Replace(authToken, "Token", "Bearer", 1)
	}
	ac := authConfig.Load().(map[string]*UserInfo)
	ui := ac[authToken]
	if ui == nil {
		invalidAuthTokenRequests.Inc()
		if *logInvalidAuthTokens {
			httpserver.Errorf(w, r, "cannot find the provided auth token %q in config", authToken)
		} else {
			errStr := fmt.Sprintf("cannot find the provided auth token %q in config", authToken)
			http.Error(w, errStr, http.StatusBadRequest)
		}
		return true
	}
	ui.requests.Inc()
	targetURL, headers, err := createTargetURL(ui, r.URL)
	if err != nil {
		httpserver.Errorf(w, r, "cannot determine targetURL: %s", err)
		return true
	}
	r.Header.Set("vm-target-url", targetURL.String())
	for _, h := range headers {
		r.Header.Set(h.Name, h.Value)
	}
	maxProxiedConnections := *maxProxiedConnections
	if ui.MaxProxiedConnections != 0 {
		maxProxiedConnections = ui.MaxProxiedConnections
	}
	proxyRequest(w, r, maxProxiedConnections)
	return true
}

func proxyRequest(w http.ResponseWriter, r *http.Request, maxProxiedConnections int) {
	defer func() {
		err := recover()
		if err == nil || err == http.ErrAbortHandler {
			// Suppress http.ErrAbortHandler panic.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1353
			return
		}
		// Forward other panics to the caller.
		panic(err)
	}()
	getReverseProxy(maxProxiedConnections).ServeHTTP(w, r)
}

var (
	configReloadRequests     = metrics.NewCounter(`vmauth_http_requests_total{path="/-/reload"}`)
	invalidAuthTokenRequests = metrics.NewCounter(`vmauth_http_request_errors_total{reason="invalid_auth_token"}`)
	missingRouteRequests     = metrics.NewCounter(`vmauth_http_request_errors_total{reason="missing_route"}`)
)

var (
	reverseProxy     *reverseproxy.LimitedReversProxy
	reverseProxyOnce sync.Once
)

func getReverseProxy(maxProxiedConnections int) *reverseproxy.LimitedReversProxy {
	reverseProxyOnce.Do(func() {
		initReverseProxy(maxProxiedConnections)
	})
	return reverseProxy
}

// initReverseProxy must be called after flag.Parse(), since it uses command-line flags.
func initReverseProxy(maxProxiedConnections int) {
	reverseProxy = reverseproxy.NewLimited(maxProxiedConnections)
}

func usage() {
	const s = `
vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmauth.html .
`
	flagutil.Usage(s)
}
