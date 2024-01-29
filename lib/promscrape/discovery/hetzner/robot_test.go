package hetzner

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseRobotServerListResponse(t *testing.T) {
	data := `[
		{
		  "server":{
			"server_ip":"123.123.123.123",
			"server_ipv6_net":"2a01:f48:111:4221::",
			"server_number":321,
			"server_name":"server1",
			"product":"DS 3000",
			"dc":"NBG1-DC1",
			"traffic":"5 TB",
			"status":"ready",
			"cancelled":false,
			"paid_until":"2010-09-02",
			"ip":[
			  "123.123.123.123"
			],
			"subnet":[
			  {
				"ip":"2a01:4f8:111:4221::",
				"mask":"64"
			  }
			]
		  }
		},
		{
		  "server":{
			"server_ip":"123.123.123.124",
			"server_ipv6_net":"2a01:f48:111:4221::",
			"server_number":421,
			"server_name":"server2",
			"product":"X5",
			"dc":"FSN1-DC10",
			"traffic":"2 TB",
			"status":"ready",
			"cancelled":false,
			"paid_until":"2010-06-11",
			"ip":[
			  "123.123.123.124"
			],
			"subnet":null
		  }
		}
	  ]
`
	rsl, err := parseRobotServers([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error parseRobotServersList when parsing data: %s", err)
	}
	rslExpected := []RobotServer{
		{
			ServerIP:     "123.123.123.123",
			ServerIPV6:   "2a01:f48:111:4221::",
			ServerNumber: 321,
			ServerName:   "server1",
			Product:      "DS 3000",
			DC:           "NBG1-DC1",
			Status:       "ready",
			Canceled:     false,
			Subnet: []RobotSubnet{
				{
					IP:   "2a01:4f8:111:4221::",
					Mask: "64",
				},
			},
		},
		{
			ServerIP:     "123.123.123.124",
			ServerIPV6:   "2a01:f48:111:4221::",
			ServerNumber: 421,
			ServerName:   "server2",
			Product:      "X5",
			DC:           "FSN1-DC10",
			Status:       "ready",
			Canceled:     false,
			Subnet:       nil,
		},
	}
	if !reflect.DeepEqual(rsl, rslExpected) {
		t.Fatalf("unexpected parseRobotServersList parsed;\ngot\n%+v\nwant\n%+v", rsl, rslExpected)
	}

	port := 123
	labelss := appendRobotTargetLabels(nil, &rsl[0], port)

	expectedLabels := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                        "123.123.123.123:123",
			"__meta_hetzner_role":                "robot",
			"__meta_hetzner_server_id":           "321",
			"__meta_hetzner_server_name":         "server1",
			"__meta_hetzner_server_status":       "ready",
			"__meta_hetzner_public_ipv4":         "123.123.123.123",
			"__meta_hetzner_public_ipv6_network": "2a01:4f8:111:4221::/64",
			"__meta_hetzner_datacenter":          "nbg1-dc1",
			"__meta_hetzner_robot_product":       "DS 3000",
			"__meta_hetzner_robot_cancelled":     "false",
		}),
	}
	discoveryutils.TestEqualLabelss(t, labelss, expectedLabels)
}
