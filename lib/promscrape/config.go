package promscrape

import (
	"flag"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/metrics"
	xxhash "github.com/cespare/xxhash/v2"
	"gopkg.in/yaml.v2"
)

var (
	strictParse = flag.Bool("promscrape.config.strictParse", false, "Whether to allow only supported fields in -promscrape.config . "+
		"By default unsupported fields are silently skipped")
	dryRun = flag.Bool("promscrape.config.dryRun", false, "Checks -promscrape.config file for errors and unsupported fields and then exits. "+
		"Returns non-zero exit code on parsing errors and emits these errors to stderr. "+
		"See also -promscrape.config.strictParse command-line flag. "+
		"Pass -loggerLevel=ERROR if you don't need to see info messages in the output.")
	dropOriginalLabels = flag.Bool("promscrape.dropOriginalLabels", false, "Whether to drop original labels for scrape targets at /targets and /api/v1/targets pages. "+
		"This may be needed for reducing memory usage when original labels for big number of scrape targets occupy big amounts of memory. "+
		"Note that this reduces debuggability for improper per-target relabeling configs")
	clusterMembersCount = flag.Int("promscrape.cluster.membersCount", 0, "The number of members in a cluster of scrapers. "+
		"Each member must have an unique -promscrape.cluster.memberNum in the range 0 ... promscrape.cluster.membersCount-1 . "+
		"Each member then scrapes roughly 1/N of all the targets. By default cluster scraping is disabled, i.e. a single scraper scrapes all the targets")
	clusterMemberNum = flag.Int("promscrape.cluster.memberNum", 0, "The number of number in the cluster of scrapers. "+
		"It must be an unique value in the range 0 ... promscrape.cluster.membersCount-1 across scrapers in the cluster")
	clusterReplicationFactor = flag.Int("promscrape.cluster.replicationFactor", 1, "The number of members in the cluster, which scrape the same targets. "+
		"If the replication factor is greater than 2, then the deduplication must be enabled at remote storage side. See https://docs.victoriametrics.com/#deduplication")
)

// Config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Config struct {
	Global            GlobalConfig   `yaml:"global,omitempty"`
	ScrapeConfigs     []ScrapeConfig `yaml:"scrape_configs,omitempty"`
	ScrapeConfigFiles []string       `yaml:"scrape_config_files,omitempty"`

	// This is set to the directory from where the config has been loaded.
	baseDir string
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
	for i := range cfg.ScrapeConfigs {
		cfg.ScrapeConfigs[i].mustStart(cfg.baseDir)
	}
	jobNames := cfg.getJobNames()
	tsmGlobal.registerJobNames(jobNames)
	logger.Infof("started service discovery routines in %.3f seconds", time.Since(startTime).Seconds())
}

func (cfg *Config) mustStop() {
	startTime := time.Now()
	logger.Infof("stopping service discovery routines...")
	for i := range cfg.ScrapeConfigs {
		cfg.ScrapeConfigs[i].mustStop()
	}
	logger.Infof("stopped service discovery routines in %.3f seconds", time.Since(startTime).Seconds())
}

// getJobNames returns all the scrape job names from the cfg.
func (cfg *Config) getJobNames() []string {
	a := make([]string, 0, len(cfg.ScrapeConfigs))
	for i := range cfg.ScrapeConfigs {
		a = append(a, cfg.ScrapeConfigs[i].JobName)
	}
	return a
}

