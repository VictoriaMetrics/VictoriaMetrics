package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmestimator/protoparser"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddrs = flagutil.NewArrayString("httpListenAddr", "TCP address to listen for incoming HTTP requests")
	configPath      = flag.String("config", "config.yaml", "Path to YAML configuration file")

	prometheusWriteRequests = metrics.NewCounter(`vmestimator_http_requests_total{path="/api/v1/write", protocol="promremotewrite"}`)
)

func main() {
	flag.CommandLine.SetOutput(os.Stdout)
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Fatalf("cannot load config: %v", err)
	}

	estimators := make([]*estimator, 0, len(cfg.Streams))
	for _, ec := range cfg.Streams {
		e, err := newEstimator(ec)
		if err != nil {
			logger.Fatalf("cannot create estimator: %v", err)
		}
		estimators = append(estimators, e)
	}

	if *cardinalityMetricsExposeAt == `/metrics` {
		metrics.RegisterMetricsWriter(func(w io.Writer) {
			writeCardinalityMetrics(w, estimators)
		})
	}

	groupLabelsMap := make(map[string]struct{})
	for _, e := range estimators {
		for _, l := range e.groupBy {
			groupLabelsMap[l] = struct{}{}
		}
	}

	groupLabels := make([]string, 0, len(groupLabelsMap))
	for k := range groupLabelsMap {
		groupLabels = append(groupLabels, k)
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8490"}
	}

	logger.Infof("starting vmestimator at %q", listenAddrs)
	startTime := time.Now()

	go httpserver.Serve(listenAddrs, func(w http.ResponseWriter, r *http.Request) bool {
		cmPath := *cardinalityMetricsExposeAt
		if cmPath != "/metrics" && cmPath != "" && r.URL.Path == cmPath {
			w.WriteHeader(http.StatusOK)
			writeCardinalityMetrics(w, estimators)
			return true
		}

		path, _ := strings.CutPrefix(r.URL.Path, `/cardinality`)
		switch path {
		case "/api/v1/write":
			prometheusWriteRequests.Inc()
			err := protoparser.Parse(r.Body, groupLabels, func(tss []protoparser.TimeSerie) {
				for _, e := range estimators {
					e.insertMany(tss)
				}
			})
			if err != nil {
				httpserver.Errorf(w, r, "error parsing remote write request: %s", err)
				return true
			}
			w.WriteHeader(http.StatusNoContent)
			return true
		case "/reset":
			for _, e := range estimators {
				e.reset()
			}
			w.WriteHeader(http.StatusOK)
			return true
		}
		return false
	}, httpserver.ServeOptions{})

	logger.Infof("started vmestimator in %.3f seconds", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down webservice at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Errorf("cannot stop http server: %s", err)
	}
	for _, e := range estimators {
		e.stop()
	}
	logger.Infof("shutting down vmestimator")
}
