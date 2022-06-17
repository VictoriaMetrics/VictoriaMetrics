package azure

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval is check interval for Consul service discovery.
var SDCheckInterval = flag.Duration("promscrape.azureSDCheckInterval", 60*time.Second, "Interval for checking for changes in Azure. "+
	"This works only if consul_sd_configs is configured in '-promscrape.config' file. "+
	"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#azure_sd_config for details")

// SDConfig represents service discovery config for Consul.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#azure_sd_config
type SDConfig struct {
	Environment          string          `yaml:"environment,omitempty"`
	Port                 int             `yaml:"port"`
	SubscriptionID       string          `yaml:"subscription_id"`
	TenantID             string          `yaml:"tenant_id,omitempty"`
	ClientID             string          `yaml:"client_id,omitempty"`
	ClientSecret         promauth.Secret `yaml:"client_secret,omitempty"`
	AuthenticationMethod string          `yaml:"authentication_method,omitempty"`
	ResourceGroup        string          `yaml:"resource_group,omitempty"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`

	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.consulSDCheckInterval` command-line option.
}

// UnmarshalYAML implements interface.
// checks required params and set default values.
func (sdc *SDConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type sdConfig SDConfig
	if err := unmarshal((*sdConfig)(sdc)); err != nil {
		return err
	}
	if sdc.Port == 0 {
		sdc.Port = 80
	}
	if len(sdc.Environment) == 0 {
		sdc.Environment = "AZURECLOUD"
	}
	if len(sdc.AuthenticationMethod) == 0 {
		sdc.AuthenticationMethod = "OAuth"
	}
	checkRequired := func(name, value string) error {
		if len(value) == 0 {
			return fmt.Errorf("required param for azure sd: %q is not set", name)
		}
		return nil
	}
	if err := checkRequired("subscription_id", sdc.SubscriptionID); err != nil {
		return err
	}
	switch sdc.AuthenticationMethod {
	case "", "OAuth":
		if err := checkRequired("client_id", sdc.ClientID); err != nil {
			return err
		}
		if err := checkRequired("client_secret", sdc.ClientSecret.S); err != nil {
			return err
		}
		if err := checkRequired("tenant_id", sdc.TenantID); err != nil {
			return err
		}
	case "ManagedIdentity":
	default:
		return fmt.Errorf("unsupported `authentication_method`: %q for azure_sd_configs, only `OAuth` and `ManagedIdentity` supported by vmagent", sdc.AuthenticationMethod)
	}
	return nil
}

// GetLabels returns Consul labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	ac, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	vms, err := getVirtualMachines(ac)
	if err != nil {
		return nil, err
	}
	return appendMachineLabels(vms, sdc), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	configMap.Delete(sdc)
}

func appendMachineLabels(vms []virtualMachine, sdc *SDConfig) []map[string]string {
	ms := make([]map[string]string, 0, len(vms))
	for i := range vms {
		vm := &vms[i]
		for _, ips := range vm.ipAddresses {
			if len(ips.privateIP) == 0 {
				continue
			}
			addr := discoveryutils.JoinHostPort(ips.privateIP, sdc.Port)
			m := map[string]string{
				"__address__":                     addr,
				"__meta_azure_subscription_id":    sdc.SubscriptionID,
				"__meta_azure_machine_id":         vm.ID,
				"__meta_azure_machine_name":       vm.Name,
				"__meta_azure_machine_location":   vm.Location,
				"__meta_azure_machine_private_ip": ips.privateIP,
			}
			if len(sdc.TenantID) > 0 {
				m["__meta_azure_tenant_id"] = sdc.TenantID
			}
			// /subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP/providers/PROVIDER/TYPE/NAME
			idPath := strings.Split(vm.ID, "/")
			if len(idPath) > 4 {
				m["__meta_azure_machine_resource_group"] = idPath[4]
			}
			if len(vm.Properties.StorageProfile.OsDisk.OsType) > 0 {
				m["__meta_azure_machine_os_type"] = vm.Properties.StorageProfile.OsDisk.OsType
			}
			if len(vm.Properties.OsProfile.ComputerName) > 0 {
				m["__meta_azure_machine_computer_name"] = vm.Properties.OsProfile.ComputerName
			}

			if len(ips.publicIP) > 0 {
				m["__meta_azure_machine_public_ip"] = ips.publicIP
			}
			if len(vm.scaleSet) > 0 {
				m["__meta_azure_machine_scale_set"] = vm.scaleSet
			}
			for k, v := range vm.Tags {
				m[discoveryutils.SanitizeLabelName("__meta_azure_machine_tag_"+k)] = v
			}

			ms = append(ms, m)
		}
	}

	return ms
}
