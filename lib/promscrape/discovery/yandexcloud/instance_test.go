package yandexcloud

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_addInstanceLabels(t *testing.T) {
	type args struct {
		instances []instance
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "empty_response",
			args: args{},
		},
		{
			name: "one_server",
			args: args{
				instances: []instance{
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
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
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
			},
		},
		{
			name: "with_public_ip",
			args: args{
				instances: []instance{
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
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
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
			},
		},
		{
			name: "with_dns_record",
			args: args{
				instances: []instance{
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
											{FQDN: "server-1.example.com"},
										},
									},
									DNSRecords: []dnsRecord{
										{FQDN: "server-1.example.local"},
									},
								},
							},
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addInstanceLabels(tt.args.instances)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
