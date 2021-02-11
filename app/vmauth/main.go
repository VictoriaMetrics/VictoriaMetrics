package main

import (
	"flag"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	httpListenAddr = flag.String("httpListenAddr", ":8427", "TCP address to listen for http connections")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
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
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "missing `Authorization: Basic *` header", http.StatusUnauthorized)
		return true
	}
	ac := authConfig.Load().(map[string]*UserInfo)
	ui := ac[username]
	if ui == nil || ui.Password != password {
		httpserver.Errorf(w, r, "cannot find the provided username %q or password in config", username)
		return true
	}
	ui.requests.Inc()
	targetURL, err := createTargetURL(ui, r.URL)
	if err != nil {
		httpserver.Errorf(w, r, "cannot determine targetURL: %s", err)
		return true
	}
	if _, err := url.Parse(targetURL); err != nil {
		httpserver.Errorf(w, r, "invalid targetURL=%q: %s", targetURL, err)
		return true
	}
	r.Header.Set("vm-target-url", targetURL)
	reverseProxy.ServeHTTP(w, r)
	return true
}

var reverseProxy = &httputil.ReverseProxy{
	Director: func(r *http.Request) {
		targetURL := r.Header.Get("vm-target-url")
		target, err := url.Parse(targetURL)
		if err != nil {
			logger.Panicf("BUG: unexpected error when parsing targetURL=%q: %s", targetURL, err)
		}
		r.URL = target
	},
	Transport: func() *http.Transport {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		// Automatic compression must be disabled in order to fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/535
		tr.DisableCompression = true
		// Disable HTTP/2.0, since VictoriaMetrics components don't support HTTP/2.0 (because there is no sense in this).
		tr.ForceAttemptHTTP2 = false
		return tr
	}(),
	FlushInterval: time.Second,
	ErrorLog:      logger.StdErrorLogger(),
}

func usage() {
	const s = `
vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://victoriametrics.github.io/vmauth.html .
`
	flagutil.Usage(s)
}
