package yandexcloud

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func getInstancesLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
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

func addInstanceLabels(instances []instance) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, server := range instances {
		m := promutil.NewLabels(24)
		m.Add("__address__", server.FQDN)
		m.Add("__meta_yandexcloud_instance_name", server.Name)
		m.Add("__meta_yandexcloud_instance_fqdn", server.FQDN)
		m.Add("__meta_yandexcloud_instance_id", server.ID)
		m.Add("__meta_yandexcloud_instance_status", server.Status)
		m.Add("__meta_yandexcloud_instance_platform_id", server.PlatformID)
		m.Add("__meta_yandexcloud_instance_resources_cores", server.Resources.Cores)
		m.Add("__meta_yandexcloud_instance_resources_core_fraction", server.Resources.CoreFraction)
		m.Add("__meta_yandexcloud_instance_resources_memory", server.Resources.Memory)
		m.Add("__meta_yandexcloud_folder_id", server.FolderID)
		for k, v := range server.Labels {
			m.Add(discoveryutil.SanitizeLabelName("__meta_yandexcloud_instance_label_"+k), v)
		}
		for _, ni := range server.NetworkInterfaces {
			privateIPLabel := fmt.Sprintf("__meta_yandexcloud_instance_private_ip_%s", ni.Index)
			m.Add(privateIPLabel, ni.PrimaryV4Address.Address)
			if len(ni.PrimaryV4Address.OneToOneNat.Address) > 0 {
				publicIPLabel := fmt.Sprintf("__meta_yandexcloud_instance_public_ip_%s", ni.Index)
				m.Add(publicIPLabel, ni.PrimaryV4Address.OneToOneNat.Address)
			}
			for j, dnsRecord := range ni.PrimaryV4Address.DNSRecords {
				dnsRecordLabel := fmt.Sprintf("__meta_yandexcloud_instance_private_dns_%d", j)
				m.Add(dnsRecordLabel, dnsRecord.FQDN)
			}
			for j, dnsRecord := range ni.PrimaryV4Address.OneToOneNat.DNSRecords {
				dnsRecordLabel := fmt.Sprintf("__meta_yandexcloud_instance_public_dns_%d", j)
				m.Add(dnsRecordLabel, dnsRecord.FQDN)
			}
		}
		ms = append(ms, m)
	}
	return ms
}
