package azure

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval is check interval for Azure service discovery.
var SDCheckInterval = flag.Duration("promscrape.azureSDCheckInterval", 60*time.Second, "Interval for checking for changes in Azure. "+
	"This works only if azure_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#azure_sd_configs for details")

// SDConfig represents service discovery config for Azure.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#azure_sd_config
type SDConfig struct {
	Environment string `yaml:"environment,omitempty"`

	// AuthenticationMethod can be either Oauth or ManagedIdentity.
	// See https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview
	AuthenticationMethod string `yaml:"authentication_method,omitempty"`

	SubscriptionID string           `yaml:"subscription_id"`
	TenantID       string           `yaml:"tenant_id,omitempty"`
	ClientID       string           `yaml:"client_id,omitempty"`
	ClientSecret   *promauth.Secret `yaml:"client_secret,omitempty"`
	ResourceGroup  string           `yaml:"resource_group,omitempty"`

	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.azureSDCheckInterval` command-line option.

	Port int `yaml:"port"`

	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

// GetLabels returns Azure labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	ac, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	vms, err := getVirtualMachines(ac)
	if err != nil {
		return nil, err
	}
	return appendMachineLabels(vms, ac.port, sdc), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.c.Stop()
	}
}

func appendMachineLabels(vms []virtualMachine, port int, sdc *SDConfig) []*promutil.Labels {
	ms := make([]*promutil.Labels, 0, len(vms))
	for i := range vms {
		vm := &vms[i]
		for _, ips := range vm.ipAddresses {
			if ips.privateIP == "" {
				continue
			}
			addr := discoveryutil.JoinHostPort(ips.privateIP, port)
			m := promutil.NewLabels(16)
			m.Add("__address__", addr)
			m.Add("__meta_azure_subscription_id", sdc.SubscriptionID)
			m.Add("__meta_azure_machine_id", vm.ID)
			m.Add("__meta_azure_machine_name", vm.Name)
			m.Add("__meta_azure_machine_location", vm.Location)
			m.Add("__meta_azure_machine_private_ip", ips.privateIP)
			if sdc.TenantID != "" {
				m.Add("__meta_azure_tenant_id", sdc.TenantID)
			}
			// /subscriptions/SUBSCRIPTION_ID/resourceGroups/RESOURCE_GROUP/providers/PROVIDER/TYPE/NAME
			idPath := strings.Split(vm.ID, "/")
			if len(idPath) > 4 {
				m.Add("__meta_azure_machine_resource_group", idPath[4])
			}
			if vm.Properties.StorageProfile.OsDisk.OsType != "" {
				m.Add("__meta_azure_machine_os_type", vm.Properties.StorageProfile.OsDisk.OsType)
			}
			if vm.Properties.OsProfile.ComputerName != "" {
				m.Add("__meta_azure_machine_computer_name", vm.Properties.OsProfile.ComputerName)
			}
			if ips.publicIP != "" {
				m.Add("__meta_azure_machine_public_ip", ips.publicIP)
			}
			if vm.scaleSet != "" {
				m.Add("__meta_azure_machine_scale_set", vm.scaleSet)
			}
			if vm.Properties.HardwareProfile.VMSize != "" {
				m.Add("__meta_azure_machine_size", vm.Properties.HardwareProfile.VMSize)
			}
			for k, v := range vm.Tags {
				m.Add(discoveryutil.SanitizeLabelName("__meta_azure_machine_tag_"+k), v)
			}
			ms = append(ms, m)
		}
	}
	return ms
}
