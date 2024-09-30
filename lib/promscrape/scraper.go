package promscrape

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/azure"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consulagent"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/digitalocean"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dns"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/docker"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dockerswarm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/ec2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/eureka"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/gce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/hetzner"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/http"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kuma"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/nomad"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/openstack"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/ovhcloud"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/vultr"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/yandexcloud"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

var (
	configCheckInterval = flag.Duration("promscrape.configCheckInterval", 0, "Interval for checking for changes in -promscrape.config file. "+
		"By default, the checking is disabled. See how to reload -promscrape.config file at https://docs.victoriametrics.com/vmagent/#configuration-update")
	suppressDuplicateScrapeTargetErrors = flag.Bool("promscrape.suppressDuplicateScrapeTargetErrors", false, "Whether to suppress 'duplicate scrape target' errors; "+
		"see https://docs.victoriametrics.com/vmagent/#troubleshooting for details")
	promscrapeConfigFile = flag.String("promscrape.config", "", "Optional path to Prometheus config file with 'scrape_configs' section containing targets to scrape. "+
		"The path can point to local file and to http url. "+
		"See https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter for details")

	fileSDCheckInterval = flag.Duration("promscrape.fileSDCheckInterval", time.Minute, "Interval for checking for changes in 'file_sd_config'. "+
		"See https://docs.victoriametrics.com/sd_configs/#file_sd_configs for details")
)

// CheckConfig checks -promscrape.config for errors and unsupported options.
func CheckConfig() error {
	if *promscrapeConfigFile == "" {
		return nil
	}
	_, err := loadConfig(*promscrapeConfigFile)
	return err
}

// Init initializes Prometheus scraper with config from the `-promscrape.config`.
//
// Scraped data is passed to pushData.
func Init(pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest)) {
	mustInitClusterMemberID()
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

	// PendingScrapeConfigs - zero value means, that all scrapeConfigs are inited and ready for work.
	PendingScrapeConfigs atomic.Int32

	// configData contains -promscrape.config data
	configData atomic.Pointer[[]byte]
)

// WriteConfigData writes -promscrape.config contents to w
func WriteConfigData(w io.Writer) {
	p := configData.Load()
	if p == nil {
		// Nothing to write to w
		return
	}
	_, _ = w.Write(*p)
}

func runScraper(configFile string, pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) {
	if configFile == "" {
		// Nothing to scrape.
		return
	}

	metrics.RegisterSet(configMetricsSet)

	// Register SIGHUP handler for config reload before loadConfig.
	// This guarantees that the config will be re-read if the signal arrives just after loadConfig.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	logger.Infof("reading scrape configs from %q", configFile)
	cfg, err := loadConfig(configFile)
	if err != nil {
		logger.Fatalf("cannot read %q: %s", configFile, err)
	}
	marshaledData := cfg.marshal()
	configData.Store(&marshaledData)
	cfg.mustStart()

	configSuccess.Set(1)
	configTimestamp.Set(fasttime.UnixTimestamp())

	scs := newScrapeConfigs(pushData, globalStopCh)
	scs.add("azure_sd_configs", *azure.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getAzureSDScrapeWork(swsPrev) })
	scs.add("consul_sd_configs", *consul.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getConsulSDScrapeWork(swsPrev) })
	scs.add("consulagent_sd_configs", *consulagent.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getConsulAgentSDScrapeWork(swsPrev) })
	scs.add("digitalocean_sd_configs", *digitalocean.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDigitalOceanDScrapeWork(swsPrev) })
	scs.add("dns_sd_configs", *dns.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDNSSDScrapeWork(swsPrev) })
	scs.add("docker_sd_configs", *docker.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDockerSDScrapeWork(swsPrev) })
	scs.add("dockerswarm_sd_configs", *dockerswarm.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getDockerSwarmSDScrapeWork(swsPrev) })
	scs.add("ec2_sd_configs", *ec2.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getEC2SDScrapeWork(swsPrev) })
	scs.add("eureka_sd_configs", *eureka.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getEurekaSDScrapeWork(swsPrev) })
	scs.add("file_sd_configs", *fileSDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getFileSDScrapeWork(swsPrev) })
	scs.add("gce_sd_configs", *gce.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getGCESDScrapeWork(swsPrev) })
	scs.add("hetzner_sd_configs", *hetzner.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getHetznerSDScrapeWork(swsPrev) })
	scs.add("http_sd_configs", *http.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getHTTPDScrapeWork(swsPrev) })
	scs.add("kubernetes_sd_configs", *kubernetes.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getKubernetesSDScrapeWork(swsPrev) })
	scs.add("kuma_sd_configs", *kuma.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getKumaSDScrapeWork(swsPrev) })
	scs.add("nomad_sd_configs", *nomad.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getNomadSDScrapeWork(swsPrev) })
	scs.add("openstack_sd_configs", *openstack.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getOpenStackSDScrapeWork(swsPrev) })
	scs.add("ovhcloud_sd_configs", *ovhcloud.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getOVHCloudSDScrapeWork(swsPrev) })
	scs.add("vultr_sd_configs", *vultr.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getVultrSDScrapeWork(swsPrev) })
	scs.add("yandexcloud_sd_configs", *yandexcloud.SDCheckInterval, func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork { return cfg.getYandexCloudSDScrapeWork(swsPrev) })
	scs.add("static_configs", 0, func(cfg *Config, _ []*ScrapeWork) []*ScrapeWork { return cfg.getStaticScrapeWork() })

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
			cfgNew, err := loadConfig(configFile)
			if err != nil {
				configReloadErrors.Inc()
				configSuccess.Set(0)
				logger.Errorf("cannot read %q on SIGHUP: %s; continuing with the previous config", configFile, err)
				goto waitForChans
			}
			configSuccess.Set(1)
			if !cfgNew.mustRestart(cfg) {
				logger.Infof("nothing changed in %q", configFile)
				goto waitForChans
			}
			cfg = cfgNew
			marshaledData = cfg.marshal()
			configData.Store(&marshaledData)
			configReloads.Inc()
			configTimestamp.Set(fasttime.UnixTimestamp())
		case <-tickerCh:
			cfgNew, err := loadConfig(configFile)
			if err != nil {
				configReloadErrors.Inc()
				configSuccess.Set(0)
				logger.Errorf("cannot read %q: %s; continuing with the previous config", configFile, err)
				goto waitForChans
			}
			configSuccess.Set(1)
			if !cfgNew.mustRestart(cfg) {
				goto waitForChans
			}
			cfg = cfgNew
			marshaledData = cfg.marshal()
			configData.Store(&marshaledData)
			configReloads.Inc()
			configTimestamp.Set(fasttime.UnixTimestamp())
		case <-globalStopCh:
			cfg.mustStop()
			logger.Infof("stopping Prometheus scrapers")
			startTime := time.Now()
			scs.stop()
			logger.Infof("stopped Prometheus scrapers in %.3f seconds", time.Since(startTime).Seconds())
			return
		}
	}
}

