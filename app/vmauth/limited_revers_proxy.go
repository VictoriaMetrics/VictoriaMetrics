package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type LimitedReversProxy struct {
	reversProxy *httputil.ReverseProxy
	limiter     chan struct{}
}

func NewReversProxy(maxConn int) *LimitedReversProxy {
	limitedProxy := &LimitedReversProxy{
		limiter:     make(chan struct{}, maxConn),
		reversProxy: getDefaultReversProxy(),
	}
	for i := 0; i < maxConn; i++ {
		limitedProxy.limiter <- struct{}{}
	}
	return limitedProxy
}

func (lrr *LimitedReversProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	select {
	case <-lrr.limiter:
		lrr.reversProxy.ServeHTTP(w, req)
		lrr.limiter <- struct{}{}
	default:
		message := fmt.Sprintf("cannot handle more than %d connections", 100)
		http.Error(w, message, http.StatusTooManyRequests)
	}
}

func getDefaultReversProxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
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
			tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
			if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
				tr.MaxIdleConns = tr.MaxIdleConnsPerHost
			}
			return tr
		}(),
		FlushInterval: time.Second,
		ErrorLog:      logger.StdErrorLogger(),
	}
}
