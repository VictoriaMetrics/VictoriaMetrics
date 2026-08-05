package linode

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAddInstanceLabels(t *testing.T) {
	f := func(instances []instance, detailedIPs []ipAddress, ipv6Ranges []ipv6Range, labelssExpected []*promutil.Labels) {
		t.Helper()
		labelss := addInstanceLabels(instances, detailedIPs, ipv6Ranges, 80, ",")
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// Fixture shapes mirror Prometheus discovery/linode testdata (no_region_filter).
	instances := []instance{
		{
			ID: 26838044, Label: "prometheus-linode-sd-exporter-1", Status: "running",
			Type: "g6-standard-2", Image: "linode/arch", Region: "us-east", Hypervisor: "kvm",
			IPv4:  []string{"45.33.82.151", "96.126.108.16", "192.168.170.51", "192.168.201.25"},
			IPv6:  "2600:3c03::f03c:92ff:fe1a:1382/128",
			Tags:  []string{"monitoring"},
			Specs: specs{Disk: 81920, Memory: 4096, VCPUs: 2, GPUs: 0, Transfer: 4000},
		},
		{
			ID: 26848419, Label: "prometheus-linode-sd-exporter-2", Status: "running",
			Type: "g6-standard-2", Image: "linode/debian10", Region: "eu-west", Hypervisor: "kvm",
			IPv4:  []string{"139.162.196.43"},
			IPv6:  "2a01:7e00::f03c:92ff:fe1a:9976/128",
			Tags:  []string{"monitoring"},
			Specs: specs{Disk: 81920, Memory: 4096, VCPUs: 2, GPUs: 0, Transfer: 4000},
		},
		{
			ID: 26837938, Label: "prometheus-linode-sd-exporter-3", Status: "running",
			Type: "g6-standard-1", Image: "linode/ubuntu20.04", Region: "ca-central", Hypervisor: "kvm",
			IPv4:  []string{"192.53.120.25"},
			IPv6:  "2600:3c04::f03c:92ff:fe1a:fb68/128",
			Tags:  []string{"monitoring"},
			Specs: specs{Disk: 51200, Memory: 2048, VCPUs: 1, GPUs: 0, Transfer: 2000},
		},
		{
			ID: 26837992, Label: "prometheus-linode-sd-exporter-4", Status: "running",
			Type: "g6-nanode-1", Image: "linode/ubuntu20.04", Region: "us-east", Hypervisor: "kvm",
			IPv4:  []string{"66.228.47.103", "172.104.18.104", "192.168.148.94"},
			IPv6:  "2600:3c03::f03c:92ff:fe1a:fb4c/128",
			Tags:  []string{"monitoring"},
			Specs: specs{Disk: 25600, Memory: 1024, VCPUs: 1, GPUs: 0, Transfer: 1000},
		},
		// No IPv4 — must be skipped (Prometheus parity).
		{
			ID: 999, Label: "no-ipv4", Status: "running",
			Type: "g6-nanode-1", Image: "linode/ubuntu20.04", Region: "us-east",
			IPv4: nil, IPv6: "2600:3c03::1/128",
		},
	}

	detailedIPs := []ipAddress{
		{Address: "192.53.120.25", Public: true, RDNS: "li2216-25.members.linode.com"},
		{Address: "66.228.47.103", Public: true, RDNS: "li328-103.members.linode.com"},
		{Address: "172.104.18.104", Public: true, RDNS: "li1832-104.members.linode.com"},
		{Address: "192.168.148.94", Public: false, RDNS: ""},
		{Address: "192.168.170.51", Public: false, RDNS: ""},
		{Address: "96.126.108.16", Public: true, RDNS: "li365-16.members.linode.com"},
		{Address: "45.33.82.151", Public: true, RDNS: "li1028-151.members.linode.com"},
		{Address: "192.168.201.25", Public: false, RDNS: ""},
		{Address: "139.162.196.43", Public: true, RDNS: "li1359-43.members.linode.com"},
		{Address: "2600:3c03::f03c:92ff:fe1a:1382", Public: true, RDNS: ""},
		{Address: "2a01:7e00::f03c:92ff:fe1a:9976", Public: true, RDNS: ""},
		{Address: "2600:3c04::f03c:92ff:fe1a:fb68", Public: true, RDNS: ""},
		{Address: "2600:3c03::f03c:92ff:fe1a:fb4c", Public: true, RDNS: ""},
	}

	ipv6Ranges := []ipv6Range{
		{Range: "2600:3c03:e000:123::", Prefix: 64, RouteTarget: "2600:3c03::f03c:92ff:fe1a:fb4c"},
		{Range: "2600:3c04:e001:456::", Prefix: 64, RouteTarget: "2600:3c04::f03c:92ff:fe1a:fb68"},
	}

	// Expected values match prometheus/discovery/linode/linode_test.go "no_region" case.
	f(instances, detailedIPs, ipv6Ranges, []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "45.33.82.151:80",
			"__meta_linode_instance_id":          "26838044",
			"__meta_linode_instance_label":       "prometheus-linode-sd-exporter-1",
			"__meta_linode_image":                "linode/arch",
			"__meta_linode_private_ipv4":         "192.168.170.51",
			"__meta_linode_public_ipv4":          "45.33.82.151",
			"__meta_linode_public_ipv6":          "2600:3c03::f03c:92ff:fe1a:1382",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "li1028-151.members.linode.com",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "us-east",
			"__meta_linode_type":                 "g6-standard-2",
			"__meta_linode_status":               "running",
			"__meta_linode_tags":                 ",monitoring,",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "85899345920",
			"__meta_linode_specs_memory_bytes":   "4294967296",
			"__meta_linode_specs_vcpus":          "2",
			"__meta_linode_specs_transfer_bytes": "4194304000",
			"__meta_linode_extra_ips":            ",96.126.108.16,192.168.201.25,",
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "139.162.196.43:80",
			"__meta_linode_instance_id":          "26848419",
			"__meta_linode_instance_label":       "prometheus-linode-sd-exporter-2",
			"__meta_linode_image":                "linode/debian10",
			"__meta_linode_private_ipv4":         "",
			"__meta_linode_public_ipv4":          "139.162.196.43",
			"__meta_linode_public_ipv6":          "2a01:7e00::f03c:92ff:fe1a:9976",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "li1359-43.members.linode.com",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "eu-west",
			"__meta_linode_type":                 "g6-standard-2",
			"__meta_linode_status":               "running",
			"__meta_linode_tags":                 ",monitoring,",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "85899345920",
			"__meta_linode_specs_memory_bytes":   "4294967296",
			"__meta_linode_specs_vcpus":          "2",
			"__meta_linode_specs_transfer_bytes": "4194304000",
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "192.53.120.25:80",
			"__meta_linode_instance_id":          "26837938",
			"__meta_linode_instance_label":       "prometheus-linode-sd-exporter-3",
			"__meta_linode_image":                "linode/ubuntu20.04",
			"__meta_linode_private_ipv4":         "",
			"__meta_linode_public_ipv4":          "192.53.120.25",
			"__meta_linode_public_ipv6":          "2600:3c04::f03c:92ff:fe1a:fb68",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "li2216-25.members.linode.com",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "ca-central",
			"__meta_linode_type":                 "g6-standard-1",
			"__meta_linode_status":               "running",
			"__meta_linode_tags":                 ",monitoring,",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "53687091200",
			"__meta_linode_specs_memory_bytes":   "2147483648",
			"__meta_linode_specs_vcpus":          "1",
			"__meta_linode_specs_transfer_bytes": "2097152000",
			"__meta_linode_ipv6_ranges":          ",2600:3c04:e001:456::/64,",
		}),
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "66.228.47.103:80",
			"__meta_linode_instance_id":          "26837992",
			"__meta_linode_instance_label":       "prometheus-linode-sd-exporter-4",
			"__meta_linode_image":                "linode/ubuntu20.04",
			"__meta_linode_private_ipv4":         "192.168.148.94",
			"__meta_linode_public_ipv4":          "66.228.47.103",
			"__meta_linode_public_ipv6":          "2600:3c03::f03c:92ff:fe1a:fb4c",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "li328-103.members.linode.com",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "us-east",
			"__meta_linode_type":                 "g6-nanode-1",
			"__meta_linode_status":               "running",
			"__meta_linode_tags":                 ",monitoring,",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "26843545600",
			"__meta_linode_specs_memory_bytes":   "1073741824",
			"__meta_linode_specs_vcpus":          "1",
			"__meta_linode_specs_transfer_bytes": "1048576000",
			"__meta_linode_extra_ips":            ",172.104.18.104,",
			"__meta_linode_ipv6_ranges":          ",2600:3c03:e000:123::/64,",
		}),
	})
}

