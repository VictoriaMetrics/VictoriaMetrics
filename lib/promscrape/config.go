package promscrape

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dns"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dockerswarm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/ec2"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/eureka"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/gce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/openstack"
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
)

// Config represents essential parts from Prometheus config defined at https://prometheus.io/docs/prometheus/latest/configuration/configuration/
type Config struct {
	Global        GlobalConfig   `yaml:"global"`
	ScrapeConfigs []ScrapeConfig `yaml:"scrape_configs"`

	// This is set to the directory from where the config has been loaded.
	baseDir string
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
	HonorTimestamps      bool                        `yaml:"honor_timestamps,omitempty"`
	Scheme               string                      `yaml:"scheme,omitempty"`
	Params               map[string][]string         `yaml:"params,omitempty"`
	BasicAuth            *promauth.BasicAuthConfig   `yaml:"basic_auth,omitempty"`
	BearerToken          string                      `yaml:"bearer_token,omitempty"`
	BearerTokenFile      string                      `yaml:"bearer_token_file,omitempty"`
	ProxyURL             netutil.ProxyURL            `yaml:"proxy_url,omitempty"`
	TLSConfig            *promauth.TLSConfig         `yaml:"tls_config,omitempty"`
	StaticConfigs        []StaticConfig              `yaml:"static_configs,omitempty"`
	FileSDConfigs        []FileSDConfig              `yaml:"file_sd_configs,omitempty"`
	KubernetesSDConfigs  []kubernetes.SDConfig       `yaml:"kubernetes_sd_configs,omitempty"`
	OpenStackSDConfigs   []openstack.SDConfig        `yaml:"openstack_sd_configs,omitempty"`
	ConsulSDConfigs      []consul.SDConfig           `yaml:"consul_sd_configs,omitempty"`
	EurekaSDConfigs      []eureka.SDConfig           `yaml:"eureka_sd_configs,omitempty"`
	DockerSwarmConfigs   []dockerswarm.SDConfig      `yaml:"dockerswarm_sd_configs,omitempty"`
	DNSSDConfigs         []dns.SDConfig              `yaml:"dns_sd_configs,omitempty"`
	EC2SDConfigs         []ec2.SDConfig              `yaml:"ec2_sd_configs,omitempty"`
	GCESDConfigs         []gce.SDConfig              `yaml:"gce_sd_configs,omitempty"`
	RelabelConfigs       []promrelabel.RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []promrelabel.RelabelConfig `yaml:"metric_relabel_configs,omitempty"`
	SampleLimit          int                         `yaml:"sample_limit,omitempty"`

	// These options are supported only by lib/promscrape.
	DisableCompression bool `yaml:"disable_compression,omitempty"`
	DisableKeepAlive   bool `yaml:"disable_keepalive,omitempty"`
	StreamParse        bool `yaml:"stream_parse,omitempty"`

	// This is set in loadConfig
	swc *scrapeWorkConfig
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
	data, err := ioutil.ReadFile(path)
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
func loadConfig(path string) (cfg *Config, data []byte, err error) {
	data, err = ioutil.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read Prometheus config from %q: %w", path, err)
	}
	var cfgObj Config
	if err := cfgObj.parse(data, path); err != nil {
		return nil, nil, fmt.Errorf("cannot parse Prometheus config from %q: %w", path, err)
	}
	return &cfgObj, data, nil
}

// IsDryRun returns true if -promscrape.config.dryRun command-line flag is set
func IsDryRun() bool {
	return *dryRun
}

