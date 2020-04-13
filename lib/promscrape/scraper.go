package promscrape

import (
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

var (
	fileSDCheckInterval = flag.Duration("promscrape.fileSDCheckInterval", 30*time.Second, "Interval for checking for changes in 'file_sd_config'. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config")
	kubernetesSDCheckInterval = flag.Duration("promscrape.kubernetesSDCheckInterval", 30*time.Second, "Interval for checking for changes in Kubernetes API server. "+
		"This works only if `kubernetes_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config for details")
	promscrapeConfigFile = flag.String("promscrape.config", "", "Optional path to Prometheus config file with 'scrape_configs' section containing targets to scrape. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config for details")
)

// Init initializes Prometheus scraper with config from the `-promscrape.config`.
//
// Scraped data is passed to pushData.
func Init(pushData func(wr *prompbmarshal.WriteRequest)) {
	globalStopCh = make(chan struct{})
	scraperWG.Add(1)
	go func() {
		defer scraperWG.Done()
		runScraper(*promscrapeConfigFile, pushData, globalStopCh)
	}()
}

// Stop stops Prometheus scraper.
func Stop() {
	close(globalStopCh)
	scraperWG.Wait()
}

var (
	globalStopCh chan struct{}
	scraperWG    sync.WaitGroup
)

func runScraper(configFile string, pushData func(wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) {
	if configFile == "" {
		// Nothing to scrape.
		return
	}
	sighupCh := make(chan os.Signal, 1)
	signal.Notify(sighupCh, syscall.SIGHUP)

	logger.Infof("reading Prometheus configs from %q", configFile)
	cfg, err := loadConfig(configFile)
	if err != nil {
		logger.Fatalf("cannot read %q: %s", configFile, err)
	}
	swsStatic := cfg.getStaticScrapeWork()
	swsFileSD := cfg.getFileSDScrapeWork(nil)
	swsK8S := cfg.getKubernetesSDScrapeWork()

	mustStop := false
	for !mustStop {
		stopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStaticScrapers(swsStatic, pushData, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runFileSDScrapers(swsFileSD, cfg, pushData, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runKubernetesSDScrapers(swsK8S, cfg, pushData, stopCh)
		}()

	waitForChans:
		select {
		case <-sighupCh:
			logger.Infof("SIGHUP received; reloading Prometheus configs from %q", configFile)
			cfgNew, err := loadConfig(configFile)
			if err != nil {
				logger.Errorf("cannot read %q: %s; continuing with the previous config", configFile, err)
				goto waitForChans
			}
			cfg = cfgNew
			swsStatic = cfg.getStaticScrapeWork()
			swsFileSD = cfg.getFileSDScrapeWork(swsFileSD)
			swsK8S = cfg.getKubernetesSDScrapeWork()
		case <-globalStopCh:
			mustStop = true
		}

		logger.Infof("stopping Prometheus scrapers")
		startTime := time.Now()
		close(stopCh)
		wg.Wait()
		logger.Infof("stopped Prometheus scrapers in %.3f seconds", time.Since(startTime).Seconds())
		configReloads.Inc()
	}
}

var configReloads = metrics.NewCounter(`vm_promscrape_config_reloads_total`)

func runStaticScrapers(sws []ScrapeWork, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if len(sws) == 0 {
		return
	}
	logger.Infof("starting %d scrapers for `static_config` targets", len(sws))
	staticTargets.Set(uint64(len(sws)))
	runScrapeWorkers(sws, pushData, stopCh)
	staticTargets.Set(0)
	logger.Infof("stopped all the %d scrapers for `static_config` targets", len(sws))
}

var staticTargets = metrics.NewCounter(`vm_promscrape_targets{type="static"}`)

func runKubernetesSDScrapers(sws []ScrapeWork, cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if cfg.kubernetesSDConfigsCount() == 0 {
		return
	}
	ticker := time.NewTicker(*kubernetesSDCheckInterval)
	defer ticker.Stop()
	mustStop := false
	for !mustStop {
		localStopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func(sws []ScrapeWork) {
			defer wg.Done()
			logger.Infof("starting %d scrapers for `kubernetes_sd_config` targets", len(sws))
			kubernetesSDTargets.Set(uint64(len(sws)))
			runScrapeWorkers(sws, pushData, localStopCh)
			kubernetesSDTargets.Set(0)
			logger.Infof("stopped all the %d scrapers for `kubernetes_sd_config` targets", len(sws))
		}(sws)
	waitForChans:
		select {
		case <-ticker.C:
			swsNew := cfg.getKubernetesSDScrapeWork()
			if equalStaticConfigForScrapeWorks(swsNew, sws) {
				// Nothing changed, continue waiting for updated scrape work
				goto waitForChans
			}
			logger.Infof("restarting scrapers for changed `kubernetes_sd_config` targets")
			sws = swsNew
		case <-stopCh:
			mustStop = true
		}

		close(localStopCh)
		wg.Wait()
		kubernetesSDReloads.Inc()
	}
}

var (
	kubernetesSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="kubernetes_sd"}`)
	kubernetesSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="kubernetes_sd"}`)
)

func runFileSDScrapers(sws []ScrapeWork, cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if cfg.fileSDConfigsCount() == 0 {
		return
	}
	ticker := time.NewTicker(*fileSDCheckInterval)
	defer ticker.Stop()
	mustStop := false
	for !mustStop {
		localStopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func(sws []ScrapeWork) {
			defer wg.Done()
			logger.Infof("starting %d scrapers for `file_sd_config` targets", len(sws))
			fileSDTargets.Set(uint64(len(sws)))
			runScrapeWorkers(sws, pushData, localStopCh)
			fileSDTargets.Set(0)
			logger.Infof("stopped all the %d scrapers for `file_sd_config` targets", len(sws))
		}(sws)
	waitForChans:
		select {
		case <-ticker.C:
			swsNew := cfg.getFileSDScrapeWork(sws)
			if equalStaticConfigForScrapeWorks(swsNew, sws) {
				// Nothing changed, continue waiting for updated scrape work
				goto waitForChans
			}
			logger.Infof("restarting scrapers for changed `file_sd_config` targets")
			sws = swsNew
		case <-stopCh:
			mustStop = true
		}

		close(localStopCh)
		wg.Wait()
		fileSDReloads.Inc()
	}
}

var (
	fileSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="file_sd"}`)
	fileSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="file_sd"}`)
)

