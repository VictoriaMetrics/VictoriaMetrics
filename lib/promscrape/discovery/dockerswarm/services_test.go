package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func TestParseServicesResponse(t *testing.T) {
	f := func(data string, servicesExpected []service) {
		t.Helper()

		services, err := parseServicesResponse([]byte(data))
		if err != nil {
			t.Fatalf("parseServicesResponse() error: %s", err)
		}
		if !reflect.DeepEqual(services, servicesExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", services, servicesExpected)
		}
	}

	// parse ok
	data := `[
  {
    "ID": "tgsci5gd31aai3jyudv98pqxf",
    "Version": {
      "Index": 25
    },
    "CreatedAt": "2020-10-06T11:17:31.948808444Z",
    "UpdatedAt": "2020-10-06T11:17:31.950195138Z",
    "Spec": {
      "Name": "redis2",
      "Labels": {},
      "TaskTemplate": {
        "ContainerSpec": {
          "Image": "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
          "Init": false,
          "DNSConfig": {},
          "Isolation": "default"
        },
        "Resources": {
          "Limits": {},
          "Reservations": {}
        }
      },
      "Mode": {
        "Replicated": {}
      },
      "EndpointSpec": {
        "Mode": "vip",
        "Ports": [
          {
            "Protocol": "tcp",
            "TargetPort": 6379,
            "PublishedPort": 8081,
            "PublishMode": "ingress"
          }
        ]
      }
    },
    "Endpoint": {
      "Spec": {
        "Mode": "vip",
        "Ports": [
          {
            "Protocol": "tcp",
            "TargetPort": 6379,
            "PublishedPort": 8081,
            "PublishMode": "ingress"
          }
        ]
      },
      "Ports": [
        {
          "Protocol": "tcp",
          "TargetPort": 6379,
          "PublishedPort": 8081,
          "PublishMode": "ingress"
        }
      ],
      "VirtualIPs": [
        {
          "NetworkID": "qs0hog6ldlei9ct11pr3c77v1",
          "Addr": "10.0.0.3/24"
        }
      ]
    }
  }
]`

	servicesExpected := []service{
		{
			ID: "tgsci5gd31aai3jyudv98pqxf",
			Spec: serviceSpec{
				Labels: map[string]string{},
				Name:   "redis2",
				TaskTemplate: taskTemplate{
					ContainerSpec: containerSpec{
						Image: "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
					},
				},
				Mode: serviceSpecMode{
					Replicated: map[string]any{},
				},
			},
			Endpoint: serviceEndpoint{
				Ports: []portConfig{
					{
						Protocol:      "tcp",
						PublishMode:   "ingress",
						PublishedPort: 8081,
					},
				},
				VirtualIPs: []virtualIP{
					{
						NetworkID: "qs0hog6ldlei9ct11pr3c77v1",
						Addr:      "10.0.0.3/24",
					},
				},
			},
		},
	}
	f(data, servicesExpected)
}

func TestAddServicesLabels(t *testing.T) {
	f := func(services []service, networksLabels map[string]*promutil.Labels, labelssExpected []*promutil.Labels) {
		t.Helper()

		labelss := addServicesLabels(services, networksLabels, 9100)
		discoveryutil.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// add 2 services with network labels join
	services := []service{
		{
			ID: "tgsci5gd31aai3jyudv98pqxf",
			Spec: serviceSpec{
				Labels: map[string]string{},
				Name:   "redis2",
				TaskTemplate: taskTemplate{
					ContainerSpec: containerSpec{
						Hostname: "node1",
						Image:    "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
					},
				},
				Mode: serviceSpecMode{
					Replicated: map[string]any{},
				},
			},
			Endpoint: serviceEndpoint{
				Ports: []portConfig{
					{
						Protocol:    "tcp",
						Name:        "redis",
						PublishMode: "ingress",
					},
				},
				VirtualIPs: []virtualIP{
					{
						NetworkID: "qs0hog6ldlei9ct11pr3c77v1",
						Addr:      "10.0.0.3/24",
					},
				},
			},
		},
	}
	networksLabels := map[string]*promutil.Labels{
		"qs0hog6ldlei9ct11pr3c77v1": promutil.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_network_id":         "qs0hog6ldlei9ct11pr3c77v1",
			"__meta_dockerswarm_network_ingress":    "true",
			"__meta_dockerswarm_network_internal":   "false",
			"__meta_dockerswarm_network_label_key1": "value1",
			"__meta_dockerswarm_network_name":       "ingress",
			"__meta_dockerswarm_network_scope":      "swarm",
		}),
	}
	labelssExpected := []*promutil.Labels{
		promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                           "10.0.0.3:0",
			"__meta_dockerswarm_network_id":                         "qs0hog6ldlei9ct11pr3c77v1",
			"__meta_dockerswarm_network_ingress":                    "true",
			"__meta_dockerswarm_network_internal":                   "false",
			"__meta_dockerswarm_network_label_key1":                 "value1",
			"__meta_dockerswarm_network_name":                       "ingress",
			"__meta_dockerswarm_network_scope":                      "swarm",
			"__meta_dockerswarm_service_endpoint_port_name":         "redis",
			"__meta_dockerswarm_service_endpoint_port_publish_mode": "ingress",
			"__meta_dockerswarm_service_id":                         "tgsci5gd31aai3jyudv98pqxf",
			"__meta_dockerswarm_service_mode":                       "replicated",
			"__meta_dockerswarm_service_name":                       "redis2",
			"__meta_dockerswarm_service_task_container_hostname":    "node1",
			"__meta_dockerswarm_service_task_container_image":       "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
			"__meta_dockerswarm_service_updating_status":            "",
		}),
	}
	f(services, networksLabels, labelssExpected)
}
