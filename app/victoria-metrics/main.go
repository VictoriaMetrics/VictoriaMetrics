package main

import (
	"flag"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vminsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var httpListenAddr = flag.String("httpListenAddr", ":8428", "TCP address to listen for http connections")

func main() {
	flag.Parse()
	reSep := regexp.MustCompile(`[a-zA-Z0-9_:]*`)
	flag.VisitAll(func(f *flag.Flag) {
		// Validate influxMeasurementFieldSeparator flag
		if f.Name == "influxMeasurementFieldSeparator" && !reSep.MatchString(f.Value.String()) {
			logger.Errorf("The influxMeasurementFieldSeparator flag has invalid value of '%s'. It can only be ASCII letter, digit, underscore or colon.", f.Value.String())
			flag.PrintDefaults()
			os.Exit(1)
		}
	})
	buildinfo.Init()
	logger.Init()
	logger.Infof("starting VictoraMetrics at %q...", *httpListenAddr)
	startTime := time.Now()
	vmstorage.Init()
	vmselect.Init()
	vminsert.Init()

	go httpserver.Serve(*httpListenAddr, requestHandler)
	logger.Infof("started VictoriaMetrics in %s", time.Since(startTime))

	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)

	logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	vminsert.Stop()
	logger.Infof("successfully shut down the webservice in %s", time.Since(startTime))

	vmstorage.Stop()
	vmselect.Stop()

	logger.Infof("the VictoriaMetrics has been stopped in %s", time.Since(startTime))
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
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
