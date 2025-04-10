package yandexcloud

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAddInstanceLabels(t *testing.T) {
	f := func(instances []instance, labelssExpected []*promutil.Labels) {
		t.Helper()

		labelss := addInstanceLabels(instances)
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// empty response
	f(nil, nil)

	// one server
	instances := []instance{
		{
			Name:       "server-1",
			ID:         "test",
			FQDN:       "server-1.ru-central1.internal",
			FolderID:   "test",
			Status:     "RUNNING",
			PlatformID: "s2.micro",
			Resources: resources{
				Cores:        "2",
				CoreFraction: "20",
				Memory:       "4",
			},
			NetworkInterfaces: []networkInterface{
				{
					Index: "0",
					PrimaryV4Address: primaryV4Address{
						Address: "192.168.1.1",
					},
				},
			},
		},
	}
	labelssExpected := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                         "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_name":                    "server-1",
			"__meta_yandexcloud_instance_fqdn":                    "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_id":                      "test",
			"__meta_yandexcloud_instance_status":                  "RUNNING",
			"__meta_yandexcloud_instance_platform_id":             "s2.micro",
			"__meta_yandexcloud_instance_resources_cores":         "2",
			"__meta_yandexcloud_instance_resources_core_fraction": "20",
			"__meta_yandexcloud_instance_resources_memory":        "4",
			"__meta_yandexcloud_folder_id":                        "test",
			"__meta_yandexcloud_instance_private_ip_0":            "192.168.1.1",
		}),
	}
	f(instances, labelssExpected)

	// with public ip
	instances = []instance{
		{
			Name:       "server-1",
			ID:         "test",
			FQDN:       "server-1.ru-central1.internal",
			FolderID:   "test",
			Status:     "RUNNING",
			PlatformID: "s2.micro",
			Resources: resources{
				Cores:        "2",
				CoreFraction: "20",
				Memory:       "4",
			},
			NetworkInterfaces: []networkInterface{
				{
					Index: "0",
					PrimaryV4Address: primaryV4Address{
						Address: "192.168.1.1",
						OneToOneNat: oneToOneNat{
							Address: "1.1.1.1",
						},
					},
				},
			},
		},
	}
	labelssExpected = []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                         "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_fqdn":                    "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_name":                    "server-1",
			"__meta_yandexcloud_instance_id":                      "test",
			"__meta_yandexcloud_instance_status":                  "RUNNING",
			"__meta_yandexcloud_instance_platform_id":             "s2.micro",
			"__meta_yandexcloud_instance_resources_cores":         "2",
			"__meta_yandexcloud_instance_resources_core_fraction": "20",
			"__meta_yandexcloud_instance_resources_memory":        "4",
			"__meta_yandexcloud_folder_id":                        "test",
			"__meta_yandexcloud_instance_private_ip_0":            "192.168.1.1",
			"__meta_yandexcloud_instance_public_ip_0":             "1.1.1.1",
		}),
	}
	f(instances, labelssExpected)

	// with dns record
	instances = []instance{
		{
			Name:       "server-1",
			ID:         "test",
			FQDN:       "server-1.ru-central1.internal",
			FolderID:   "test",
			Status:     "RUNNING",
			PlatformID: "s2.micro",
			Resources: resources{
				Cores:        "2",
				CoreFraction: "20",
				Memory:       "4",
			},
			NetworkInterfaces: []networkInterface{
				{
					Index: "0",
					PrimaryV4Address: primaryV4Address{
						Address: "192.168.1.1",
						OneToOneNat: oneToOneNat{
							Address: "1.1.1.1",
							DNSRecords: []dnsRecord{
								{
									FQDN: "server-1.example.com",
								},
							},
						},
						DNSRecords: []dnsRecord{
							{
								FQDN: "server-1.example.local",
							},
						},
					},
				},
			},
		},
	}
	labelssExpected = []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                         "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_name":                    "server-1",
			"__meta_yandexcloud_instance_fqdn":                    "server-1.ru-central1.internal",
			"__meta_yandexcloud_instance_id":                      "test",
			"__meta_yandexcloud_instance_status":                  "RUNNING",
			"__meta_yandexcloud_instance_platform_id":             "s2.micro",
			"__meta_yandexcloud_instance_resources_cores":         "2",
			"__meta_yandexcloud_instance_resources_core_fraction": "20",
			"__meta_yandexcloud_instance_resources_memory":        "4",
			"__meta_yandexcloud_folder_id":                        "test",
			"__meta_yandexcloud_instance_private_ip_0":            "192.168.1.1",
			"__meta_yandexcloud_instance_public_ip_0":             "1.1.1.1",
			"__meta_yandexcloud_instance_private_dns_0":           "server-1.example.local",
			"__meta_yandexcloud_instance_public_dns_0":            "server-1.example.com",
		}),
	}
	f(instances, labelssExpected)
}
