package promscrape

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
	"go.mongodb.org/mongo-driver/bson"
)

var (
	configCheckInterval = flag.Duration("promscrape.configCheckInterval", 0, "Interval for checking for changes in '-promscrape.config' file. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes")
	fileSDCheckInterval = flag.Duration("promscrape.fileSDCheckInterval", 30*time.Second, "Interval for checking for changes in 'file_sd_config'. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config for details")
	kubernetesSDCheckInterval = flag.Duration("promscrape.kubernetesSDCheckInterval", 30*time.Second, "Interval for checking for changes in Kubernetes API server. "+
		"This works only if `kubernetes_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config for details")
	ec2SDCheckInterval = flag.Duration("promscrape.ec2SDCheckInterval", time.Minute, "Interval for checking for changes in ec2. "+
		"This works only if `ec2_sd_configs` is configured in '-promscrape.config' file. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config for details")
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
		staticReloadCh := make(chan struct{})
		fileReloadCh := make(chan struct{})
		k8sReloadCh := make(chan struct{})
		ec2ReloadCh := make(chan struct{})
		gceReloadCh := make(chan struct{})

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers("static", cfg, pushData, staticReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers("file", cfg, pushData, fileReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers("k8s", cfg, pushData, k8sReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers("ec2", cfg, pushData, ec2ReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers("gce", cfg, pushData, gceReloadCh, stopCh)
		}()

		reloadConfigFile := func() {
			cfgNew, dataNew, err := loadConfig(configFile)
			if err != nil {
				logger.Errorf("cannot read %q on SIGHUP: %s; continuing with the previous config", configFile, err)
				return
			}
			if bytes.Equal(data, dataNew) {
				logger.Infof("nothing changed in %q", configFile)
				return
			}
			cfg = cfgNew
			*cfg = *cfgNew
			data = dataNew
			staticReloadCh <- struct{}{}
			fileReloadCh <- struct{}{}
			k8sReloadCh <- struct{}{}
			configReloads.Inc()
		}
	waitForChans:
		select {
		case <-sighupCh:
			logger.Infof("SIGHUP received; reloading Prometheus configs from %q", configFile)
			reloadConfigFile()
			configReloads.Inc()
			goto waitForChans
		case <-tickerCh:
			reloadConfigFile()
			configReloads.Inc()
			goto waitForChans
		case <-globalStopCh:
			close(stopCh)
			mustStop = true
		}

		if !mustStop {
			logger.Infof("found changes in %q; applying these changes", configFile)
		}
		logger.Infof("stopping Prometheus scrapers")
		startTime := time.Now()
		wg.Wait()
		logger.Infof("stopped Prometheus scrapers in %.3f seconds", time.Since(startTime).Seconds())
	}
}

var configReloads = metrics.NewCounter(`vm_promscrape_config_reloads_total`)

var staticTargets = metrics.NewCounter(`vm_promscrape_targets{type="static"}`)
var staticReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="static"}`)

type SwWithStopCh struct {
	sw     ScrapeWork
	stopCh chan struct{}
}

func runSDScrapers(t string, cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), reloadCh <-chan struct{}, stopCh <-chan struct{}) {
	var sws []ScrapeWork
	var sdTargets *metrics.Counter
	var sdReloader *metrics.Counter
	swsWithStopCh := make(map[string]*SwWithStopCh)

	switch t {
	case "file":
		sdTargets = fileSDTargets
		sdReloader = fileSDReloads
	case "static":
		sdTargets = staticTargets
		sdReloader = staticReloads
	case "k8s":
		sdTargets = kubernetesSDTargets
		sdReloader = kubernetesSDReloads
	case "ec2":
		sdTargets = ec2SDTargets
		sdReloader = ec2SDReloads
	case "gce":
		sdTargets = gceSDTargets
		sdReloader = gceSDReloads
	default:
		return
	}

	loadSwsByType := func(t string, sws []ScrapeWork) []ScrapeWork {
		switch t {
		case "file":
			if cfg.fileSDConfigsCount() == 0 {
				logger.Infof("no fileSDConfigs found, exists...")
				return []ScrapeWork{}
			}
			newSws := cfg.getFileSDScrapeWork(sws)
			return newSws
		case "static":
			newSws := cfg.getStaticScrapeWork()
			if len(sws) == 0 {
				return []ScrapeWork{}
			}
			return newSws
		case "k8s":
			if cfg.kubernetesSDConfigsCount() == 0 {
				return []ScrapeWork{}
			}
			newSws := cfg.getKubernetesSDScrapeWork()
			return newSws
		case "ec2":
			if cfg.ec2SDConfigsCount() == 0 {
				return []ScrapeWork{}
			}
			newSws := cfg.getEC2SDScrapeWork()
			return newSws
		case "gce":
			if cfg.gceSDConfigsCount() == 0 {
				return []ScrapeWork{}
			}
			newSws := cfg.getGCESDScrapeWork()
			return newSws
		default:
			return []ScrapeWork{}
		}

	}
	sws = loadSwsByType(t, nil)

	ticker := time.NewTicker(*fileSDCheckInterval)
	defer ticker.Stop()
	var wg sync.WaitGroup

	runChangedSws := func() {
		newSwsWithStopCh := make(map[string]*SwWithStopCh)
		for _, sw := range sws {
			swHash, err := hashScrapeWork(sw)
			if err != nil {
				logger.Warnf("hash for sw: %v failed, err is: %v", sw, err)
				continue
			}
			logger.Infof("hash for sw: %v success, hash is: %v", sw, swHash)
			newSwsWithStopCh[swHash] = &SwWithStopCh{
				sw:     sw,
				stopCh: make(chan struct{}),
			}
		}
		// 1. run new sw worker
		for newSwHash := range newSwsWithStopCh {
			if swWithStopCh, exists := swsWithStopCh[newSwHash]; exists {
				// same sw worker exists, copy ch
				newSwsWithStopCh[newSwHash].stopCh = swWithStopCh.stopCh
				logger.Infof("old exists sw: %v keep running with hash: %s", swWithStopCh.sw, newSwHash)
				continue
			} else {
				logger.Infof("new sw: %v start running with hash: %s", newSwsWithStopCh[newSwHash].sw, newSwHash)
			}
			// new sw, run it
			wg.Add(1)
			sdTargets.Inc()
			go func(swWithStopCh *SwWithStopCh) {
				defer wg.Done()
				defer sdTargets.Dec()
				runScrapeWorker(swWithStopCh.sw, pushData, swWithStopCh.stopCh)
			}(newSwsWithStopCh[newSwHash])

		}
		// 2. clear old sw worker
		for oldSwHash, swWithStopCh := range swsWithStopCh {
			if _, exists := newSwsWithStopCh[oldSwHash]; exists {
				continue
			}
			logger.Infof("clear old sw: %v start with hash: %s", swWithStopCh.sw, oldSwHash)
			close(swWithStopCh.stopCh)
		}

		// replace swsWithStopCh with new swsWithStopCh
		swsWithStopCh = newSwsWithStopCh
	}

	loadCfg := func(swsNew []ScrapeWork) {
		logger.Infof("reloading scrapers for changed `file_sd_config` targets")
		logger.Infof("begore reload cfg, fd targets number is: %d", fileSDTargets.Get())
		sws = swsNew
		runChangedSws()
		logger.Infof("after reload cfg, fd targets number is: %d", fileSDTargets.Get())

	}

	reloadCfg := func() {
		swsNew := loadSwsByType(t, sws)
		if equalStaticConfigForScrapeWorks(swsNew, sws) {
			return
		}
		loadCfg(swsNew)
	}

	loadCfg(sws)

waitForChans:
	for {
		select {
		case <-ticker.C:
			reloadCfg()
			sdReloader.Inc()
		case <-stopCh:
			break waitForChans
		case <-reloadCh:
			reloadCfg()
			sdReloader.Inc()
		}
	}

	for _, swWithStopCh := range swsWithStopCh {
		close(swWithStopCh.stopCh)
	}

	wg.Wait()

}

var (
	kubernetesSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="kubernetes_sd"}`)
	kubernetesSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="kubernetes_sd"}`)
)

var (
	ec2SDTargets = metrics.NewCounter(`vm_promscrape_targets{type="ec2_sd"}`)
	ec2SDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="ec2_sd"}`)
)

var (
	gceSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="gce_sd"}`)
	gceSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="gce_sd"}`)
)

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

func hashScrapeWork(sw ScrapeWork) (string, error) {
	// make ID 0 before hash
	sw.ID = 0
	b, err := bson.Marshal(sw)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(b)
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func runScrapeWorker(sw ScrapeWork, pushData func(wr *prompbmarshal.WriteRequest), stopCh <-chan struct{}) {
	tsmGlobal.Register(sw)
	c := newClient(&sw)
	var lsw scrapeWork
	lsw.Config = sw
	lsw.ReadData = c.ReadData
	lsw.PushData = pushData
	lsw.run(stopCh)
	tsmGlobal.Unregister(sw)
}
