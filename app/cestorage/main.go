package main

import (
	"flag"
	"io"
	"os"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddrs = flagutil.NewArrayString("httpListenAddr", "TCP address to listen for incoming HTTP requests")
	configPath      = flag.String("config", "config.yaml", "Path to YAML configuration file")
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

	estimators := make([]*Estimator, 0, len(cfg.Streams))
	for _, ec := range cfg.Streams {
		e, err := newEstimator(ec)
		if err != nil {
			logger.Fatalf("cannot create estimator: %v", err)
		}
		estimators = append(estimators, e)
	}

	// Register dynamic cardinality metrics so they appear in the /metrics output.
	metrics.RegisterMetricsWriter(func(w io.Writer) {
		for _, e := range estimators {
			e.writeMetrics(w)
		}
	})

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8490"}
	}

	logger.Infof("starting cestorage at %q", listenAddrs)
	startTime := time.Now()

	go httpserver.Serve(listenAddrs, requestHandler(estimators), httpserver.ServeOptions{})

	logger.Infof("started cestorage in %.3f seconds", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down webservice at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Errorf("cannot stop http server: %s", err)
	}
	for _, e := range estimators {
		e.Stop()
	}
	logger.Infof("shutting down cestorage")
}
