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
)

var (
	configCheckInterval = flag.Duration("promscrape.configCheckInterval", 0, "Interval for checking for changes in '-promscrape.config' file. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes")
	staticCheckInterval = flag.Duration("promscrape.staticCheckInterval", 30*time.Second, "Interval for checking for changes in static config file. ")
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
		staticReloadCh := make(chan struct{})
		fileSDReloadCh := make(chan struct{})
		kubernetesSDReloadCh := make(chan struct{})
		EC2SDReloadCh := make(chan struct{})
		GCESDReloadCh := make(chan struct{})

		stopCh := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers(Static, cfg, pushData, staticReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers(FileSD, cfg, pushData, fileSDReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers(KubernetesSD, cfg, pushData, kubernetesSDReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers(EC2SD, cfg, pushData, EC2SDReloadCh, stopCh)
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSDScrapers(GCESD, cfg, pushData, GCESDReloadCh, stopCh)
		}()

		reloadConfig := func() {
			cfgNew, dataNew, err := loadConfig(configFile)
			if err != nil {
				logger.Errorf("cannot read %q on SIGHUP: %s; continuing with the previous config", configFile, err)
				return
			}
			if bytes.Equal(data, dataNew) {
				logger.Infof("nothing changed in %q", configFile)
				return
			}
			logger.Infof("found changes in %q; applying these changes", configFile)
			*cfg = *cfgNew
			data = dataNew
			staticReloadCh <- struct{}{}
			fileSDReloadCh <- struct{}{}
			kubernetesSDReloadCh <- struct{}{}
			EC2SDReloadCh <- struct{}{}
			GCESDReloadCh <- struct{}{}
			configReloads.Inc()
		}

	waitForChans:
		select {
		case <-sighupCh:
			logger.Infof("SIGHUP received; reloading Prometheus configs from %q", configFile)
			reloadConfig()
			goto waitForChans
		case <-tickerCh:
			reloadConfig()
			goto waitForChans
		case <-globalStopCh:
			close(stopCh)
			mustStop = true
		}

		logger.Infof("stopping Prometheus scrapers")
		startTime := time.Now()
		wg.Wait()
		logger.Infof("stopped Prometheus scrapers in %.3f seconds", time.Since(startTime).Seconds())
	}
}

var configReloads = metrics.NewCounter(`vm_promscrape_config_reloads_total`)
var (
	staticTargets = metrics.NewCounter(`vm_promscrape_targets{type="static"}`)
	staticReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="static"}`)

	kubernetesSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="kubernetes_sd"}`)
	kubernetesSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="kubernetes_sd"}`)

	ec2SDTargets = metrics.NewCounter(`vm_promscrape_targets{type="ec2_sd"}`)
	ec2SDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="ec2_sd"}`)

	gceSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="gce_sd"}`)
	gceSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="gce_sd"}`)

	fileSDTargets = metrics.NewCounter(`vm_promscrape_targets{type="file_sd"}`)
	fileSDReloads = metrics.NewCounter(`vm_promscrape_reloads_total{type="file_sd"}`)
)

type SDScraperType int

const (
	Static SDScraperType = iota
	FileSD
	KubernetesSD
	EC2SD
	GCESD
)

type SwWithStopCh struct {
	sw     ScrapeWork
	stopCh chan struct{}
}

