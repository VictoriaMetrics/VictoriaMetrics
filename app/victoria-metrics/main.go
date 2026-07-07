package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	vminsertcommon "github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/common"
	vminsertrelabel "github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert/relabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "TCP addresses to listen for incoming http requests. See also -tls and -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	dryRun = flag.Bool("dryRun", false, "Whether to check config files without running VictoriaMetrics. The following config files are checked: "+
		"-promscrape.config, -relabelConfig and -streamAggr.config. Unknown config entries aren't allowed in -promscrape.config by default. "+
		"This can be changed with -promscrape.config.strictParse=false command-line flag")
	maxIngestionRate = flag.Int("maxIngestionRate", 0, "The maximum number of samples vmsingle can receive per second. Data ingestion is paused when the limit is exceeded. "+
		"By default there are no limits on samples ingestion rate.")
	vmselectMaxConcurrentRequests = flag.Int("search.maxConcurrentRequests", getDefaultMaxConcurrentRequests(), "The maximum number of concurrent search requests. "+
		"It shouldn't be high, since a single request can saturate all the CPU cores, while many concurrently executed requests may require high amounts of memory. "+
		"See also -search.maxQueueDuration and -search.maxMemoryPerQuery")
	vmselectMaxQueueDuration = flag.Duration("search.maxQueueDuration", 10*time.Second, "The maximum time the request waits for execution when -search.maxConcurrentRequests "+
		"limit is reached; see also -search.maxQueryDuration")
)

func getDefaultMaxConcurrentRequests() int {
	// A single request can saturate all the CPU cores, so there is no sense
	// in allowing higher number of concurrent requests - they will just contend
	// for unavailable CPU time.
	n := min(cgroup.AvailableCPUs()*2, 16)
	return n
}

func main() {
	// VictoriaMetrics is optimized for reduced memory allocations,
	// so it can run with the reduced GOGC in order to reduce the used memory,
	// while keeping CPU usage spent in GC at low levels.
	//
	// Some workloads may need increased GOGC values. Then such values can be set via GOGC environment variable.
	// It is recommended increasing GOGC if go_memstats_gc_cpu_fraction metric exposed at /metrics page
	// exceeds 0.05 for extended periods of time.
	cgroup.SetGOGC(30)

	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	if promscrape.IsDryRun() {
		*dryRun = true
	}
	if *dryRun {
		if err := promscrape.CheckConfig(); err != nil {
			logger.Fatalf("error when checking -promscrape.config: %s", err)
		}
		if err := vminsertrelabel.CheckRelabelConfig(); err != nil {
			logger.Fatalf("error when checking -relabelConfig: %s", err)
		}
		if err := vminsertcommon.CheckStreamAggrConfig(); err != nil {
			logger.Fatalf("error when checking -streamAggr.config: %s", err)
		}
		logger.Infof("-promscrape.config is ok; exiting with 0 status code")
		return
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8428"}
	}
	logger.Infof("starting VictoriaMetrics at %q...", listenAddrs)
	startTime := time.Now()
	vmstorage.Init(*vmselectMaxConcurrentRequests, *vmselectMaxQueueDuration, promql.ResetRollupResultCacheIfNeeded)
	vmselect.Init(*vmselectMaxConcurrentRequests, *vmselectMaxQueueDuration)
	vminsertcommon.StartIngestionRateLimiter(*maxIngestionRate)
	vminsert.Init()

	startSelfScraper()

	go httpserver.Serve(listenAddrs, requestHandler, httpserver.ServeOptions{
		UseProxyProtocol: useProxyProtocol,
	})
	logger.Infof("started VictoriaMetrics in %.3f seconds", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	pushmetrics.Stop()

	stopSelfScraper()

	logger.Infof("gracefully shutting down webservice at %q", listenAddrs)
	startTime = time.Now()
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())
	vminsertcommon.StopIngestionRateLimiter()
	vminsert.Stop()

	vmstorage.Stop()
	vmselect.Stop()

	logger.Infof("the VictoriaMetrics has been stopped in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Single-node VictoriaMetrics</h2></br>")
		fmt.Fprintf(w, "Version %s<br>", buildinfo.Version)
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/'>https://docs.victoriametrics.com/</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"vmui", "Web UI"},
			{"targets", "status for discovered active targets"},
			{"service-discovery", "labels before and after relabeling for discovered targets"},
			{"metric-relabel-debug", "debug metric relabeling"},
			{"expand-with-exprs", "WITH expressions' tutorial"},
			{"api/v1/targets", "advanced information about discovered targets in JSON format"},
			{"config", "-promscrape.config contents"},
			{"metrics", "available service metrics"},
			{"flags", "command-line flags"},
			{"api/v1/status/tsdb", "tsdb status page"},
			{"api/v1/status/top_queries", "top queries"},
			{"api/v1/status/active_queries", "active queries"},
			{"-/reload", "reload configuration"},
		})
		return true
	}
	if vminsert.RequestHandler(w, r) {
		return true
	}
	if vmselect.RequestHandler(w, r) {
		return true
	}
	if vmstorage.RequestHandler(w, r) {
		return true
	}
	return false
}

func usage() {
	const s = `
victoria-metrics is a time series database and monitoring solution.

See the docs at https://docs.victoriametrics.com/victoriametrics/
`
	flagutil.Usage(s)
}
