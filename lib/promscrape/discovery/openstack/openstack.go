package openstack

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.openstackSDCheckInterval", 30*time.Second, "Interval for checking for changes in openstack API server. "+
	"This works only if openstack_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#openstack_sd_configs for details")

// SDConfig is the configuration for OpenStack based service discovery.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config
type SDConfig struct {
	IdentityEndpoint            string           `yaml:"identity_endpoint,omitempty"`
	Username                    string           `yaml:"username,omitempty"`
	UserID                      string           `yaml:"userid,omitempty"`
	Password                    *promauth.Secret `yaml:"password,omitempty"`
	ProjectName                 string           `yaml:"project_name,omitempty"`
	ProjectID                   string           `yaml:"project_id,omitempty"`
	DomainName                  string           `yaml:"domain_name,omitempty"`
	DomainID                    string           `yaml:"domain_id,omitempty"`
	ApplicationCredentialName   string           `yaml:"application_credential_name,omitempty"`
	ApplicationCredentialID     string           `yaml:"application_credential_id,omitempty"`
	ApplicationCredentialSecret *promauth.Secret `yaml:"application_credential_secret,omitempty"`
	Role                        string           `yaml:"role"`
	Region                      string           `yaml:"region"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.openstackSDCheckInterval` command-line option.
	Port         int                 `yaml:"port,omitempty"`
	AllTenants   bool                `yaml:"all_tenants,omitempty"`
	TLSConfig    *promauth.TLSConfig `yaml:"tls_config,omitempty"`
	Availability string              `yaml:"availability,omitempty"`
}

// GetLabels returns OpenStack labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Role {
	case "hypervisor":
		return getHypervisorLabels(cfg)
	case "instance":
		return getInstancesLabels(cfg)
	default:
		return nil, fmt.Errorf("skipping unexpected role=%q; must be one of `instance` or `hypervisor`", sdc.Role)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.CloseIdleConnections()
	}
}
