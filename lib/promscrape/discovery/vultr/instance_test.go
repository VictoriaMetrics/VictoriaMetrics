package vultr

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

func TestGetInstances_Success(t *testing.T) {
	s := newMockVultrServer(func() []byte {
		const resp = `{
			"instances": [{
			"id": "fake-id-07f7-4b68-88ac-fake-id",
			"os": "Ubuntu 22.04 x64",
			"ram": 1024,
			"disk": 25,
			"main_ip": "64.176.84.27",
			"vcpu_count": 1,
			"region": "sgp",
			"plan": "vc2-1c-1gb",
			"date_created": "2024-04-05T05:41:28+00:00",
			"status": "active",
			"allowed_bandwidth": 1,
			"netmask_v4": "255.255.254.0",
			"gateway_v4": "64.176.63.2",
			"power_status": "running",
			"server_status": "installingbooting",
			"v6_network": "2002:18f0:4100:263a::",
			"v6_main_ip": "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			"v6_network_size": 64,
			"label": "vultr-sd",
			"internal_ip": "",
			"kvm": "https:\/\/my.vultr.com\/subs\/vps\/novnc\/api.php?data=secret_data_string",
			"hostname": "vultr-sd",
			"tag": "",
			"tags": [],
			"os_id": 1743,
			"app_id": 0,
			"image_id": "",
			"firewall_group_id": "",
			"features": ["ipv6"],
			"user_scheme": "root"
		}]
	}`

		return []byte(resp)
	})

	// Prepare a discovery HTTP client who calls mock server.
	client, err := discoveryutils.NewClient(s.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	if err != nil {
		t.Fatalf("unexpected error wen creating http client: %s", err)
	}
	cfg := &apiConfig{
		c: client,
	}

	// execute `getInstances`
	instances, err := getInstances(cfg)
	if err != nil {
		t.Fatalf("unexpected error in getInstances(): %s", err)
	}

	expectedInstances := []Instance{
		{
			ID:               "fake-id-07f7-4b68-88ac-fake-id",
			OS:               "Ubuntu 22.04 x64",
			RAM:              1024,
			Disk:             25,
			MainIP:           "64.176.84.27",
			VCPUCount:        1,
			Region:           "sgp",
			Plan:             "vc2-1c-1gb",
			AllowedBandwidth: 1,
			ServerStatus:     "installingbooting",
			V6MainIP:         "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			Label:            "vultr-sd",
			InternalIP:       "",
			Hostname:         "vultr-sd",
			Tags:             []string{},
			OSID:             1743,
			Features:         []string{"ipv6"},
		},
	}
	if !reflect.DeepEqual(instances, expectedInstances) {
		t.Fatalf("unexpected result\ngot\n%#v\nwant\n%#v", instances, expectedInstances)
	}
}

func TestGetInstances_Failure(t *testing.T) {
	s := newMockVultrServer(func() []byte {
		return []byte("some error")
	})

	// Prepare a discovery HTTP client who calls mock server.
	client, err := discoveryutils.NewClient(s.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	if err != nil {
		t.Fatalf("unexpected error wen creating http client: %s", err)
	}
	cfg := &apiConfig{
		c: client,
	}

	// execute `getInstances`
	if _, err := getInstances(cfg); err == nil {
		t.Fatalf("expecting non-nil error from getInstances()")
	}
}

func TestGetInstances_Paging(t *testing.T) {
	// Prepare a mock Vultr server.
	// requestCount control the mock response for different page request.
	requestCount := 0
	s := newMockVultrServer(func() []byte {
		// for the 1st request, response with `next` cursor
		if requestCount == 0 {
			requestCount++
			return []byte(mockListInstanceSuccessPage0Resp)
		}
		// for the 2nd+ request, response with empty `next`.
		return []byte(mockListInstanceSuccessPage1Resp)
	})

	// Prepare a discovery HTTP client who calls mock server.
	client, err := discoveryutils.NewClient(s.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	if err != nil {
		t.Fatalf("unexpected error wen creating http client: %s", err)
	}
	cfg := &apiConfig{
		c: client,
	}

	// execute `getInstances`
	instances, err := getInstances(cfg)
	if err != nil {
		t.Fatalf("unexpected error in getInstances(): %s", err)
	}
	if !reflect.DeepEqual(expectSuccessPagingInstances, instances) {
		t.Fatalf("unexpected getInstances() result\ngot\n%#v\nwant\n%#v", instances, expectSuccessPagingInstances)
	}
}

// ------------ Test dataset ------------
var (
	// mockListInstanceSuccessPage0Resp contains `next` cursor
	mockListInstanceSuccessPage0Resp = `{
	"instances": [{
		"id": "1-fake-id-07f7-4b68-88ac-fake-id",
		"os": "Ubuntu 22.04 x64",
		"ram": 1024,
		"disk": 25,
		"main_ip": "64.176.84.27",
		"vcpu_count": 1,
		"region": "sgp",
		"plan": "vc2-1c-1gb",
		"date_created": "2024-04-05T05:41:28+00:00",
		"status": "active",
		"allowed_bandwidth": 1,
		"netmask_v4": "255.255.254.0",
		"gateway_v4": "64.176.63.2",
		"power_status": "running",
		"server_status": "installingbooting",
		"v6_network": "2002:18f0:4100:263a::",
		"v6_main_ip": "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
		"v6_network_size": 64,
		"label": "vultr-sd",
		"internal_ip": "",
		"kvm": "https:\/\/my.vultr.com\/subs\/vps\/novnc\/api.php?data=secret_data_string",
		"hostname": "vultr-sd",
		"tag": "",
		"tags": [],
		"os_id": 1743,
		"app_id": 0,
		"image_id": "",
		"firewall_group_id": "",
		"features": ["ipv6"],
		"user_scheme": "root"
	}],
	"meta": {
		"links": {
			"next": "fake-cursor-string"
		}
	}
}`
	// mockListInstanceSuccessPage1Resp contains empty 'next' cursor
	mockListInstanceSuccessPage1Resp = `{
	"instances": [{
		"id": "2-fake-id-07f7-4b68-88ac-fake-id",
		"os": "Ubuntu 22.04 x64",
		"ram": 1024,
		"disk": 25,
		"main_ip": "64.176.84.27",
		"vcpu_count": 1,
		"region": "sgp",
		"plan": "vc2-1c-1gb",
		"date_created": "2024-04-05T05:41:28+00:00",
		"status": "active",
		"allowed_bandwidth": 1,
		"netmask_v4": "255.255.254.0",
		"gateway_v4": "64.176.63.2",
		"power_status": "running",
		"server_status": "installingbooting",
		"v6_network": "2002:18f0:4100:263a::",
		"v6_main_ip": "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
		"v6_network_size": 64,
		"label": "vultr-sd",
		"internal_ip": "",
		"kvm": "https:\/\/my.vultr.com\/subs\/vps\/novnc\/api.php?data=secret_data_string",
		"hostname": "vultr-sd",
		"tag": "",
		"tags": [],
		"os_id": 1743,
		"app_id": 0,
		"image_id": "",
		"firewall_group_id": "",
		"features": ["ipv6"],
		"user_scheme": "root"
	}],
	"meta": {
		"links": {
			"next": ""
		}
	}
}`
	expectSuccessPagingInstances = []Instance{
		{
			ID:               "1-fake-id-07f7-4b68-88ac-fake-id",
			OS:               "Ubuntu 22.04 x64",
			RAM:              1024,
			Disk:             25,
			MainIP:           "64.176.84.27",
			VCPUCount:        1,
			Region:           "sgp",
			Plan:             "vc2-1c-1gb",
			AllowedBandwidth: 1,
			ServerStatus:     "installingbooting",
			V6MainIP:         "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			Label:            "vultr-sd",
			InternalIP:       "",
			Hostname:         "vultr-sd",
			Tags:             []string{},
			OSID:             1743,
			Features:         []string{"ipv6"},
		},
		{
			ID:               "2-fake-id-07f7-4b68-88ac-fake-id",
			OS:               "Ubuntu 22.04 x64",
			RAM:              1024,
			Disk:             25,
			MainIP:           "64.176.84.27",
			VCPUCount:        1,
			Region:           "sgp",
			Plan:             "vc2-1c-1gb",
			AllowedBandwidth: 1,
			ServerStatus:     "installingbooting",
			V6MainIP:         "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			Label:            "vultr-sd",
			InternalIP:       "",
			Hostname:         "vultr-sd",
			Tags:             []string{},
			OSID:             1743,
			Features:         []string{"ipv6"},
		},
	}
)
