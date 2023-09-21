package promscrape

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/http"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kuma"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/nomad"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/openstack"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/yandexcloud"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
	"gopkg.in/yaml.v2"
)

var (
	noStaleMarkers       = flag.Bool("promscrape.noStaleMarkers", false, "Whether to disable sending Prometheus stale markers for metrics when scrape target disappears. This option may reduce memory usage if stale markers aren't needed for your setup. This option also disables populating the scrape_series_added metric. See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series")
	seriesLimitPerTarget = flag.Int("promscrape.seriesLimitPerTarget", 0, "Optional limit on the number of unique time series a single scrape target can expose. See https://docs.victoriametrics.com/vmagent.html#cardinality-limiter for more info")
	strictParse          = flag.Bool("promscrape.config.strictParse", true, "Whether to deny unsupported fields in -promscrape.config . Set to false in order to silently skip unsupported fields")
	dryRun               = flag.Bool("promscrape.config.dryRun", false, "Checks -promscrape.config file for errors and unsupported fields and then exits. "+
		"Returns non-zero exit code on parsing errors and emits these errors to stderr. "+
		"See also -promscrape.config.strictParse command-line flag. "+
		"Pass -loggerLevel=ERROR if you don't need to see info messages in the output.")
	dropOriginalLabels = flag.Bool("promscrape.dropOriginalLabels", false, "Whether to drop original labels for scrape targets at /targets and /api/v1/targets pages. "+
		"This may be needed for reducing memory usage when original labels for big number of scrape targets occupy big amounts of memory. "+
		"Note that this reduces debuggability for improper per-target relabeling configs")
	clusterMembersCount = flag.Int("promscrape.cluster.membersCount", 1, "The number of members in a cluster of scrapers. "+
		"Each member must have a unique -promscrape.cluster.memberNum in the range 0 ... promscrape.cluster.membersCount-1 . "+
		"Each member then scrapes roughly 1/N of all the targets. By default, cluster scraping is disabled, i.e. a single scraper scrapes all the targets")
	clusterMemberNum = flag.String("promscrape.cluster.memberNum", "0", "The number of vmagent instance in the cluster of scrapers. "+
		"It must be a unique value in the range 0 ... promscrape.cluster.membersCount-1 across scrapers in the cluster. "+
		"Can be specified as pod name of Kubernetes StatefulSet - pod-name-Num, where Num is a numeric part of pod name. "+
		"See also -promscrape.cluster.memberLabel")
	clusterMemberLabel = flag.String("promscrape.cluster.memberLabel", "", "If non-empty, then the label with this name and the -promscrape.cluster.memberNum value "+
		"is added to all the scraped metrics")
	clusterReplicationFactor = flag.Int("promscrape.cluster.replicationFactor", 1, "The number of members in the cluster, which scrape the same targets. "+
		"If the replication factor is greater than 1, then the deduplication must be enabled at remote storage side. See https://docs.victoriametrics.com/#deduplication")
	clusterName = flag.String("promscrape.cluster.name", "", "Optional name of the cluster. If multiple vmagent clusters scrape the same targets, "+
		"then each cluster must have unique name in order to properly de-duplicate samples received from these clusters. "+
		"See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2679")
)

var clusterMemberID int

func mustInitClusterMemberID() {
	s := *clusterMemberNum
	// special case for kubernetes deployment, where pod-name formatted at some-pod-name-1
	// obtain memberNum from last segment
	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2359
	if idx := strings.LastIndexByte(s, '-'); idx >= 0 {
		s = s[idx+1:]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		logger.Fatalf("cannot parse -promscrape.cluster.memberNum=%q: %s", *clusterMemberNum, err)
	}
	if *clusterMembersCount < 1 {
		logger.Fatalf("-promscrape.cluster.membersCount can't be lower than 1: got %d", *clusterMembersCount)
	}
	if n < 0 || n >= *clusterMembersCount {
		logger.Fatalf("-promscrape.cluster.memberNum must be in the range [0..%d] according to -promscrape.cluster.membersCount=%d",
			*clusterMembersCount, *clusterMembersCount)
	}
	clusterMemberID = n
}

// Config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Config struct {
	Global            GlobalConfig    `yaml:"global,omitempty"`
	ScrapeConfigs     []*ScrapeConfig `yaml:"scrape_configs,omitempty"`
	ScrapeConfigFiles []string        `yaml:"scrape_config_files,omitempty"`

	// This is set to the directory from where the config has been loaded.
	baseDir string
}

func (cfg *Config) unmarshal(data []byte, isStrict bool) error {
	var err error
	data, err = envtemplate.ReplaceBytes(data)
	if err != nil {
		return fmt.Errorf("cannot expand environment variables: %w", err)
	}
	if isStrict {
		if err = yaml.UnmarshalStrict(data, cfg); err != nil {
			err = fmt.Errorf("%w; pass -promscrape.config.strictParse=false command-line flag for ignoring unknown fields in yaml config", err)
		}
	} else {
		err = yaml.Unmarshal(data, cfg)
	}
	return err
}

func (cfg *Config) marshal() []byte {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		logger.Panicf("BUG: cannot marshal Config: %s", err)
	}
	return data
}

