package kubernetes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestGetAPIPathsWithNamespaces(t *testing.T) {
	f := func(role string, namespaces []string, selectors []Selector, expectedPaths, expectedNamespaces []string) {
		t.Helper()
		paths, resultNamespaces := getAPIPathsWithNamespaces(role, namespaces, selectors)
		if !reflect.DeepEqual(paths, expectedPaths) {
			t.Fatalf("unexpected paths; got\n%q\nwant\n%q", paths, expectedPaths)
		}
		if !reflect.DeepEqual(resultNamespaces, expectedNamespaces) {
			t.Fatalf("unexpected namespaces; got\n%q\nwant\n%q", resultNamespaces, expectedNamespaces)
		}
	}

	// role=node
	f("node", nil, nil, []string{"/api/v1/nodes"}, []string{""})
	f("node", []string{"foo", "bar"}, nil, []string{"/api/v1/nodes"}, []string{""})
	f("node", nil, []Selector{
		{
			Role:  "pod",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes"}, []string{""})
	f("node", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"}, []string{""})
	f("node", []string{"x", "y"}, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"}, []string{""})

	// role=pod
	f("pod", nil, nil, []string{"/api/v1/pods"}, []string{""})
	f("pod", []string{"foo", "bar"}, nil, []string{
		"/api/v1/namespaces/foo/pods",
		"/api/v1/namespaces/bar/pods",
	}, []string{"foo", "bar"})
	f("pod", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
	}, []string{"/api/v1/pods"}, []string{""})
	f("pod", nil, []Selector{
		{
			Role:  "pod",
			Label: "foo",
		},
		{
			Role:  "pod",
			Label: "x",
			Field: "y",
		},
	}, []string{"/api/v1/pods?labelSelector=foo%2Cx&fieldSelector=y"}, []string{""})
	f("pod", []string{"x", "y"}, []Selector{
		{
			Role:  "pod",
			Label: "foo",
		},
		{
			Role:  "pod",
			Label: "x",
			Field: "y",
		},
	}, []string{
		"/api/v1/namespaces/x/pods?labelSelector=foo%2Cx&fieldSelector=y",
		"/api/v1/namespaces/y/pods?labelSelector=foo%2Cx&fieldSelector=y",
	}, []string{"x", "y"})

	// role=service
	f("service", nil, nil, []string{"/api/v1/services"}, []string{""})
	f("service", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/services",
		"/api/v1/namespaces/y/services",
	}, []string{"x", "y"})
	f("service", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
		{
			Role:  "service",
			Field: "bar",
		},
	}, []string{"/api/v1/services?fieldSelector=bar"}, []string{""})
	f("service", []string{"x", "y"}, []Selector{
		{
			Role:  "service",
			Label: "abc=de",
		},
	}, []string{
		"/api/v1/namespaces/x/services?labelSelector=abc%3Dde",
		"/api/v1/namespaces/y/services?labelSelector=abc%3Dde",
	}, []string{"x", "y"})

	// role=endpoints
	f("endpoints", nil, nil, []string{"/api/v1/endpoints"}, []string{""})
	f("endpoints", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/endpoints",
		"/api/v1/namespaces/y/endpoints",
	}, []string{"x", "y"})
	f("endpoints", []string{"x", "y"}, []Selector{
		{
			Role:  "endpoints",
			Label: "bbb",
		},
		{
			Role:  "node",
			Label: "aa",
		},
	}, []string{
		"/api/v1/namespaces/x/endpoints?labelSelector=bbb",
		"/api/v1/namespaces/y/endpoints?labelSelector=bbb",
	}, []string{"x", "y"})

	// role=endpointslices
	f("endpointslices", nil, nil, []string{"/apis/discovery.k8s.io/v1beta1/endpointslices"}, []string{""})
	f("endpointslices", []string{"x", "y"}, []Selector{
		{
			Role:  "endpointslices",
			Field: "field",
			Label: "label",
		},
	}, []string{
		"/apis/discovery.k8s.io/v1beta1/namespaces/x/endpointslices?labelSelector=label&fieldSelector=field",
		"/apis/discovery.k8s.io/v1beta1/namespaces/y/endpointslices?labelSelector=label&fieldSelector=field",
	}, []string{"x", "y"})

	// role=ingress
	f("ingress", nil, nil, []string{"/apis/networking.k8s.io/v1beta1/ingresses"}, []string{""})
	f("ingress", []string{"x", "y"}, []Selector{
		{
			Role:  "node",
			Field: "xyay",
		},
		{
			Role:  "ingress",
			Field: "abc",
		},
		{
			Role:  "ingress",
			Label: "cde",
		},
		{
			Role:  "ingress",
			Label: "baaa",
		},
	}, []string{
		"/apis/networking.k8s.io/v1beta1/namespaces/x/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
		"/apis/networking.k8s.io/v1beta1/namespaces/y/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
	}, []string{"x", "y"})
}

func TestParseBookmark(t *testing.T) {
	data := `{"kind": "Pod", "apiVersion": "v1", "metadata": {"resourceVersion": "12746"} }`
	bm, err := parseBookmark([]byte(data))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "12746"
	if bm.Metadata.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resourceVersion; got %q; want %q", bm.Metadata.ResourceVersion, expectedResourceVersion)
	}
}

