package dockerswarm

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestParseTasks(t *testing.T) {
	f := func(data string, tasksExpected []task) {
		t.Helper()

		tasks, err := parseTasks([]byte(data))
		if err != nil {
			t.Fatalf("parseTasks() error: %s", err)
		}
		if !reflect.DeepEqual(tasks, tasksExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", tasks, tasksExpected)
		}
	}

	// parse ok
	data := `[
  {
    "ID": "t4rdm7j2y9yctbrksiwvsgpu5",
    "Version": {
      "Index": 23
    },
    "Spec": {
      "ContainerSpec": {
        "Image": "redis:3.0.6@sha256:6a692a76c2081888b589e26e6ec835743119fe453d67ecf03df7de5b73d69842",
        "Init": false,
        "Labels": {
	    "label1": "value1"
        }
      },
      "Resources": {
        "Limits": {},
        "Reservations": {}
      },
      "Placement": {
        "Platforms": [
          {
            "Architecture": "amd64",
            "OS": "linux"
          }
        ]
      },
      "ForceUpdate": 0
    },
    "ServiceID": "t91nf284wzle1ya09lqvyjgnq",
    "Slot": 1,
    "NodeID": "qauwmifceyvqs0sipvzu8oslu",
    "Status": {
      "State": "running",
      "ContainerStatus": {
        "ContainerID": "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
        "ExitCode": 0
      },
      "PortStatus": {}
    },
    "DesiredState": "running"
  }
]
`

	tasksExpected := []task{
		{
			ID:        "t4rdm7j2y9yctbrksiwvsgpu5",
			ServiceID: "t91nf284wzle1ya09lqvyjgnq",
			NodeID:    "qauwmifceyvqs0sipvzu8oslu",
			Spec: taskSpec{
				ContainerSpec: taskContainerSpec{
					Labels: map[string]string{
						"label1": "value1",
					},
				},
			},
			DesiredState: "running",
			Slot:         1,
			Status: taskStatus{
				State: "running",
				ContainerStatus: containerStatus{
					ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
				},
				PortStatus: portStatus{},
			},
		},
	}
	f(data, tasksExpected)
}

