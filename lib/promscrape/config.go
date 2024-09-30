package promscrape

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/metrics"
	"github.com/cespare/xxhash/v2"
	"gopkg.in/yaml.v2"
)

var (
	noStaleMarkers       = flag.Bool("promscrape.noStaleMarkers", false, "Whether to disable sending Prometheus stale markers for metrics when scrape target disappears. This option may reduce memory usage if stale markers aren't needed for your setup. This option also disables populating the scrape_series_added metric. See https://prometheus.io/docs/concepts/jobs_instances/#automatically-generated-labels-and-time-series")
	seriesLimitPerTarget = flag.Int("promscrape.seriesLimitPerTarget", 0, "Optional limit on the number of unique time series a single scrape target can expose. See https://docs.victoriametrics.com/vmagent/#cardinality-limiter for more info")
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
		"Each member then scrapes roughly 1/N of all the targets. By default, cluster scraping is disabled, i.e. a single scraper scrapes all the targets. "+
		"See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more info")
	clusterMemberNum = flag.String("promscrape.cluster.memberNum", "0", "The number of vmagent instance in the cluster of scrapers. "+
		"It must be a unique value in the range 0 ... promscrape.cluster.membersCount-1 across scrapers in the cluster. "+
		"Can be specified as pod name of Kubernetes StatefulSet - pod-name-Num, where Num is a numeric part of pod name. "+
		"See also -promscrape.cluster.memberLabel . See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more info")
	clusterMemberLabel = flag.String("promscrape.cluster.memberLabel", "", "If non-empty, then the label with this name and the -promscrape.cluster.memberNum value "+
		"is added to all the scraped metrics. See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more info")
	clusterMemberURLTemplate = flag.String("promscrape.cluster.memberURLTemplate", "", "An optional template for URL to access vmagent instance with the given -promscrape.cluster.memberNum value. "+
		"Every %d occurrence in the template is substituted with -promscrape.cluster.memberNum at urls to vmagent instances responsible for scraping the given target "+
		"at /service-discovery page. For example -promscrape.cluster.memberURLTemplate='http://vmagent-%d:8429/targets'. "+
		"See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more details")
	clusterReplicationFactor = flag.Int("promscrape.cluster.replicationFactor", 1, "The number of members in the cluster, which scrape the same targets. "+
		"If the replication factor is greater than 1, then the deduplication must be enabled at remote storage side. "+
		"See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more info")
	clusterName = flag.String("promscrape.cluster.name", "", "Optional name of the cluster. If multiple vmagent clusters scrape the same targets, "+
		"then each cluster must have unique name in order to properly de-duplicate samples received from these clusters. "+
		"See https://docs.victoriametrics.com/vmagent/#scraping-big-number-of-targets for more info")
	maxScrapeSize = flagutil.NewBytes("promscrape.maxScrapeSize", 16*1024*1024, "The maximum size of scrape response in bytes to process from Prometheus targets. "+
		"Bigger responses are rejected. See also max_scrape_size option at https://docs.victoriametrics.com/sd_configs/#scrape_configs")
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
	logger.Infof("started %d service discovery routines in %.3f seconds", len(cfg.ScrapeConfigs), time.Since(startTime).Seconds())
}

