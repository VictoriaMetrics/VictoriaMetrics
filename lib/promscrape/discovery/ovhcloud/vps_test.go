package ovhcloud

import (
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_getVpsLabels(t *testing.T) {
	mockSvr := newMockOVHCloudServer(func(path string) ([]byte, error) {
		switch path {
		case "/vps":
			return []byte(`["vps-000e0e00.vps.ovh.ca"]`), nil
		case "/vps/vps-000e0e00.vps.ovh.ca":
			return mockVpsDetail, nil
		case "/vps/vps-000e0e00.vps.ovh.ca/ips":
			return []byte(`["139.99.154.158","2402:1f00:8100:401::bb6"]`), nil
		default:
			return []byte{}, errors.New("invalid request")
		}
	})
	c, _ := discoveryutils.NewClient(mockSvr.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	td := atomic.Value{}
	td.Store(time.Duration(1))
	cfg := &apiConfig{
		client:            c,
		applicationKey:    "",
		applicationSecret: "",
		consumerKey:       "",
		timeDelta:         td,
	}

	expectLabels := &promutils.Labels{}
	expectLabels.Add("__address__", "139.99.154.158")
	expectLabels.Add("instance", "vps-000e0e00.vps.ovh.ca")
	expectLabels.Add("__meta_ovhcloud_vps_offer", "VPS vps2020-starter-1-2-20")
	expectLabels.Add("__meta_ovhcloud_vps_datacenter", "[]")
	expectLabels.Add("__meta_ovhcloud_vps_model_vcore", "1")
	expectLabels.Add("__meta_ovhcloud_vps_maximum_additional_ip", "16")
	expectLabels.Add("__meta_ovhcloud_vps_version", "2019v1")
	expectLabels.Add("__meta_ovhcloud_vps_model_name", "vps-starter-1-2-20")
	expectLabels.Add("__meta_ovhcloud_vps_disk", "20")
	expectLabels.Add("__meta_ovhcloud_vps_memory", "2048")
	expectLabels.Add("__meta_ovhcloud_vps_zone", "Region OpenStack: os-syd2")
	expectLabels.Add("__meta_ovhcloud_vps_display_name", "vps-000e0e00.vps.ovh.ca")
	expectLabels.Add("__meta_ovhcloud_vps_cluster", "")
	expectLabels.Add("__meta_ovhcloud_vps_state", "running")
	expectLabels.Add("__meta_ovhcloud_vps_name", "vps-000e0e00.vps.ovh.ca")
	expectLabels.Add("__meta_ovhcloud_vps_netboot_mode", "local")
	expectLabels.Add("__meta_ovhcloud_vps_memory_limit", "2048")
	expectLabels.Add("__meta_ovhcloud_vps_offer_type", "ssd")
	expectLabels.Add("__meta_ovhcloud_vps_vcore", "1")
	expectLabels.Add("__meta_ovhcloud_vps_ipv4", "139.99.154.158")
	expectLabels.Add("__meta_ovhcloud_vps_ipv6", "2402:1f00:8100:401::bb6")
	expect := []*promutils.Labels{
		expectLabels,
	}

	result, err := getVPSLabels(cfg)
	if err != nil {
		t.Fatalf("getDedicatedServerLabels unexpected error: %v", err)
	}

	if !reflect.DeepEqual(expect, result) {
		t.Fatalf("getDedicatedServerLabels incorrect, want: %v, got: %v", expect, result)
	}
}

var mockVpsDetail = []byte(
	`{
	"model": {
		"name": "vps-starter-1-2-20",
		"offer": "VPS vps2020-starter-1-2-20",
		"availableOptions": [],
		"maximumAdditionnalIp": 16,
		"version": "2019v1",
		"datacenter": [],
		"vcore": 1,
		"memory": 2048,
		"disk": 20
	},
	"netbootMode": "local",
	"cluster": "",
	"name": "vps-000e0e00.vps.ovh.ca",
	"displayName": "vps-000e0e00.vps.ovh.ca",
	"vcore": 1,
	"monitoringIpBlocks": [],
	"zone": "Region OpenStack: os-syd2",
	"memoryLimit": 2048,
	"offerType": "ssd",
	"state": "running",
	"keymap": null,
	"slaMonitoring": false,
	"iam": {
		"id": "00ea0000-0b0f-0000-0000-e000000a0000",
		"urn": "urn:v1:eu:resource:vps:vps-000e0e00.vps.ovh.ca"
	}
}`)
