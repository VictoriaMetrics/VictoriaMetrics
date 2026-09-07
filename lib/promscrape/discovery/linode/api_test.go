package linode

import (
	"encoding/json"
	"testing"
)

func TestParseListInstances(t *testing.T) {
	data := []byte(`{
  "data": [
    {
      "id": 26838044,
      "label": "prometheus-linode-sd-exporter-1",
      "group": "",
      "status": "running",
      "type": "g6-standard-2",
      "ipv4": ["45.33.82.151", "96.126.108.16"],
      "ipv6": "2600:3c03::f03c:92ff:fe1a:1382/128",
      "image": "linode/arch",
      "region": "us-east",
      "specs": {"disk": 81920, "memory": 4096, "vcpus": 2, "gpus": 0, "transfer": 4000},
      "backups": {"enabled": false},
      "hypervisor": "kvm",
      "tags": ["monitoring"]
    }
  ],
  "page": 1,
  "pages": 1,
  "results": 1
}`)
	var resp listResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("cannot unmarshal list response: %s", err)
	}
	if resp.Page != 1 || resp.Pages != 1 {
		t.Fatalf("unexpected pagination: page=%d pages=%d", resp.Page, resp.Pages)
	}
	var instances []instance
	if err := json.Unmarshal(resp.Data, &instances); err != nil {
		t.Fatalf("cannot unmarshal instances: %s", err)
	}
	if len(instances) != 1 {
		t.Fatalf("unexpected instances len: %d", len(instances))
	}
	inst := instances[0]
	if inst.ID != 26838044 || inst.Label != "prometheus-linode-sd-exporter-1" {
		t.Fatalf("unexpected instance: %+v", inst)
	}
	if len(inst.IPv4) != 2 || inst.IPv4[0] != "45.33.82.151" {
		t.Fatalf("unexpected ipv4: %v", inst.IPv4)
	}
	if inst.Specs.Disk != 81920 || inst.Specs.Memory != 4096 {
		t.Fatalf("unexpected specs: %+v", inst.Specs)
	}
	if inst.Backups.Enabled {
		t.Fatalf("expected backups disabled")
	}
}

func TestParseIPAddressesNullRDNS(t *testing.T) {
	data := []byte(`[
    {
      "address": "192.168.148.94",
      "public": false,
      "rdns": null
    },
    {
      "address": "66.228.47.103",
      "public": true,
      "rdns": "li328-103.members.linode.com"
    }
  ]`)
	var ips []ipAddress
	if err := json.Unmarshal(data, &ips); err != nil {
		t.Fatalf("cannot unmarshal ips: %s", err)
	}
	if len(ips) != 2 {
		t.Fatalf("unexpected len: %d", len(ips))
	}
	if ips[0].RDNS != "" {
		t.Fatalf("expected empty rdns for null, got %q", ips[0].RDNS)
	}
	if ips[1].RDNS != "li328-103.members.linode.com" {
		t.Fatalf("unexpected rdns: %q", ips[1].RDNS)
	}
}

func TestParseIPv6Ranges(t *testing.T) {
	data := []byte(`[
    {
      "range": "2600:3c03:e000:123::",
      "prefix": 64,
      "route_target": "2600:3c03::f03c:92ff:fe1a:fb4c"
    }
  ]`)
	var ranges []ipv6Range
	if err := json.Unmarshal(data, &ranges); err != nil {
		t.Fatalf("cannot unmarshal ranges: %s", err)
	}
	if len(ranges) != 1 || ranges[0].Prefix != 64 {
		t.Fatalf("unexpected ranges: %+v", ranges)
	}
	if ranges[0].RouteTarget != "2600:3c03::f03c:92ff:fe1a:fb4c" {
		t.Fatalf("unexpected route_target: %q", ranges[0].RouteTarget)
	}
}