func (cfg *Config) mustStart() {
	startTime := time.Now()
	logger.Infof("starting service discovery routines...")
	for _, sc := range cfg.ScrapeConfigs {
		sc.mustStart(cfg.baseDir)
	}
	jobNames := cfg.getJobNames()
	tsmGlobal.registerJobNames(jobNames)
	logger.Infof("started service discovery routines in %.3f seconds", time.Since(startTime).Seconds())
}

func (cfg *Config) mustRestart(prevCfg *Config) {
	startTime := time.Now()
	logger.Infof("restarting service discovery routines...")

	prevScrapeCfgByName := make(map[string]*ScrapeConfig, len(prevCfg.ScrapeConfigs))
	for _, scPrev := range prevCfg.ScrapeConfigs {
		prevScrapeCfgByName[scPrev.JobName] = scPrev
	}

	// Restart all the scrape jobs on Global config change.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2884
	needGlobalRestart := !areEqualGlobalConfigs(&cfg.Global, &prevCfg.Global)

	// Loop over the new jobs, start new ones and restart updated ones.
	var started, stopped, restarted int
	currentJobNames := make(map[string]struct{}, len(cfg.ScrapeConfigs))
	for i, sc := range cfg.ScrapeConfigs {
		currentJobNames[sc.JobName] = struct{}{}
		scPrev := prevScrapeCfgByName[sc.JobName]
		if scPrev == nil {
			// New scrape config has been appeared. Start it.
			sc.mustStart(cfg.baseDir)
			started++
			continue
		}
		if !needGlobalRestart && areEqualScrapeConfigs(scPrev, sc) {
			// The scrape config didn't change, so no need to restart it.
			// Use the reference to the previous job, so it could be stopped properly later.
			cfg.ScrapeConfigs[i] = scPrev
		} else {
			// The scrape config has been changed. Stop the previous scrape config and start new one.
			scPrev.mustStop()
			sc.mustStart(cfg.baseDir)
			restarted++
		}
	}
	// Stop preious jobs which weren't found in the current configuration.
	for _, scPrev := range prevCfg.ScrapeConfigs {
		if _, ok := currentJobNames[scPrev.JobName]; !ok {
			scPrev.mustStop()
			stopped++
		}
	}
	jobNames := cfg.getJobNames()
	tsmGlobal.registerJobNames(jobNames)
	logger.Infof("restarted service discovery routines in %.3f seconds, stopped=%d, started=%d, restarted=%d", time.Since(startTime).Seconds(), stopped, started, restarted)
}

func areEqualGlobalConfigs(a, b *GlobalConfig) bool {
	sa := a.marshalJSON()
	sb := b.marshalJSON()
	return string(sa) == string(sb)
}

func areEqualScrapeConfigs(a, b *ScrapeConfig) bool {
	sa := a.marshalJSON()
	sb := b.marshalJSON()
	return string(sa) == string(sb)
}

func (sc *ScrapeConfig) unmarshalJSON(data []byte) error {
	return json.Unmarshal(data, sc)
}

func (sc *ScrapeConfig) marshalJSON() []byte {
	data, err := json.Marshal(sc)
	if err != nil {
		logger.Panicf("BUG: cannot marshal ScrapeConfig: %s", err)
	}
	return data
}

func (gc *GlobalConfig) marshalJSON() []byte {
	data, err := json.Marshal(gc)
	if err != nil {
		logger.Panicf("BUG: cannot marshal GlobalConfig: %s", err)
	}
	return data
}

func (cfg *Config) mustStop() {
	startTime := time.Now()
	logger.Infof("stopping service discovery routines...")
	for _, sc := range cfg.ScrapeConfigs {
		sc.mustStop()
	}
	logger.Infof("stopped service discovery routines in %.3f seconds", time.Since(startTime).Seconds())
}

// getJobNames returns all the scrape job names from the cfg.
func (cfg *Config) getJobNames() []string {
	a := make([]string, 0, len(cfg.ScrapeConfigs))
	for _, sc := range cfg.ScrapeConfigs {
		a = append(a, sc.JobName)
	}
	return a
}

