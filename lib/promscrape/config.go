package promscrape

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
	"gopkg.in/yaml.v2"
)

var (
	strictParse = flag.Bool("promscrape.config.strictParse", false, "Whether to allow only supported fields in '-promscrape.config'. "+
		"This option may be used for errors detection in '-promscrape.config' file")
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
	ScrapeInterval time.Duration     `yaml:"scrape_interval"`
	ScrapeTimeout  time.Duration     `yaml:"scrape_timeout"`
	ExternalLabels map[string]string `yaml:"external_labels"`
}

// ScrapeConfig represents essential parts for `scrape_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config
type ScrapeConfig struct {
	JobName              string                      `yaml:"job_name"`
	ScrapeInterval       time.Duration               `yaml:"scrape_interval"`
	ScrapeTimeout        time.Duration               `yaml:"scrape_timeout"`
	MetricsPath          string                      `yaml:"metrics_path"`
	HonorLabels          bool                        `yaml:"honor_labels"`
	HonorTimestamps      bool                        `yaml:"honor_timestamps"`
	Scheme               string                      `yaml:"scheme"`
	Params               map[string][]string         `yaml:"params"`
	BasicAuth            *promauth.BasicAuthConfig   `yaml:"basic_auth"`
	BearerToken          string                      `yaml:"bearer_token"`
	BearerTokenFile      string                      `yaml:"bearer_token_file"`
	TLSConfig            *promauth.TLSConfig         `yaml:"tls_config"`
	StaticConfigs        []StaticConfig              `yaml:"static_configs"`
	FileSDConfigs        []FileSDConfig              `yaml:"file_sd_configs"`
	KubernetesSDConfigs  []KubernetesSDConfig        `yaml:"kubernetes_sd_configs"`
	RelabelConfigs       []promrelabel.RelabelConfig `yaml:"relabel_configs"`
	MetricRelabelConfigs []promrelabel.RelabelConfig `yaml:"metric_relabel_configs"`
	SampleLimit          int                         `yaml:"sample_limit"`

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

// KubernetesSDConfig represents kubernetes-based service discovery config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config
type KubernetesSDConfig struct {
	APIServer       string                    `yaml:"api_server"`
	Role            string                    `yaml:"role"`
	BasicAuth       *promauth.BasicAuthConfig `yaml:"basic_auth"`
	BearerToken     string                    `yaml:"bearer_token"`
	BearerTokenFile string                    `yaml:"bearer_token_file"`
	TLSConfig       *promauth.TLSConfig       `yaml:"tls_config"`
	Namespaces      KubernetesNamespaces      `yaml:"namespaces"`
	Selectors       []kubernetes.Selector     `yaml:"selectors"`
}

// KubernetesNamespaces represents namespaces for KubernetesSDConfig
type KubernetesNamespaces struct {
	Names []string `yaml:"names"`
}

// StaticConfig represents essential parts for `static_config` section of Prometheus config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config
type StaticConfig struct {
	Targets []string          `yaml:"targets"`
	Labels  map[string]string `yaml:"labels"`
}

func loadStaticConfigs(path string) ([]StaticConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `static_configs` from %q: %s", path, err)
	}
	var stcs []StaticConfig
	if err := yaml.UnmarshalStrict(data, &stcs); err != nil {
		return nil, fmt.Errorf("cannot unmarshal `static_configs` from %q: %s", path, err)
	}
	return stcs, nil
}

// loadConfig loads Prometheus config from the given path.
func loadConfig(path string) (cfg *Config, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read Prometheus config from %q: %s", path, err)
	}
	var cfgObj Config
	if err := cfgObj.parse(data, path); err != nil {
		return nil, fmt.Errorf("cannot parse Prometheus config from %q: %s", path, err)
	}
	return &cfgObj, nil
}

func (cfg *Config) parse(data []byte, path string) error {
	if err := unmarshalMaybeStrict(data, cfg); err != nil {
		return fmt.Errorf("cannot unmarshal data: %s", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("cannot obtain abs path for %q: %s", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)
	for i := range cfg.ScrapeConfigs {
		sc := &cfg.ScrapeConfigs[i]
		swc, err := getScrapeWorkConfig(sc, cfg.baseDir, &cfg.Global)
		if err != nil {
			return fmt.Errorf("cannot parse `scrape_config` #%d: %s", i+1, err)
		}
		sc.swc = swc
	}
	return nil
}

func unmarshalMaybeStrict(data []byte, dst interface{}) error {
	var err error
	if *strictParse {
		err = yaml.UnmarshalStrict(data, dst)
	} else {
		err = yaml.Unmarshal(data, dst)
	}
	return err
}

func (cfg *Config) kubernetesSDConfigsCount() int {
	n := 0
	for i := range cfg.ScrapeConfigs {
		n += len(cfg.ScrapeConfigs[i].KubernetesSDConfigs)
	}
	return n
}

func (cfg *Config) fileSDConfigsCount() int {
	n := 0
	for i := range cfg.ScrapeConfigs {
		n += len(cfg.ScrapeConfigs[i].FileSDConfigs)
	}
	return n
}

// getKubernetesSDcrapeWork returns `kubernetes_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getKubernetesSDScrapeWork() []ScrapeWork {
	var dst []ScrapeWork
	for _, sc := range cfg.ScrapeConfigs {
		for _, sdc := range sc.KubernetesSDConfigs {
			dst = sdc.appendScrapeWork(dst, cfg.baseDir, sc.swc)
		}
	}
	return dst
}

// getFileSDScrapeWork returns `file_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getFileSDScrapeWork(prev []ScrapeWork) []ScrapeWork {
	// Create a map for the previous scrape work.
	swPrev := make(map[string][]ScrapeWork)
	for i := range prev {
		sw := &prev[i]
		label := promrelabel.GetLabelByName(sw.Labels, "__meta_filepath")
		if label == nil {
			logger.Panicf("BUG: missing `__meta_filepath` label")
		} else {
			swPrev[label.Value] = append(swPrev[label.Value], *sw)
		}
	}
	var dst []ScrapeWork
	for _, sc := range cfg.ScrapeConfigs {
		for _, sdc := range sc.FileSDConfigs {
			dst = sdc.appendScrapeWork(dst, swPrev, cfg.baseDir, sc.swc)
		}
	}
	return dst
}

