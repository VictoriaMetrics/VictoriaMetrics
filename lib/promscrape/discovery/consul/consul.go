package consul

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDConfig represents service discovery config for Consul.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config
type SDConfig struct {
	Server     string           `yaml:"server,omitempty"`
	Token      *promauth.Secret `yaml:"token"`
	Datacenter string           `yaml:"datacenter"`

	// Namespace only supported at enterprise consul.
	// https://www.consul.io/docs/enterprise/namespaces
	Namespace string `yaml:"namespace,omitempty"`
	// Partition only supported at enterprise consul.
	// https://developer.hashicorp.com/consul/docs/enterprise/admin-partitions
	Partition string `yaml:"partition,omitempty"`

	Scheme            string                     `yaml:"scheme,omitempty"`
	Username          string                     `yaml:"username"`
	Password          *promauth.Secret           `yaml:"password"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	Services          []string                   `yaml:"services,omitempty"`
	Tags              []string                   `yaml:"tags,omitempty"`
	NodeMeta          map[string]string          `yaml:"node_meta,omitempty"`
	TagSeparator      *string                    `yaml:"tag_separator,omitempty"`
	AllowStale        *bool                      `yaml:"allow_stale,omitempty"`
	// See https://developer.hashicorp.com/consul/api-docs/features/filtering
	// list of supported filters https://developer.hashicorp.com/consul/api-docs/catalog#filtering-1
	Filter string `yaml:"filter,omitempty"`

	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.consulSDCheckInterval` command-line option.
}

// GetLabels returns Consul labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ms := getServiceNodesLabels(cfg)
	return ms, nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		// v can be nil if GetLabels wasn't called yet.
		cfg := v.(*apiConfig)
		cfg.mustStop()
	}
}