// GlobalConfig represents essential parts for `global` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type GlobalConfig struct {
	ScrapeInterval *promutils.Duration `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  *promutils.Duration `yaml:"scrape_timeout,omitempty"`
	ExternalLabels *promutils.Labels   `yaml:"external_labels,omitempty"`
}

// ScrapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type ScrapeConfig struct {
	JobName        string              `yaml:"job_name"`
	ScrapeInterval *promutils.Duration `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  *promutils.Duration `yaml:"scrape_timeout,omitempty"`
	MetricsPath    string              `yaml:"metrics_path,omitempty"`
	HonorLabels    bool                `yaml:"honor_labels,omitempty"`

	// HonorTimestamps is set to false by default contrary to Prometheus, which sets it to true by default,
	// because of the issue with gaps on graphs when scraping cadvisor or similar targets, which export invalid timestamps.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697#issuecomment-1654614799 for details.
	HonorTimestamps bool `yaml:"honor_timestamps,omitempty"`

	Scheme               string                      `yaml:"scheme,omitempty"`
	Params               map[string][]string         `yaml:"params,omitempty"`
	HTTPClientConfig     promauth.HTTPClientConfig   `yaml:",inline"`
	ProxyURL             *proxy.URL                  `yaml:"proxy_url,omitempty"`
	RelabelConfigs       []promrelabel.RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []promrelabel.RelabelConfig `yaml:"metric_relabel_configs,omitempty"`
	SampleLimit          int                         `yaml:"sample_limit,omitempty"`

	AzureSDConfigs        []azure.SDConfig        `yaml:"azure_sd_configs,omitempty"`
	ConsulSDConfigs       []consul.SDConfig       `yaml:"consul_sd_configs,omitempty"`
	ConsulAgentSDConfigs  []consulagent.SDConfig  `yaml:"consulagent_sd_configs,omitempty"`
	DigitaloceanSDConfigs []digitalocean.SDConfig `yaml:"digitalocean_sd_configs,omitempty"`
	DNSSDConfigs          []dns.SDConfig          `yaml:"dns_sd_configs,omitempty"`
	DockerSDConfigs       []docker.SDConfig       `yaml:"docker_sd_configs,omitempty"`
	DockerSwarmSDConfigs  []dockerswarm.SDConfig  `yaml:"dockerswarm_sd_configs,omitempty"`
	EC2SDConfigs          []ec2.SDConfig          `yaml:"ec2_sd_configs,omitempty"`
	EurekaSDConfigs       []eureka.SDConfig       `yaml:"eureka_sd_configs,omitempty"`
	FileSDConfigs         []FileSDConfig          `yaml:"file_sd_configs,omitempty"`
	GCESDConfigs          []gce.SDConfig          `yaml:"gce_sd_configs,omitempty"`
	HTTPSDConfigs         []http.SDConfig         `yaml:"http_sd_configs,omitempty"`
	KubernetesSDConfigs   []kubernetes.SDConfig   `yaml:"kubernetes_sd_configs,omitempty"`
	KumaSDConfigs         []kuma.SDConfig         `yaml:"kuma_sd_configs,omitempty"`
	NomadSDConfigs        []nomad.SDConfig        `yaml:"nomad_sd_configs,omitempty"`
	OpenStackSDConfigs    []openstack.SDConfig    `yaml:"openstack_sd_configs,omitempty"`
	StaticConfigs         []StaticConfig          `yaml:"static_configs,omitempty"`
	YandexCloudSDConfigs  []yandexcloud.SDConfig  `yaml:"yandexcloud_sd_configs,omitempty"`

	// These options are supported only by lib/promscrape.
	DisableCompression  bool                       `yaml:"disable_compression,omitempty"`
	DisableKeepAlive    bool                       `yaml:"disable_keepalive,omitempty"`
	StreamParse         bool                       `yaml:"stream_parse,omitempty"`
	ScrapeAlignInterval *promutils.Duration        `yaml:"scrape_align_interval,omitempty"`
	ScrapeOffset        *promutils.Duration        `yaml:"scrape_offset,omitempty"`
	SeriesLimit         int                        `yaml:"series_limit,omitempty"`
	NoStaleMarkers      *bool                      `yaml:"no_stale_markers,omitempty"`
	ProxyClientConfig   promauth.ProxyClientConfig `yaml:",inline"`

	// This is set in loadConfig
	swc *scrapeWorkConfig
}

func (sc *ScrapeConfig) mustStart(baseDir string) {
	swosFunc := func(metaLabels *promutils.Labels) interface{} {
		target := metaLabels.Get("__address__")
		sw, err := sc.swc.getScrapeWork(target, nil, metaLabels)
		if err != nil {
			logger.Errorf("cannot create kubernetes_sd_config target %q for job_name %q: %s", target, sc.swc.jobName, err)
			return nil
		}
		return sw
	}
	for i := range sc.KubernetesSDConfigs {
		sc.KubernetesSDConfigs[i].MustStart(baseDir, swosFunc)
	}
}

func (sc *ScrapeConfig) mustStop() {
	for i := range sc.AzureSDConfigs {
		sc.AzureSDConfigs[i].MustStop()
	}
	for i := range sc.ConsulSDConfigs {
		sc.ConsulSDConfigs[i].MustStop()
	}
	for i := range sc.ConsulAgentSDConfigs {
		sc.ConsulAgentSDConfigs[i].MustStop()
	}
	for i := range sc.DigitaloceanSDConfigs {
		sc.DigitaloceanSDConfigs[i].MustStop()
	}
	for i := range sc.DNSSDConfigs {
		sc.DNSSDConfigs[i].MustStop()
	}
	for i := range sc.DockerSDConfigs {
		sc.DockerSDConfigs[i].MustStop()
	}
	for i := range sc.DockerSwarmSDConfigs {
		sc.DockerSwarmSDConfigs[i].MustStop()
	}
	for i := range sc.EC2SDConfigs {
		sc.EC2SDConfigs[i].MustStop()
	}
	for i := range sc.EurekaSDConfigs {
		sc.EurekaSDConfigs[i].MustStop()
	}
	for i := range sc.GCESDConfigs {
		sc.GCESDConfigs[i].MustStop()
	}
	for i := range sc.HTTPSDConfigs {
		sc.HTTPSDConfigs[i].MustStop()
	}
	for i := range sc.KubernetesSDConfigs {
		sc.KubernetesSDConfigs[i].MustStop()
	}
	for i := range sc.KumaSDConfigs {
		sc.KumaSDConfigs[i].MustStop()
	}
	for i := range sc.NomadSDConfigs {
		sc.NomadSDConfigs[i].MustStop()
	}
	for i := range sc.OpenStackSDConfigs {
		sc.OpenStackSDConfigs[i].MustStop()
	}
}