func runSDScrapers(t SDScraperType, cfg *Config, pushData func(wr *prompbmarshal.WriteRequest), reloadCh <-chan struct{}, stopCh <-chan struct{}) {
	var sws []ScrapeWork
	var sdTargets *metrics.Counter
	var sdReloader *metrics.Counter
	var reloadInterval *time.Duration
	var sdName string = ""
	swsWithStopCh := make(map[string]*SwWithStopCh)

	switch t {
	case Static:
		sdTargets = staticTargets
		sdReloader = staticReloads
		reloadInterval = staticCheckInterval
		sdName = "static"
	case FileSD:
		sdTargets = fileSDTargets
		sdReloader = fileSDReloads
		reloadInterval = fileSDCheckInterval
		sdName = "file"
	case KubernetesSD:
		sdTargets = kubernetesSDTargets
		sdReloader = kubernetesSDReloads
		reloadInterval = kubernetesSDCheckInterval
		sdName = "kubernetes"
	case EC2SD:
		sdTargets = ec2SDTargets
		sdReloader = ec2SDReloads
		reloadInterval = ec2SDCheckInterval
		sdName = "ec2"
	case GCESD:
		sdTargets = gceSDTargets
		sdReloader = gceSDReloads
		reloadInterval = gceSDCheckInterval
		sdName = "gce"
	default:
		return
	}

	loadSwsByType := func(t SDScraperType, sws []ScrapeWork) []ScrapeWork {
		switch t {
		case Static:
			return cfg.getStaticScrapeWork()
		case FileSD:
			return cfg.getFileSDScrapeWork(sws)
		case KubernetesSD:
			return cfg.getKubernetesSDScrapeWork()
		case EC2SD:
			return cfg.getEC2SDScrapeWork()
		case GCESD:
			return cfg.getGCESDScrapeWork()
		default:
			return []ScrapeWork{}
		}
	}

	sws = loadSwsByType(t, nil)
	ticker := time.NewTicker(*reloadInterval)
	defer ticker.Stop()
	var wg sync.WaitGroup

	runChangedSws := func() {
		newSwsWithStopCh := make(map[string]*SwWithStopCh)
		for _, sw := range sws {
			swHash, err := hashForScrapeWork(&sw)
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
		for newSwHash, _ := range newSwsWithStopCh {
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
			logger.Infof("starting %d scrapers for `%s_sd_config` targets", len(sws), sdName)
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
		if equalScrapeWorks(swsNew, sws) {
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
		case <-reloadCh:
			reloadCfg()
			sdReloader.Inc()
		case <-stopCh:
			break waitForChans
		}

		for _, swWithStopCh := range swsWithStopCh {
			close(swWithStopCh.stopCh)
		}

		wg.Wait()
	}
}

func equalScrapeWorks(as, bs []ScrapeWork) bool {
	if len(as) != len(bs) {
		return false
	}
	for i := range as {
		if !equalScrapeWork(&as[i], &bs[i]) {
			return false
		}
	}
	return true
}

func equalScrapeWork(a, b *ScrapeWork) bool {
	aHash, err := hashForScrapeWork(a)
	if err != nil {
		return false
	}
	bHash, err := hashForScrapeWork(b)
	if err != nil {
		return false
	}
	if aHash != bHash {
		return false
	}
	return true
}

func hashForScrapeWork(sw *ScrapeWork) (string, error) {
	if sw == nil {
		return "", nil
	}

	buff := bytes.NewBuffer([]byte{})
	buff.WriteString(sw.ScrapeURL)
	buff.WriteString(fmt.Sprintf("%d|", sw.ScrapeInterval))
	buff.WriteString(fmt.Sprintf("%d|", sw.ScrapeTimeout))
	buff.WriteString(fmt.Sprintf("%t|", sw.HonorLabels))
	buff.WriteString(fmt.Sprintf("%t|", sw.HonorTimestamps))
	for _, label := range sw.Labels {
		buff.WriteString(fmt.Sprintf("%s:%s|", label.Name, label.Value))
	}
	if sw.AuthConfig != nil {
		buff.WriteString(fmt.Sprintf("%s|", sw.AuthConfig.Authorization))
		if sw.AuthConfig.TLSRootCA != nil {
			certs := sw.AuthConfig.TLSRootCA.Subjects()
			for _, cert := range certs {
				buff.Write(cert)
			}
		}
		if sw.AuthConfig.TLSCertificate != nil {
			if sw.AuthConfig.TLSCertificate.Leaf != nil {
				buff.Write(sw.AuthConfig.TLSCertificate.Leaf.Raw)
			}
		}
		buff.WriteString(fmt.Sprintf("%s|", sw.AuthConfig.TLSServerName))
		buff.WriteString(fmt.Sprintf("%t|", sw.AuthConfig.TLSInsecureSkipVerify))
	}
	for _, relabelConfig := range sw.MetricRelabelConfigs {
		for _, sourceLabel := range relabelConfig.SourceLabels {
			buff.WriteString(fmt.Sprintf("%s|", sourceLabel))
		}
		buff.WriteString(fmt.Sprintf("%s|", relabelConfig.Separator))
		buff.WriteString(fmt.Sprintf("%s|", relabelConfig.TargetLabel))

		if relabelConfig.Regex != nil {
			buff.WriteString(fmt.Sprintf("%s|", relabelConfig.Regex.String()))
		}

		buff.WriteString(fmt.Sprintf("%d|", relabelConfig.Modulus))
		buff.WriteString(fmt.Sprintf("%s|", relabelConfig.Replacement))
		buff.WriteString(fmt.Sprintf("%s|", relabelConfig.Action))
	}
	buff.WriteString(fmt.Sprintf("%d|", sw.SampleLimit))
	h := sha256.New()
	n, err := h.Write(buff.Bytes())
	if n != buff.Len() {
		return "", fmt.Errorf("hash for sw failed, as write to sha256 expected %d, but only %s done", buff.Len(), n)
	}
	if err != nil {
		return "", err
	}
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
