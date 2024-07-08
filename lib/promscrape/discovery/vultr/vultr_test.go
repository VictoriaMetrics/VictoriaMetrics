package vultr

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetInstanceLabels(t *testing.T) {
	input := []Instance{
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
			Tags:             []string{"mock tags"},
			OSID:             1743,
			Features:         []string{"ipv6"},
		},
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
			Tags:             []string{"mock tags"},
			OSID:             1743,
			Features:         []string{"ipv6"},
		},
	}

	expect := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                "64.176.84.27:8080",
			"__meta_vultr_instance_id":                   "fake-id-07f7-4b68-88ac-fake-id",
			"__meta_vultr_instance_label":                "vultr-sd",
			"__meta_vultr_instance_os":                   "Ubuntu 22.04 x64",
			"__meta_vultr_instance_os_id":                "1743",
			"__meta_vultr_instance_region":               "sgp",
			"__meta_vultr_instance_plan":                 "vc2-1c-1gb",
			"__meta_vultr_instance_main_ip":              "64.176.84.27",
			"__meta_vultr_instance_internal_ip":          "",
			"__meta_vultr_instance_main_ipv6":            "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			"__meta_vultr_instance_hostname":             "vultr-sd",
			"__meta_vultr_instance_server_status":        "installingbooting",
			"__meta_vultr_instance_vcpu_count":           "1",
			"__meta_vultr_instance_ram_mb":               "1024",
			"__meta_vultr_instance_allowed_bandwidth_gb": "1",
			"__meta_vultr_instance_disk_gb":              "25",
			"__meta_vultr_instance_features":             ",ipv6,",
			"__meta_vultr_instance_tags":                 ",mock tags,",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                "64.176.84.27:8080",
			"__meta_vultr_instance_id":                   "fake-id-07f7-4b68-88ac-fake-id",
			"__meta_vultr_instance_label":                "vultr-sd",
			"__meta_vultr_instance_os":                   "Ubuntu 22.04 x64",
			"__meta_vultr_instance_os_id":                "1743",
			"__meta_vultr_instance_region":               "sgp",
			"__meta_vultr_instance_plan":                 "vc2-1c-1gb",
			"__meta_vultr_instance_main_ip":              "64.176.84.27",
			"__meta_vultr_instance_internal_ip":          "",
			"__meta_vultr_instance_main_ipv6":            "2002:18f0:4100:263a:5300:07ff:fdd7:691c",
			"__meta_vultr_instance_hostname":             "vultr-sd",
			"__meta_vultr_instance_server_status":        "installingbooting",
			"__meta_vultr_instance_vcpu_count":           "1",
			"__meta_vultr_instance_ram_mb":               "1024",
			"__meta_vultr_instance_allowed_bandwidth_gb": "1",
			"__meta_vultr_instance_disk_gb":              "25",
			"__meta_vultr_instance_features":             ",ipv6,",
			"__meta_vultr_instance_tags":                 ",mock tags,",
		}),
	}
	labels := getInstanceLabels(input, 8080)
	discoveryutils.TestEqualLabelss(t, labels, expect)
}
