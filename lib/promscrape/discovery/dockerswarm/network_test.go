package dockerswarm

import (
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func Test_addNetworkLabels(t *testing.T) {
	type args struct {
		networks []network
	}
	tests := []struct {
		name string
		args args
		want []*promutils.Labels
	}{
		{
			name: "ingress network",
			args: args{
				networks: []network{
					{
						ID:      "qs0hog6ldlei9ct11pr3c77v1",
						Ingress: true,
						Scope:   "swarm",
						Name:    "ingress",
						Labels: map[string]string{
							"key1": "value1",
						},
					},
				},
			},
			want: []*promutils.Labels{
				promutils.NewLabelsFromMap(map[string]string{
					"__meta_dockerswarm_network_id":         "qs0hog6ldlei9ct11pr3c77v1",
					"__meta_dockerswarm_network_ingress":    "true",
					"__meta_dockerswarm_network_internal":   "false",
					"__meta_dockerswarm_network_label_key1": "value1",
					"__meta_dockerswarm_network_name":       "ingress",
					"__meta_dockerswarm_network_scope":      "swarm",
				})},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNetworkLabelsByNetworkID(tt.args.networks)
			var networkIDs []string
			for networkID := range got {
				networkIDs = append(networkIDs, networkID)
			}
			sort.Strings(networkIDs)
			var labelss []*promutils.Labels
			for _, networkID := range networkIDs {
				labelss = append(labelss, got[networkID])
			}
			discoveryutils.TestEqualLabelss(t, labelss, tt.want)
		})
	}
}

func Test_parseNetworks(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []network
		wantErr bool
	}{
		{
			name: "parse two networks",
			args: args{
				data: []byte(`[
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
]`),
			},
			want: []network{
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseNetworks(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseNetworks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseNetworks() \ngot  %v, \nwant %v", got, tt.want)
			}
		})
	}
}