// mustRestart restarts service discovery routines at cfg if they were changed comparing to prevCfg.
//
// It returns true if at least a single scraper has been restarted.
func (cfg *Config) mustRestart(prevCfg *Config) bool {
	startTime := time.Now()

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
	// Stop previous jobs which weren't found in the current configuration.
	for _, scPrev := range prevCfg.ScrapeConfigs {
		if _, ok := currentJobNames[scPrev.JobName]; !ok {
			scPrev.mustStop()
			stopped++
		}
	}
	jobNames := cfg.getJobNames()
	tsmGlobal.registerJobNames(jobNames)
	updated := started + stopped + restarted
	if updated == 0 {
		return false
	}
	logger.Infof("updated %d service discovery routines in %.3f seconds, started=%d, stopped=%d, restarted=%d",
		updated, time.Since(startTime).Seconds(), started, stopped, restarted)
	return true
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
	logger.Infof("stopped %d service discovery routines in %.3f seconds", len(cfg.ScrapeConfigs), time.Since(startTime).Seconds())
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
	MaxScrapeSize  string              `yaml:"max_scrape_size,omitempty"`
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

	// This silly option is needed for compatibility with Prometheus.
	// vmagent was supporting disable_compression option since the beginning, while Prometheus developers
	// decided adding enable_compression option in https://github.com/prometheus/prometheus/pull/13166
	// That's why it needs to be supported too :(
	EnableCompression *bool `yaml:"enable_compression,omitempty"`

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
	HetznerSDConfigs      []hetzner.SDConfig      `yaml:"hetzner_sd_configs,omitempty"`
	HTTPSDConfigs         []http.SDConfig         `yaml:"http_sd_configs,omitempty"`
	KubernetesSDConfigs   []kubernetes.SDConfig   `yaml:"kubernetes_sd_configs,omitempty"`
	KumaSDConfigs         []kuma.SDConfig         `yaml:"kuma_sd_configs,omitempty"`
	NomadSDConfigs        []nomad.SDConfig        `yaml:"nomad_sd_configs,omitempty"`
	OpenStackSDConfigs    []openstack.SDConfig    `yaml:"openstack_sd_configs,omitempty"`
	OVHCloudSDConfigs     []ovhcloud.SDConfig     `yaml:"ovhcloud_sd_configs,omitempty"`
	StaticConfigs         []StaticConfig          `yaml:"static_configs,omitempty"`
	VultrSDConfigs        []vultr.SDConfig        `yaml:"vultr_configs,omitempty"`
	YandexCloudSDConfigs  []yandexcloud.SDConfig  `yaml:"yandexcloud_sd_configs,omitempty"`

	// These options are supported only by lib/promscrape.
	DisableCompression  bool                       `yaml:"disable_compression,omitempty"`
	DisableKeepAlive    bool                       `yaml:"disable_keepalive,omitempty"`
	StreamParse         bool                       `yaml:"stream_parse,omitempty"`
	ScrapeAlignInterval *promutils.Duration        `yaml:"scrape_align_interval,omitempty"`
	ScrapeOffset        *promutils.Duration        `yaml:"scrape_offset,omitempty"`
	SeriesLimit         *int                       `yaml:"series_limit,omitempty"`
	NoStaleMarkers      *bool                      `yaml:"no_stale_markers,omitempty"`
	ProxyClientConfig   promauth.ProxyClientConfig `yaml:",inline"`

	// This is set in loadConfig
	swc *scrapeWorkConfig
}

