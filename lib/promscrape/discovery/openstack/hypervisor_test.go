package openstack

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_parseHypervisorDetail(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    hypervisorDetail
		wantErr bool
	}{
		{
			name: "bad data",
			args: args{
				data: []byte(`{ff}`),
			},
			wantErr: true,
		},
		{
			name: "1 hypervisor",
			args: args{
				data: []byte(`{
    "hypervisors": [
        {
            "cpu_info": {
                "arch": "x86_64",
                "model": "Nehalem",
                "vendor": "Intel",
                "features": [
                    "pge",
                    "clflush"
                ],
                "topology": {
                    "cores": 1,
                    "threads": 1,
                    "sockets": 4
                }
            },
            "current_workload": 0,
            "status": "enabled",
            "state": "up",
            "disk_available_least": 0,
            "host_ip": "1.1.1.1",
            "free_disk_gb": 1028,
            "free_ram_mb": 7680,
            "hypervisor_hostname": "host1",
            "hypervisor_type": "fake",
            "hypervisor_version": 1000,
            "id": 2,
            "local_gb": 1028,
            "local_gb_used": 0,
            "memory_mb": 8192,
            "memory_mb_used": 512,
            "running_vms": 0,
            "service": {
                "host": "host1",
                "id": 6,
                "disabled_reason": null
            },
            "vcpus": 2,
            "vcpus_used": 0
        }
    ]}`),
			},
			want: hypervisorDetail{
				Hypervisors: []hypervisor{
					{
						HostIP:   "1.1.1.1",
						ID:       2,
						Hostname: "host1",
						Status:   "enabled",
						State:    "up",
						Type:     "fake",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHypervisorDetail(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseHypervisorDetail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("parseHypervisorDetail() got = %v, want %v", *got, tt.want)
			}
		})
	}
}

func Test_addHypervisorLabels(t *testing.T) {
	type args struct {
		hvs  []hypervisor
		port int
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "",
			args: args{
				port: 9100,
				hvs: []hypervisor{
					{
						Type:     "fake",
						ID:       5,
						State:    "enabled",
						Status:   "up",
						Hostname: "fakehost",
						HostIP:   "1.2.2.2",
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__address__":                          "1.2.2.2:9100",
					"__meta_openstack_hypervisor_host_ip":  "1.2.2.2",
					"__meta_openstack_hypervisor_hostname": "fakehost",
					"__meta_openstack_hypervisor_id":       "5",
					"__meta_openstack_hypervisor_state":    "enabled",
					"__meta_openstack_hypervisor_status":   "up",
					"__meta_openstack_hypervisor_type":     "fake",
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addHypervisorLabels(tt.args.hvs, tt.args.port)
			discoveryutils.TestEqualLabelss(t, got, tt.want)
		})
	}
}