func TestAddTasksLabels(t *testing.T) {
	f := func(tasks []task, nodesLabels []*promutils.Labels, networkLabels map[string]*promutils.Labels, serviceLabels []*promutils.Labels, services []service, labelssExpected []*promutils.Labels) {
		t.Helper()

		labelss := addTasksLabels(tasks, nodesLabels, serviceLabels, networkLabels, services, 9100)
		discoveryutils.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// adds 1 task with nodes labels
	tasks := []task{
		{
			ID:           "t4rdm7j2y9yctbrksiwvsgpu5",
			ServiceID:    "t91nf284wzle1ya09lqvyjgnq",
			NodeID:       "qauwmifceyvqs0sipvzu8oslu",
			DesiredState: "running",
			Slot:         1,
			Status: taskStatus{
				State: "running",
				ContainerStatus: containerStatus{
					ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
				},
				PortStatus: portStatus{
					Ports: []portConfig{
						{
							PublishMode:   "ingress",
							Name:          "redis",
							Protocol:      "tcp",
							PublishedPort: 6379,
						},
					},
				}},
		},
	}

	svcLabels := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_service_id":   "t91nf284wzle1ya09lqvyjgnq",
			"__meta_dockerswarm_service_name": "real_service_name",
			"__meta_dockerswarm_service_mode": "real_service_mode",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_service_id":   "fake_service_id",
			"__meta_dockerswarm_service_name": "fake_service_name",
			"__meta_dockerswarm_service_mode": "fake_service_mode",
		}),
	}

	nodesLabels := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                   "172.31.40.97:9100",
			"__meta_dockerswarm_node_address":               "172.31.40.97",
			"__meta_dockerswarm_node_availability":          "active",
			"__meta_dockerswarm_node_engine_version":        "19.03.11",
			"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
			"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
			"__meta_dockerswarm_node_platform_architecture": "x86_64",
			"__meta_dockerswarm_node_platform_os":           "linux",
			"__meta_dockerswarm_node_role":                  "manager",
			"__meta_dockerswarm_node_status":                "ready",
		}),
	}

	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                   "172.31.40.97:6379",
			"__meta_dockerswarm_node_address":               "172.31.40.97",
			"__meta_dockerswarm_node_availability":          "active",
			"__meta_dockerswarm_node_engine_version":        "19.03.11",
			"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
			"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
			"__meta_dockerswarm_node_platform_architecture": "x86_64",
			"__meta_dockerswarm_node_platform_os":           "linux",
			"__meta_dockerswarm_node_role":                  "manager",
			"__meta_dockerswarm_node_status":                "ready",
			"__meta_dockerswarm_task_container_id":          "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
			"__meta_dockerswarm_task_desired_state":         "running",
			"__meta_dockerswarm_task_id":                    "t4rdm7j2y9yctbrksiwvsgpu5",
			"__meta_dockerswarm_task_port_publish_mode":     "ingress",
			"__meta_dockerswarm_task_slot":                  "1",
			"__meta_dockerswarm_task_state":                 "running",
			"__meta_dockerswarm_service_id":                 "t91nf284wzle1ya09lqvyjgnq",
			"__meta_dockerswarm_service_name":               "real_service_name",
			"__meta_dockerswarm_service_mode":               "real_service_mode",
		}),
	}
	f(tasks, nodesLabels, nil, svcLabels, nil, labelssExpected)

	//  adds 1 task with nodes, network and services labels
	tasks = []task{
		{
			ID:           "t4rdm7j2y9yctbrksiwvsgpu5",
			ServiceID:    "tgsci5gd31aai3jyudv98pqxf",
			NodeID:       "qauwmifceyvqs0sipvzu8oslu",
			DesiredState: "running",
			Slot:         1,
			NetworksAttachments: []networkAttachment{
				{
					Network: network{
						ID: "qs0hog6ldlei9ct11pr3c77v1",
					},
					Addresses: []string{"10.10.15.15/24"},
				},
			},
			Status: taskStatus{
				State: "running",
				ContainerStatus: containerStatus{
					ContainerID: "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
				},
				PortStatus: portStatus{},
			},
		},
	}

	networksLabels := map[string]*promutils.Labels{
		"qs0hog6ldlei9ct11pr3c77v1": promutils.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_network_id":         "qs0hog6ldlei9ct11pr3c77v1",
			"__meta_dockerswarm_network_ingress":    "true",
			"__meta_dockerswarm_network_internal":   "false",
			"__meta_dockerswarm_network_label_key1": "value1",
			"__meta_dockerswarm_network_name":       "ingress",
			"__meta_dockerswarm_network_scope":      "swarm",
		}),
	}

	nodesLabels = []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                   "172.31.40.97:9100",
			"__meta_dockerswarm_node_address":               "172.31.40.97",
			"__meta_dockerswarm_node_availability":          "active",
			"__meta_dockerswarm_node_engine_version":        "19.03.11",
			"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
			"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
			"__meta_dockerswarm_node_platform_architecture": "x86_64",
			"__meta_dockerswarm_node_platform_os":           "linux",
			"__meta_dockerswarm_node_role":                  "manager",
			"__meta_dockerswarm_node_status":                "ready",
		}),
	}

	svcLabels = []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_service_id":   "tgsci5gd31aai3jyudv98pqxf",
			"__meta_dockerswarm_service_name": "redis2",
			"__meta_dockerswarm_service_mode": "replicated",
		}),
		promutils.NewLabelsFromMap(map[string]string{
			"__meta_dockerswarm_service_id":   "fake_service_id",
			"__meta_dockerswarm_service_name": "fake_service_name",
			"__meta_dockerswarm_service_mode": "fake_service_mode",
		}),
	}

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
						Protocol:      "tcp",
						Name:          "redis",
						PublishMode:   "ingress",
						PublishedPort: 6379,
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

	labelssExpected = []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                                   "10.10.15.15:6379",
			"__meta_dockerswarm_network_id":                 "qs0hog6ldlei9ct11pr3c77v1",
			"__meta_dockerswarm_network_ingress":            "true",
			"__meta_dockerswarm_network_internal":           "false",
			"__meta_dockerswarm_network_label_key1":         "value1",
			"__meta_dockerswarm_network_name":               "ingress",
			"__meta_dockerswarm_network_scope":              "swarm",
			"__meta_dockerswarm_node_address":               "172.31.40.97",
			"__meta_dockerswarm_node_availability":          "active",
			"__meta_dockerswarm_node_engine_version":        "19.03.11",
			"__meta_dockerswarm_node_hostname":              "ip-172-31-40-97",
			"__meta_dockerswarm_node_id":                    "qauwmifceyvqs0sipvzu8oslu",
			"__meta_dockerswarm_node_platform_architecture": "x86_64",
			"__meta_dockerswarm_node_platform_os":           "linux",
			"__meta_dockerswarm_node_role":                  "manager",
			"__meta_dockerswarm_node_status":                "ready",
			"__meta_dockerswarm_task_container_id":          "33034b69f6fa5f808098208752fd1fe4e0e1ca86311988cea6a73b998cdc62e8",
			"__meta_dockerswarm_task_desired_state":         "running",
			"__meta_dockerswarm_task_id":                    "t4rdm7j2y9yctbrksiwvsgpu5",
			"__meta_dockerswarm_task_port_publish_mode":     "ingress",
			"__meta_dockerswarm_task_slot":                  "1",
			"__meta_dockerswarm_task_state":                 "running",
			"__meta_dockerswarm_service_id":                 "tgsci5gd31aai3jyudv98pqxf",
			"__meta_dockerswarm_service_name":               "redis2",
			"__meta_dockerswarm_service_mode":               "replicated",
		}),
	}
	f(tasks, nodesLabels, networksLabels, svcLabels, services, labelssExpected)
}
