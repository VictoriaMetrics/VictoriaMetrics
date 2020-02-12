package main

import (
	"flag"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	configPath     = flag.String("config", "config.yaml", "Path to alert configuration file")
	httpListenAddr = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
)

func main() {
	buildinfo.Init()
	logger.Init()

	logger.Infof("reading alert rules configuration file from %s", *configPath)
	alerts := config.Parse(*configPath)
	w := &watchdog{storage: &storage.VMStorage{}}
	go func() {
		w.run(alerts)
	}()
	go func() {
		httpserver.Serve(*httpListenAddr, func(w http.ResponseWriter, r *http.Request) bool {
			panic("not implemented")
		})
	}()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	httpserver.Stop(*httpListenAddr)
	w.stop()
}

type watchdog struct {
	storage *storage.VMStorage
}

func (w *watchdog) run(a config.Alerts) {

}

func (w *watchdog) stop() {
	panic("not implemented")
}
