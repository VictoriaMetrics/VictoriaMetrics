package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var (
	httpListenAddr    = flag.String("httpListenAddr", ":8428", "TCP address to listen for http connections")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Remove superflouos samples from time series if they are located closer to each other than this duration. "+
		"This may be useful for reducing overhead when multiple identically configured Prometheus instances write data to the same VictoriaMetrics. "+
		"Deduplication is disabled if the -dedup.minScrapeInterval is 0")
	dryRun = flag.Bool("dryRun", false, "Whether to check only -promscrape.config and then exit. "+
		"Unknown config entries are allowed in -promscrape.config by default. This can be changed with -promscrape.config.strictParse")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
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
		logger.Infof("-promscrape.config is ok; exitting with 0 status code")
		return
	}

	logger.Infof("starting VictoriaMetrics at %q...", *httpListenAddr)
	startTime := time.Now()
	storage.SetMinScrapeIntervalForDeduplication(*minScrapeInterval)
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	vmselect.Init()
	vminsert.Init()
	startSelfScraper()

	go httpserver.Serve(*httpListenAddr, requestHandler)
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
		fmt.Fprintf(w, "<h2>Single-node VictoriaMetrics.</h2></br>")
		fmt.Fprintf(w, "See docs at <a href='https://victoriametrics.github.io/'>https://victoriametrics.github.io/</a></br>")
		fmt.Fprintf(w, "Useful endpoints: </br>")
		writeAPIHelp(w, [][]string{
			{"/targets", "discovered targets list"},
			{"/api/v1/targets", "advanced information about discovered targets in JSON format"},
			{"/metrics", "available service metrics"},
			{"/api/v1/status/tsdb", "tsdb status page"},
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

func writeAPIHelp(w io.Writer, pathList [][]string) {
	pathPrefix := httpserver.GetPathPrefix()
	for _, p := range pathList {
		p, doc := p[0], p[1]
		p = path.Join(pathPrefix, p)
		fmt.Fprintf(w, "<a href='%s'>%q</a> - %s<br/>", p, p, doc)
	}
}
