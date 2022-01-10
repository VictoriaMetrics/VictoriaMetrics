package promscrape

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/digitalocean"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dns"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/docker"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dockerswarm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/ec2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/eureka"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/gce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/http"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/openstack"
	"github.com/VictoriaMetrics/metrics"
)

var (
	configCheckInterval = flag.Duration("promscrape.configCheckInterval", 0, "Interval for checking for changes in '-promscrape.config' file. "+
		"By default the checking is disabled. Send SIGHUP signal in order to force config check for changes")
	suppressDuplicateScrapeTargetErrors = flag.Bool("promscrape.suppressDuplicateScrapeTargetErrors", false, "Whether to suppress 'duplicate scrape target' errors; "+
		"see https://docs.victoriametrics.com/vmagent.html#troubleshooting for details")
	promscrapeConfigFile = flag.String("promscrape.config", "", "Optional path to Prometheus config file with 'scrape_configs' section containing targets to scrape. "+
		"The path can point to local file and to http url. "+
		"See https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter for details")

	fileSDCheckInterval = flag.Duration("promscrape.fileSDCheckInterval", 30*time.Second, "Interval for checking for changes in 'file_sd_config'. "+
		"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config for details")
)

// CheckConfig checks -promscrape.config for errors and unsupported options.
func CheckConfig() error {
	if *promscrapeConfigFile == "" {
		return fmt.Errorf("missing -promscrape.config option")
	}
	_, _, err := loadConfig(*promscrapeConfigFile)
	return err
}

// Init initializes Prometheus scraper with config from the `-promscrape.config`.
//
// Scraped data is passed to pushData.
func Init(pushData func(wr *prompbmarshal.WriteRequest)) {
	globalStopChan = make(chan struct{})
	scraperWG.Add(1)
	go func() {
		defer scraperWG.Done()
		runScraper(*promscrapeConfigFile, pushData, globalStopChan)
	}()
}

// Stop stops Prometheus scraper.
func Stop() {
	close(globalStopChan)
	scraperWG.Wait()
}

var (
	globalStopChan chan struct{}
	scraperWG      sync.WaitGroup
	// PendingScrapeConfigs - zero value means, that
	// all scrapeConfigs are inited and ready for work.
	PendingScrapeConfigs int32

	// configData contains -promscrape.config data
	configData atomic.Value
)

// WriteConfigData writes -promscrape.config contents to w
func WriteConfigData(w io.Writer) {
	v := configData.Load()
	if v == nil {
		// Nothing to write to w
		return
	}
	b := v.(*[]byte)
	_, _ = w.Write(*b)
}

func runScraper(configFile string, pushData func(wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) {
	if configFile == "" {
		// Nothing to scrape.
		return
	}

	// Register SIGHUP handler for config reload before loadConfig.
	// This guarantees that the config will be re-read if the signal arrives just after loadConfig.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	logger.Infof("reading Prometheus configs from %q", configFile)
	cfg, data, err := loadConfig(configFile)
	if err != nil {
		logger.Fatalf("cannot read %q: %s", configFile, err)
	}
	marshaledData := cfg.marshal()
	configData.Store(&marshaledData)
	cfg.mustStart()

	scs := newScrapeConfigs(pushData, globalStopCh)
	scs.add("consul_sd_configs", *consul.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getConsulSDScrapeWork(swsPrev) })
	scs.add("digitalocean_sd_configs", *digitalocean.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDigitalOceanDScrapeWork(swsPrev) })
	scs.add("dns_sd_configs", *dns.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDNSSDScrapeWork(swsPrev) })
	scs.add("docker_sd_configs", *docker.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDockerSDScrapeWork(swsPrev) })
	scs.add("dockerswarm_sd_configs", *dockerswarm.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDockerSwarmSDScrapeWork(swsPrev) })
	scs.add("ec2_sd_configs", *ec2.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getEC2SDScrapeWork(swsPrev) })
	scs.add("eureka_sd_configs", *eureka.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getEurekaSDScrapeWork(swsPrev) })
	scs.add("file_sd_configs", *fileSDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getFileSDScrapeWork(swsPrev) })
	scs.add("gce_sd_configs", *gce.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getGCESDScrapeWork(swsPrev) })
	scs.add("http_sd_configs", *http.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getHTTPDScrapeWork(swsPrev) })
	scs.add("kubernetes_sd_configs", *kubernetes.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getKubernetesSDScrapeWork(swsPrev) })
	scs.add("openstack_sd_configs", *openstack.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getOpenStackSDScrapeWork(swsPrev) })
	scs.add("static_configs", 0, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getStaticScrapeWork() })

	var tickerCh <-chan time.Time
	if *configCheckInterval > 0 {
		ticker := time.NewTicker(*configCheckInterval)
		tickerCh = ticker.C
		defer ticker.Stop()
	}
	for {
		scs.updateConfig(cfg)
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
			cfg.mustStop()
			cfgNew.mustStart()
			cfg = cfgNew
			data = dataNew
			marshaledData = cfgNew.marshal()
			configData.Store(&marshaledData)
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
			cfg.mustStop()
			cfgNew.mustStart()
			cfg = cfgNew
			data = dataNew
			configData.Store(&marshaledData)
		case <-globalStopCh:
			cfg.mustStop()
			logger.Infof("stopping Prometheus scrapers")
			startTime := time.Now()
			scs.stop()
			logger.Infof("stopped Prometheus scrapers in %.3f seconds", time.Since(startTime).Seconds())
			return
		}
		logger.Infof("found changes in %q; applying these changes", configFile)
		configReloads.Inc()
	}
}

