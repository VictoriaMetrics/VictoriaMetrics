package vlselect

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect/logsql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. "+
		"See also -search.maxQueueDuration")
	maxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the search request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
	maxQueryDuration = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum duration for query execution. It can be overridden on a per-query basis via 'timeout' query arg")
)

func getDefaultMaxConcurrentRequests() int {
	n := cgroup.AvailableCPUs()
	if n <= 4 {
		n *= 2
	}
	if n > 16 {
		// A single request can saturate all the CPU cores, so there is no sense
		// in allowing higher number of concurrent requests - they will just contend
		// for unavailable CPU time.
		n = 16
	}
	return n
}

// Init initializes vlselect
func Init() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentRequests)
}

// Stop stops vlselect
func Stop() {
}

var concurrencyLimitCh chan struct{}

var (
	concurrencyLimitReached = metrics.NewCounter(`vl_concurrent_select_limit_reached_total`)
	concurrencyLimitTimeout = metrics.NewCounter(`vl_concurrent_select_limit_timeout_total`)

	_ = metrics.NewGauge(`vl_concurrent_select_capacity`, func() float64 {
		return float64(cap(concurrencyLimitCh))
	})
	_ = metrics.NewGauge(`vl_concurrent_select_current`, func() float64 {
		return float64(len(concurrencyLimitCh))
	})
)

//go:embed vmui
var vmuiFiles embed.FS

var vmuiFileServer = http.FileServer(http.FS(vmuiFiles))

// RequestHandler handles select requests for VictoriaLogs
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/select/") {
		// Skip requests, which do not start with /select/, since these aren't our requests.
		return false
	}
	path = strings.ReplaceAll(path, "//", "/")

	if path == "/select/vmui" {
		// VMUI access via incomplete url without `/` in the end. Redirect to complete url.
		// Use relative redirect, since the hostname and path prefix may be incorrect if VictoriaMetrics
		// is hidden behind vmauth or similar proxy.
		_ = r.ParseForm()
		newURL := "vmui/?" + r.Form.Encode()
		httpserver.Redirect(w, newURL)
		return true
	}
	if strings.HasPrefix(path, "/select/vmui/") {
		if strings.HasPrefix(path, "/select/vmui/static/") {
			// Allow clients caching static contents for long period of time, since it shouldn't change over time.
			// Path to static contents (such as js and css) must be changed whenever its contents is changed.
			// See https://developer.chrome.com/docs/lighthouse/performance/uses-long-cache-ttl/
			w.Header().Set("Cache-Control", "max-age=31536000")
		}
		r.URL.Path = strings.TrimPrefix(path, "/select")
		vmuiFileServer.ServeHTTP(w, r)
		return true
	}

	// Limit the number of concurrent queries, which can consume big amounts of CPU time.
	startTime := time.Now()
	ctx := r.Context()
	d := getMaxQueryDuration(r)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	stopCh := ctxWithTimeout.Done()
	select {
	case concurrencyLimitCh <- struct{}{}:
		defer func() { <-concurrencyLimitCh }()
	default:
		// Sleep for a while until giving up. This should resolve short bursts in requests.
		concurrencyLimitReached.Inc()
		select {
		case concurrencyLimitCh <- struct{}{}:
			defer func() { <-concurrencyLimitCh }()
		case <-stopCh:
			switch ctxWithTimeout.Err() {
			case context.Canceled:
				remoteAddr := httpserver.GetQuotedRemoteAddr(r)
				requestURI := httpserver.GetRequestURI(r)
				logger.Infof("client has canceled the pending request after %.3f seconds: remoteAddr=%s, requestURI: %q",
					time.Since(startTime).Seconds(), remoteAddr, requestURI)
			case context.DeadlineExceeded:
				concurrencyLimitTimeout.Inc()
				err := &httpserver.ErrorWithStatusCode{
					Err: fmt.Errorf("couldn't start executing the request in %.3f seconds, since -search.maxConcurrentRequests=%d concurrent requests "+
						"are executed. Possible solutions: to reduce query load; to add more compute resources to the server; "+
						"to increase -search.maxQueueDuration=%s; to increase -search.maxQueryDuration=%s; to increase -search.maxConcurrentRequests; "+
						"to pass bigger value to 'timeout' query arg",
						d.Seconds(), *maxConcurrentRequests, maxQueueDuration, maxQueryDuration),
					StatusCode: http.StatusServiceUnavailable,
				}
				httpserver.Errorf(w, r, "%s", err)
			}
			return true
		}
	}

	if path == "/select/logsql/tail" {
		logsqlTailRequests.Inc()
		// Process live tailing request without timeout (e.g. use ctx instead of ctxWithTimeout),
		// since it is OK to run live tailing requests for very long time.
		logsql.ProcessLiveTailRequest(ctx, w, r)
		return true
	}

	ok := processSelectRequest(ctxWithTimeout, w, r, path)
	if !ok {
		return false
	}

	err := ctxWithTimeout.Err()
	switch err {
	case nil:
		// nothing to do
	case context.Canceled:
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Infof("client has canceled the request after %.3f seconds: remoteAddr=%s, requestURI: %q",
			time.Since(startTime).Seconds(), remoteAddr, requestURI)
	case context.DeadlineExceeded:
		err = &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("the request couldn't be executed in %.3f seconds; possible solutions: "+
				"to increase -search.maxQueryDuration=%s; to pass bigger value to 'timeout' query arg", d.Seconds(), maxQueryDuration),
			StatusCode: http.StatusServiceUnavailable,
		}
		httpserver.Errorf(w, r, "%s", err)
	default:
		httpserver.Errorf(w, r, "unexpected error: %s", err)
	}

	return true
}

func processSelectRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, path string) bool {
	httpserver.EnableCORS(w, r)
	switch path {
	case "/select/logsql/field_names":
		logsqlFieldNamesRequests.Inc()
		logsql.ProcessFieldNamesRequest(ctx, w, r)
		return true
	case "/select/logsql/field_values":
		logsqlFieldValuesRequests.Inc()
		logsql.ProcessFieldValuesRequest(ctx, w, r)
		return true
	case "/select/logsql/hits":
		logsqlHitsRequests.Inc()
		logsql.ProcessHitsRequest(ctx, w, r)
		return true
	case "/select/logsql/query":
		logsqlQueryRequests.Inc()
		logsql.ProcessQueryRequest(ctx, w, r)
		return true
	case "/select/logsql/stats_query":
		logsqlStatsQueryRequests.Inc()
		logsql.ProcessStatsQueryRequest(ctx, w, r)
		return true
	case "/select/logsql/stats_query_range":
		logsqlStatsQueryRangeRequests.Inc()
		logsql.ProcessStatsQueryRangeRequest(ctx, w, r)
		return true
	case "/select/logsql/stream_field_names":
		logsqlStreamFieldNamesRequests.Inc()
		logsql.ProcessStreamFieldNamesRequest(ctx, w, r)
		return true
	case "/select/logsql/stream_field_values":
		logsqlStreamFieldValuesRequests.Inc()
		logsql.ProcessStreamFieldValuesRequest(ctx, w, r)
		return true
	case "/select/logsql/stream_ids":
		logsqlStreamIDsRequests.Inc()
		logsql.ProcessStreamIDsRequest(ctx, w, r)
		return true
	case "/select/logsql/streams":
		logsqlStreamsRequests.Inc()
		logsql.ProcessStreamsRequest(ctx, w, r)
		return true
	default:
		return false
	}
}

// getMaxQueryDuration returns the maximum duration for query from r.
func getMaxQueryDuration(r *http.Request) time.Duration {
	dms, err := httputils.GetDuration(r, "timeout", 0)
	if err != nil {
		dms = 0
	}
	d := time.Duration(dms) * time.Millisecond
	if d <= 0 || d > *maxQueryDuration {
		d = *maxQueryDuration
	}
	return d
}

var (
	logsqlFieldNamesRequests        = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/field_names"}`)
	logsqlFieldValuesRequests       = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/field_values"}`)
	logsqlHitsRequests              = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/hits"}`)
	logsqlQueryRequests             = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/query"}`)
	logsqlStatsQueryRequests        = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/stats_query"}`)
	logsqlStatsQueryRangeRequests   = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/stats_query_range"}`)
	logsqlStreamFieldNamesRequests  = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/stream_field_names"}`)
	logsqlStreamFieldValuesRequests = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/stream_field_values"}`)
	logsqlStreamIDsRequests         = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/stream_ids"}`)
	logsqlStreamsRequests           = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/streams"}`)
	logsqlTailRequests              = metrics.NewCounter(`vl_http_requests_total{path="/select/logsql/tail"}`)
)
