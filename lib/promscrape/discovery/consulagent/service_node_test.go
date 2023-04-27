package consulagent

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseServiceNodesFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		sns, err := consul.ParseServiceNodes([]byte(s))
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
	sns, err := consul.ParseServiceNodes([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(sns) != 1 {
		t.Fatalf("unexpected length of ServiceNodes; got %d; want %d", len(sns), 1)
	}
	sn := sns[0]

	agentData := `
{
  "Member": {
    "Addr": "10.1.10.12"
  },
  "Config": {
    "Datacenter": "dc1",
    "NodeName": "foobar"
  },
  "Meta": {
    "instance_type": "t2.medium"
  }
}
`
	agent, err := consul.ParseAgent([]byte(agentData))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Check sn.appendTargetLabels()
	tagSeparator := ","
	labelss := appendTargetLabels(sn, nil, "redis", tagSeparator, agent)
	expectedLabelss := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                       "10.1.10.12:8000",
			"__meta_consulagent_address":                        "10.1.10.12",
			"__meta_consulagent_dc":                             "dc1",
			"__meta_consulagent_health":                         "passing",
			"__meta_consulagent_metadata_instance_type":         "t2.medium",
			"__meta_consulagent_namespace":                      "ns-dev",
			"__meta_consulagent_node":                           "foobar",
			"__meta_consulagent_service":                        "redis",
			"__meta_consulagent_service_address":                "10.1.10.12",
			"__meta_consulagent_service_id":                     "redis",
			"__meta_consulagent_service_metadata_redis_version": "4.0",
			"__meta_consulagent_service_port":                   "8000",
			"__meta_consulagent_tagged_address_lan":             "10.1.10.12:8000",
			"__meta_consulagent_tagged_address_wan":             "198.18.1.2:80",
			"__meta_consulagent_tag_foo":                        "bar",
			"__meta_consulagent_tag_primary":                    "",
			"__meta_consulagent_tagpresent_foo":                 "true",
			"__meta_consulagent_tagpresent_primary":             "true",
			"__meta_consulagent_tags":                           ",primary,foo=bar,",
		}),
	}
	discoveryutils.TestEqualLabelss(t, labelss, expectedLabelss)
}
