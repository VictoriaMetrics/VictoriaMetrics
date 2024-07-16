package openstack

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseHypervisorDetail_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		_, err := parseHypervisorDetail([]byte(data))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// bad data
	f(`{ff}`)
}

func TestParseHypervisorDetail_Success(t *testing.T) {
	f := func(data string, resultExpected *hypervisorDetail) {
		t.Helper()

		result, err := parseHypervisorDetail([]byte(data))
		if err != nil {
			t.Fatalf("parseHypervisorDetail() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%#v\nwant\n%#v", result, resultExpected)
		}
	}

	// 1 hypervisor
	data := `{
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
    ]}`

	resultExpected := &hypervisorDetail{
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
	}
	f(data, resultExpected)
}

func TestAddHypervisorLabels(t *testing.T) {
	f := func(hvs []hypervisor, labelssExpected []*promutils.Labels) {
		t.Helper()

		labelss := addHypervisorLabels(hvs, 9100)
		discoveryutils.TestEqualLabelss(t, labelss, labelssExpected)
	}

	hvs := []hypervisor{
		{
			Type:     "fake",
			ID:       5,
			State:    "enabled",
			Status:   "up",
			Hostname: "fakehost",
			HostIP:   "1.2.2.2",
		},
	}
	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                          "1.2.2.2:9100",
			"__meta_openstack_hypervisor_host_ip":  "1.2.2.2",
			"__meta_openstack_hypervisor_hostname": "fakehost",
			"__meta_openstack_hypervisor_id":       "5",
			"__meta_openstack_hypervisor_state":    "enabled",
			"__meta_openstack_hypervisor_status":   "up",
			"__meta_openstack_hypervisor_type":     "fake",
		}),
	}
	f(hvs, labelssExpected)
}