// GlobalConfig represents essential parts for `global` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type GlobalConfig struct {
	ScrapeInterval time.Duration     `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout  time.Duration     `yaml:"scrape_timeout,omitempty"`
	ExternalLabels map[string]string `yaml:"external_labels,omitempty"`
}

// ScrapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type ScrapeConfig struct {
	JobName              string                      `yaml:"job_name"`
	ScrapeInterval       time.Duration               `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout        time.Duration               `yaml:"scrape_timeout,omitempty"`
	MetricsPath          string                      `yaml:"metrics_path,omitempty"`
	HonorLabels          bool                        `yaml:"honor_labels,omitempty"`
	HonorTimestamps      *bool                       `yaml:"honor_timestamps,omitempty"`
	FollowRedirects      *bool                       `yaml:"follow_redirects,omitempty"`
	Scheme               string                      `yaml:"scheme,omitempty"`
	Params               map[string][]string         `yaml:"params,omitempty"`
	HTTPClientConfig     promauth.HTTPClientConfig   `yaml:",inline"`
	ProxyURL             *proxy.URL                  `yaml:"proxy_url,omitempty"`
	RelabelConfigs       []promrelabel.RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []promrelabel.RelabelConfig `yaml:"metric_relabel_configs,omitempty"`
	SampleLimit          int                         `yaml:"sample_limit,omitempty"`

	ConsulSDConfigs       []consul.SDConfig       `yaml:"consul_sd_configs,omitempty"`
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
	OpenStackSDConfigs    []openstack.SDConfig    `yaml:"openstack_sd_configs,omitempty"`
	StaticConfigs         []StaticConfig          `yaml:"static_configs,omitempty"`

	// These options are supported only by lib/promscrape.
	RelabelDebug        bool                       `yaml:"relabel_debug,omitempty"`
	MetricRelabelDebug  bool                       `yaml:"metric_relabel_debug,omitempty"`
	DisableCompression  bool                       `yaml:"disable_compression,omitempty"`
	DisableKeepAlive    bool                       `yaml:"disable_keepalive,omitempty"`
	StreamParse         bool                       `yaml:"stream_parse,omitempty"`
	ScrapeAlignInterval time.Duration              `yaml:"scrape_align_interval,omitempty"`
	ScrapeOffset        time.Duration              `yaml:"scrape_offset,omitempty"`
	SeriesLimit         int                        `yaml:"series_limit,omitempty"`
	ProxyClientConfig   promauth.ProxyClientConfig `yaml:",inline"`

	// This is set in loadConfig
	swc *scrapeWorkConfig
}

func (sc *ScrapeConfig) mustStart(baseDir string) {
	swosFunc := func(metaLabels map[string]string) interface{} {
		target := metaLabels["__address__"]
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
	for i := range sc.ConsulSDConfigs {
		sc.ConsulSDConfigs[i].MustStop()
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
	for i := range sc.OpenStackSDConfigs {
		sc.OpenStackSDConfigs[i].MustStop()
	}
}

// FileSDConfig represents file-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config
type FileSDConfig struct {
	Files []string `yaml:"files"`
	// `refresh_interval` is ignored. See `-prometheus.fileSDCheckInterval`
}

// StaticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type StaticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

func loadStaticConfigs(path string) ([]StaticConfig, error) {
	data, err := fs.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `static_configs` from %q: %w", path, err)
	}
	data = envtemplate.Replace(data)
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

func loadScrapeConfigFiles(baseDir string, scrapeConfigFiles []string) ([]ScrapeConfig, []byte, error) {
	var scrapeConfigs []ScrapeConfig
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
			data = envtemplate.Replace(data)
			var scs []ScrapeConfig
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
	if err := unmarshalMaybeStrict(data, cfg); err != nil {
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
	for i := range cfg.ScrapeConfigs {
		jobName := cfg.ScrapeConfigs[i].JobName
		if _, ok := m[jobName]; ok {
			return nil, fmt.Errorf("duplicate `job_name` in `scrape_configs` loaded from %q: %q", path, jobName)
		}
		m[jobName] = struct{}{}
	}

	// Initialize cfg.ScrapeConfigs
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
		swc, err := getScrapeWorkConfig(sc, cfg.baseDir, &cfg.Global)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `scrape_config` #%d: %w", i+1, err)
		}
		sc.swc = swc
	}
	return dataNew, nil
}

func unmarshalMaybeStrict(data []byte, dst interface{}) error {
	data = envtemplate.Replace(data)
	var err error
	if *strictParse {
		err = yaml.UnmarshalStrict(data, dst)
	} else {
		err = yaml.Unmarshal(data, dst)
	}
	return err
}

func getSWSByJob(sws []*ScrapeWork) map[string][]*ScrapeWork {
	m := make(map[string][]*ScrapeWork)
	for _, sw := range sws {
		m[sw.jobNameOriginal] = append(m[sw.jobNameOriginal], sw)
	}
	return m
}

// getConsulSDScrapeWork returns `consul_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getConsulSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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

// getDigitalOceanDScrapeWork returns `digitalocean_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDigitalOceanDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	// Create a map for the previous scrape work.
	swsMapPrev := make(map[string][]*ScrapeWork)
	for _, sw := range prev {
		filepath := promrelabel.GetLabelValueByName(sw.Labels, "__vm_filepath")
		if len(filepath) == 0 {
			logger.Panicf("BUG: missing `__vm_filepath` label")
		} else {
			swsMapPrev[filepath] = append(swsMapPrev[filepath], sw)
		}
	}
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
		for j := range sc.FileSDConfigs {
			sdc := &sc.FileSDConfigs[j]
			dst = sdc.appendScrapeWork(dst, swsMapPrev, cfg.baseDir, sc.swc)
		}
	}
	return dst
}