func (sc *ScrapeConfig) mustStart(baseDir string) {
	swosFunc := func(metaLabels *promutils.Labels) any {
		target := metaLabels.Get("__address__")
		sw, err := sc.swc.getScrapeWork(target, nil, metaLabels)
		if err != nil {
			logger.Errorf("cannot create kubernetes_sd_config target %q for job_name=%s: %s", target, sc.swc.jobName, err)
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
	for i := range sc.HetznerSDConfigs {
		sc.HetznerSDConfigs[i].MustStop()
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
	for i := range sc.OVHCloudSDConfigs {
		sc.OVHCloudSDConfigs[i].MustStop()
	}
	for i := range sc.VultrSDConfigs {
		sc.VultrSDConfigs[i].MustStop()
	}
	for i := range sc.YandexCloudSDConfigs {
		sc.YandexCloudSDConfigs[i].MustStop()
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
	data, err := fscore.ReadFileOrHTTP(path)
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
func loadConfig(path string) (*Config, error) {
	data, err := fscore.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read Prometheus config from %q: %w", path, err)
	}
	var c Config
	if err := c.parseData(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse Prometheus config from %q: %w", path, err)
	}
	return &c, nil
}

func loadScrapeConfigFiles(baseDir string, scrapeConfigFiles []string, isStrict bool) ([]*ScrapeConfig, error) {
	var scrapeConfigs []*ScrapeConfig
	for _, filePath := range scrapeConfigFiles {
		filePath := fscore.GetFilepath(baseDir, filePath)
		paths := []string{filePath}
		if strings.Contains(filePath, "*") {
			ps, err := filepath.Glob(filePath)
			if err != nil {
				logger.Errorf("skipping pattern %q at `scrape_config_files` because of error: %s", filePath, err)
				continue
			}
			sort.Strings(ps)
			paths = ps
		}
		for _, path := range paths {
			data, err := fscore.ReadFileOrHTTP(path)
			if err != nil {
				logger.Errorf("skipping %q at `scrape_config_files` because of error: %s", path, err)
				continue
			}
			data, err = envtemplate.ReplaceBytes(data)
			if err != nil {
				logger.Errorf("skipping %q at `scrape_config_files` because of failure to expand environment vars: %s", path, err)
				continue
			}
			var scs []*ScrapeConfig
			if isStrict {
				if err = yaml.UnmarshalStrict(data, &scs); err != nil {
					return nil, fmt.Errorf("cannot unmarshal data from `scrape_config_files` %s: %w; "+
						"pass -promscrape.config.strictParse=false command-line flag for ignoring invalid scrape_config_files", path, err)
				}
			} else {
				if err = yaml.Unmarshal(data, &scs); err != nil {
					logger.Errorf("skipping %q at `scrape_config_files` because of failure to parse it: %s", path, err)
					continue
				}
			}
			scrapeConfigs = append(scrapeConfigs, scs...)
		}
	}
	return scrapeConfigs, nil
}

// IsDryRun returns true if -promscrape.config.dryRun command-line flag is set
func IsDryRun() bool {
	return *dryRun
}

func (cfg *Config) parseData(data []byte, path string) error {
	if err := cfg.unmarshal(data, *strictParse); err != nil {
		cfg.ScrapeConfigs = nil
		return fmt.Errorf("cannot unmarshal data: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		cfg.ScrapeConfigs = nil
		return fmt.Errorf("cannot obtain abs path for %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)

	// Load cfg.ScrapeConfigFiles into c.ScrapeConfigs
	scs, err := loadScrapeConfigFiles(cfg.baseDir, cfg.ScrapeConfigFiles, *strictParse)
	if err != nil {
		return err
	}
	cfg.ScrapeConfigFiles = nil
	cfg.ScrapeConfigs = append(cfg.ScrapeConfigs, scs...)

	// Check that all the scrape configs have unique JobName
	m := make(map[string]struct{}, len(cfg.ScrapeConfigs))
	for _, sc := range cfg.ScrapeConfigs {
		jobName := sc.JobName
		if _, ok := m[jobName]; ok {
			cfg.ScrapeConfigs = nil
			return fmt.Errorf("duplicate `job_name` in `scrape_configs` loaded from %q: %q", path, jobName)
		}
		m[jobName] = struct{}{}
	}

	// Initialize cfg.ScrapeConfigs
	validScrapeConfigs := cfg.ScrapeConfigs[:0]
	for _, sc := range cfg.ScrapeConfigs {
		// Make a copy of sc in order to remove references to `data` memory.
		// This should prevent from memory leaks on config reload.
		sc = sc.clone()

		swc, err := getScrapeWorkConfig(sc, cfg.baseDir, &cfg.Global)
		if err != nil {
			logger.Errorf("skipping `scrape_config` for job_name=%s because of error: %s", sc.JobName, err)
			continue
		}
		sc.swc = swc
		validScrapeConfigs = append(validScrapeConfigs, sc)
	}
	tailScrapeConfigs := cfg.ScrapeConfigs[len(validScrapeConfigs):]
	cfg.ScrapeConfigs = validScrapeConfigs
	for i := range tailScrapeConfigs {
		tailScrapeConfigs[i] = nil
	}

	return nil
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
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.AzureSDConfigs {
			visitor(&sc.AzureSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "azure_sd_config", prev)
}

// getConsulSDScrapeWork returns `consul_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getConsulSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.ConsulSDConfigs {
			visitor(&sc.ConsulSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "consul_sd_config", prev)
}

// getConsulAgentSDScrapeWork returns `consulagent_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getConsulAgentSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.ConsulAgentSDConfigs {
			visitor(&sc.ConsulAgentSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "consulagent_sd_config", prev)
}

// getDigitalOceanDScrapeWork returns `digitalocean_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDigitalOceanDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.DigitaloceanSDConfigs {
			visitor(&sc.DigitaloceanSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "digitalocean_sd_config", prev)
}

// getDNSSDScrapeWork returns `dns_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDNSSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.DNSSDConfigs {
			visitor(&sc.DNSSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "dns_sd_config", prev)
}

// getDockerSDScrapeWork returns `docker_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDockerSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.DockerSDConfigs {
			visitor(&sc.DockerSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "docker_sd_config", prev)
}

// getDockerSwarmSDScrapeWork returns `dockerswarm_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDockerSwarmSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.DockerSwarmSDConfigs {
			visitor(&sc.DockerSwarmSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "dockerswarm_sd_config", prev)
}

// getEC2SDScrapeWork returns `ec2_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getEC2SDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.EC2SDConfigs {
			visitor(&sc.EC2SDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "ec2_sd_config", prev)
}

// getEurekaSDScrapeWork returns `eureka_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getEurekaSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.EurekaSDConfigs {
			visitor(&sc.EurekaSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "eureka_sd_config", prev)
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
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.GCESDConfigs {
			visitor(&sc.GCESDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "gce_sd_config", prev)
}

// getHetznerSDScrapeWork returns `hetzner_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getHetznerSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.HetznerSDConfigs {
			visitor(&sc.HetznerSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "hetzner_sd_config", prev)
}