var (
	configMetricsSet   = metrics.NewSet()
	configReloads      = configMetricsSet.NewCounter(`vm_promscrape_config_reloads_total`)
	configReloadErrors = configMetricsSet.NewCounter(`vm_promscrape_config_reloads_errors_total`)
	configSuccess      = configMetricsSet.NewGauge(`vm_promscrape_config_last_reload_successful`, nil)
	configTimestamp    = configMetricsSet.NewCounter(`vm_promscrape_config_last_reload_success_timestamp_seconds`)
)

type scrapeConfigs struct {
	pushData     func(at *auth.Token, wr *prompbmarshal.WriteRequest)
	wg           sync.WaitGroup
	stopCh       chan struct{}
	globalStopCh <-chan struct{}
	scfgs        []*scrapeConfig
}

func newScrapeConfigs(pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) *scrapeConfigs {
	return &scrapeConfigs{
		pushData:     pushData,
		stopCh:       make(chan struct{}),
		globalStopCh: globalStopCh,
	}
}

func (scs *scrapeConfigs) add(name string, checkInterval time.Duration, getScrapeWork func(cfg *Config, swsPrev []*ScrapeWork) []*ScrapeWork) {
	PendingScrapeConfigs.Add(1)
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
	pushData      func(at *auth.Token, wr *prompbmarshal.WriteRequest)
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
		if sg.scrapersStarted.Get() > 0 {
			// update duration only if at least one scraper has started
			// otherwise this SD is considered as inactive
			scfg.discoveryDuration.UpdateDuration(startTime)
		}
	}
	updateScrapeWork(cfg)
	PendingScrapeConfigs.Add(-1)

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
	pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest)

	changesCount    *metrics.Counter
	activeScrapers  *metrics.Counter
	scrapersStarted *metrics.Counter
	scrapersStopped *metrics.Counter

	globalStopCh <-chan struct{}
}

func newScraperGroup(name string, pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest), globalStopCh <-chan struct{}) *scraperGroup {
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
		sc.cancel()
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
	swsMap := make(map[string]*promutils.Labels, len(sws))
	var swsToStart []*ScrapeWork
	for _, sw := range sws {
		key := sw.key()
		originalLabels, ok := swsMap[key]
		if ok {
			if !*suppressDuplicateScrapeTargetErrors {
				logger.Errorf("skipping duplicate scrape target with identical labels; endpoint=%s, labels=%s; "+
					"make sure service discovery and relabeling is set up properly; "+
					"see also https://docs.victoriametrics.com/vmagent/#troubleshooting; "+
					"original labels for target1: %s; original labels for target2: %s",
					sw.ScrapeURL, sw.Labels.String(), originalLabels.String(), sw.OriginalLabels.String())
			}
			droppedTargetsMap.Register(sw.OriginalLabels, sw.RelabelConfigs, targetDropReasonDuplicate, nil)
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
			sc.cancel()
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
		sc, err := newScraper(sw, sg.name, sg.pushData)
		if err != nil {
			logger.Errorf("skipping scraper for url=%s, job=%s because of error: %s", sw.ScrapeURL, sg.name, err)
			continue
		}
		sg.activeScrapers.Inc()
		sg.scrapersStarted.Inc()
		sg.wg.Add(1)
		tsmGlobal.Register(&sc.sw)
		go func() {
			defer func() {
				sg.wg.Done()
				close(sc.stoppedCh)
			}()
			sc.sw.run(sc.ctx.Done(), sg.globalStopCh)
			tsmGlobal.Unregister(&sc.sw)
			sg.activeScrapers.Dec()
			sg.scrapersStopped.Inc()
		}()
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

	ctx    context.Context
	cancel context.CancelFunc

	// stoppedCh is unblocked when the given scraper is stopped.
	stoppedCh chan struct{}
}

func newScraper(sw *ScrapeWork, group string, pushData func(at *auth.Token, wr *prompbmarshal.WriteRequest)) (*scraper, error) {
	ctx, cancel := context.WithCancel(context.Background())
	sc := &scraper{
		ctx:       ctx,
		cancel:    cancel,
		stoppedCh: make(chan struct{}),
	}
	c, err := newClient(ctx, sw)
	if err != nil {
		return nil, err
	}
	sc.sw.Config = sw
	sc.sw.ScrapeGroup = group
	sc.sw.ReadData = c.ReadData
	sc.sw.PushData = pushData
	return sc, nil
}