// FileSDConfig represents file-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config
type FileSDConfig struct {
	Files []string `yaml:"files"`
	// `refresh_interval` is ignored. See `-promscrape.fileSDCheckInterval`
}

// StaticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type StaticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  *promutils.Labels `yaml:"labels,omitempty"`
}

func loadStaticConfigs(path string) ([]StaticConfig, error) {
	data, err := fs.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `static_configs` from %q: %w", path, err)
	}
	data, err = envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment vars in %q: %w", path, err)
	}
	var stcs []StaticConfig
	if err := yaml.UnmarshalStrict(data, &stcs); err != nil {
		return nil, fmt.Errorf("cannot unmarshal `static_configs` from %q: %w", path, err)
	}
	return stcs, nil
}

// loadConfig loads Prometheus config from the given path.
func loadConfig(path string) (*Config, []byte, error) {
	data, err := fs.ReadFileOrHTTP(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read Prometheus config from %q: %w", path, err)
	}
	var c Config
	dataNew, err := c.parseData(data, path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot parse Prometheus config from %q: %w", path, err)
	}
	return &c, dataNew, nil
}

func loadScrapeConfigFiles(baseDir string, scrapeConfigFiles []string) ([]*ScrapeConfig, []byte, error) {
	var scrapeConfigs []*ScrapeConfig
	var scsData []byte
	for _, filePath := range scrapeConfigFiles {
		filePath := fs.GetFilepath(baseDir, filePath)
		paths := []string{filePath}
		if strings.Contains(filePath, "*") {
			ps, err := filepath.Glob(filePath)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid pattern %q: %w", filePath, err)
			}
			sort.Strings(ps)
			paths = ps
		}
		for _, path := range paths {
			data, err := fs.ReadFileOrHTTP(path)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot load %q: %w", path, err)
			}
			data, err = envtemplate.ReplaceBytes(data)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot expand environment vars in %q: %w", path, err)
			}
			var scs []*ScrapeConfig
			if err = yaml.UnmarshalStrict(data, &scs); err != nil {
				return nil, nil, fmt.Errorf("cannot parse %q: %w", path, err)
			}
			scrapeConfigs = append(scrapeConfigs, scs...)
			scsData = append(scsData, '\n')
			scsData = append(scsData, data...)
		}
	}
	return scrapeConfigs, scsData, nil
}

// IsDryRun returns true if -promscrape.config.dryRun command-line flag is set
func IsDryRun() bool {
	return *dryRun
}

func (cfg *Config) parseData(data []byte, path string) ([]byte, error) {
	if err := cfg.unmarshal(data, *strictParse); err != nil {
		return nil, fmt.Errorf("cannot unmarshal data: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain abs path for %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)

	// Load cfg.ScrapeConfigFiles into c.ScrapeConfigs
	scs, scsData, err := loadScrapeConfigFiles(cfg.baseDir, cfg.ScrapeConfigFiles)
	if err != nil {
		return nil, fmt.Errorf("cannot load `scrape_config_files` from %q: %w", path, err)
	}
	cfg.ScrapeConfigFiles = nil
	cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scs...)
	dataNew := append(data, scsData...)

	// Check that all the scrape configs have unique JobName
	m := make(map[string]struct{}, len(cfg.ScrapeConfigs))
	for _, sc := range cfg.ScrapeConfigs {
		jobName := sc.JobName
		if _, ok := m[jobName]; ok {
			return nil, fmt.Errorf("duplicate `job_name` in `scrape_configs` loaded from %q: %q", path, jobName)
		}
		m[jobName] = struct{}{}
	}

	// Initialize cfg.ScrapeConfigs
	// drop jobs with invalid config with error log
	var validScrapeConfigs []*ScrapeConfig
	for i, sc := range cfg.ScrapeConfigs {
		// Make a copy of sc in order to remove references to `data` memory.
		// This should prevent from memory leaks on config reload.
		sc = sc.clone()
		cfg.ScrapeConfigs[i] = sc

		swc, err := getScrapeWorkConfig(sc, cfg.baseDir, &cfg.Global)
		if err != nil {
			logger.Errorf("cannot parse `scrape_config`: %w", err)
			continue
		}
		sc.swc = swc
		validScrapeConfigs = append(validScrapeConfigs, sc)
	}
	cfg.ScrapeConfigs = validScrapeConfigs
	return dataNew, nil
}

func (sc *ScrapeConfig) clone() *ScrapeConfig {
	data := sc.marshalJSON()
	var scCopy ScrapeConfig
	if err := scCopy.unmarshalJSON(data); err != nil {
		logger.Panicf("BUG: cannot unmarshal scrape config: %s", err)
	}
	return &scCopy
}

func getSWSByJob(sws []*ScrapeWork) map[string][]*ScrapeWork {
	m := make(map[string][]*ScrapeWork)
	for _, sw := range sws {
		m[sw.jobNameOriginal] = append(m[sw.jobNameOriginal], sw)
	}
	return m
}

// getAzureSDScrapeWork returns `azure_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getAzureSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.AzureSDConfigs {
			sdc := &sc.AzureSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "azure_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering azure targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getConsulSDScrapeWork returns `consul_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getConsulSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.ConsulSDConfigs {
			sdc := &sc.ConsulSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "consul_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering consul targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getConsulAgentSDScrapeWork returns `consulagent_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getConsulAgentSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.ConsulAgentSDConfigs {
			sdc := &sc.ConsulAgentSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "consulagent_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering consulagent targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getDigitalOceanDScrapeWork returns `digitalocean_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDigitalOceanDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.DigitaloceanSDConfigs {
			sdc := &sc.DigitaloceanSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "digitalocean_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering digitalocean targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getDNSSDScrapeWork returns `dns_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDNSSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.DNSSDConfigs {
			sdc := &sc.DNSSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "dns_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering dns targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getDockerSDScrapeWork returns `docker_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDockerSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.DockerSDConfigs {
			sdc := &sc.DockerSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "docker_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering docker targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getDockerSwarmSDScrapeWork returns `dockerswarm_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDockerSwarmSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.DockerSwarmSDConfigs {
			sdc := &sc.DockerSwarmSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "dockerswarm_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering dockerswarm targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getEC2SDScrapeWork returns `ec2_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getEC2SDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.EC2SDConfigs {
			sdc := &sc.EC2SDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "ec2_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering ec2 targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getEurekaSDScrapeWork returns `eureka_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getEurekaSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.EurekaSDConfigs {
			sdc := &sc.EurekaSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "eureka_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering eureka targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getFileSDScrapeWork returns `file_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getFileSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		for j := range sc.FileSDConfigs {
			sdc := &sc.FileSDConfigs[j]
			dst = sdc.appendScrapeWork(dst, cfg.baseDir, sc.swc)
		}
	}
	return dst
}