func TestAddInstanceLabelsPrivateOnlyFallback(t *testing.T) {
	instances := []instance{
		{
			ID: 2, Label: "vpc-only", Status: "running", Type: "g6-nanode-1",
			Image: "linode/ubuntu", Region: "us-east", Hypervisor: "kvm",
			IPv4:  []string{"10.0.0.5"},
			Specs: specs{Disk: 1, Memory: 1, VCPUs: 1, Transfer: 1},
		},
		// Has IPv4 list entries but no matching detailed IPs — must be skipped.
		{
			ID: 3, Label: "orphan", Status: "running", Type: "g6-nanode-1",
			Image: "linode/ubuntu", Region: "us-east",
			IPv4:  []string{"10.0.0.99"},
			Specs: specs{Disk: 1, Memory: 1, VCPUs: 1, Transfer: 1},
		},
	}
	ips := []ipAddress{{Address: "10.0.0.5", Public: false, RDNS: ""}}
	got := addInstanceLabels(instances, ips, nil, 80, ",")
	want := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "10.0.0.5:80",
			"__meta_linode_instance_id":          "2",
			"__meta_linode_instance_label":       "vpc-only",
			"__meta_linode_image":                "linode/ubuntu",
			"__meta_linode_private_ipv4":         "10.0.0.5",
			"__meta_linode_public_ipv4":          "",
			"__meta_linode_public_ipv6":          "",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "us-east",
			"__meta_linode_type":                 "g6-nanode-1",
			"__meta_linode_status":               "running",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "1048576",
			"__meta_linode_specs_memory_bytes":   "1048576",
			"__meta_linode_specs_vcpus":          "1",
			"__meta_linode_specs_transfer_bytes": "1048576",
		}),
	}
	discoveryutil.TestEqualLabelss(t, got, want)
}

