package eureka

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_addInstanceLabels(t *testing.T) {
	type args struct {
		applications *applications
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "1 application",
			args: args{
				applications: &applications{
					Applications: []Application{
						{
							Name: "test-app",
							Instances: []Instance{
								{
									Status:         "Ok",
									HealthCheckURL: "some-url",
									HomePageURL:    "some-home-url",
									StatusPageURL:  "some-status-url",
									HostName:       "host-1",
									IPAddr:         "10.15.11.11",
									CountryID:      5,
									VipAddress:     "10.15.11.11",
									InstanceID:     "some-id",
									Metadata: MetaData{Items: []Tag{
										{
											Content: "value-1",
											XMLName: struct{ Space, Local string }{Local: "key-1"},
										},
									}},
									Port: Port{
										Port: 9100,
									},
								},
							},
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                                   "host-1:9100",
					"instance":                                      "some-id",
					"__meta_eureka_app_instance_hostname":           "host-1",
					"__meta_eureka_app_name":                        "test-app",
					"__meta_eureka_app_instance_healthcheck_url":    "some-url",
					"__meta_eureka_app_instance_ip_addr":            "10.15.11.11",
					"__meta_eureka_app_instance_vip_address":        "10.15.11.11",
					"__meta_eureka_app_instance_secure_vip_address": "",
					"__meta_eureka_app_instance_country_id":         "5",
					"__meta_eureka_app_instance_homepage_url":       "some-home-url",
					"__meta_eureka_app_instance_statuspage_url":     "some-status-url",
					"__meta_eureka_app_instance_id":                 "some-id",
					"__meta_eureka_app_instance_metadata_key_1":     "value-1",
					"__meta_eureka_app_instance_port":               "9100",
					"__meta_eureka_app_instance_port_enabled":       "false",
					"__meta_eureka_app_instance_status":             "Ok",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addInstanceLabels(tt.args.applications)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
