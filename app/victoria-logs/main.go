package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddr   = flag.String("httpListenAddr", ":9428", "TCP address to listen for http connections. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flag.Bool("httpListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	gogc = flag.Int("gogc", 100, "GOGC to use. See https://tip.golang.org/doc/gc-guide")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	cgroup.SetGOGC(*gogc)
	buildinfo.Init()
	logger.Init()

	logger.Infof("starting VictoriaLogs at %q...", *httpListenAddr)
	startTime := time.Now()

	vlstorage.Init()
	vlselect.Init()
	vlinsert.Init()

	go httpserver.Serve(*httpListenAddr, *useProxyProtocol, requestHandler)
	logger.Infof("started VictoriaLogs in %.3f seconds; see https://docs.victoriametrics.com/VictoriaLogs/", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())

	vlinsert.Stop()
	vlselect.Stop()
	vlstorage.Stop()

	fs.MustStopDirRemover()

	logger.Infof("the VictoriaLogs has been stopped in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path == "/" {
		if r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<h2>Single-node VictoriaLogs</h2></br>")
		fmt.Fprintf(w, "See docs at <a href='https://docs.victoriametrics.com/VictoriaLogs/'>https://docs.victoriametrics.com/VictoriaLogs/</a></br>")
		fmt.Fprintf(w, "Useful endpoints:</br>")
		httpserver.WriteAPIHelp(w, [][2]string{
			{"select/vmui", "Web UI for VictoriaLogs"},
			{"metrics", "available service metrics"},
			{"flags", "command-line flags"},
		})
		return true
	}
	if vlinsert.RequestHandler(w, r) {
		return true
	}
	if vlselect.RequestHandler(w, r) {
		return true
	}
	return false
}

func usage() {
	const s = `
victoria-logs is a log management and analytics service.

See the docs at https://docs.victoriametrics.com/VictoriaLogs/
`
	flagutil.Usage(s)
}