func TestAddInstanceLabelsCustomPortAndTagSeparator(t *testing.T) {
	instances := []instance{
		{
			ID: 1, Label: "node", Status: "running", Type: "g6-nanode-1",
			Image: "linode/ubuntu", Region: "us-east", Hypervisor: "kvm",
			IPv4:  []string{"1.2.3.4"},
			Tags:  []string{"a", "b"},
			Specs: specs{Disk: 1, Memory: 1, VCPUs: 1, Transfer: 1},
		},
	}
	ips := []ipAddress{{Address: "1.2.3.4", Public: true, RDNS: "example.com"}}
	got := addInstanceLabels(instances, ips, nil, 9100, ";")
	want := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                        "1.2.3.4:9100",
			"__meta_linode_instance_id":          "1",
			"__meta_linode_instance_label":       "node",
			"__meta_linode_image":                "linode/ubuntu",
			"__meta_linode_private_ipv4":         "",
			"__meta_linode_public_ipv4":          "1.2.3.4",
			"__meta_linode_public_ipv6":          "",
			"__meta_linode_private_ipv4_rdns":    "",
			"__meta_linode_public_ipv4_rdns":     "example.com",
			"__meta_linode_public_ipv6_rdns":     "",
			"__meta_linode_region":               "us-east",
			"__meta_linode_type":                 "g6-nanode-1",
			"__meta_linode_status":               "running",
			"__meta_linode_tags":                 ";a;b;",
			"__meta_linode_group":                "",
			"__meta_linode_gpus":                 "0",
			"__meta_linode_hypervisor":           "kvm",
			"__meta_linode_backups":              "disabled",
			"__meta_linode_specs_disk_bytes":     "1048576",
			"__meta_linode_specs_memory_bytes":   "1048576",
			"__meta_linode_specs_vcpus":          "1",
			"__meta_linode_specs_transfer_bytes": "1048576",
		}),
	}
	discoveryutil.TestEqualLabelss(t, got, want)
}
