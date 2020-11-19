package eureka

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func Test_addInstanceLabels(t *testing.T) {
	type args struct {
		applications *applications
		port         int
	}
	tests := []struct {
		name string
		args args
		want [][]prompbmarshal.Label
	}{
		{
			name: "1 application",
			args: args{
				port: 9100,
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
								},
							},
						},
					},
				},
			},
			want: [][]prompbmarshal.Label{
				discoveryutils.GetSortedLabels(map[string]string{
					"__address__":                                "host-1:9100",
					"instance":                                   "some-id",
					"__meta_eureka_app_instance_hostname":        "host-1",
					"__meta_eureka_app_instance_app_nanem":       "test-app",
					"__meta_eureka_app_instance_healthcheck_url": "some-url",
					"__meta_eureka_app_instance_ip_addr":         "10.15.11.11",
					"__meta_eureka_app_instance_vip_address":     "10.15.11.11",
					"__meta_eureka_app_instance_country_id":      "5",
					"__meta_eureka_app_instance_homepage_url":    "some-home-url",
					"__meta_eureka_app_instance_statuspage_url":  "some-status-url",
					"__meta_eureka_app_instance_id":              "some-id",
					"__meta_eureka_app_instance_metadata_key_1":  "value-1",
					"__meta_eureka_app_instance_status":          "Ok",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addInstanceLabels(tt.args.applications, tt.args.port)
			var sortedLabelss [][]prompbmarshal.Label
			for _, labels := range got {
				sortedLabelss = append(sortedLabelss, discoveryutils.GetSortedLabels(labels))
			}
			if !reflect.DeepEqual(sortedLabelss, tt.want) {
				t.Fatalf("unexpected labels \ngot : %v, \nwant: %v", got, tt.want)
			}
		})
	}
}
