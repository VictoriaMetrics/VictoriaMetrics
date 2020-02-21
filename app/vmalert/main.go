package main

import (
	"flag"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	configPath     = flag.String("config", "config.yaml", "Path to alert configuration file")
	httpListenAddr = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")
)

func main() {
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	logger.Infof("reading alert rules configuration file from %s", *configPath)
	alertGroups, err := config.Parse(*configPath)
	if err != nil {
		logger.Fatalf("Cannot parse configuration file %s", err)
	}
	w := &watchdog{storage: &datasource.VMStorage{}}
	for id := range alertGroups {
		go func(group config.Group) {
			w.run(group)
		}(alertGroups[id])
	}
	go func() {
		httpserver.Serve(*httpListenAddr, func(w http.ResponseWriter, r *http.Request) bool {
			panic("not implemented")
		})
	}()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	w.stop()
}

type watchdog struct {
	storage *datasource.VMStorage
}

func (w *watchdog) run(a config.Group) {

}

func (w *watchdog) stop() {
	panic("not implemented")
}