// getGCESDScrapeWork returns `gce_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getGCESDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.GCESDConfigs {
			sdc := &sc.GCESDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "gce_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering gce targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getHTTPDScrapeWork returns `http_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getHTTPDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.HTTPSDConfigs {
			sdc := &sc.HTTPSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "http_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering http targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getKubernetesSDScrapeWork returns `kubernetes_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getKubernetesSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.KubernetesSDConfigs {
			sdc := &sc.KubernetesSDConfigs[j]
			swos, err := sdc.GetScrapeWorkObjects()
			if err != nil {
				logger.Errorf("skipping kubernetes_sd_config targets for job_name %q because of error: %s", sc.swc.jobName, err)
				ok = false
				break
			}
			for _, swo := range swos {
				sw := swo.(*ScrapeWork)
				dst = append(dst, sw)
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering kubernetes_sd_config targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getKumaSDScrapeWork returns `kuma_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getKumaSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.KumaSDConfigs {
			sdc := &sc.KumaSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "kuma_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering kuma targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getNomadSDScrapeWork returns `nomad_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getNomadSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.NomadSDConfigs {
			sdc := &sc.NomadSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "nomad_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering nomad_sd_config targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getOpenStackSDScrapeWork returns `openstack_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getOpenStackSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.OpenStackSDConfigs {
			sdc := &sc.OpenStackSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "openstack_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering openstack targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getYandexCloudSDScrapeWork returns `yandexcloud_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getYandexCloudSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.YandexCloudSDConfigs {
			sdc := &sc.YandexCloudSDConfigs[j]
			var okLocal bool
			dst, okLocal = appendSDScrapeWork(dst, sdc, cfg.baseDir, sc.swc, "yandexcloud_sd_config")
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering yandexcloud targets for job %q, so preserving the previous targets", sc.swc.jobName)
			dst = append(dst[:dstLen], swsPrev...)
		}
	}
	return dst
}

// getStaticScrapeWork returns `static_configs` ScrapeWork from cfg.
func (cfg *Config) getStaticScrapeWork() []*ScrapeWork {
	var dst []*ScrapeWork
	for _, sc := range cfg.ScrapeConfigs {
		for j := range sc.StaticConfigs {
			stc := &sc.StaticConfigs[j]
			dst = stc.appendScrapeWork(dst, sc.swc, nil)
		}
	}
	return dst
}