// getHTTPDScrapeWork returns `http_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getHTTPDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.HTTPSDConfigs {
			visitor(&sc.HTTPSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "http_sd_config", prev)
}

// getKubernetesSDScrapeWork returns `kubernetes_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getKubernetesSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	const discoveryType = "kubernetes_sd_config"
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		for j := range sc.KubernetesSDConfigs {
			sdc := &sc.KubernetesSDConfigs[j]
			swos, err := sdc.GetScrapeWorkObjects()
			if err != nil {
				logger.Errorf("skipping %s targets for job_name=%s because of error: %s", discoveryType, sc.swc.jobName, err)
				ok = false
				break
			}
			for _, swo := range swos {
				sw := swo.(*ScrapeWork)
				dst = append(dst, sw)
			}
		}
		if !ok {
			dst = sc.appendPrevTargets(dst[:dstLen], swsPrevByJob, discoveryType)
		}
	}
	return dst
}

// getKumaSDScrapeWork returns `kuma_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getKumaSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.KumaSDConfigs {
			visitor(&sc.KumaSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "kuma_sd_config", prev)
}

// getNomadSDScrapeWork returns `nomad_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getNomadSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.NomadSDConfigs {
			visitor(&sc.NomadSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "nomad_sd_config", prev)
}

// getOpenStackSDScrapeWork returns `openstack_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getOpenStackSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.OpenStackSDConfigs {
			visitor(&sc.OpenStackSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "openstack_sd_config", prev)
}

// getOVHCloudSDScrapeWork returns `ovhcloud_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getOVHCloudSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.OVHCloudSDConfigs {
			visitor(&sc.OVHCloudSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "ovhcloud_sd_config", prev)
}