func TestGetScrapeWorkObjects(t *testing.T) {
	type testCase struct {
		name        string
		sdc         *SDConfig
		expectedLen int
		initObjects map[string][]byte
		// will be added for watching api.
		nextAdd map[string][][]byte
	}
	cases := []testCase{
		{
			name: "simple 1 pod with update 1",
			sdc: &SDConfig{
				Role: "pod",
			},
			expectedLen: 2,
			initObjects: map[string][]byte{
				"pod": []byte(`{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "72425"
  },
  "items": [
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "app.kubernetes.io/instance": "stack",
            "pod-template-hash": "5b9c6cf775"
        },
        "name": "stack-name-1",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
               "name": "generic-pod"
            }
        ]
    },
    "status": {
        "podIP": "10.10.2.2",
        "phase": "Running"
    }
}]}`),
			},
			nextAdd: map[string][][]byte{
				"pod": {
					[]byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "app.kubernetes.io/instance": "stack",
            "pod-template-hash": "5b9c6cf775"
        },
        "name": "stack-next-2",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
               "name": "generic-pod-2"
            }
        ]
    },
    "status": {
        "podIP": "10.10.2.5",
        "phase": "Running"
    }
}`),
				},
			},
		},
		{
			name: "endpoints with service update",
			sdc: &SDConfig{
				Role: "endpoints",
			},
			expectedLen: 2,
			initObjects: map[string][]byte{
				"service": []byte(`{
  "kind": "ServiceList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "72425"
  },
  "items": []}`),
				"endpoints": []byte(`{
  "kind": "EndpointsList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "72425"
  },
  "items": [
{
    "apiVersion": "v1",
    "kind": "Endpoints",
    "metadata": {
        "annotations": {
            "endpoints.kubernetes.io/last-change-trigger-time": "2021-04-27T02:06:55Z"
        },
        "labels": {
            "app.kubernetes.io/managed-by": "Helm"
        },
        "name": "stack-kube-state-metrics",
        "namespace": "default"
    },
    "subsets": [
        {
            "addresses": [
                {
                    "ip": "10.244.0.5",
                    "nodeName": "kind-control-plane",
                    "targetRef": {
                        "kind": "Pod",
                        "name": "stack-kube-state-metrics-db5879bf8-bg78p",
                        "namespace": "default"
                    }
                }
            ],
            "ports": [
                {
                    "name": "http",
                    "port": 8080,
                    "protocol": "TCP"
                }
            ]
        }
    ]
}
]}`),
				"pod": []byte(`{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {
    "resourceVersion": "72425"
  },
  "items": [
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "app.kubernetes.io/instance": "stack"
        },
        "name": "stack-kube-state-metrics-db5879bf8-bg78p",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "image": "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v1.9.8",
                "name": "kube-state-metrics",
                "ports": [
                    {
                        "containerPort": 8080,
                        "protocol": "TCP"
                    }
                ]
            },
            {
                "image": "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v1.9.8",
                "name": "kube-state-metrics-2",
                "ports": [
                    {
                        "containerPort": 8085,
                        "protocol": "TCP"
                    }
                ]
            }
        ]
    },
    "status": {
        "phase": "Running",
        "podIP": "10.244.0.5"
    }
}
]}`),
			},
			nextAdd: map[string][][]byte{
				"service": {
					[]byte(`{
    "apiVersion": "v1",
    "kind": "Service",
    "metadata": {
        "annotations": {
            "meta.helm.sh/release-name": "stack"
        },
        "labels": {
            "app.kubernetes.io/managed-by": "Helm",
            "app.kubernetes.io/name": "kube-state-metrics"
        },
        "name": "stack-kube-state-metrics",
        "namespace": "default"
    },
    "spec": {
        "clusterIP": "10.97.109.249",
        "ports": [
            {
                "name": "http",
                "port": 8080,
                "protocol": "TCP",
                "targetPort": 8080
            }
        ],
        "selector": {
            "app.kubernetes.io/instance": "stack",
            "app.kubernetes.io/name": "kube-state-metrics"
        },
        "type": "ClusterIP"
    }
}`),
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			updatesByRole := make(map[string]chan []byte)
			mux := http.NewServeMux()
			for role, obj := range tc.initObjects {
				watchCh := make(chan []byte, 0)
				updatesByRole[role] = watchCh
				apiPath := getAPIPath(getObjectTypeByRole(role), "", "")
				addApiUrlHandler(t, mux, apiPath, obj, watchCh)
			}
			srv := httptest.NewServer(mux)
			tc.sdc.APIServer = srv.URL
			ac, err := newAPIConfig(tc.sdc, "", func(metaLabels map[string]string) interface{} {
				var simpleResult []interface{}
				for k := range metaLabels {
					simpleResult = append(simpleResult, k)
				}
				return simpleResult
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.sdc.cfg = ac
			ac.aw.mustStart()
			defer ac.aw.mustStop()
			got, err := tc.sdc.GetScrapeWorkObjects()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for role, objs := range tc.nextAdd {
				for _, obj := range objs {
					updatesByRole[role] <- obj
				}
			}
			for _, ch := range updatesByRole {
				close(ch)
			}
			if len(tc.nextAdd) > 0 {
				// updates async, need to wait some time.
				// i guess, poll is not reliable.
				time.Sleep(500 * time.Millisecond)
			}
			got, err = tc.sdc.GetScrapeWorkObjects()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.expectedLen {
				t.Fatalf("unexpected count of objects, got: %d, want: %d", len(got), tc.expectedLen)
			}
		})
	}
}

func addApiUrlHandler(t *testing.T, mux *http.ServeMux, apiUrl string, initObjects []byte, updateObjects chan []byte) {
	t.Helper()
	mux.HandleFunc(apiUrl, func(w http.ResponseWriter, r *http.Request) {
		if needWatch := r.URL.Query().Get("watch"); len(needWatch) > 0 {
			// start watch handler
			w.WriteHeader(200)
			flusher := w.(http.Flusher)
			flusher.Flush()
			for obj := range updateObjects {
				we := WatchEvent{
					Type:   "ADDED",
					Object: obj,
				}
				szd, err := json.Marshal(we)
				if err != nil {
					t.Fatalf("cannot serialize: %v", err)
				}
				_, _ = w.Write(szd)
				flusher.Flush()
			}
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(initObjects)
	})
}
