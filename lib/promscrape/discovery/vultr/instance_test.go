package vultr

import (
	"errors"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// TestGetInstances runs general test cases for GetInstances
func TestGetInstances(t *testing.T) {
	testCases := []struct {
		name           string
		apiResponse    string
		apiError       bool
		expectError    bool
		expectResponse []Instance
	}{
		{
			name:           "success response",
			apiResponse:    mockListInstanceSuccessResp,
			apiError:       false,
			expectError:    false,
			expectResponse: expectSuccessInstances,
		},
		{
			name:           "failed response",
			apiResponse:    mockListInstanceFailedResp,
			apiError:       true,
			expectError:    true,
			expectResponse: nil,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare a mock Vultr server.
			mockServer := newMockVultrServer(func() ([]byte, error) {
				var e error
				if tt.apiError {
					e = errors.New("mock error")
				}
				return []byte(tt.apiResponse), e
			})

			// Prepare a discovery HTTP client who calls mock server.
			client, _ := discoveryutils.NewClient(mockServer.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
			cfg := &apiConfig{
				c: client,
			}

			// execute `getInstances`
			instances, err := getInstances(cfg)

			// evaluate test result
			if tt.expectError != (err != nil) {
				t.Errorf("getInstances expect (error != nil): %t, got error: %v", tt.expectError, err)
			}

			if !reflect.DeepEqual(tt.expectResponse, instances) {
				t.Errorf("getInstances expect result: %v, got: %v", tt.expectResponse, instances)
			}
		})
	}
}

// TestGetInstancesPaging run test cases for response with multiple pages.
func TestGetInstancesPaging(t *testing.T) {
	// Prepare a mock Vultr server.
	// requestCount control the mock response for different page request.
	requestCount := 0

	mockServer := newMockVultrServer(func() ([]byte, error) {
		// for the 1st request, response with `next` cursor
		if requestCount == 0 {
			requestCount++
			return []byte(mockListInstanceSuccessPage0Resp), nil
		}
		// for the 2nd+ request, response with `prev` cursor and empty `next`.
		return []byte(mockListInstanceSuccessPage1Resp), nil
	})

	// Prepare a discovery HTTP client who calls mock server.
	client, _ := discoveryutils.NewClient(mockServer.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	cfg := &apiConfig{
		c: client,
	}

	// execute `getInstances`
	instances, err := getInstances(cfg)

	// evaluate test result
	if err != nil {
		t.Errorf("getInstances expect error: %v, got error: %v", nil, err)
	}

	if !reflect.DeepEqual(expectSuccessPagingInstances, instances) {
		t.Errorf("getInstances expect result: %v, got: %v", expectSuccessPagingInstances, instances)
	}
}

// ------------ Test dataset ------------
var (
	// mockListInstanceSuccessResp is crawled from a real-world response of ListInstance API
	// with sensitive info removed/modified.
	mockListInstanceSuccessResp = `{
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
	}],
	"meta": {
		"total": 1,
		"links": {
			"next": "",
			"prev": ""
		}
	}
}`
	expectSuccessInstances = []Instance{
		{
			ID:               "fake-id-07f7-4b68-88ac-fake-id",
			Os:               "Ubuntu 22.04 x64",
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
			OsID:             1743,
			Features:         []string{"ipv6"},
		},
	}
)

var (
	mockListInstanceFailedResp = `{"error":"Invalid API token.","status":401}`
)

var (
	// mockListInstanceSuccessPage0Resp contains `next` cursor
	mockListInstanceSuccessPage0Resp = `{
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
	}],
	"meta": {
		"total": 2,
		"links": {
			"next": "fake-cursor-string",
			"prev": ""
		}
	}
}`
	// mockListInstanceSuccessPage1Resp contains `prev` cursor
	mockListInstanceSuccessPage1Resp = `{
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
	}],
	"meta": {
		"total": 2,
		"links": {
			"next": "",
			"prev": "fake-cursor-string"
		}
	}
}`
	expectSuccessPagingInstances = []Instance{
		{
			ID:               "fake-id-07f7-4b68-88ac-fake-id",
			Os:               "Ubuntu 22.04 x64",
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
			OsID:             1743,
			Features:         []string{"ipv6"},
		},
		{
			ID:               "fake-id-07f7-4b68-88ac-fake-id",
			Os:               "Ubuntu 22.04 x64",
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
			OsID:             1743,
			Features:         []string{"ipv6"},
		},
	}
)