// getVultrSDScrapeWork returns `vultr_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getVultrSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.VultrSDConfigs {
			visitor(&sc.VultrSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "vultr_sd_config", prev)
}

// getYandexCloudSDScrapeWork returns `yandexcloud_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getYandexCloudSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	visitConfigs := func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)) {
		for i := range sc.YandexCloudSDConfigs {
			visitor(&sc.YandexCloudSDConfigs[i])
		}
	}
	return cfg.getScrapeWorkGeneric(visitConfigs, "yandexcloud_sd_config", prev)
}

type targetLabelsGetter interface {
	GetLabels(baseDir string) ([]*promutils.Labels, error)
}

func (cfg *Config) getScrapeWorkGeneric(visitConfigs func(sc *ScrapeConfig, visitor func(sdc targetLabelsGetter)), discoveryType string, prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for _, sc := range cfg.ScrapeConfigs {
		dstLen := len(dst)
		ok := true
		visitConfigs(sc, func(sdc targetLabelsGetter) {
			if !ok {
				return
			}
			targetLabels, err := sdc.GetLabels(cfg.baseDir)
			if err != nil {
				logger.Errorf("skipping %s targets for job_name=%s because of error: %s", discoveryType, sc.swc.jobName, err)
				ok = false
				return
			}
			dst = appendScrapeWorkForTargetLabels(dst, sc.swc, targetLabels, discoveryType)
		})
		if !ok {
			dst = sc.appendPrevTargets(dst[:dstLen], swsPrevByJob, discoveryType)
		}
	}
	return dst
}

