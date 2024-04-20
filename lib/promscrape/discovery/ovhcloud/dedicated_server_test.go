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

func Test_getDedicatedServerLabels(t *testing.T) {
	mockSvr := newMockOVHCloudServer(func(path string) ([]byte, error) {
		switch path {
		case "/dedicated/server":
			return []byte(`["ns0000000.ip-00-00-000.eu"]`), nil
		case "/dedicated/server/ns0000000.ip-00-00-000.eu":
			return mockDedicatedServerDetail, nil
		case "/dedicated/server/ns0000000.ip-00-00-000.eu/ips":
			return []byte(`["2001:40d0:302:8874::/64","50.75.126.113/32"]`), nil
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
	expectLabels.Add("__address__", "50.75.126.113")
	expectLabels.Add("instance", "ns0000000.ip-00-00-000.eu")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_state", "ok")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_commercial_range", "RISE-3")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_link_speed", "1000")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_rack", "G000A00")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_no_intervention", "false")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_os", "centos7_64")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_support_level", "pro")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_server_id", "1000000")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_reverse", "ns0000000.ip-00-00-000.eu")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_datacenter", "gra2")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_name", "ns0000000.ip-00-00-000.eu")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_ipv4", "50.75.126.113")
	expectLabels.Add("__meta_ovhcloud_dedicated_server_ipv6", "")
	expect := []*promutils.Labels{
		expectLabels,
	}

	result, err := getDedicatedServerLabels(cfg)
	if err != nil {
		t.Fatalf("getDedicatedServerLabels unexpected error: %v", err)
	}

	if !reflect.DeepEqual(expect, result) {
		t.Fatalf("getDedicatedServerLabels incorrect, want: %v, got: %v", expect, result)
	}
}

var mockDedicatedServerDetail = []byte(
	`{
	"name": "ns0000000.ip-00-00-000.eu",
	"availabilityZone": "eu-west-gra-a",
	"datacenter": "gra2",
	"bootScript": null,
	"linkSpeed": 1000,
	"reverse": "ns0000000.ip-00-00-000.eu",
	"serverId": 1000000,
	"monitoring": false,
	"rootDevice": null,
	"noIntervention": false,
	"newUpgradeSystem": true,
	"rack": "G000A00",
	"rescueSshKey": null,
	"supportLevel": "pro",
	"powerState": "poweron",
	"commercialRange": "RISE-3",
	"professionalUse": false,
	"rescueMail": null,
	"region": "eu-west-gra",
	"bootId": 1,
	"state": "ok",
	"os": "centos7_64",
	"ip": "50.75.126.113",
	"iam": {
		"displayName": "ns0000000.ip-00-00-000.eu",
		"id": "000da00d-00d0-0b00-0000-00000a0000bd",
		"urn": "urn:v1:eu:resource:dedicatedServer:ns0000000.ip-00-00-000.eu"
	}
}`)
