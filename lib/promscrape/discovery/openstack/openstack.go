package openstack

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDConfig is the configuration for OpenStack based service discovery.
type SDConfig struct {
	IdentityEndpoint            string              `yaml:"identity_endpoint"`
	Username                    string              `yaml:"username"`
	UserID                      string              `yaml:"userid"`
	Password                    string              `yaml:"password"`
	ProjectName                 string              `yaml:"project_name"`
	ProjectID                   string              `yaml:"project_id"`
	DomainName                  string              `yaml:"domain_name"`
	DomainID                    string              `yaml:"domain_id"`
	ApplicationCredentialName   string              `yaml:"application_credential_name"`
	ApplicationCredentialID     string              `yaml:"application_credential_id"`
	ApplicationCredentialSecret string              `yaml:"application_credential_secret"`
	Role                        string              `yaml:"role"`
	Region                      string              `yaml:"region"`
	Port                        int                 `yaml:"port"`
	AllTenants                  bool                `yaml:"all_tenants"`
	TLSConfig                   *promauth.TLSConfig `yaml:"tls_config"`
	Availability                string              `yaml:"availability"`
}

// GetLabels returns gce labels according to sdc.
func GetLabels(sdc *SDConfig, baseDir string) ([]map[string]string, error) {
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
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `instance` or `hypervisor`; skipping it", sdc.Role)
	}
}