func equalStaticConfigForScrapeWorks(as, bs []ScrapeWork) bool {
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if !equalStaticConfigForScrapeWork(&as[i], &bs[i]) {
			return false
		}
	}
	return true
}

func equalStaticConfigForScrapeWork(a, b *ScrapeWork) bool {
	// `static_config` can change only ScrapeURL and Labels. So compare only them.
	if a.ScrapeURL != b.ScrapeURL {
		return false
	}
	if !equalLabels(a.Labels, b.Labels) {
		return false
	}
	return true
}

func equalLabels(as, bs []prompbmarshal.Label) bool {
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if !equalLabel(&as[i], &bs[i]) {
			return false
		}
	}
	return true
}

func equalLabel(a, b *prompbmarshal.Label) bool {
	if a.Name != b.Name {
		return false
	}
	if a.Value != b.Value {
		return false
	}
	return true
}

// runScrapeWorkers runs sws.
//
// This function returns after closing stopCh.
func runScrapeWorkers(sws []ScrapeWork, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	tsmGlobal.RegisterAll(sws)
	var wg sync.WaitGroup
	for i := range sws {
		cfg := &sws[i]
		c := newClient(cfg)
		var sw scrapeWork
		sw.Config = *cfg
		sw.ReadData = c.ReadData
		sw.PushData = pushData
		wg.Add(1)
		go func() {
			defer wg.Done()
			sw.run(stopCh)
		}()
	}
	wg.Wait()
	tsmGlobal.UnregisterAll(sws)
}
