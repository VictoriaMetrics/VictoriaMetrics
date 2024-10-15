package marathon

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetAppLabels(t *testing.T) {
	a := app{
		ID: "test-service",
		Tasks: []task{
			{
				ID:   "test-task-1",
				Host: "mesos-slave1",
			},
		},
		RunningTasks: 1,
		Labels:       map[string]string{"prometheus": "yes"},
		Container: container{
			Docker: dockerContainer{
				Image: "repo/image:tag",
			},
			PortMappings: []portMapping{
				{
					Labels:   map[string]string{"prometheus": "yes"},
					HostPort: 31000,
				},
			},
		},
	}

	expect := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__meta_marathon_app":                           "test-service",
			"__meta_marathon_image":                         "repo/image:tag",
			"__address__":                                   "mesos-slave1:31000",
			"__meta_marathon_task":                          "test-task-1",
			"__meta_marathon_port_index":                    "0",
			"__meta_marathon_port_mapping_label_prometheus": "yes",
			"__meta_marathon_app_label_prometheus":          "yes",
		}),
	}

	labelList := getAppsLabels(&AppList{Apps: []app{a}})
	discoveryutils.TestEqualLabelss(t, labelList, expect)
}
