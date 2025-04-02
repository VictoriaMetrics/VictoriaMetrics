package marathon

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

func TestGetAppsList_Success(t *testing.T) {
	s := newMockMarathonServer(func() []byte {
		return []byte(`{
  "apps": [
    {
      "id": "/myapp",
      "cmd": "env && sleep 60",
      "args": null,
      "user": null,
      "env": {
        "LD_LIBRARY_PATH": "/usr/local/lib/myLib"
      },
      "instances": 3,
      "cpus": 0.1,
      "mem": 5,
      "disk": 0,
      "executor": "",
      "constraints": [
        [
          "hostname",
          "UNIQUE",
          ""
        ]
      ],
      "uris": [
        "https://raw.github.com/mesosphere/marathon/master/README.md"
      ],
      "ports": [
        10013,
        10015
      ],
      "portDefinitions": [
         {
            "labels": {"pdl1":"pdl1", "pdl2":"pdl2"},
            "port": 1999
         }
      ],
      "requirePorts": false,
      "backoffSeconds": 1,
      "backoffFactor": 1.15,
      "maxLaunchDelaySeconds": 3600,
      "container": null,
      "healthChecks": [],
      "dependencies": [],
      "upgradeStrategy": {
        "minimumHealthCapacity": 1,
        "maximumOverCapacity": 1
      },
      "labels": {},
      "acceptedResourceRoles": null,
      "version": "2015-09-25T15:13:48.343Z",
      "versionInfo": {
        "lastScalingAt": "2015-09-25T15:13:48.343Z",
        "lastConfigChangeAt": "2015-09-25T15:13:48.343Z"
      },
      "tasksStaged": 0,
      "tasksRunning": 0,
      "tasksHealthy": 0,
      "tasksUnhealthy": 0,
      "deployments": [
        {
          "id": "9538079c-3898-4e32-aa31-799bf9097f74"
        }
      ]
    }
  ]
}`)
	})

	// Prepare a discovery HTTP client who calls mock server.
	client, err := discoveryutil.NewClient(s.URL, nil, nil, nil, &promauth.HTTPClientConfig{})
	if err != nil {
		t.Fatalf("unexpected error wen creating http client: %s", err)
	}
	ac := &apiConfig{
		cs: []*discoveryutil.Client{client},
	}

	apps, err := GetAppsList(ac)
	if err != nil {
		t.Fatalf("unexpected error in GetAppsList(): %s", err)
	}

	expect := &AppList{
		Apps: []app{
			{
				ID: "/myapp",
				PortDefinitions: []portDefinition{
					{
						Labels: map[string]string{"pdl1": "pdl1", "pdl2": "pdl2"},
						Port:   1999,
					},
				},
				Labels: map[string]string{},
			},
		},
	}
	if !reflect.DeepEqual(apps, expect) {
		t.Fatalf("unexpected result, got: %v, expect: %v", apps, expect)
	}
}
