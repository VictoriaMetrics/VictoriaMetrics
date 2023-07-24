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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var (
	httpListenAddr   = flag.String("httpListenAddr", ":8428", "TCP address to listen for http connections. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flag.Bool("httpListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Leave only the last sample in every time series per each discrete interval "+
		"equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication and https://docs.victoriametrics.com/#downsampling")
	dryRun = flag.Bool("dryRun", false, "Whether to check config files without running VictoriaMetrics. The following config files are checked: "+
		"-promscrape.config, -relabelConfig and -streamAggr.config. Unknown config entries aren't allowed in -promscrape.config by default. "+
		"This can be changed with -promscrape.config.strictParse=false command-line flag")
	syntaxCheckOnly           = flag.Bool("dryRun.syntaxOnly", false, "Only check config syntax during dryrun, ignore file references in auth configs")
	inmemoryDataFlushInterval = flag.Duration("inmemoryDataFlushInterval", 5*time.Second, "The interval for guaranteed saving of in-memory data to disk. "+
		"The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. "+
		"Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). "+
		"Smaller intervals increase disk IO load. Minimum supported value is 1s")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	pushmetrics.Init()

	if promscrape.IsDryRun() {
		if err := promscrape.CheckConfig(promscrape.SyntaxCheckOnly()); err != nil {
			logger.Fatalf("error when checking -promscrape.config: %s", err)
		}
		return
	}
	if *dryRun {
		if err := promscrape.CheckConfig(*syntaxCheckOnly); err != nil {
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

	logger.Infof("starting VictoriaMetrics at %q...", *httpListenAddr)
	startTime := time.Now()
	storage.SetDedupInterval(*minScrapeInterval)
	storage.SetDataFlushInterval(*inmemoryDataFlushInterval)
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	vmselect.Init()
	vminsert.Init()
	startSelfScraper()

	go httpserver.Serve(*httpListenAddr, *useProxyProtocol, requestHandler)
	logger.Infof("started VictoriaMetrics in %.3f seconds", time.Since(startTime).Seconds())

	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)

	stopSelfScraper()

	logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	vminsert.Stop()
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())

	vmstorage.Stop()
	vmselect.Stop()

	fs.MustStopDirRemover()

	logger.Infof("the VictoriaMetrics has been stopped in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Single-node VictoriaMetrics</h2></br>")
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

See the docs at https://docs.victoriametrics.com/
`
	flagutil.Usage(s)
}
