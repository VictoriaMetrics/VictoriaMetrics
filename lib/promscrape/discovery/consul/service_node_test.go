package consul

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestParseServiceNodesFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		sns, err := ParseServiceNodes([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if sns != nil {
			t.Fatalf("unexpected non-nil ServiceNodes: %v", sns)
		}
	}
	f(``)
	f(`[1,23]`)
	f(`{"items":[{"metadata":1}]}`)
}

func TestParseServiceNodesSuccess(t *testing.T) {
	data := `
[
  {
    "Node": {
      "ID": "40e4a748-2192-161a-0510-9bf59fe950b5",
      "Node": "foobar",
      "Address": "10.1.10.12",
      "Datacenter": "dc1",
      "TaggedAddresses": {
        "lan": "10.1.10.12",
        "wan": "10.1.10.12"
      },
      "Meta": {
        "instance_type": "t2.medium"
      }
    },
    "Service": {
      "ID": "redis",
      "Service": "redis",
      "Tags": ["primary","foo=bar"],
      "Address": "10.1.10.12",
      "TaggedAddresses": {
        "lan": {
          "address": "10.1.10.12",
          "port": 8000
        },
        "wan": {
          "address": "198.18.1.2",
          "port": 80
        }
      },
      "Meta": {
        "redis_version": "4.0"
      },
      "Port": 8000,
      "Weights": {
        "Passing": 10,
        "Warning": 1
      },
      "Namespace": "ns-dev",
      "Partition": "part-foobar"
    },
    "Checks": [
      {
        "Node": "foobar",
        "CheckID": "service:redis",
        "Name": "Service 'redis' check",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "redis",
        "ServiceName": "redis",
        "ServiceTags": ["primary"],
        "Namespace": "default"
      },
      {
        "Node": "foobar",
        "CheckID": "serfHealth",
        "Name": "Serf Health Status",
        "Status": "passing",
        "Notes": "",
        "Output": "",
        "ServiceID": "",
        "ServiceName": "",
        "ServiceTags": [],
        "Namespace": "default"
      }
    ]
  }
]
`
	sns, err := ParseServiceNodes([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sns) != 1 {
		t.Fatalf("unexpected length of ServiceNodes; got %d; want %d", len(sns), 1)
	}
	sn := sns[0]

	// Check sn.appendTargetLabels()
	tagSeparator := ","
	labelss := sn.appendTargetLabels(nil, "redis", tagSeparator)
	expectedLabelss := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                  "10.1.10.12:8000",
			"__meta_consul_address":                        "10.1.10.12",
			"__meta_consul_dc":                             "dc1",
			"__meta_consul_health":                         "passing",
			"__meta_consul_metadata_instance_type":         "t2.medium",
			"__meta_consul_namespace":                      "ns-dev",
			"__meta_consul_node":                           "foobar",
			"__meta_consul_partition":                      "part-foobar",
			"__meta_consul_service":                        "redis",
			"__meta_consul_service_address":                "10.1.10.12",
			"__meta_consul_service_id":                     "redis",
			"__meta_consul_service_metadata_redis_version": "4.0",
			"__meta_consul_service_port":                   "8000",
			"__meta_consul_tagged_address_lan":             "10.1.10.12",
			"__meta_consul_tagged_address_wan":             "10.1.10.12",
			"__meta_consul_tag_foo":                        "bar",
			"__meta_consul_tag_primary":                    "",
			"__meta_consul_tagpresent_foo":                 "true",
			"__meta_consul_tagpresent_primary":             "true",
			"__meta_consul_tags":                           ",primary,foo=bar,",
		}),
	}
	discoveryutil.TestEqualLabelss(t, labelss, expectedLabelss)
}