func getScrapeWorkConfig(sc *ScrapeConfig, baseDir string, globalCfg *GlobalConfig) (*scrapeWorkConfig, error) {
	jobName := sc.JobName
	if jobName == "" {
		return nil, fmt.Errorf("missing `job_name` field in `scrape_config`")
	}
	scrapeInterval := sc.ScrapeInterval.Duration()
	if scrapeInterval <= 0 {
		scrapeInterval = globalCfg.ScrapeInterval.Duration()
		if scrapeInterval <= 0 {
			scrapeInterval = defaultScrapeInterval
		}
	}
	scrapeTimeout := sc.ScrapeTimeout.Duration()
	if scrapeTimeout <= 0 {
		scrapeTimeout = globalCfg.ScrapeTimeout.Duration()
		if scrapeTimeout <= 0 {
			scrapeTimeout = defaultScrapeTimeout
		}
	}
	if scrapeTimeout > scrapeInterval {
		// Limit the `scrape_timeout` with `scrape_interval` like Prometheus does.
		// This guarantees that the scraper can miss only a single scrape if the target sometimes responds slowly.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1281#issuecomment-840538907
		scrapeTimeout = scrapeInterval
	}
	honorLabels := sc.HonorLabels
	honorTimestamps := sc.HonorTimestamps
	denyRedirects := false
	if sc.HTTPClientConfig.FollowRedirects != nil {
		denyRedirects = !*sc.HTTPClientConfig.FollowRedirects
	}
	metricsPath := sc.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	scheme := strings.ToLower(sc.Scheme)
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("unexpected `scheme` for `job_name` %q: %q; supported values: http or https", jobName, scheme)
	}
	params := sc.Params
	ac, err := sc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config for `job_name` %q: %w", jobName, err)
	}
	proxyAC, err := sc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config for `job_name` %q: %w", jobName, err)
	}
	relabelConfigs, err := promrelabel.ParseRelabelConfigs(sc.RelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	metricRelabelConfigs, err := promrelabel.ParseRelabelConfigs(sc.MetricRelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `metric_relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	externalLabels := globalCfg.ExternalLabels
	noStaleTracking := *noStaleMarkers
	if sc.NoStaleMarkers != nil {
		noStaleTracking = *sc.NoStaleMarkers
	}
	seriesLimit := *seriesLimitPerTarget
	if sc.SeriesLimit > 0 {
		seriesLimit = sc.SeriesLimit
	}
	swc := &scrapeWorkConfig{
		scrapeInterval:       scrapeInterval,
		scrapeIntervalString: scrapeInterval.String(),
		scrapeTimeout:        scrapeTimeout,
		scrapeTimeoutString:  scrapeTimeout.String(),
		jobName:              jobName,
		metricsPath:          metricsPath,
		scheme:               scheme,
		params:               params,
		proxyURL:             sc.ProxyURL,
		proxyAuthConfig:      proxyAC,
		authConfig:           ac,
		honorLabels:          honorLabels,
		honorTimestamps:      honorTimestamps,
		denyRedirects:        denyRedirects,
		externalLabels:       externalLabels,
		relabelConfigs:       relabelConfigs,
		metricRelabelConfigs: metricRelabelConfigs,
		sampleLimit:          sc.SampleLimit,
		disableCompression:   sc.DisableCompression,
		disableKeepAlive:     sc.DisableKeepAlive,
		streamParse:          sc.StreamParse,
		scrapeAlignInterval:  sc.ScrapeAlignInterval.Duration(),
		scrapeOffset:         sc.ScrapeOffset.Duration(),
		seriesLimit:          seriesLimit,
		noStaleMarkers:       noStaleTracking,
	}
	return swc, nil
}

type scrapeWorkConfig struct {
	scrapeInterval       time.Duration
	scrapeIntervalString string
	scrapeTimeout        time.Duration
	scrapeTimeoutString  string
	jobName              string
	metricsPath          string
	scheme               string
	params               map[string][]string
	proxyURL             *proxy.URL
	proxyAuthConfig      *promauth.Config
	authConfig           *promauth.Config
	honorLabels          bool
	honorTimestamps      bool
	denyRedirects        bool
	externalLabels       *promutils.Labels
	relabelConfigs       *promrelabel.ParsedConfigs
	metricRelabelConfigs *promrelabel.ParsedConfigs
	sampleLimit          int
	disableCompression   bool
	disableKeepAlive     bool
	streamParse          bool
	scrapeAlignInterval  time.Duration
	scrapeOffset         time.Duration
	seriesLimit          int
	noStaleMarkers       bool
}

type targetLabelsGetter interface {
	GetLabels(baseDir string) ([]*promutils.Labels, error)
}

func appendSDScrapeWork(dst []*ScrapeWork, sdc targetLabelsGetter, baseDir string, swc *scrapeWorkConfig, discoveryType string) ([]*ScrapeWork, bool) {
	targetLabels, err := sdc.GetLabels(baseDir)
	if err != nil {
		logger.Errorf("skipping %s targets for job_name %q because of error: %s", discoveryType, swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, discoveryType), true
}

func appendScrapeWorkForTargetLabels(dst []*ScrapeWork, swc *scrapeWorkConfig, targetLabels []*promutils.Labels, discoveryType string) []*ScrapeWork {
	startTime := time.Now()
	// Process targetLabels in parallel in order to reduce processing time for big number of targetLabels.
	type result struct {
		sw  *ScrapeWork
		err error
	}
	goroutines := cgroup.AvailableCPUs()
	resultCh := make(chan result, len(targetLabels))
	workCh := make(chan *promutils.Labels, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			for metaLabels := range workCh {
				target := metaLabels.Get("__address__")
				sw, err := swc.getScrapeWork(target, nil, metaLabels)
				if err != nil {
					err = fmt.Errorf("skipping %s target %q for job_name %q because of error: %w", discoveryType, target, swc.jobName, err)
				}
				resultCh <- result{
					sw:  sw,
					err: err,
				}
			}
		}()
	}
	for _, metaLabels := range targetLabels {
		workCh <- metaLabels
	}
	close(workCh)
	for range targetLabels {
		r := <-resultCh
		if r.err != nil {
			logger.Errorf("%s", r.err)
			continue
		}
		if r.sw != nil {
			dst = append(dst, r.sw)
		}
	}
	metrics.GetOrCreateHistogram(fmt.Sprintf("vm_promscrape_target_relabel_duration_seconds{type=%q}", discoveryType)).UpdateDuration(startTime)
	return dst
}

func (sdc *FileSDConfig) appendScrapeWork(dst []*ScrapeWork, baseDir string, swc *scrapeWorkConfig) []*ScrapeWork {
	metaLabels := promutils.GetLabels()
	defer promutils.PutLabels(metaLabels)
	for _, file := range sdc.Files {
		pathPattern := fs.GetFilepath(baseDir, file)
		paths := []string{pathPattern}
		if strings.Contains(pathPattern, "*") {
			var err error
			paths, err = filepath.Glob(pathPattern)
			if err != nil {
				// Do not return this error, since other files may contain valid scrape configs.
				logger.Errorf("invalid pattern %q in `file_sd_config->files` section of job_name=%q: %s; skipping it", file, swc.jobName, err)
				continue
			}
		}
		for _, path := range paths {
			stcs, err := loadStaticConfigs(path)
			if err != nil {
				// Do not return this error, since other paths may contain valid scrape configs.
				logger.Errorf("cannot load file %q for job_name=%q at `file_sd_configs`: %s; skipping this file", path, swc.jobName, err)
				continue
			}
			pathShort := path
			if strings.HasPrefix(pathShort, baseDir) {
				pathShort = path[len(baseDir):]
				if len(pathShort) > 0 && pathShort[0] == filepath.Separator {
					pathShort = pathShort[1:]
				}
			}
			metaLabels.Reset()
			metaLabels.Add("__meta_filepath", pathShort)
			for i := range stcs {
				dst = stcs[i].appendScrapeWork(dst, swc, metaLabels)
			}
		}
	}
	return dst
}

func (stc *StaticConfig) appendScrapeWork(dst []*ScrapeWork, swc *scrapeWorkConfig, metaLabels *promutils.Labels) []*ScrapeWork {
	for _, target := range stc.Targets {
		if target == "" {
			// Do not return this error, since other targets may be valid
			logger.Errorf("`static_configs` target for `job_name` %q cannot be empty; skipping it", swc.jobName)
			continue
		}
		sw, err := swc.getScrapeWork(target, stc.Labels, metaLabels)
		if err != nil {
			// Do not return this error, since other targets may be valid
			logger.Errorf("error when parsing `static_configs` target %q for `job_name` %q: %s; skipping it", target, swc.jobName, err)
			continue
		}
		if sw != nil {
			dst = append(dst, sw)
		}
	}
	return dst
}

func appendScrapeWorkKey(dst []byte, labels *promutils.Labels) []byte {
	for _, label := range labels.GetLabels() {
		// Do not use strconv.AppendQuote, since it is slow according to CPU profile.
		dst = append(dst, label.Name...)
		dst = append(dst, '=')
		dst = append(dst, label.Value...)
		dst = append(dst, ',')
	}
	return dst
}

func needSkipScrapeWork(key string, membersCount, replicasCount, memberNum int) bool {
	if membersCount <= 1 {
		return false
	}
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(key))
	idx := int(h % uint64(membersCount))
	if replicasCount < 1 {
		replicasCount = 1
	}
	for i := 0; i < replicasCount; i++ {
		if idx == memberNum {
			return false
		}
		idx++
		if idx >= membersCount {
			idx = 0
		}
	}
	return true
}

var scrapeWorkKeyBufPool bytesutil.ByteBufferPool

func (swc *scrapeWorkConfig) getScrapeWork(target string, extraLabels, metaLabels *promutils.Labels) (*ScrapeWork, error) {
	labels := promutils.GetLabels()
	defer promutils.PutLabels(labels)

	mergeLabels(labels, swc, target, extraLabels, metaLabels)
	var originalLabels *promutils.Labels
	if !*dropOriginalLabels {
		originalLabels = labels.Clone()
	}
	labels.Labels = swc.relabelConfigs.Apply(labels.Labels, 0)
	// Remove labels starting from "__meta_" prefix according to https://www.robustperception.io/life-of-a-label/
	labels.RemoveMetaLabels()

	// Verify whether the scrape work must be skipped because of `-promscrape.cluster.*` configs.
	// Perform the verification on labels after the relabeling in order to guarantee that targets with the same set of labels
	// go to the same vmagent shard.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1687#issuecomment-940629495
	if *clusterMembersCount > 1 {
		bb := scrapeWorkKeyBufPool.Get()
		bb.B = appendScrapeWorkKey(bb.B[:0], labels)
		needSkip := needSkipScrapeWork(bytesutil.ToUnsafeString(bb.B), *clusterMembersCount, *clusterReplicationFactor, clusterMemberID)
		scrapeWorkKeyBufPool.Put(bb)
		if needSkip {
			return nil, nil
		}
	}
	if !*dropOriginalLabels {
		originalLabels.Sort()
		// Reduce memory usage by interning all the strings in originalLabels.
		originalLabels.InternStrings()
	}
	if labels.Len() == 0 {
		// Drop target without labels.
		droppedTargetsMap.Register(originalLabels, swc.relabelConfigs)
		return nil, nil
	}
	scrapeURL, address := promrelabel.GetScrapeURL(labels, swc.params)
	if scrapeURL == "" {
		// Drop target without URL.
		droppedTargetsMap.Register(originalLabels, swc.relabelConfigs)
		return nil, nil
	}
	if _, err := url.Parse(scrapeURL); err != nil {
		return nil, fmt.Errorf("invalid target url=%q for job=%q: %w", scrapeURL, swc.jobName, err)
	}

	var at *auth.Token
	tenantID := labels.Get("__tenant_id__")
	if len(tenantID) > 0 {
		newToken, err := auth.NewToken(tenantID)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __tenant_id__=%q for job=%q: %w", tenantID, swc.jobName, err)
		}
		at = newToken
	}

	// Read __scrape_interval__ and __scrape_timeout__ from labels.
	scrapeInterval := swc.scrapeInterval
	if s := labels.Get("__scrape_interval__"); len(s) > 0 {
		d, err := promutils.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __scrape_interval__=%q: %w", s, err)
		}
		scrapeInterval = d
	}
	scrapeTimeout := swc.scrapeTimeout
	if s := labels.Get("__scrape_timeout__"); len(s) > 0 {
		d, err := promutils.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __scrape_timeout__=%q: %w", s, err)
		}
		scrapeTimeout = d
	}
	// Read series_limit option from __series_limit__ label.
	// See https://docs.victoriametrics.com/vmagent.html#cardinality-limiter
	seriesLimit := swc.seriesLimit
	if s := labels.Get("__series_limit__"); len(s) > 0 {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __series_limit__=%q: %w", s, err)
		}
		seriesLimit = n
	}
	// Read stream_parse option from __stream_parse__ label.
	// See https://docs.victoriametrics.com/vmagent.html#stream-parsing-mode
	streamParse := swc.streamParse
	if s := labels.Get("__stream_parse__"); len(s) > 0 {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __stream_parse__=%q: %w", s, err)
		}
		streamParse = b
	}
	// Remove labels with "__" prefix according to https://www.robustperception.io/life-of-a-label/
	labels.RemoveLabelsWithDoubleUnderscorePrefix()
	// Add missing "instance" label according to https://www.robustperception.io/life-of-a-label
	if labels.Get("instance") == "" {
		labels.Add("instance", address)
	}
	if *clusterMemberLabel != "" && *clusterMemberNum != "" {
		labels.Add(*clusterMemberLabel, *clusterMemberNum)
	}
	// Remove references to deleted labels, so GC could clean strings for label name and label value past len(labels.Labels).
	// This should reduce memory usage when relabeling creates big number of temporary labels with long names and/or values.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 for details.
	labelsCopy := labels.Clone()
	// Sort labels in alphabetical order of their names.
	labelsCopy.Sort()
	// Reduce memory usage by interning all the strings in labels.
	labelsCopy.InternStrings()

	sw := &ScrapeWork{
		ScrapeURL:            scrapeURL,
		ScrapeInterval:       scrapeInterval,
		ScrapeTimeout:        scrapeTimeout,
		HonorLabels:          swc.honorLabels,
		HonorTimestamps:      swc.honorTimestamps,
		DenyRedirects:        swc.denyRedirects,
		OriginalLabels:       originalLabels,
		Labels:               labelsCopy,
		ExternalLabels:       swc.externalLabels,
		ProxyURL:             swc.proxyURL,
		ProxyAuthConfig:      swc.proxyAuthConfig,
		AuthConfig:           swc.authConfig,
		RelabelConfigs:       swc.relabelConfigs,
		MetricRelabelConfigs: swc.metricRelabelConfigs,
		SampleLimit:          swc.sampleLimit,
		DisableCompression:   swc.disableCompression,
		DisableKeepAlive:     swc.disableKeepAlive,
		StreamParse:          streamParse,
		ScrapeAlignInterval:  swc.scrapeAlignInterval,
		ScrapeOffset:         swc.scrapeOffset,
		SeriesLimit:          seriesLimit,
		NoStaleMarkers:       swc.noStaleMarkers,
		AuthToken:            at,

		jobNameOriginal: swc.jobName,
	}
	return sw, nil
}

func mergeLabels(dst *promutils.Labels, swc *scrapeWorkConfig, target string, extraLabels, metaLabels *promutils.Labels) {
	if n := dst.Len(); n > 0 {
		logger.Panicf("BUG: len(dst.Labels) must be 0; got %d", n)
	}
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	dst.Add("job", swc.jobName)
	dst.Add("__address__", target)
	dst.Add("__scheme__", swc.scheme)
	dst.Add("__metrics_path__", swc.metricsPath)
	dst.Add("__scrape_interval__", swc.scrapeIntervalString)
	dst.Add("__scrape_timeout__", swc.scrapeTimeoutString)
	for k, args := range swc.params {
		if len(args) == 0 {
			continue
		}
		k = "__param_" + k
		v := args[0]
		dst.Add(k, v)
	}
	dst.AddFrom(extraLabels)
	dst.AddFrom(metaLabels)
	dst.RemoveDuplicates()
}

const (
	defaultScrapeInterval = time.Minute
	defaultScrapeTimeout  = 10 * time.Second
)
