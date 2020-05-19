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
		httpserver.Errorf(w, "Missing `Authorization: Basic *` header")
		return true
	}
	ac := authConfig.Load().(map[string]*UserInfo)
	info := ac[username]
	if info == nil || info.Password != password {
		httpserver.Errorf(w, "Cannot find the provided username %q or password in config", username)
		return true
	}
	info.requests.Inc()

	targetURL := createTargetURL(info.URLPrefix, r.URL)
	if _, err := url.Parse(targetURL); err != nil {
		httpserver.Errorf(w, "Invalid targetURL=%q: %s", targetURL, err)
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
	FlushInterval: time.Second,
	ErrorLog:      logger.StdErrorLogger(),
}