var configReloads = metrics.NewCounter(`vm_promscrape_config_reloads_total`)

type scrapeConfigs struct {
	pushData     func(wr *prompbmarshal.WriteRequest)
	wg           sync.WaitGroup
	stopCh       chan struct{}
	globalStopCh <-chan struct{}
	scfgs        []*scrapeConfig
}

func newScrapeConfigs(pushData func(wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) *scrapeConfigs {
	return &scrapeConfigs{
		pushData:     pushData,
		stopCh:       make(chan struct{}),
		globalStopCh: globalStopCh,
	}
}

func (scs *scrapeConfigs) add(name string, checkInterval time.Duration, getScrapeWork func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork) {
	atomic.AddInt32(&PendingScrapeConfigs, 1)
	scfg := &scrapeConfig{
		name:          name,
		pushData:      scs.pushData,
		getScrapeWork: getScrapeWork,
		checkInterval: checkInterval,
		cfgCh:         make(chan *Config, 1),
		stopCh:        scs.stopCh,

		discoveryDuration: metrics.GetOrCreateHistogram(fmt.Sprintf("vm_promscrape_service_discovery_duration_seconds{type=%q}", name)),
	}
	scs.wg.Add(1)
	go func() {
		defer scs.wg.Done()
		scfg.run(scs.globalStopCh)
	}()
	scs.scfgs = append(scs.scfgs, scfg)
}

func (scs *scrapeConfigs) updateConfig(cfg *Config) {
	for _, scfg := range scs.scfgs {
		scfg.cfgCh <- cfg
	}
}

func (scs *scrapeConfigs) stop() {
	close(scs.stopCh)
	scs.wg.Wait()
	scs.scfgs = nil
}

type scrapeConfig struct {
	name          string
	pushData      func(wr *prompbmarshal.WriteRequest)
	getScrapeWork func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork
	checkInterval time.Duration
	cfgCh         chan *Config
	stopCh        <-chan struct{}

	discoveryDuration *metrics.Histogram
}

func (scfg *scrapeConfig) run(globalStopCh <-chan struct{}) {
	sg := newScraperGroup(scfg.name, scfg.pushData, globalStopCh)
	defer sg.stop()

	var tickerCh <-chan time.Time
	if scfg.checkInterval > 0 {
		ticker := time.NewTicker(scfg.checkInterval)
		defer ticker.Stop()
		tickerCh = ticker.C
	}

	cfg := <-scfg.cfgCh
	var swsPrev []*ScrapeWork
	updateScrapeWork := func(cfg *Config) {
		startTime := time.Now()
		sws := scfg.getScrapeWork(cfg, swsPrev)
		sg.update(sws)
		swsPrev = sws
		scfg.discoveryDuration.UpdateDuration(startTime)
	}
	updateScrapeWork(cfg)
	atomic.AddInt32(&PendingScrapeConfigs, -1)

	for {

		select {
		case <-scfg.stopCh:
			return
		case cfg = <-scfg.cfgCh:
		case <-tickerCh:
		}
		updateScrapeWork(cfg)
	}
}

type scraperGroup struct {
	name     string
	wg       sync.WaitGroup
	mLock    sync.Mutex
	m        map[string]*scraper
	pushData func(wr *prompbmarshal.WriteRequest)

	changesCount    *metrics.Counter
	activeScrapers  *metrics.Counter
	scrapersStarted *metrics.Counter
	scrapersStopped *metrics.Counter

	globalStopCh <-chan struct{}
}

func newScraperGroup(name string, pushData func(wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) *scraperGroup {
	sg := &scraperGroup{
		name:     name,
		m:        make(map[string]*scraper),
		pushData: pushData,

		changesCount:    metrics.NewCounter(fmt.Sprintf(`vm_promscrape_config_changes_total{type=%q}`, name)),
		activeScrapers:  metrics.NewCounter(fmt.Sprintf(`vm_promscrape_active_scrapers{type=%q}`, name)),
		scrapersStarted: metrics.NewCounter(fmt.Sprintf(`vm_promscrape_scrapers_started_total{type=%q}`, name)),
		scrapersStopped: metrics.NewCounter(fmt.Sprintf(`vm_promscrape_scrapers_stopped_total{type=%q}`, name)),

		globalStopCh: globalStopCh,
	}
	metrics.NewGauge(fmt.Sprintf(`vm_promscrape_targets{type=%q, status="up"}`, name), func() float64 {
		return float64(tsmGlobal.StatusByGroup(sg.name, true))
	})
	metrics.NewGauge(fmt.Sprintf(`vm_promscrape_targets{type=%q, status="down"}`, name), func() float64 {
		return float64(tsmGlobal.StatusByGroup(sg.name, false))
	})
	return sg
}

func (sg *scraperGroup) stop() {
	sg.mLock.Lock()
	for _, sc := range sg.m {
		close(sc.stopCh)
	}
	sg.m = nil
	sg.mLock.Unlock()
	sg.wg.Wait()
}

func (sg *scraperGroup) update(sws []*ScrapeWork) {
	sg.mLock.Lock()
	defer sg.mLock.Unlock()

	additionsCount := 0
	deletionsCount := 0
	swsMap := make(map[string][]prompbmarshal.Label, len(sws))
	var swsToStart []*ScrapeWork
	for _, sw := range sws {
		key := sw.key()
		originalLabels, ok := swsMap[key]
		if ok {
			if !*suppressDuplicateScrapeTargetErrors {
				logger.Errorf("skipping duplicate scrape target with identical labels; endpoint=%s, labels=%s; "+
					"make sure service discovery and relabeling is set up properly; "+
					"see also https://docs.victoriametrics.com/vmagent.html#troubleshooting; "+
					"original labels for target1: %s; original labels for target2: %s",
					sw.ScrapeURL, sw.LabelsString(), promLabelsString(originalLabels), promLabelsString(sw.OriginalLabels))
			}
			droppedTargetsMap.Register(sw.OriginalLabels)
			continue
		}
		swsMap[key] = sw.OriginalLabels
		if sg.m[key] != nil {
			// The scraper for the given key already exists.
			continue
		}
		swsToStart = append(swsToStart, sw)
	}

	// Stop deleted scrapers before starting new scrapers in order to prevent
	// series overlap when old scrape target is substituted by new scrape target.
	var stoppedChs []<-chan struct{}
	for key, sc := range sg.m {
		if _, ok := swsMap[key]; !ok {
			close(sc.stopCh)
			stoppedChs = append(stoppedChs, sc.stoppedCh)
			delete(sg.m, key)
			deletionsCount++
		}
	}
	// Wait until all the deleted scrapers are stopped before starting new scrapers.
	for _, ch := range stoppedChs {
		<-ch
	}

	// Start new scrapers only after the deleted scrapers are stopped.
	for _, sw := range swsToStart {
		sc := newScraper(sw, sg.name, sg.pushData)
		sg.activeScrapers.Inc()
		sg.scrapersStarted.Inc()
		sg.wg.Add(1)
		tsmGlobal.Register(sw)
		go func(sw *ScrapeWork) {
			defer func() {
				sg.wg.Done()
				close(sc.stoppedCh)
			}()
			sc.sw.run(sc.stopCh, sg.globalStopCh)
			tsmGlobal.Unregister(sw)
			sg.activeScrapers.Dec()
			sg.scrapersStopped.Inc()
		}(sw)
		key := sw.key()
		sg.m[key] = sc
		additionsCount++
	}

	if additionsCount > 0 || deletionsCount > 0 {
		sg.changesCount.Add(additionsCount + deletionsCount)
		logger.Infof("%s: added targets: %d, removed targets: %d; total targets: %d", sg.name, additionsCount, deletionsCount, len(sg.m))
	}
}

type scraper struct {
	sw scrapeWork

	// stopCh is unblocked when the given scraper must be stopped.
	stopCh chan struct{}

	// stoppedCh is unblocked when the given scraper is stopped.
	stoppedCh chan struct{}
}

func newScraper(sw *ScrapeWork, group string, pushData func(wr *prompbmarshal.WriteRequest)) *scraper {
	sc := &scraper{
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
	c := newClient(sw)
	sc.sw.Config = sw
	sc.sw.ScrapeGroup = group
	sc.sw.ReadData = c.ReadData
	sc.sw.GetStreamReader = c.GetStreamReader
	sc.sw.PushData = pushData
	return sc
}