// getStaticScrapeWork returns `static_configs` ScrapeWork from from cfg.
func (cfg *Config) getStaticScrapeWork() []ScrapeWork {
	var dst []ScrapeWork
	for _, sc := range cfg.ScrapeConfigs {
		for _, stc := range sc.StaticConfigs {
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
		return nil, fmt.Errorf("cannot parse auth config for `job_name` %q: %s", jobName, err)
	}
	var relabelConfigs []promrelabel.ParsedRelabelConfig
	relabelConfigs, err = promrelabel.ParseRelabelConfigs(relabelConfigs[:0], sc.RelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `relabel_configs` for `job_name` %q: %s", jobName, err)
	}
	var metricRelabelConfigs []promrelabel.ParsedRelabelConfig
	metricRelabelConfigs, err = promrelabel.ParseRelabelConfigs(metricRelabelConfigs[:0], sc.MetricRelabelConfigs)
	if err != nil {
		return nil, fmt.Errorf("cannot parse `metric_relabel_configs` for `job_name` %q: %s", jobName, err)
	}
	swc := &scrapeWorkConfig{
		scrapeInterval:       scrapeInterval,
		scrapeTimeout:        scrapeTimeout,
		jobName:              jobName,
		metricsPath:          metricsPath,
		scheme:               scheme,
		params:               params,
		authConfig:           ac,
		honorLabels:          honorLabels,
		honorTimestamps:      honorTimestamps,
		externalLabels:       globalCfg.ExternalLabels,
		relabelConfigs:       relabelConfigs,
		metricRelabelConfigs: metricRelabelConfigs,
		sampleLimit:          sc.SampleLimit,
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
	authConfig           *promauth.Config
	honorLabels          bool
	honorTimestamps      bool
	externalLabels       map[string]string
	relabelConfigs       []promrelabel.ParsedRelabelConfig
	metricRelabelConfigs []promrelabel.ParsedRelabelConfig
	sampleLimit          int
}

func (sdc *KubernetesSDConfig) appendScrapeWork(dst []ScrapeWork, baseDir string, swc *scrapeWorkConfig) []ScrapeWork {
	ac, err := promauth.NewConfig(baseDir, sdc.BasicAuth, sdc.BearerToken, sdc.BearerTokenFile, sdc.TLSConfig)
	if err != nil {
		logger.Errorf("cannot parse auth config for `kubernetes_sd_config` for `job_name` %q: %s; skipping it", swc.jobName, err)
		return dst
	}
	cfg := &kubernetes.APIConfig{
		Server:     sdc.APIServer,
		AuthConfig: ac,
		Namespaces: sdc.Namespaces.Names,
		Selectors:  sdc.Selectors,
	}
	switch sdc.Role {
	case "node":
		targetLabels, err := kubernetes.GetNodesLabels(cfg)
		if err != nil {
			logger.Errorf("error when discovering kubernetes nodes for `job_name` %q: %s; skipping it", swc.jobName, err)
			return dst
		}
		return appendKubernetesScrapeWork(dst, swc, targetLabels, sdc.Role)
	case "service":
		targetLabels, err := kubernetes.GetServicesLabels(cfg)
		if err != nil {
			logger.Errorf("error when discovering kubernetes services for `job_name` %q: %s; skipping it", swc.jobName, err)
			return dst
		}
		return appendKubernetesScrapeWork(dst, swc, targetLabels, sdc.Role)
	case "pod":
		targetLabels, err := kubernetes.GetPodsLabels(cfg)
		if err != nil {
			logger.Errorf("error when discovering kubernetes pods for `job_name` %q: %s; skipping it", swc.jobName, err)
			return dst
		}
		return appendKubernetesScrapeWork(dst, swc, targetLabels, sdc.Role)
	case "endpoints":
		targetLabels, err := kubernetes.GetEndpointsLabels(cfg)
		if err != nil {
			logger.Errorf("error when discovering kubernetes endpoints for `job_name` %q: %s; skipping it", swc.jobName, err)
			return dst
		}
		return appendKubernetesScrapeWork(dst, swc, targetLabels, sdc.Role)
	case "ingress":
		targetLabels, err := kubernetes.GetIngressesLabels(cfg)
		if err != nil {
			logger.Errorf("error when discovering kubernetes ingresses for `job_name` %q: %s; skipping it", swc.jobName, err)
			return dst
		}
		return appendKubernetesScrapeWork(dst, swc, targetLabels, sdc.Role)
	default:
		logger.Errorf("unexpected `role`: %q; must be one of `node`, `service`, `pod`, `endpoints` or `ingress`; skipping it", sdc.Role)
		return dst
	}
}

func appendKubernetesScrapeWork(dst []ScrapeWork, swc *scrapeWorkConfig, targetLabels []map[string]string, role string) []ScrapeWork {
	for _, metaLabels := range targetLabels {
		target := metaLabels["__address__"]
		var err error
		dst, err = appendScrapeWork(dst, swc, target, nil, metaLabels)
		if err != nil {
			logger.Errorf("error when parsing `kubernetes_sd_config` target %q with role %q for `job_name` %q: %s; skipping it",
				target, role, swc.jobName, err)
			continue
		}
	}
	return dst
}

func (sdc *FileSDConfig) appendScrapeWork(dst []ScrapeWork, swPrev map[string][]ScrapeWork, baseDir string, swc *scrapeWorkConfig) []ScrapeWork {
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
				if sws := swPrev[path]; sws != nil {
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
			}
			for i := range stcs {
				dst = stcs[i].appendScrapeWork(dst, swc, metaLabels)
			}
		}
	}
	return dst
}