func (sc *ScrapeConfig) appendPrevTargets(dst []*ScrapeWork, swsPrevByJob map[string][]*ScrapeWork, discoveryType string) []*ScrapeWork {
	swsPrev := swsPrevByJob[sc.swc.jobName]
	if len(swsPrev) == 0 {
		return dst
	}
	logger.Errorf("preserving the previous %s targets for job_name=%s because of temporary discovery error", discoveryType, sc.swc.jobName)
	return append(dst, swsPrev...)
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
	mss := maxScrapeSize.N
	if sc.MaxScrapeSize != "" {
		n, err := flagutil.ParseBytes(sc.MaxScrapeSize)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `max_scrape_size` value %q for `job_name` %q`: %w", sc.MaxScrapeSize, jobName, err)
		}
		if n > 0 {
			mss = n
		}
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
	if sc.SeriesLimit != nil {
		seriesLimit = *sc.SeriesLimit
	}
	disableCompression := sc.DisableCompression
	if sc.EnableCompression != nil {
		disableCompression = !*sc.EnableCompression
	}
	swc := &scrapeWorkConfig{
		scrapeInterval:       scrapeInterval,
		scrapeIntervalString: scrapeInterval.String(),
		scrapeTimeout:        scrapeTimeout,
		scrapeTimeoutString:  scrapeTimeout.String(),
		maxScrapeSize:        mss,
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
		disableCompression:   disableCompression,
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
	maxScrapeSize        int64
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
					err = fmt.Errorf("skipping %s target %q for job_name%s because of error: %w", discoveryType, target, swc.jobName, err)
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
		pathPattern := fscore.GetFilepath(baseDir, file)
		paths := []string{pathPattern}
		if strings.Contains(pathPattern, "*") {
			var err error
			paths, err = filepath.Glob(pathPattern)
			if err != nil {
				// Do not return this error, since other files may contain valid scrape configs.
				logger.Errorf("skipping entry %q in `file_sd_config->files` for job_name=%s because of error: %s", file, swc.jobName, err)
				continue
			}
		}
		for _, path := range paths {
			stcs, err := loadStaticConfigs(path)
			if err != nil {
				// Do not return this error, since other paths may contain valid scrape configs.
				logger.Errorf("skipping file %s for job_name=%s at `file_sd_configs` because of error: %s", path, swc.jobName, err)
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
			logger.Errorf("skipping empty `static_configs` target for job_name=%s", swc.jobName)
			continue
		}
		sw, err := swc.getScrapeWork(target, stc.Labels, metaLabels)
		if err != nil {
			// Do not return this error, since other targets may be valid
			logger.Errorf("skipping `static_configs` target %q for job_name=%s because of error: %s", target, swc.jobName, err)
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

func getClusterMemberNumsForScrapeWork(key string, membersCount, replicasCount int) []int {
	if membersCount <= 1 {
		return []int{0}
	}
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(key))
	idx := int(h % uint64(membersCount))
	if replicasCount < 1 {
		replicasCount = 1
	}
	memberNums := make([]int, replicasCount)
	for i := 0; i < replicasCount; i++ {
		memberNums[i] = idx
		idx++
		if idx >= membersCount {
			idx = 0
		}
	}
	return memberNums
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

	if labels.Len() == 0 {
		// Drop target without labels.
		originalLabels = sortOriginalLabelsIfNeeded(originalLabels)
		droppedTargetsMap.Register(originalLabels, swc.relabelConfigs, targetDropReasonRelabeling, nil)
		return nil, nil
	}

	// Verify whether the scrape work must be skipped because of `-promscrape.cluster.*` configs.
	// Perform the verification on labels after the relabeling in order to guarantee that targets with the same set of labels
	// go to the same vmagent shard.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1687#issuecomment-940629495
	if *clusterMembersCount > 1 {
		bb := scrapeWorkKeyBufPool.Get()
		bb.B = appendScrapeWorkKey(bb.B[:0], labels)
		memberNums := getClusterMemberNumsForScrapeWork(bytesutil.ToUnsafeString(bb.B), *clusterMembersCount, *clusterReplicationFactor)
		scrapeWorkKeyBufPool.Put(bb)
		if !slices.Contains(memberNums, clusterMemberID) {
			originalLabels = sortOriginalLabelsIfNeeded(originalLabels)
			droppedTargetsMap.Register(originalLabels, swc.relabelConfigs, targetDropReasonSharding, memberNums)
			return nil, nil
		}
	}
	scrapeURL, address := promrelabel.GetScrapeURL(labels, swc.params)
	if scrapeURL == "" {
		// Drop target without URL.
		originalLabels = sortOriginalLabelsIfNeeded(originalLabels)
		droppedTargetsMap.Register(originalLabels, swc.relabelConfigs, targetDropReasonMissingScrapeURL, nil)
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
	// See https://docs.victoriametrics.com/vmagent/#cardinality-limiter
	seriesLimit := swc.seriesLimit
	if s := labels.Get("__series_limit__"); len(s) > 0 {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __series_limit__=%q: %w", s, err)
		}
		seriesLimit = n
	}
	// Read sample_limit option from __sample_limit__ label.
	// See https://docs.victoriametrics.com/vmagent/#automatically-generated-metrics
	sampleLimit := swc.sampleLimit
	if s := labels.Get("__sample_limit__"); len(s) > 0 {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __sample_limit__=%q: %w", s, err)
		}
		sampleLimit = n
	}
	// Read stream_parse option from __stream_parse__ label.
	// See https://docs.victoriametrics.com/vmagent/#stream-parsing-mode
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

	originalLabels = sortOriginalLabelsIfNeeded(originalLabels)
	sw := &ScrapeWork{
		ScrapeURL:            scrapeURL,
		ScrapeInterval:       scrapeInterval,
		ScrapeTimeout:        scrapeTimeout,
		MaxScrapeSize:        swc.maxScrapeSize,
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
		SampleLimit:          sampleLimit,
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

func sortOriginalLabelsIfNeeded(originalLabels *promutils.Labels) *promutils.Labels {
	if originalLabels == nil {
		return nil
	}
	originalLabels.Sort()
	// Reduce memory usage by interning all the strings in originalLabels.
	originalLabels.InternStrings()
	return originalLabels
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
