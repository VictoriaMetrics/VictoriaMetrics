package openstack

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_addInstanceLabels(t *testing.T) {
	type args struct {
		servers []server
		port    int
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "empty_response",
			args: args{
				port: 9100,
			},
		},
		{
			name: "one_server",
			args: args{
				port: 9100,
				servers: []server{
					{
						ID:       "10",
						Status:   "enabled",
						Name:     "server-1",
						HostID:   "some-host-id",
						TenantID: "some-tenant-id",
						UserID:   "some-user-id",
						Flavor: struct {
							ID string `json:"id"`
						}{ID: "5"},
						Addresses: map[string][]struct {
							Address string `json:"addr"`
							Version int    `json:"version"`
							Type    string `json:"OS-EXT-IPS:type"`
						}{
							"test": {
								{
									Address: "192.168.0.1",
									Version: 4,
									Type:    "fixed",
								},
							},
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                      "192.168.0.1:9100",
					"__meta_openstack_address_pool":    "test",
					"__meta_openstack_instance_flavor": "5",
					"__meta_openstack_instance_id":     "10",
					"__meta_openstack_instance_name":   "server-1",
					"__meta_openstack_instance_status": "enabled",
					"__meta_openstack_private_ip":      "192.168.0.1",
					"__meta_openstack_project_id":      "some-tenant-id",
					"__meta_openstack_user_id":         "some-user-id",
				}),
			},
		},
		{
			name: "with_public_ip",
			args: args{
				port: 9100,
				servers: []server{
					{
						ID:       "10",
						Status:   "enabled",
						Name:     "server-2",
						HostID:   "some-host-id",
						TenantID: "some-tenant-id",
						UserID:   "some-user-id",
						Flavor: struct {
							ID string `json:"id"`
						}{ID: "5"},
						Addresses: map[string][]struct {
							Address string `json:"addr"`
							Version int    `json:"version"`
							Type    string `json:"OS-EXT-IPS:type"`
						}{
							"test": {
								{
									Address: "192.168.0.1",
									Version: 4,
									Type:    "fixed",
								},
								{
									Address: "1.5.5.5",
									Version: 4,
									Type:    "floating",
								},
							},
							"internal": {
								{
									Address: "10.10.0.1",
									Version: 4,
									Type:    "fixed",
								},
							},
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                      "10.10.0.1:9100",
					"__meta_openstack_address_pool":    "internal",
					"__meta_openstack_instance_flavor": "5",
					"__meta_openstack_instance_id":     "10",
					"__meta_openstack_instance_name":   "server-2",
					"__meta_openstack_instance_status": "enabled",
					"__meta_openstack_private_ip":      "10.10.0.1",
					"__meta_openstack_project_id":      "some-tenant-id",
					"__meta_openstack_user_id":         "some-user-id",
				}),
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                      "192.168.0.1:9100",
					"__meta_openstack_address_pool":    "test",
					"__meta_openstack_instance_flavor": "5",
					"__meta_openstack_instance_id":     "10",
					"__meta_openstack_instance_name":   "server-2",
					"__meta_openstack_instance_status": "enabled",
					"__meta_openstack_private_ip":      "192.168.0.1",
					"__meta_openstack_public_ip":       "1.5.5.5",
					"__meta_openstack_project_id":      "some-tenant-id",
					"__meta_openstack_user_id":         "some-user-id",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addInstanceLabels(tt.args.servers, tt.args.port)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}

func Test_parseServersDetail(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    serversDetail
		wantErr bool
	}{
		{
			name: "parse ok",
			args: args{
				data: []byte(`{
   "servers":[
      {
         "id":"c9f68076-01a3-489a-aebe-8b773c71e7f3",
         "name":"test10",
         "status":"ACTIVE",
         "tenant_id":"d34be4e44f9c444eab9a5ec7b953951f",
         "user_id":"e55737f142ac42f18093037760656bd7",
         "metadata":{
            
         },
         "hostId":"e26db8db23736877aa92ebbbe11743b2a2a3b107aada00a8a0cf474b",
         "image":{
            "id":"253f7a69-dc79-4fb2-86f8-9ec92c94107a",
            "links":[
               {
                  "rel":"bookmark",
                  "href":"http://10.20.20.1:8774/images/253f7a69-dc79-4fb2-86f8-9ec92c94107a"
               }
            ]
         },
         "flavor":{
            "id":"1"
         },
         "addresses":{
            "test":[
               {
                  "version":4,
                  "addr":"192.168.222.15",
                  "OS-EXT-IPS:type":"fixed",
                  "OS-EXT-IPS-MAC:mac_addr":"fa:16:3e:b0:40:af"
               },
               {
                  "version":4,
                  "addr":"10.20.20.69",
                  "OS-EXT-IPS:type":"floating",
                  "OS-EXT-IPS-MAC:mac_addr":"fa:16:3e:b0:40:af"
               }
            ]
         },
         "accessIPv4":"",
         "accessIPv6":"",
         "key_name":"microstack",
         "security_groups":[
            {
               "name":"default"
            }
         ]
      }
   ]
}`),
			},
			want: serversDetail{
				Servers: []server{
					{
						Flavor: struct {
							ID string `json:"id"`
						}{ID: "1"},
						ID:       "c9f68076-01a3-489a-aebe-8b773c71e7f3",
						TenantID: "d34be4e44f9c444eab9a5ec7b953951f",
						UserID:   "e55737f142ac42f18093037760656bd7",
						Name:     "test10",
						HostID:   "e26db8db23736877aa92ebbbe11743b2a2a3b107aada00a8a0cf474b",
						Status:   "ACTIVE",
						Metadata: map[string]string{},
						Addresses: map[string][]struct {
							Address string `json:"addr"`
							Version int    `json:"version"`
							Type    string `json:"OS-EXT-IPS:type"`
						}{
							"test": {
								{
									Address: "192.168.222.15",
									Version: 4,
									Type:    "fixed",
								},
								{
									Address: "10.20.20.69",
									Version: 4,
									Type:    "floating",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServersDetail(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseServersDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("parseServersDetail() \ngot = %v,\nwant= %v", *got, tt.want)
			}
		})
	}
}
