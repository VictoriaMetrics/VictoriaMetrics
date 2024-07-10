package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseNodes(t *testing.T) {
	f := func(data string, resultExpected []node) {
		t.Helper()

		result, err := parseNodes([]byte(data))
		if err != nil {
			t.Fatalf("parseNodes() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result;\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	// parse ok
	data := `[
  {
    "ID": "qauwmifceyvqs0sipvzu8oslu",
    "Version": {
      "Index": 16
    },
    "Spec": {
      "Role": "manager",
      "Availability": "active"
    },
    "Description": {
      "Hostname": "ip-172-31-40-97",
      "Platform": {
        "Architecture": "x86_64",
        "OS": "linux"
      },
      "Resources": {
        "NanoCPUs": 1000000000,
        "MemoryBytes": 1026158592
      },
      "Engine": {
        "EngineVersion": "19.03.11"
      }
    },
    "Status": {
      "State": "ready",
      "Addr": "172.31.40.97"
    }
  }
]
`
	resultExpected := []node{
		{
			ID: "qauwmifceyvqs0sipvzu8oslu",
			Spec: nodeSpec{
				Role:         "manager",
				Availability: "active",
			},
			Status: nodeStatus{
				State: "ready",
				Addr:  "172.31.40.97",
			},
			Description: nodeDescription{
				Hostname: "ip-172-31-40-97",
				Platform: nodePlatform{
					Architecture: "x86_64",
					OS:           "linux",
				},
				Engine: nodeEngine{
					EngineVersion: "19.03.11",
				},
			},
		},
	}
	f(data, resultExpected)
}

func TestAddNodeLabels(t *testing.T) {
	f := func(nodes []node, port int, resultExpected []*promutils.Labels) {
		t.Helper()

		result := addNodeLabels(nodes, port)
		discoveryutils.TestEqualLabelss(t, result, resultExpected)
	}

	// add labels to one node
	nodes := []node{
		{
			ID: "qauwmifceyvqs0sipvzu8oslu",
			Spec: nodeSpec{
				Role:         "manager",
				Availability: "active",
			},
			Status: nodeStatus{
				State: "ready",
				Addr:  "172.31.40.97",
			},
			Description: nodeDescription{
				Hostname: "ip-172-31-40-97",
				Platform: nodePlatform{
					Architecture: "x86_64",
					OS:           "linux",
				},
				Engine: nodeEngine{
					EngineVersion: "19.03.11",
				},
			},
		},
	}
	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                   "172.31.40.97:9100",
			"__meta_dockerswarm_node_address":               "172.31.40.97",
			"__meta_dockerswarm_node_availability":          "active",
			"__meta_dockerswarm_node_engine_version":        "19.03.11",
			"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
			"__meta_dockerswarm_node_manager_address":       "",
			"__meta_dockerswarm_node_manager_leader":        "false",
			"__meta_dockerswarm_node_manager_reachability":  "",
			"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
			"__meta_dockerswarm_node_platform_architecture": "x86_64",
			"__meta_dockerswarm_node_platform_os":           "linux",
			"__meta_dockerswarm_node_role":                  "manager",
			"__meta_dockerswarm_node_status":                "ready",
		}),
	}
	f(nodes, 9100, labelssExpected)
}