func (cfg *Config) parse(data []byte, path string) error {
	if err := unmarshalMaybeStrict(data, cfg); err != nil {
		return fmt.Errorf("cannot unmarshal data: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot obtain abs path for %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
		swc, err := getScrapeWorkConfig(sc, cfg.baseDir, &cfg.Global)
		if err != nil {
			return fmt.Errorf("cannot parse `scrape_config` #%d: %w", i+1, err)
		}
		sc.swc = swc
	}
	return nil
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
			var okLocal bool
			dst, okLocal = appendKubernetesScrapeWork(dst, sdc, cfg.baseDir, sc.swc)
			if ok {
				ok = okLocal
			}
		}
		if ok {
			continue
		}
		swsPrev := swsPrevByJob[sc.swc.jobName]
		if len(swsPrev) > 0 {
			logger.Errorf("there were errors when discovering kubernetes targets for job %q, so preserving the previous targets", sc.swc.jobName)
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
			dst, okLocal = appendOpenstackScrapeWork(dst, sdc, cfg.baseDir, sc.swc)
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

// getDockerSwarmSDScrapeWork returns `dockerswarm_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getDockerSwarmSDScrapeWork(prev []*ScrapeWork) []*ScrapeWork {
	swsPrevByJob := getSWSByJob(prev)
	dst := make([]*ScrapeWork, 0, len(prev))
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
		dstLen := len(dst)
		ok := true
		for j := range sc.DockerSwarmConfigs {
			sdc := &sc.DockerSwarmConfigs[j]
			var okLocal bool
			dst, okLocal = appendDockerSwarmScrapeWork(dst, sdc, cfg.baseDir, sc.swc)
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
			dst, okLocal = appendConsulScrapeWork(dst, sdc, cfg.baseDir, sc.swc)
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
			dst, okLocal = appendEurekaScrapeWork(dst, sdc, cfg.baseDir, sc.swc)
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
			dst, okLocal = appendDNSScrapeWork(dst, sdc, sc.swc)
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
			dst, okLocal = appendEC2ScrapeWork(dst, sdc, sc.swc)
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
			dst, okLocal = appendGCEScrapeWork(dst, sdc, sc.swc)
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
	honorLabels := sc.HonorLabels
	honorTimestamps := sc.HonorTimestamps
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
	ac, err := promauth.NewConfig(baseDir, sc.BasicAuth, sc.BearerToken, sc.BearerTokenFile, sc.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config for `job_name` %q: %w", jobName, err)
	}
	var relabelConfigs []promrelabel.ParsedRelabelConfig
	relabelConfigs, err = promrelabel.ParseRelabelConfigs(relabelConfigs[:0], sc.RelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	var metricRelabelConfigs []promrelabel.ParsedRelabelConfig
	metricRelabelConfigs, err = promrelabel.ParseRelabelConfigs(metricRelabelConfigs[:0], sc.MetricRelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `metric_relabel_configs` for `job_name` %q: %w", jobName, err)
	}
	swc := &scrapeWorkConfig{
		scrapeInterval:       scrapeInterval,
		scrapeTimeout:        scrapeTimeout,
		jobName:              jobName,
		metricsPath:          metricsPath,
		scheme:               scheme,
		params:               params,
		proxyURL:             sc.ProxyURL.URL(),
		authConfig:           ac,
		honorLabels:          honorLabels,
		honorTimestamps:      honorTimestamps,
		externalLabels:       globalCfg.ExternalLabels,
		relabelConfigs:       relabelConfigs,
		metricRelabelConfigs: metricRelabelConfigs,
		sampleLimit:          sc.SampleLimit,
		disableCompression:   sc.DisableCompression,
		disableKeepAlive:     sc.DisableKeepAlive,
		streamParse:          sc.StreamParse,
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
	proxyURL             *url.URL
	authConfig           *promauth.Config
	honorLabels          bool
	honorTimestamps      bool
	externalLabels       map[string]string
	relabelConfigs       []promrelabel.ParsedRelabelConfig
	metricRelabelConfigs []promrelabel.ParsedRelabelConfig
	sampleLimit          int
	disableCompression   bool
	disableKeepAlive     bool
	streamParse          bool
}

func appendKubernetesScrapeWork(dst []*ScrapeWork, sdc *kubernetes.SDConfig, baseDir string, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := kubernetes.GetLabels(sdc, baseDir)
	if err != nil {
		logger.Errorf("error when discovering kubernetes targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "kubernetes_sd_config"), true
}

func appendOpenstackScrapeWork(dst []*ScrapeWork, sdc *openstack.SDConfig, baseDir string, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := openstack.GetLabels(sdc, baseDir)
	if err != nil {
		logger.Errorf("error when discovering openstack targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "openstack_sd_config"), true
}

func appendDockerSwarmScrapeWork(dst []*ScrapeWork, sdc *dockerswarm.SDConfig, baseDir string, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := dockerswarm.GetLabels(sdc, baseDir)
	if err != nil {
		logger.Errorf("error when discovering dockerswarm targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "dockerswarm_sd_config"), true
}

func appendConsulScrapeWork(dst []*ScrapeWork, sdc *consul.SDConfig, baseDir string, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := consul.GetLabels(sdc, baseDir)
	if err != nil {
		logger.Errorf("error when discovering consul targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "consul_sd_config"), true
}

func appendEurekaScrapeWork(dst []*ScrapeWork, sdc *eureka.SDConfig, baseDir string, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := eureka.GetLabels(sdc, baseDir)
	if err != nil {
		logger.Errorf("error when discovering eureka targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "eureka_sd_config"), true
}

func appendDNSScrapeWork(dst []*ScrapeWork, sdc *dns.SDConfig, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := dns.GetLabels(sdc)
	if err != nil {
		logger.Errorf("error when discovering dns targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "dns_sd_config"), true
}

func appendEC2ScrapeWork(dst []*ScrapeWork, sdc *ec2.SDConfig, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := ec2.GetLabels(sdc)
	if err != nil {
		logger.Errorf("error when discovering ec2 targets for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "ec2_sd_config"), true
}

func appendGCEScrapeWork(dst []*ScrapeWork, sdc *gce.SDConfig, swc *scrapeWorkConfig) ([]*ScrapeWork, bool) {
	targetLabels, err := gce.GetLabels(sdc)
	if err != nil {
		logger.Errorf("error when discovering gce targets for `job_name` %q: %s; skippint it", swc.jobName, err)
		return dst, false
	}
	return appendScrapeWorkForTargetLabels(dst, swc, targetLabels, "gce_sd_config"), true
}

func appendScrapeWorkForTargetLabels(dst []*ScrapeWork, swc *scrapeWorkConfig, targetLabels []map[string]string, sectionName string) []*ScrapeWork {
	for _, metaLabels := range targetLabels {
		target := metaLabels["__address__"]
		var err error
		dst, err = appendScrapeWork(dst, swc, target, nil, metaLabels)
		if err != nil {
			logger.Errorf("error when parsing `%s` target %q for `job_name` %q: %s; skipping it", sectionName, target, swc.jobName, err)
			continue
		}
	}
	return dst
}

func (sdc *FileSDConfig) appendScrapeWork(dst []*ScrapeWork, swsMapPrev map[string][]*ScrapeWork, baseDir string, swc *scrapeWorkConfig) []*ScrapeWork {
	for _, file := range sdc.Files {
		pathPattern := getFilepath(baseDir, file)
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
		var err error
		dst, err = appendScrapeWork(dst, swc, target, stc.Labels, metaLabels)
		if err != nil {
			// Do not return this error, since other targets may be valid
			logger.Errorf("error when parsing `static_configs` target %q for `job_name` %q: %s; skipping it", target, swc.jobName, err)
			continue
		}
	}
	return dst
}

func appendScrapeWork(dst []*ScrapeWork, swc *scrapeWorkConfig, target string, extraLabels, metaLabels map[string]string) ([]*ScrapeWork, error) {
	labels := mergeLabels(swc.jobName, swc.scheme, target, swc.metricsPath, extraLabels, swc.externalLabels, metaLabels, swc.params)
	var originalLabels []prompbmarshal.Label
	if !*dropOriginalLabels {
		originalLabels = append([]prompbmarshal.Label{}, labels...)
		promrelabel.SortLabels(originalLabels)
		// Reduce memory usage by interning all the strings in originalLabels.
		internLabelStrings(originalLabels)
	}
	labels = promrelabel.ApplyRelabelConfigs(labels, 0, swc.relabelConfigs, false)
	labels = promrelabel.RemoveMetaLabels(labels[:0], labels)
	// Remove references to already deleted labels, so GC could clean strings for label name and label value past len(labels).
	// This should reduce memory usage when relabeling creates big number of temporary labels with long names and/or values.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 for details.
	labels = append([]prompbmarshal.Label{}, labels...)

	if len(labels) == 0 {
		// Drop target without labels.
		droppedTargetsMap.Register(originalLabels)
		return dst, nil
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
		return dst, nil
	}
	if strings.Contains(addressRelabeled, "/") {
		// Drop target with '/'
		droppedTargetsMap.Register(originalLabels)
		return dst, nil
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
		return dst, fmt.Errorf("invalid url %q for scheme=%q (%q), target=%q (%q), metrics_path=%q (%q) for `job_name` %q: %w",
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
	// Reduce memory usage by interning all the strings in labels.
	internLabelStrings(labels)
	dst = append(dst, &ScrapeWork{
		ScrapeURL:            scrapeURL,
		ScrapeInterval:       swc.scrapeInterval,
		ScrapeTimeout:        swc.scrapeTimeout,
		HonorLabels:          swc.honorLabels,
		HonorTimestamps:      swc.honorTimestamps,
		OriginalLabels:       originalLabels,
		Labels:               labels,
		ProxyURL:             swc.proxyURL,
		AuthConfig:           swc.authConfig,
		MetricRelabelConfigs: swc.metricRelabelConfigs,
		SampleLimit:          swc.sampleLimit,
		DisableCompression:   swc.disableCompression,
		DisableKeepAlive:     swc.disableKeepAlive,
		StreamParse:          swc.streamParse,

		jobNameOriginal: swc.jobName,
	})
	return dst, nil
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

func mergeLabels(job, scheme, target, metricsPath string, extraLabels, externalLabels, metaLabels map[string]string, params map[string][]string) []prompbmarshal.Label {
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	m := make(map[string]string)
	for k, v := range externalLabels {
		m[k] = v
	}
	m["job"] = job
	m["__address__"] = target
	m["__scheme__"] = scheme
	m["__metrics_path__"] = metricsPath
	for k, args := range params {
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

func getFilepath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
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
