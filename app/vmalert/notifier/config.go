package notifier

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dns"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// Config contains list of supported configuration settings
// for Notifier
type Config struct {
	// Scheme defines the HTTP scheme for Notifier address
	Scheme string `yaml:"scheme,omitempty"`
	// PathPrefix is added to URL path before adding alertManagerPath value
	PathPrefix string `yaml:"path_prefix,omitempty"`

	// ConsulSDConfigs contains list of settings for service discovery via Consul
	// see https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config
	ConsulSDConfigs []consul.SDConfig `yaml:"consul_sd_configs,omitempty"`
	// DNSSDConfigs contains list of settings for service discovery via DNS.
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config
	DNSSDConfigs []dns.SDConfig `yaml:"dns_sd_configs,omitempty"`

	// StaticConfigs contains list of static targets
	StaticConfigs []StaticConfig `yaml:"static_configs,omitempty"`

	// HTTPClientConfig contains HTTP configuration for Notifier clients
	HTTPClientConfig promauth.HTTPClientConfig `yaml:",inline"`
	// RelabelConfigs contains list of relabeling rules for entities discovered via SD
	RelabelConfigs []promrelabel.RelabelConfig `yaml:"relabel_configs,omitempty"`
	// AlertRelabelConfigs contains list of relabeling rules alert labels
	AlertRelabelConfigs []promrelabel.RelabelConfig `yaml:"alert_relabel_configs,omitempty"`
	// The timeout used when sending alerts.
	Timeout *promutils.Duration `yaml:"timeout,omitempty"`

	// Checksum stores the hash of yaml definition for the config.
	// May be used to detect any changes to the config file.
	Checksum string

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline"`

	// This is set to the directory from where the config has been loaded.
	baseDir string

	// stores already parsed RelabelConfigs object
	parsedRelabelConfigs *promrelabel.ParsedConfigs
	// stores already parsed AlertRelabelConfigs object
	parsedAlertRelabelConfigs *promrelabel.ParsedConfigs
}

// StaticConfig contains list of static targets in the following form:
//
//	targets:
//	[ - '<host>' ]
type StaticConfig struct {
	Targets []string `yaml:"targets"`
	// HTTPClientConfig contains HTTP configuration for the Targets
	HTTPClientConfig promauth.HTTPClientConfig `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (cfg *Config) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type config Config
	if err := unmarshal((*config)(cfg)); err != nil {
		return err
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}
	if cfg.Timeout.Duration() == 0 {
		cfg.Timeout = promutils.NewDuration(time.Second * 10)
	}
	rCfg, err := promrelabel.ParseRelabelConfigs(cfg.RelabelConfigs)
	if err != nil {
		return fmt.Errorf("failed to parse relabeling config: %w", err)
	}
	cfg.parsedRelabelConfigs = rCfg
	arCfg, err := promrelabel.ParseRelabelConfigs(cfg.AlertRelabelConfigs)
	if err != nil {
		return fmt.Errorf("failed to parse alert relabeling config: %w", err)
	}
	cfg.parsedAlertRelabelConfigs = arCfg

	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration for checksum: %w", err)
	}
	h := md5.New()
	h.Write(b)
	cfg.Checksum = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

func parseConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}
	var cfg *Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	if len(cfg.XXX) > 0 {
		var keys []string
		for k := range cfg.XXX {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("unknown fields in %s", strings.Join(keys, ", "))
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain abs path for %q: %w", path, err)
	}
	cfg.baseDir = filepath.Dir(absPath)
	return cfg, nil
}

func parseLabels(target string, metaLabels *promutils.Labels, cfg *Config) (string, *promutils.Labels, error) {
	labels := mergeLabels(target, metaLabels, cfg)
	labels.Labels = cfg.parsedRelabelConfigs.Apply(labels.Labels, 0)
	labels.RemoveMetaLabels()
	labels.Sort()
	// Remove references to already deleted labels, so GC could clean strings for label name and label value past len(labels).
	// This should reduce memory usage when relabeling creates big number of temporary labels with long names and/or values.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/825 for details.
	labels = labels.Clone()

	if labels.Len() == 0 {
		return "", nil, nil
	}
	scheme := labels.Get("__scheme__")
	if len(scheme) == 0 {
		scheme = "http"
	}
	alertsPath := labels.Get("__alerts_path__")
	if !strings.HasPrefix(alertsPath, "/") {
		alertsPath = "/" + alertsPath
	}
	address := labels.Get("__address__")
	if len(address) == 0 {
		return "", nil, nil
	}
	address = addMissingPort(scheme, address)
	u := fmt.Sprintf("%s://%s%s", scheme, address, alertsPath)
	if _, err := url.Parse(u); err != nil {
		return "", nil, fmt.Errorf("invalid url %q for scheme=%q (%q), target=%q, metrics_path=%q (%q): %w",
			u, cfg.Scheme, scheme, target, address, alertsPath, err)
	}
	return u, labels, nil
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

func mergeLabels(target string, metaLabels *promutils.Labels, cfg *Config) *promutils.Labels {
	// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
	m := promutils.NewLabels(3 + metaLabels.Len())
	address := target
	scheme := cfg.Scheme
	alertsPath := path.Join("/", cfg.PathPrefix, alertManagerPath)
	// try to extract optional scheme and alertsPath from __address__.
	if strings.HasPrefix(address, "http://") {
		scheme = "http"
		address = address[len("http://"):]
	} else if strings.HasPrefix(address, "https://") {
		scheme = "https"
		address = address[len("https://"):]
	}
	if n := strings.IndexByte(address, '/'); n >= 0 {
		alertsPath = address[n:]
		address = address[:n]
	}
	m.Add("__address__", address)
	m.Add("__scheme__", scheme)
	m.Add("__alerts_path__", alertsPath)
	m.AddFrom(metaLabels)
	return m
}
