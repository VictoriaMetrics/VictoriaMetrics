package promscrape

import (
	"bytes"
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
	configCheckInterval = flag.Duration("promscrape.configCheckInterval", 0, "Interval for checking for changes in '-promscrape.config' file. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes")
	fileSDCheckInterval = flag.Duration("promscrape.fileSDCheckInterval", 30*time.Second, "Interval for checking for changes in 'file_sd_config'. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config for details")
	kubernetesSDCheckInterval = flag.Duration("promscrape.kubernetesSDCheckInterval", 30*time.Second, "Interval for checking for changes in Kubernetes API server. "+
		"This works only if `kubernetes_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config for details")
	gceSDCheckInterval = flag.Duration("promscrape.gceSDCheckInterval", time.Minute, "Interval for checking for changes in gce. "+
		"This works only if `gce_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config for details")
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
	cfg, data, err := loadConfig(configFile)
	if err != nil {
		logger.Fatalf("cannot read %q: %s", configFile, err)
	}

	var tickerCh <-chan time.Time
	if *configCheckInterval > 0 {
		ticker := time.NewTicker(*configCheckInterval)
		tickerCh = ticker.C
		defer ticker.Stop()
	}

	mustStop := false
	for !mustStop {
		stopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			runStaticScrapers(cfg, pushData, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runFileSDScrapers(cfg, pushData, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runKubernetesSDScrapers(cfg, pushData, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runGCESDScrapers(cfg, pushData, stopCh)
		}()

	waitForChans:
		select {
		case <-sighupCh:
			logger.Infof("SIGHUP received; reloading Prometheus configs from %q", configFile)
			cfgNew, dataNew, err := loadConfig(configFile)
			if err != nil {
				logger.Errorf("cannot read %q on SIGHUP: %s; continuing with the previous config", configFile, err)
				goto waitForChans
			}
			if bytes.Equal(data, dataNew) {
				logger.Infof("nothing changed in %q", configFile)
				goto waitForChans
			}
			cfg = cfgNew
			data = dataNew
		case <-tickerCh:
			cfgNew, dataNew, err := loadConfig(configFile)
			if err != nil {
				logger.Errorf("cannot read %q: %s; continuing with the previous config", configFile, err)
				goto waitForChans
			}
			if bytes.Equal(data, dataNew) {
				// Nothing changed since the previous loadConfig
				goto waitForChans
			}
			cfg = cfgNew
			data = dataNew
		case <-globalStopCh:
			mustStop = true
		}

		if !mustStop {
			logger.Infof("found changes in %q; applying these changes", configFile)
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

func runStaticScrapers(cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	sws := cfg.getStaticScrapeWork()
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

func runKubernetesSDScrapers(cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if cfg.kubernetesSDConfigsCount() == 0 {
		return
	}
	sws := cfg.getKubernetesSDScrapeWork()
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

func runGCESDScrapers(cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if cfg.gceSDConfigsCount() == 0 {
		return
	}
	sws := cfg.getGCESDScrapeWork()
	ticker := time.NewTicker(*gceSDCheckInterval)
	defer ticker.Stop()
	mustStop := false
	for !mustStop {
		localStopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func(sws []ScrapeWork) {
			defer wg.Done()
			logger.Infof("starting %d scrapers for `gce_sd_config` targets", len(sws))
			gceSDTargets.Set(uint64(len(sws)))
			runScrapeWorkers(sws, pushData, localStopCh)
			gceSDTargets.Set(0)
			logger.Infof("stopped all the %d scrapers for `gce_sd_config` targets", len(sws))
		}(sws)
	waitForChans:
		select {
		case <-ticker.C:
			swsNew := cfg.getGCESDScrapeWork()
			if equalStaticConfigForScrapeWorks(swsNew, sws) {
				// Nothing changed, continue waiting for updated scrape work
				goto waitForChans
			}
			logger.Infof("restarting scrapers for changed `gce_sd_config` targets")
			sws = swsNew
		case <-stopCh:
			mustStop = true
		}

		close(localStopCh)
		wg.Wait()
		gceSDReloads.Inc()
	}
}

var (
	gceSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="gce_sd"}`)
	gceSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="gce_sd"}`)
)

func runFileSDScrapers(cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	if cfg.fileSDConfigsCount() == 0 {
		return
	}
	sws := cfg.getFileSDScrapeWork(nil)
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