// getGCESDScrapeWork returns `gce_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getGCESDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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

// getOpenStackSDScrapeWork returns `openstack_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getOpenStackSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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

// getStaticScrapeWork returns `static_configs` ScrapeWork from from cfg.
func (cfg *Config) getStaticScrapeWork() []*ScrapeWork {
	var dst []*ScrapeWork
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
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
	scrapeInterval := sc.ScrapeInterval
	if scrapeInterval <= 0 {
		scrapeInterval = globalCfg.ScrapeInterval
		if scrapeInterval <= 0 {
			scrapeInterval = defaultScrapeInterval
		}
	}
	scrapeTimeout := sc.ScrapeTimeout
	if scrapeTimeout <= 0 {
		scrapeTimeout = globalCfg.ScrapeTimeout
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
	honorTimestamps := true
	if sc.HonorTimestamps != nil {
		honorTimestamps = *sc.HonorTimestamps
	}
	denyRedirects := false
	if sc.FollowRedirects != nil {
		denyRedirects = !*sc.FollowRedirects
	}
	metricsPath := sc.MetricsPath
	if metricsPath == "" {
		metricsPath = "/metrics"
	}
	scheme := sc.Scheme
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
	relabelConfigs, err := promrelabel.ParseRelabelConfigs(sc.RelabelConfigs, sc.RelabelDebug)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	metricRelabelConfigs, err := promrelabel.ParseRelabelConfigs(sc.MetricRelabelConfigs, sc.MetricRelabelDebug)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `metric_relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	if (*streamParse || sc.StreamParse) && sc.SampleLimit > 0 {
		return nil, fmt.Errorf("cannot use stream parsing mode when `sample_limit` is set for `job_name` %q", jobName)
	}
	if (*streamParse || sc.StreamParse) && sc.SeriesLimit > 0 {
		return nil, fmt.Errorf("cannot use stream parsing mode when `series_limit` is set for `job_name` %q", jobName)
	}
	swc := &scrapeWorkConfig{
		scrapeInterval:       scrapeInterval,
		scrapeTimeout:        scrapeTimeout,
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
		externalLabels:       globalCfg.ExternalLabels,
		relabelConfigs:       relabelConfigs,
		metricRelabelConfigs: metricRelabelConfigs,
		sampleLimit:          sc.SampleLimit,
		disableCompression:   sc.DisableCompression,
		disableKeepAlive:     sc.DisableKeepAlive,
		streamParse:          sc.StreamParse,
		scrapeAlignInterval:  sc.ScrapeAlignInterval,
		scrapeOffset:         sc.ScrapeOffset,
		seriesLimit:          sc.SeriesLimit,
	}
	return swc, nil
}

type scrapeWorkConfig struct {
	scrapeInterval       time.Duration
	scrapeTimeout        time.Duration
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
	externalLabels       map[string]string
	relabelConfigs       *promrelabel.ParsedConfigs
	metricRelabelConfigs *promrelabel.ParsedConfigs
	sampleLimit          int
	disableCompression   bool
	disableKeepAlive     bool
	streamParse          bool
	scrapeAlignInterval  time.Duration
	scrapeOffset         time.Duration
	seriesLimit          int
}

type targetLabelsGetter interface {
	GetLabels(baseDir string) ([]map[string]string, error)
}

func appendSDScrapeWork(dst []*ScrapeWork, sdc targetLabelsGetter, baseDir string, swc *scrapeWorkConfig, discoveryType string) ([]*ScrapeWork, bool) {
	targetLabels, err := sdc.GetLabels(baseDir)
	if err != nil {
		logger.Errorf("skipping %s targets for job_name %q because of error: %s", discoveryType, swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, discoveryType), true
}

func appendScrapeWorkForTargetLabels(dst []*ScrapeWork, swc *scrapeWorkConfig, targetLabels []map[string]string, discoveryType string) []*ScrapeWork {
	startTime := time.Now()
	// Process targetLabels in parallel in order to reduce processing time for big number of targetLabels.
	type result struct {
		sw  *ScrapeWork
		err error
	}
	goroutines := cgroup.AvailableCPUs()
	resultCh := make(chan result, len(targetLabels))
	workCh := make(chan map[string]string, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			for metaLabels := range workCh {
				target := metaLabels["__address__"]
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

func (sdc *FileSDConfig) appendScrapeWork(dst []*ScrapeWork, swsMapPrev map[string][]*ScrapeWork, baseDir string, swc *scrapeWorkConfig) []*ScrapeWork {
	for _, file := range sdc.Files {
		pathPattern := fs.GetFilepath(baseDir, file)
		paths := []string{pathPattern}
		if strings.Contains(pathPattern, "*") {
			var err error
			paths, err = filepath.Glob(pathPattern)
			if err != nil {
				// Do not return this error, since other files may contain valid scrape configs.
				logger.Errorf("invalid pattern %q in `files` section: %s; skipping it", file, err)
				continue
			}
		}
		for _, path := range paths {
			stcs, err := loadStaticConfigs(path)
			if err != nil {
				// Do not return this error, since other paths may contain valid scrape configs.
				if sws := swsMapPrev[path]; sws != nil {
					// Re-use the previous valid scrape work for this path.
					logger.Errorf("keeping the previously loaded `static_configs` from %q because of error when re-loading the file: %s", path, err)
					dst = append(dst, sws...)
				} else {
					logger.Errorf("skipping loading `static_configs` from %q because of error: %s", path, err)
				}
				continue
			}
			pathShort := path
			if strings.HasPrefix(pathShort, baseDir) {
				pathShort = path[len(baseDir):]
				if len(pathShort) > 0 && pathShort[0] == filepath.Separator {
					pathShort = pathShort[1:]
				}
			}
			metaLabels := map[string]string{
				"__meta_filepath": pathShort,
				"__vm_filepath":   path, // This label is needed for internal promscrape logic
			}
			for i := range stcs {
				dst = stcs[i].appendScrapeWork(dst, swc, metaLabels)
			}
		}
	}
	return dst
}

func (stc *StaticConfig) appendScrapeWork(dst []*ScrapeWork, swc *scrapeWorkConfig, metaLabels map[string]string) []*ScrapeWork {
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

func appendScrapeWorkKey(dst []byte, labels []prompbmarshal.Label) []byte {
	for _, label := range labels {
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

func (swc *scrapeWorkConfig) getScrapeWork(target string, extraLabels, metaLabels map[string]string) (*ScrapeWork, error) {
	labels := mergeLabels(swc, target, extraLabels, metaLabels)
	var originalLabels []prompbmarshal.Label
	if !*dropOriginalLabels {
		originalLabels = append([]prompbmarshal.Label{}, labels...)
	}
	labels = swc.relabelConfigs.Apply(labels, 0, false)
	labels = promrelabel.RemoveMetaLabels(labels[:0], labels)
	// Remove references to already deleted labels, so GC could clean strings for label name and label value past len(labels).
	// This should reduce memory usage when relabeling creates big number of temporary labels with long names and/or values.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 for details.
	labels = append([]prompbmarshal.Label{}, labels...)

	// Verify whether the scrape work must be skipped because of `-promscrape.cluster.*` configs.
	// Perform the verification on labels after the relabeling in order to guarantee that targets with the same set of labels
	// go to the same vmagent shard.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1687#issuecomment-940629495
	if *clusterMembersCount > 1 {
		bb := scrapeWorkKeyBufPool.Get()
		bb.B = appendScrapeWorkKey(bb.B[:0], labels)
		needSkip := needSkipScrapeWork(bytesutil.ToUnsafeString(bb.B), *clusterMembersCount, *clusterReplicationFactor, *clusterMemberNum)
		scrapeWorkKeyBufPool.Put(bb)
		if needSkip {
			return nil, nil
		}
	}
	if !*dropOriginalLabels {
		promrelabel.SortLabels(originalLabels)
		// Reduce memory usage by interning all the strings in originalLabels.
		internLabelStrings(originalLabels)
	}
	if len(labels) == 0 {
		// Drop target without labels.
		droppedTargetsMap.Register(originalLabels)
		return nil, nil
	}
	// See https://www.robustperception.io/life-of-a-label
	schemeRelabeled := promrelabel.GetLabelValueByName(labels, "__scheme__")
	if len(schemeRelabeled) == 0 {
		schemeRelabeled = "http"
	}
	addressRelabeled := promrelabel.GetLabelValueByName(labels, "__address__")
	if len(addressRelabeled) == 0 {
		// Drop target without scrape address.
		droppedTargetsMap.Register(originalLabels)
		return nil, nil
	}
	if strings.Contains(addressRelabeled, "/") {
		// Drop target with '/'
		droppedTargetsMap.Register(originalLabels)
		return nil, nil
	}
	addressRelabeled = addMissingPort(schemeRelabeled, addressRelabeled)
	metricsPathRelabeled := promrelabel.GetLabelValueByName(labels, "__metrics_path__")
	if metricsPathRelabeled == "" {
		metricsPathRelabeled = "/metrics"
	}
	if !strings.HasPrefix(metricsPathRelabeled, "/") {
		metricsPathRelabeled = "/" + metricsPathRelabeled
	}
	paramsRelabeled := getParamsFromLabels(labels, swc.params)
	optionalQuestion := "?"
	if len(paramsRelabeled) == 0 || strings.Contains(metricsPathRelabeled, "?") {
		optionalQuestion = ""
	}
	paramsStr := url.Values(paramsRelabeled).Encode()
	scrapeURL := fmt.Sprintf("%s://%s%s%s%s", schemeRelabeled, addressRelabeled, metricsPathRelabeled, optionalQuestion, paramsStr)
	if _, err := url.Parse(scrapeURL); err != nil {
		return nil, fmt.Errorf("invalid url %q for scheme=%q (%q), target=%q (%q), metrics_path=%q (%q) for `job_name` %q: %w",
			scrapeURL, swc.scheme, schemeRelabeled, target, addressRelabeled, swc.metricsPath, metricsPathRelabeled, swc.jobName, err)
	}
	// Set missing "instance" label according to https://www.robustperception.io/life-of-a-label
	if promrelabel.GetLabelByName(labels, "instance") == nil {
		labels = append(labels, prompbmarshal.Label{
			Name:  "instance",
			Value: addressRelabeled,
		})
		promrelabel.SortLabels(labels)
	}
	// Read __scrape_interval__ and __scrape_timeout__ from labels.
	scrapeInterval := swc.scrapeInterval
	if s := promrelabel.GetLabelValueByName(labels, "__scrape_interval__"); len(s) > 0 {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __scrape_interval__=%q: %w", s, err)
		}
		scrapeInterval = d
	}
	scrapeTimeout := swc.scrapeTimeout
	if s := promrelabel.GetLabelValueByName(labels, "__scrape_timeout__"); len(s) > 0 {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __scrape_timeout__=%q: %w", s, err)
		}
		scrapeTimeout = d
	}
	// Read series_limit option from __series_limit__ label.
	// See https://docs.victoriametrics.com/vmagent.html#cardinality-limiter
	seriesLimit := swc.seriesLimit
	if s := promrelabel.GetLabelValueByName(labels, "__series_limit__"); len(s) > 0 {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __series_limit__=%q: %w", s, err)
		}
		seriesLimit = n
	}
	// Read stream_parse option from __stream_parse__ label.
	// See https://docs.victoriametrics.com/vmagent.html#stream-parsing-mode
	streamParse := swc.streamParse
	if s := promrelabel.GetLabelValueByName(labels, "__stream_parse__"); len(s) > 0 {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("cannot parse __stream_parse__=%q: %w", s, err)
		}
		streamParse = b
	}
	// Reduce memory usage by interning all the strings in labels.
	internLabelStrings(labels)
	sw := &ScrapeWork{
		ScrapeURL:            scrapeURL,
		ScrapeInterval:       scrapeInterval,
		ScrapeTimeout:        scrapeTimeout,
		HonorLabels:          swc.honorLabels,
		HonorTimestamps:      swc.honorTimestamps,
		DenyRedirects:        swc.denyRedirects,
		OriginalLabels:       originalLabels,
		Labels:               labels,
		ProxyURL:             swc.proxyURL,
		ProxyAuthConfig:      swc.proxyAuthConfig,
		AuthConfig:           swc.authConfig,
		MetricRelabelConfigs: swc.metricRelabelConfigs,
		SampleLimit:          swc.sampleLimit,
		DisableCompression:   swc.disableCompression,
		DisableKeepAlive:     swc.disableKeepAlive,
		StreamParse:          streamParse,
		ScrapeAlignInterval:  swc.scrapeAlignInterval,
		ScrapeOffset:         swc.scrapeOffset,
		SeriesLimit:          seriesLimit,

		jobNameOriginal: swc.jobName,
	}
	return sw, nil
}

func internLabelStrings(labels []prompbmarshal.Label) {
	for i := range labels {
		label := &labels[i]
		label.Name = internString(label.Name)
		label.Value = internString(label.Value)
	}
}

func internString(s string) string {
	internStringsMapLock.Lock()
	defer internStringsMapLock.Unlock()

	if sInterned, ok := internStringsMap[s]; ok {
		return sInterned
	}
	// Make a new copy for s in order to remove references from possible bigger string s refers to.
	sCopy := string(append([]byte{}, s...))
	internStringsMap[sCopy] = sCopy
	if len(internStringsMap) > 100e3 {
		internStringsMap = make(map[string]string, 100e3)
	}
	return sCopy
}

var (
	internStringsMapLock sync.Mutex
	internStringsMap     = make(map[string]string, 100e3)
)

func getParamsFromLabels(labels []prompbmarshal.Label, paramsOrig map[string][]string) map[string][]string {
	// See https://www.robustperception.io/life-of-a-label
	m := make(map[string][]string)
	for i := range labels {
		label := &labels[i]
		if !strings.HasPrefix(label.Name, "__param_") {
			continue
		}
		name := label.Name[len("__param_"):]
		values := []string{label.Value}
		if p := paramsOrig[name]; len(p) > 1 {
			values = append(values, p[1:]...)
		}
		m[name] = values
	}
	return m
}

func mergeLabels(swc *scrapeWorkConfig, target string, extraLabels, metaLabels map[string]string) []prompbmarshal.Label {
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	m := make(map[string]string, 6+len(swc.externalLabels)+len(swc.params)+len(extraLabels)+len(metaLabels))
	for k, v := range swc.externalLabels {
		m[k] = v
	}
	m["job"] = swc.jobName
	m["__address__"] = target
	m["__scheme__"] = swc.scheme
	m["__metrics_path__"] = swc.metricsPath
	m["__scrape_interval__"] = swc.scrapeInterval.String()
	m["__scrape_timeout__"] = swc.scrapeTimeout.String()
	for k, args := range swc.params {
		if len(args) == 0 {
			continue
		}
		k = "__param_" + k
		v := args[0]
		m[k] = v
	}
	for k, v := range extraLabels {
		m[k] = v
	}
	for k, v := range metaLabels {
		m[k] = v
	}
	result := make([]prompbmarshal.Label, 0, len(m))
	for k, v := range m {
		result = append(result, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	return result
}

func addMissingPort(scheme, target string) string {
	if strings.Contains(target, ":") {
		return target
	}
	if scheme == "https" {
		target += ":443"
	} else {
		target += ":80"
	}
	return target
}

const (
	defaultScrapeInterval = time.Minute
	defaultScrapeTimeout  = 10 * time.Second
)
