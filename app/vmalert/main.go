package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/provider"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	configPath     = flag.String("config", "config.yaml", "Path to alert configuration file")
	httpListenAddr = flag.String("httpListenAddr", ":8880", "Address to listen for http connections")

	datasourceURL      = flag.String("datasource.url", "", "Victoria Metrics or VMSelect url. Required parameter. e.g. http://127.0.0.1:8428")
	basicAuthUsername  = flag.String("datasource.basicAuth.username", "", "Optional basic auth username to use for -datasource.url")
	basicAuthPassword  = flag.String("datasource.basicAuth.password", "", "Optional basic auth password to use for -datasource.url")
	evaluationInterval = flag.Duration("evaluationInterval", 1*time.Minute, "How often to evaluate the rules. Default 1m")
	providerURL        = flag.String("provider.url", "", "Prometheus alertmanager url. Required parameter. e.g. http://127.0.0.1:9093")
)

func main() {
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	ctx, cancel := context.WithCancel(context.Background())

	logger.Infof("reading alert rules configuration file from %s", *configPath)
	alertGroups, err := config.Parse(*configPath)
	if err != nil {
		logger.Fatalf("Cannot parse configuration file %s", err)
	}
	addr := getWebServerAddr(*httpListenAddr, false)
	w := &watchdog{
		stg: datasource.NewVMStorage(*datasourceURL, *basicAuthUsername, *basicAuthPassword, &http.Client{}),
		p: provider.NewAlertManager(*providerURL, func(group, name string) string {
			return addr + fmt.Sprintf("/%s/%s/status", group, name)
		}, &http.Client{}),
	}
	for id := range alertGroups {
		go func(group config.Group) {
			w.run(ctx, group, *evaluationInterval)
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
	cancel()
	w.stop()
}

type watchdog struct {
	stg *datasource.VMStorage
	p   provider.AlertProvider
}

func (w *watchdog) run(ctx context.Context, a config.Group, evaluationInterval time.Duration) {
	t := time.NewTicker(evaluationInterval)
	var metrics []datasource.Metric
	var err error
	var alerts []provider.Alert
	defer t.Stop()
	for {
		select {
		case <-t.C:
			for _, r := range a.Rules {
				if metrics, err = w.stg.Query(ctx, r.Expr); err != nil {
					logger.Errorf("error reading metrics %s", err)
					continue
				}
				// todo check for and calculate alert states
				if len(metrics) < 1 {
					continue
				}
				alerts = provider.AlertsFromMetrics(metrics, a.Name, r)
				// todo save to storage
				if err := w.p.Send(alerts); err != nil {
					logger.Errorf("error sending alerts %s", err)
					continue
				}
				// todo is alert still active/pending?
			}

		case <-ctx.Done():
			logger.Infof("%s receive stop signal", a.Name)
			return
		}
	}
}

func getWebServerAddr(httpListenAddr string, isSecure bool) string {
	if strings.Index(httpListenAddr, ":") != 0 {
		if isSecure {
			return "https://" + httpListenAddr
		}
		return "http://" + httpListenAddr
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		panic("error getting the interface addresses ")
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return "http://" + ipnet.IP.String() + httpListenAddr
			}
		}
	}
	// no loopback ip return internal address
	return "http://127.0.0.1" + httpListenAddr
}

func (w *watchdog) stop() {
	panic("not implemented")
}
