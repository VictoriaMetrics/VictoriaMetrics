package yandexcloud

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func getInstancesLabels(cfg *apiConfig) ([]map[string]string, error) {
	organizations, err := cfg.getOrganizations()
	if err != nil {
		return nil, err
	}
	clouds, err := cfg.getClouds(organizations)
	if err != nil {
		return nil, err
	}
	folders, err := cfg.getFolders(clouds)
	if err != nil {
		return nil, err
	}

	var instances []instance
	for _, fld := range folders {
		inst, err := cfg.getInstances(fld.ID)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst...)
	}

	logger.Infof("yandexcloud_sd: collected %d instances", len(instances))

	return addInstanceLabels(instances), nil
}

func addInstanceLabels(instances []instance) []map[string]string {
	var ms []map[string]string
	for _, server := range instances {
		m := map[string]string{
			"__address__":                                         server.FQDN,
			"__meta_yandexcloud_instance_name":                    server.Name,
			"__meta_yandexcloud_instance_fqdn":                    server.FQDN,
			"__meta_yandexcloud_instance_id":                      server.ID,
			"__meta_yandexcloud_instance_status":                  server.Status,
			"__meta_yandexcloud_instance_platform_id":             server.PlatformID,
			"__meta_yandexcloud_instance_resources_cores":         server.Resources.Cores,
			"__meta_yandexcloud_instance_resources_core_fraction": server.Resources.CoreFraction,
			"__meta_yandexcloud_instance_resources_memory":        server.Resources.Memory,
			"__meta_yandexcloud_folder_id":                        server.FolderID,
		}
		for k, v := range server.Labels {
			m[discoveryutils.SanitizeLabelName("__meta_yandexcloud_instance_label_"+k)] = v
		}

		for _, ni := range server.NetworkInterfaces {
			privateIPLabel := fmt.Sprintf("__meta_yandexcloud_instance_private_ip_%s", ni.Index)
			m[privateIPLabel] = ni.PrimaryV4Address.Address
			if len(ni.PrimaryV4Address.OneToOneNat.Address) > 0 {
				publicIPLabel := fmt.Sprintf("__meta_yandexcloud_instance_public_ip_%s", ni.Index)
				m[publicIPLabel] = ni.PrimaryV4Address.OneToOneNat.Address
			}

			for j, dnsRecord := range ni.PrimaryV4Address.DNSRecords {
				dnsRecordLabel := fmt.Sprintf("__meta_yandexcloud_instance_private_dns_%d", j)
				m[dnsRecordLabel] = dnsRecord.FQDN
			}

			for j, dnsRecord := range ni.PrimaryV4Address.OneToOneNat.DNSRecords {
				dnsRecordLabel := fmt.Sprintf("__meta_yandexcloud_instance_public_dns_%d", j)
				m[dnsRecordLabel] = dnsRecord.FQDN
			}
		}

		ms = append(ms, m)
	}

	return ms
}
