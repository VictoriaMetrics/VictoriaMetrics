package promscrape

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"gopkg.in/yaml.v2"
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
	BasicAuth            *BasicAuthConfig            `yaml:"basic_auth"`
	BearerToken          string                      `yaml:"bearer_token"`
	BearerTokenFile      string                      `yaml:"bearer_token_file"`
	TLSConfig            *TLSConfig                  `yaml:"tls_config"`
	StaticConfigs        []StaticConfig              `yaml:"static_configs"`
	FileSDConfigs        []FileSDConfig              `yaml:"file_sd_configs"`
	RelabelConfigs       []promrelabel.RelabelConfig `yaml:"relabel_configs"`
	MetricRelabelConfigs []promrelabel.RelabelConfig `yaml:"metric_relabel_configs"`
	ScrapeLimit          int                         `yaml:"scrape_limit"`

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

// TLSConfig represents TLS config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
type TLSConfig struct {
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	ServerName         string `yaml:"server_name"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

// BasicAuthConfig represents basic auth config.
type BasicAuthConfig struct {
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
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
	if err := yaml.Unmarshal(data, cfg); err != nil {
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

func (cfg *Config) fileSDConfigsCount() int {
	n := 0
	for i := range cfg.ScrapeConfigs {
		n += len(cfg.ScrapeConfigs[i].FileSDConfigs)
	}
	return n
}

// getFileSDScrapeWork returns `file_sd_configs` ScrapeWork from cfg.
func (cfg *Config) getFileSDScrapeWork(prev []ScrapeWork) ([]ScrapeWork, error) {
	var sws []ScrapeWork
	for i := range cfg.ScrapeConfigs {
		var err error
		sws, err = cfg.ScrapeConfigs[i].appendFileSDScrapeWork(sws, prev, cfg.baseDir)
		if err != nil {
			return nil, fmt.Errorf("error when parsing `scrape_config` #%d: %s", i+1, err)
		}
	}
	return sws, nil
}

// getStaticScrapeWork returns `static_configs` ScrapeWork from from cfg.
func (cfg *Config) getStaticScrapeWork() ([]ScrapeWork, error) {
	var sws []ScrapeWork
	for i := range cfg.ScrapeConfigs {
		var err error
		sws, err = cfg.ScrapeConfigs[i].appendStaticScrapeWork(sws)
		if err != nil {
			return nil, fmt.Errorf("error when parsing `scrape_config` #%d: %s", i+1, err)
		}
	}
	return sws, nil
}

func (sc *ScrapeConfig) appendFileSDScrapeWork(dst, prev []ScrapeWork, baseDir string) ([]ScrapeWork, error) {
	if len(sc.FileSDConfigs) == 0 {
		// Fast path - no `file_sd_configs`
		return dst, nil
	}
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
	for i := range sc.FileSDConfigs {
		var err error
		dst, err = sc.FileSDConfigs[i].appendScrapeWork(dst, swPrev, baseDir, sc.swc)
		if err != nil {
			return nil, fmt.Errorf("error when parsing `file_sd_config` #%d: %s", i+1, err)
		}
	}
	return dst, nil
}

func (sc *ScrapeConfig) appendStaticScrapeWork(dst []ScrapeWork) ([]ScrapeWork, error) {
	for i := range sc.StaticConfigs {
		var err error
		dst, err = sc.StaticConfigs[i].appendScrapeWork(dst, sc.swc)
		if err != nil {
			return nil, fmt.Errorf("error when parsing `static_config` #%d: %s", i+1, err)
		}
	}
	return dst, nil
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
	var authorization string
	if sc.BasicAuth != nil {
		if sc.BasicAuth.Username == "" {
			return nil, fmt.Errorf("missing `username` in `basic_auth` section for `job_name` %q", jobName)
		}
		username := sc.BasicAuth.Username
		password := sc.BasicAuth.Password
		if sc.BasicAuth.PasswordFile != "" {
			if sc.BasicAuth.Password != "" {
				return nil, fmt.Errorf("both `password`=%q and `password_file`=%q are set in `basic_auth` section for `job_name` %q",
					sc.BasicAuth.Password, sc.BasicAuth.PasswordFile, jobName)
			}
			path := getFilepath(baseDir, sc.BasicAuth.PasswordFile)
			pass, err := readPasswordFromFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read password from `password_file`=%q set in `basic_auth` section for `job_name` %q: %s",
					sc.BasicAuth.PasswordFile, jobName, err)
			}
			password = pass
		}
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := username + ":" + password
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		authorization = "Basic " + token64
	}
	bearerToken := sc.BearerToken
	if sc.BearerTokenFile != "" {
		if sc.BearerToken != "" {
			return nil, fmt.Errorf("both `bearer_token`=%q and `bearer_token_file`=%q are set for `job_name` %q", sc.BearerToken, sc.BearerTokenFile, jobName)
		}
		path := getFilepath(baseDir, sc.BearerTokenFile)
		token, err := readPasswordFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("cannot read bearer token from `bearer_token_file`=%q for `job_name` %q: %s", sc.BearerTokenFile, jobName, err)
		}
		bearerToken = token
	}
	if bearerToken != "" {
		if authorization != "" {
			return nil, fmt.Errorf("cannot use both `basic_auth` and `bearer_token` for `job_name` %q", jobName)
		}
		authorization = "Bearer " + bearerToken
	}
	var tlsRootCA *x509.CertPool
	var tlsCertificate *tls.Certificate
	tlsServerName := ""
	tlsInsecureSkipVerify := false
	if sc.TLSConfig != nil {
		tlsServerName = sc.TLSConfig.ServerName
		tlsInsecureSkipVerify = sc.TLSConfig.InsecureSkipVerify
		if sc.TLSConfig.CertFile != "" || sc.TLSConfig.KeyFile != "" {
			certPath := getFilepath(baseDir, sc.TLSConfig.CertFile)
			keyPath := getFilepath(baseDir, sc.TLSConfig.KeyFile)
			cert, err := tls.LoadX509KeyPair(certPath, keyPath)
			if err != nil {
				return nil, fmt.Errorf("cannot load TLS certificate for `job_name` %q from `cert_file`=%q, `key_file`=%q: %s",
					jobName, sc.TLSConfig.CertFile, sc.TLSConfig.KeyFile, err)
			}
			tlsCertificate = &cert
		}
		if sc.TLSConfig.CAFile != "" {
			path := getFilepath(baseDir, sc.TLSConfig.CAFile)
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("cannot read `ca_file` %q for `job_name` %q: %s", sc.TLSConfig.CAFile, jobName, err)
			}
			tlsRootCA = x509.NewCertPool()
			if !tlsRootCA.AppendCertsFromPEM(data) {
				return nil, fmt.Errorf("cannot parse data from `ca_file` %q for `job_name` %q", sc.TLSConfig.CAFile, jobName)
			}
		}
	}
	var err error
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
	scrapeLimit := sc.ScrapeLimit
	swc := &scrapeWorkConfig{
		scrapeInterval:        scrapeInterval,
		scrapeTimeout:         scrapeTimeout,
		jobName:               jobName,
		metricsPath:           metricsPath,
		scheme:                scheme,
		params:                params,
		authorization:         authorization,
		honorLabels:           honorLabels,
		honorTimestamps:       honorTimestamps,
		externalLabels:        globalCfg.ExternalLabels,
		tlsRootCA:             tlsRootCA,
		tlsCertificate:        tlsCertificate,
		tlsServerName:         tlsServerName,
		tlsInsecureSkipVerify: tlsInsecureSkipVerify,
		relabelConfigs:        relabelConfigs,
		metricRelabelConfigs:  metricRelabelConfigs,
		scrapeLimit:           scrapeLimit,
	}
	return swc, nil
}

type scrapeWorkConfig struct {
	scrapeInterval        time.Duration
	scrapeTimeout         time.Duration
	jobName               string
	metricsPath           string
	scheme                string
	params                map[string][]string
	authorization         string
	honorLabels           bool
	honorTimestamps       bool
	externalLabels        map[string]string
	tlsRootCA             *x509.CertPool
	tlsCertificate        *tls.Certificate
	tlsServerName         string
	tlsInsecureSkipVerify bool
	relabelConfigs        []promrelabel.ParsedRelabelConfig
	metricRelabelConfigs  []promrelabel.ParsedRelabelConfig
	scrapeLimit           int
	metaLabels            map[string]string
}

func (sdc *FileSDConfig) appendScrapeWork(dst []ScrapeWork, swPrev map[string][]ScrapeWork, baseDir string, swc *scrapeWorkConfig) ([]ScrapeWork, error) {
	for _, file := range sdc.Files {
		pathPattern := getFilepath(baseDir, file)
		paths := []string{pathPattern}
		if strings.Contains(pathPattern, "*") {
			var err error
			paths, err = filepath.Glob(pathPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q in `files` section: %s", file, err)
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
			swcCopy := *swc
			pathShort := path
			if strings.HasPrefix(pathShort, baseDir) {
				pathShort = path[len(baseDir):]
				if len(pathShort) > 0 && pathShort[0] == filepath.Separator {
					pathShort = pathShort[1:]
				}
			}
			swcCopy.metaLabels = map[string]string{
				"__meta_filepath": pathShort,
			}
			for i := range stcs {
				dst, err = stcs[i].appendScrapeWork(dst, &swcCopy)
				if err != nil {
					// Do not return this error, since other paths may contain valid scrape configs.
					logger.Errorf("error when parsing `static_config` #%d from %q: %s", i+1, path, err)
					continue
				}
			}
		}
	}
	return dst, nil
}

func (stc *StaticConfig) appendScrapeWork(dst []ScrapeWork, swc *scrapeWorkConfig) ([]ScrapeWork, error) {
	for _, target := range stc.Targets {
		if target == "" {
			return nil, fmt.Errorf("`static_configs` target for `job_name` %q cannot be empty", swc.jobName)
		}
		labels, err := mergeLabels(swc.jobName, swc.scheme, target, swc.metricsPath, stc.Labels, swc.externalLabels, swc.metaLabels, swc.params)
		if err != nil {
			return nil, fmt.Errorf("cannot merge labels for `static_configs` target for `job_name` %q: %s", swc.jobName, err)
		}
		labels = promrelabel.ApplyRelabelConfigs(labels, 0, swc.relabelConfigs, false)
		if len(labels) == 0 {
			// Drop target without labels.
			continue
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
			continue
		}
		targetRelabeled := addMissingPort(schemeRelabeled, addressLabel.Value)
		if strings.Contains(targetRelabeled, "/") {
			// Drop target with '/'
			continue
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
			return nil, fmt.Errorf("invalid url %q for scheme=%q (%q), target=%q (%q), metrics_path=%q (%q) for `job_name` %q: %s",
				scrapeURL, swc.scheme, schemeRelabeled, target, targetRelabeled, swc.metricsPath, metricsPathRelabeled, swc.jobName, err)
		}
		dst = append(dst, ScrapeWork{
			ScrapeURL:             scrapeURL,
			ScrapeInterval:        swc.scrapeInterval,
			ScrapeTimeout:         swc.scrapeTimeout,
			HonorLabels:           swc.honorLabels,
			HonorTimestamps:       swc.honorTimestamps,
			Labels:                labels,
			Authorization:         swc.authorization,
			TLSRootCA:             swc.tlsRootCA,
			TLSCertificate:        swc.tlsCertificate,
			TLSServerName:         swc.tlsServerName,
			TLSInsecureSkipVerify: swc.tlsInsecureSkipVerify,
			MetricRelabelConfigs:  swc.metricRelabelConfigs,
			ScrapeLimit:           swc.scrapeLimit,
		})
	}
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

func mergeLabels(job, scheme, target, metricsPath string, labels, externalLabels, metaLabels map[string]string, params map[string][]string) ([]prompbmarshal.Label, error) {
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	m := map[string]string{
		"job":              job,
		"__address__":      target,
		"__scheme__":       scheme,
		"__metrics_path__": metricsPath,
	}
	for k, v := range externalLabels {
		if vOrig, ok := m[k]; ok {
			return nil, fmt.Errorf("external label `%q: %q` clashes with the previously set label with value %q", k, v, vOrig)
		}
		m[k] = v
	}
	for k, v := range metaLabels {
		if vOrig, ok := m[k]; ok {
			return nil, fmt.Errorf("meta label `%q: %q` clashes with the previously set label with value %q", k, v, vOrig)
		}
		m[k] = v
	}
	for k, v := range labels {
		if vOrig, ok := m[k]; ok {
			return nil, fmt.Errorf("label `%q: %q` clashes with the previously set label with value %q", k, v, vOrig)
		}
		m[k] = v
	}
	for k, args := range params {
		if len(args) == 0 {
			continue
		}
		k = "__param_" + k
		v := args[0]
		if vOrig, ok := m[k]; ok {
			return nil, fmt.Errorf("param `%q: %q` claches with the previously set label with value %q", k, v, vOrig)
		}
		m[k] = v
	}
	result := make([]prompbmarshal.Label, 0, len(m))
	for k, v := range m {
		result = append(result, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	return result, nil
}

func getFilepath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func readPasswordFromFile(path string) (string, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	pass := strings.TrimRightFunc(string(data), unicode.IsSpace)
	return pass, nil
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
