package docker

import (
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestAddNetworkLabels(t *testing.T) {
	f := func(networks []network, labelssExpected []*promutil.Labels) {
		t.Helper()

		networkLabels := getNetworkLabelsByNetworkID(networks)

		var networkIDs []string
		for networkID := range networkLabels {
			networkIDs = append(networkIDs, networkID)
		}
		sort.Strings(networkIDs)
		var labelss []*promutil.Labels
		for _, networkID := range networkIDs {
			labelss = append(labelss, networkLabels[networkID])
		}
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// ingress network
	networks := []network{
		{
			ID:      "qs0hog6ldlei9ct11pr3c77v1",
			Ingress: true,
			Scope:   "swarm",
			Name:    "ingress",
			Labels: map[string]string{
				"key1": "value1",
			},
		},
	}
	labelssExpected := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__meta_docker_network_id":         "qs0hog6ldlei9ct11pr3c77v1",
			"__meta_docker_network_ingress":    "true",
			"__meta_docker_network_internal":   "false",
			"__meta_docker_network_label_key1": "value1",
			"__meta_docker_network_name":       "ingress",
			"__meta_docker_network_scope":      "swarm",
		}),
	}
	f(networks, labelssExpected)
}

func TestParseNetworks(t *testing.T) {
	f := func(data string, resultExpected []network) {
		t.Helper()

		result, err := parseNetworks([]byte(data))
		if err != nil {
			t.Fatalf("parseNetworks() error: %s", err)
		}
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected networks\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	// parse two networks
	data := `[
  {
    "Name": "ingress",
    "Id": "qs0hog6ldlei9ct11pr3c77v1",
    "Created": "2020-10-06T08:39:58.957083331Z",
    "Scope": "swarm",
    "Driver": "overlay",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": [
        {
          "Subnet": "10.0.0.0/24",
          "Gateway": "10.0.0.1"
        }
      ]
    },
    "Internal": false,
    "Attachable": false,
    "Ingress": true,
    "ConfigFrom": {
      "Network": ""
    },
    "ConfigOnly": false,
    "Containers": null,
    "Options": {
      "com.docker.network.driver.overlay.vxlanid_list": "4096"
    },
    "Labels": {
      "key1": "value1"
    }
  },
  {
    "Name": "host",
    "Id": "317f0384d7e5f5c26304a0b04599f9f54bc08def4d0535059ece89955e9c4b7b",
    "Created": "2020-10-06T08:39:52.843373136Z",
    "Scope": "local",
    "Driver": "host",
    "EnableIPv6": false,
    "IPAM": {
      "Driver": "default",
      "Options": null,
      "Config": []
    },
    "Internal": false,
    "Attachable": false,
    "Ingress": false,
    "ConfigFrom": {
      "Network": ""
    },
    "ConfigOnly": false,
    "Containers": {},
    "Options": {},
    "Labels": {
      "key": "value"
    }
  }
]`
	resultExpected := []network{
		{
			ID:      "qs0hog6ldlei9ct11pr3c77v1",
			Ingress: true,
			Scope:   "swarm",
			Name:    "ingress",
			Labels: map[string]string{
				"key1": "value1",
			},
		},
		{
			ID:    "317f0384d7e5f5c26304a0b04599f9f54bc08def4d0535059ece89955e9c4b7b",
			Scope: "local",
			Name:  "host",
			Labels: map[string]string{
				"key": "value",
			},
		},
	}
	f(data, resultExpected)
}