func (stc *StaticConfig) appendScrapeWork(dst []ScrapeWork, swc *scrapeWorkConfig, metaLabels map[string]string) []ScrapeWork {
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

func appendScrapeWork(dst []ScrapeWork, swc *scrapeWorkConfig, target string, extraLabels, metaLabels map[string]string) ([]ScrapeWork, error) {
	labels := mergeLabels(swc.jobName, swc.scheme, target, swc.metricsPath, extraLabels, swc.externalLabels, metaLabels, swc.params)
	labels = promrelabel.ApplyRelabelConfigs(labels, 0, swc.relabelConfigs, false)
	if len(labels) == 0 {
		// Drop target without labels.
		return dst, nil
	}
	// See https://www.robustperception.io/life-of-a-label
	schemeRelabeled := ""
	if schemeLabel := promrelabel.GetLabelByName(labels, "__scheme__"); schemeLabel != nil {
		schemeRelabeled = schemeLabel.Value
	}
	if schemeRelabeled == "" {
		schemeRelabeled = "http"
	}
	addressLabel := promrelabel.GetLabelByName(labels, "__address__")
	if addressLabel == nil || addressLabel.Name == "" {
		// Drop target without scrape address.
		return dst, nil
	}
	targetRelabeled := addMissingPort(schemeRelabeled, addressLabel.Value)
	if strings.Contains(targetRelabeled, "/") {
		// Drop target with '/'
		return dst, nil
	}
	metricsPathRelabeled := ""
	if metricsPathLabel := promrelabel.GetLabelByName(labels, "__metrics_path__"); metricsPathLabel != nil {
		metricsPathRelabeled = metricsPathLabel.Value
	}
	if metricsPathRelabeled == "" {
		metricsPathRelabeled = "/metrics"
	}
	paramsRelabeled := getParamsFromLabels(labels, swc.params)
	optionalQuestion := "?"
	if len(paramsRelabeled) == 0 || strings.Contains(metricsPathRelabeled, "?") {
		optionalQuestion = ""
	}
	paramsStr := url.Values(paramsRelabeled).Encode()
	scrapeURL := fmt.Sprintf("%s://%s%s%s%s", schemeRelabeled, targetRelabeled, metricsPathRelabeled, optionalQuestion, paramsStr)
	if _, err := url.Parse(scrapeURL); err != nil {
		return dst, fmt.Errorf("invalid url %q for scheme=%q (%q), target=%q (%q), metrics_path=%q (%q) for `job_name` %q: %s",
			scrapeURL, swc.scheme, schemeRelabeled, target, targetRelabeled, swc.metricsPath, metricsPathRelabeled, swc.jobName, err)
	}
	dst = append(dst, ScrapeWork{
		ScrapeURL:            scrapeURL,
		ScrapeInterval:       swc.scrapeInterval,
		ScrapeTimeout:        swc.scrapeTimeout,
		HonorLabels:          swc.honorLabels,
		HonorTimestamps:      swc.honorTimestamps,
		Labels:               labels,
		AuthConfig:           swc.authConfig,
		MetricRelabelConfigs: swc.metricRelabelConfigs,
		SampleLimit:          swc.sampleLimit,
	})
	return dst, nil
}

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
