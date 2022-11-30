package kubernetes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestGetAPIPathsWithNamespaces(t *testing.T) {
	f := func(role string, namespaces []string, selectors []Selector, expectedPaths []string) {
		t.Helper()
		paths := getAPIPathsWithNamespaces(role, namespaces, selectors)
		if !reflect.DeepEqual(paths, expectedPaths) {
			t.Fatalf("unexpected paths; got\n%q\nwant\n%q", paths, expectedPaths)
		}
	}

	// role=node
	f("node", nil, nil, []string{"/api/v1/nodes"})
	f("node", []string{"foo", "bar"}, nil, []string{"/api/v1/nodes"})
	f("node", nil, []Selector{
		{
			Role:  "pod",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes"})
	f("node", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"})
	f("node", []string{"x", "y"}, []Selector{
		{
			Role:  "node",
			Label: "foo",
			Field: "bar",
		},
	}, []string{"/api/v1/nodes?labelSelector=foo&fieldSelector=bar"})

	// role=pod
	f("pod", nil, nil, []string{"/api/v1/pods"})
	f("pod", []string{"foo", "bar"}, nil, []string{
		"/api/v1/namespaces/foo/pods",
		"/api/v1/namespaces/bar/pods",
	})
	f("pod", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
	}, []string{"/api/v1/pods"})
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
	}, []string{"/api/v1/pods?labelSelector=foo%2Cx&fieldSelector=y"})
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
	})

	// role=service
	f("service", nil, nil, []string{"/api/v1/services"})
	f("service", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/services",
		"/api/v1/namespaces/y/services",
	})
	f("service", nil, []Selector{
		{
			Role:  "node",
			Label: "foo",
		},
		{
			Role:  "service",
			Field: "bar",
		},
	}, []string{"/api/v1/services?fieldSelector=bar"})
	f("service", []string{"x", "y"}, []Selector{
		{
			Role:  "service",
			Label: "abc=de",
		},
	}, []string{
		"/api/v1/namespaces/x/services?labelSelector=abc%3Dde",
		"/api/v1/namespaces/y/services?labelSelector=abc%3Dde",
	})

	// role=endpoints
	f("endpoints", nil, nil, []string{"/api/v1/endpoints"})
	f("endpoints", []string{"x", "y"}, nil, []string{
		"/api/v1/namespaces/x/endpoints",
		"/api/v1/namespaces/y/endpoints",
	})
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
	})

	// role=endpointslice
	f("endpointslice", nil, nil, []string{"/apis/discovery.k8s.io/v1/endpointslices"})
	f("endpointslice", []string{"x", "y"}, []Selector{
		{
			Role:  "endpointslice",
			Field: "field",
			Label: "label",
		},
	}, []string{
		"/apis/discovery.k8s.io/v1/namespaces/x/endpointslices?labelSelector=label&fieldSelector=field",
		"/apis/discovery.k8s.io/v1/namespaces/y/endpointslices?labelSelector=label&fieldSelector=field",
	})

	// role=ingress
	f("ingress", nil, nil, []string{"/apis/networking.k8s.io/v1/ingresses"})
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
		"/apis/networking.k8s.io/v1/namespaces/x/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
		"/apis/networking.k8s.io/v1/namespaces/y/ingresses?labelSelector=cde%2Cbaaa&fieldSelector=abc",
	})
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
		name                 string
		sdc                  *SDConfig
		expectedTargetsLen   int
		initAPIObjectsByRole map[string][]byte
		// will be added for watching api.
		watchAPIMustAddObjectsByRole map[string][][]byte
	}
	cases := []testCase{
		{
			name: "simple 1 pod with update 1",
			sdc: &SDConfig{
				Role: "pod",
			},
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string][]byte{
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
			watchAPIMustAddObjectsByRole: map[string][][]byte{
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
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string][]byte{
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
			watchAPIMustAddObjectsByRole: map[string][][]byte{
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
		{
			name:               "get nodes",
			sdc:                &SDConfig{Role: "node"},
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string][]byte{
				"node": []byte(`{
  "kind": "NodeList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/nodes",
    "resourceVersion": "22627"
  },
  "items": [
{
  "apiVersion": "v1",
  "kind": "Node",
  "metadata": {
    "annotations": {
      "kubeadm.alpha.kubernetes.io/cri-socket": "/run/containerd/containerd.sock"
    },
    "labels": {
      "beta.kubernetes.io/arch": "amd64",
      "beta.kubernetes.io/os": "linux"
    },
    "name": "kind-control-plane-new"
  },
  "status": {
    "addresses": [
      {
        "address": "10.10.2.5",
        "type": "InternalIP"
      },
      {
        "address": "kind-control-plane",
        "type": "Hostname"
      }
    ]
  }
}
]}`),
			},
			watchAPIMustAddObjectsByRole: map[string][][]byte{
				"node": {
					[]byte(`{
  "apiVersion": "v1",
  "kind": "Node",
  "metadata": {
    "annotations": {
      "kubeadm.alpha.kubernetes.io/cri-socket": "/run/containerd/containerd.sock"
    },
    "labels": {
      "beta.kubernetes.io/arch": "amd64",
      "beta.kubernetes.io/os": "linux"
    },
    "name": "kind-control-plane"
  },
  "status": {
    "addresses": [
      {
        "address": "10.10.2.2",
        "type": "InternalIP"
      },
      {
        "address": "kind-control-plane",
        "type": "Hostname"
      }
    ]
  }
}`),
				},
			},
		},
		{
			name:               "2 service with 2 added",
			sdc:                &SDConfig{Role: "service"},
			expectedTargetsLen: 4,
			initAPIObjectsByRole: map[string][]byte{
				"service": []byte(`{
  "kind": "ServiceList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/services",
    "resourceVersion": "60485"
  },
  "items": [
    {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "labels": {
          "k8s-app": "kube-dns"
        }
      },
      "spec": {
        "ports": [
          {
            "name": "dns",
            "protocol": "UDP",
            "port": 53,
            "targetPort": 53
          },
          {
            "name": "dns-tcp",
            "protocol": "TCP",
            "port": 53,
            "targetPort": 53
          }
        ],
        "selector": {
          "k8s-app": "kube-dns"
        },
        "clusterIP": "10.96.0.10",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      }
    }
  ]
}`),
			},
			watchAPIMustAddObjectsByRole: map[string][][]byte{
				"service": {
					[]byte(`{
  "metadata": {
    "name": "another-service-1",
    "namespace": "default",
    "labels": {
      "k8s-app": "kube-dns"
    }
  },
  "spec": {
    "ports": [
      {
        "name": "some-app-1-tcp",
        "protocol": "TCP",
        "port": 1053,
        "targetPort": 1053
      }
    ],
    "selector": {
      "k8s-app": "some-app-1"
    },
    "clusterIP": "10.96.0.10",
    "type": "ClusterIP"
  }
}`),
					[]byte(`{
  "metadata": {
    "name": "another-service-2",
    "namespace": "default",
    "labels": {
      "k8s-app": "kube-dns"
    }
  },
  "spec": {
    "ports": [
      {
        "name": "some-app-2-tcp",
        "protocol": "TCP",
        "port": 1053,
        "targetPort": 1053
      }
    ],
    "selector": {
      "k8s-app": "some-app-2"
    },
    "clusterIP": "10.96.0.15",
    "type": "ClusterIP"
  }
}`),
				},
			},
		},
		{
			name:               "1 ingress with 2 add",
			expectedTargetsLen: 3,
			sdc: &SDConfig{
				Role: "ingress",
			},
			initAPIObjectsByRole: map[string][]byte{
				"ingress": []byte(`{
  "kind": "IngressList",
  "apiVersion": "extensions/v1",
  "metadata": {
    "selfLink": "/apis/extensions/v1/ingresses",
    "resourceVersion": "351452"
  },
  "items": [
    {
      "metadata": {
        "name": "test-ingress",
        "namespace": "default"
      },
      "spec": {
        "backend": {
          "serviceName": "testsvc",
          "servicePort": 80
        },
        "rules": [
          {
            "host": "foobar"
          }
        ]
      },
      "status": {
        "loadBalancer": {
          "ingress": [
            {
              "ip": "172.17.0.2"
            }
          ]
        }
      }
    }
  ]
}`),
			},
			watchAPIMustAddObjectsByRole: map[string][][]byte{
				"ingress": {
					[]byte(`{
  "metadata": {
    "name": "test-ingress-1",
    "namespace": "default"
  },
  "spec": {
    "backend": {
      "serviceName": "testsvc",
      "servicePort": 801
    },
    "rules": [
      {
        "host": "foobar"
      }
    ]
  },
  "status": {
    "loadBalancer": {
      "ingress": [
        {
          "ip": "172.17.0.3"
        }
      ]
    }
  }
}`),
					[]byte(`{
  "metadata": {
    "name": "test-ingress-2",
    "namespace": "default"
  },
  "spec": {
    "backend": {
      "serviceName": "testsvc",
      "servicePort": 802
    },
    "rules": [
      {
        "host": "foobar"
      }
    ]
  },
  "status": {
    "loadBalancer": {
      "ingress": [
        {
          "ip": "172.17.0.3"
        }
      ]
    }
  }
}`),
				},
			},
		},
		{
			name: "7 endpointslices slice with 1 service update",
			sdc: &SDConfig{
				Role: "endpointslice",
			},
			expectedTargetsLen: 7,
			initAPIObjectsByRole: map[string][]byte{
				"endpointslice": []byte(`{
  "kind": "EndpointSliceList",
  "apiVersion": "discovery.k8s.io/v1",
  "metadata": {
    "selfLink": "/apis/discovery.k8s.io/v1/endpointslices",
    "resourceVersion": "1177"
  },
  "items": [
    {
      "metadata": {
        "name": "kubernetes",
        "namespace": "default",
        "labels": {
          "kubernetes.io/service-name": "kubernetes"
        }
      },
      "addressType": "IPv4",
      "endpoints": [
        {
          "addresses": [
            "172.18.0.2"
          ],
          "conditions": {
            "ready": true
          }
        }
      ],
      "ports": [
        {
          "name": "https",
          "protocol": "TCP",
          "port": 6443
        }
      ]
    },
    {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "labels": {
          "kubernetes.io/service-name": "kube-dns"
        }
      },
      "addressType": "IPv4",
      "endpoints": [
        {
          "addresses": [
            "10.244.0.3"
          ],
          "conditions": {
            "ready": true
          },
          "targetRef": {
            "kind": "Pod",
            "namespace": "kube-system",
            "name": "coredns-66bff467f8-z8czk",
            "uid": "36a545ff-dbba-4192-a5f6-1dbb0c21c73d",
            "resourceVersion": "603"
          },
          "topology": {
            "kubernetes.io/hostname": "kind-control-plane"
          }
        },
        {
          "addresses": [
            "10.244.0.4"
          ],
          "conditions": {
            "ready": true
          },
          "targetRef": {
            "kind": "Pod",
            "namespace": "kube-system",
            "name": "coredns-66bff467f8-kpbhk",
            "uid": "db38d8b4-847a-4e82-874c-fe444fba2718",
            "resourceVersion": "576"
          },
          "topology": {
            "kubernetes.io/hostname": "kind-control-plane"
          }
        }
      ],
      "ports": [
        {
          "name": "dns-tcp",
          "protocol": "TCP",
          "port": 53
        },
        {
          "name": "metrics",
          "protocol": "TCP",
          "port": 9153
        },
        {
          "name": "dns",
          "protocol": "UDP",
          "port": 53
        }
      ]
    }
  ]
}`),
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
        "name": "coredns-66bff467f8-kpbhk",
        "namespace": "kube-system"
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
},
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "labels": {
            "app.kubernetes.io/instance": "stack",
            "pod-template-hash": "5b9c6cf775"
        },
        "name": "coredns-66bff467f8-z8czk",
        "namespace": "kube-system"
    },
    "spec": {
        "containers": [
            {
               "name": "generic-pod"
            }
        ]
    },
    "status": {
        "podIP": "10.10.2.3",
        "phase": "Running"
    }
}
]}`),
				"service": []byte(`{
  "kind": "ServiceList",
  "apiVersion": "v1",
  "metadata": {
    "selfLink": "/api/v1/services",
    "resourceVersion": "60485"
  },
  "items": [
    {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "labels": {
          "k8s-app": "kube-dns"
        }
      },
      "spec": {
        "ports": [
          {
            "name": "dns",
            "protocol": "UDP",
            "port": 53,
            "targetPort": 53
          },
          {
            "name": "dns-tcp",
            "protocol": "TCP",
            "port": 53,
            "targetPort": 53
          }
        ],
        "selector": {
          "k8s-app": "kube-dns"
        },
        "clusterIP": "10.96.0.10",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      }
    }
  ]
}`),
			},
			watchAPIMustAddObjectsByRole: map[string][][]byte{
				"service": {
					[]byte(`    {
      "metadata": {
        "name": "kube-dns",
        "namespace": "kube-system",
        "labels": {
          "k8s-app": "kube-dns",
          "some-new": "label-value"
        }
      },
      "spec": {
        "ports": [
          {
            "name": "dns-tcp",
            "protocol": "TCP",
            "port": 53,
            "targetPort": 53
          }
        ],
        "selector": {
          "k8s-app": "kube-dns"
        },
        "clusterIP": "10.96.0.10",
        "type": "ClusterIP",
        "sessionAffinity": "None"
      }
    }
`),
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			watchPublishersByRole := make(map[string]*watchObjectBroadcast)
			mux := http.NewServeMux()
			for role, obj := range tc.initAPIObjectsByRole {
				watchBroadCaster := &watchObjectBroadcast{}
				watchPublishersByRole[role] = watchBroadCaster
				apiPath := getAPIPath(getObjectTypeByRole(role), "", "")
				addAPIURLHandler(t, mux, apiPath, obj, watchBroadCaster)
			}
			testAPIServer := httptest.NewServer(mux)
			tc.sdc.APIServer = testAPIServer.URL
			ac, err := newAPIConfig(tc.sdc, "", func(metaLabels *promutils.Labels) interface{} {
				var res []interface{}
				for _, label := range metaLabels.Labels {
					res = append(res, label.Name)
				}
				return res
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tc.sdc.cfg = ac
			ac.aw.mustStart()
			defer ac.aw.mustStop()
			_, err = tc.sdc.GetScrapeWorkObjects()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// need to wait, for subscribers to start.
			time.Sleep(80 * time.Millisecond)
			for role, objs := range tc.watchAPIMustAddObjectsByRole {
				for _, obj := range objs {
					watchPublishersByRole[role].pub(obj)
				}
			}
			for _, ch := range watchPublishersByRole {
				ch.shutdown()
			}
			if len(tc.watchAPIMustAddObjectsByRole) > 0 {
				// updates async, need to wait some time.
				// i guess, poll is not reliable.
				time.Sleep(80 * time.Millisecond)
			}
			got, err := tc.sdc.GetScrapeWorkObjects()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.expectedTargetsLen {
				t.Fatalf("unexpected count of objects, got: %d, want: %d", len(got), tc.expectedTargetsLen)
			}
		})
	}
}

type watchObjectBroadcast struct {
	mu          sync.Mutex
	subscribers []chan []byte
}

func (o *watchObjectBroadcast) pub(msg []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := range o.subscribers {
		c := o.subscribers[i]
		select {
		case c <- msg:
		default:
		}
	}
}

func (o *watchObjectBroadcast) sub() <-chan []byte {
	c := make(chan []byte, 5)
	o.mu.Lock()
	o.subscribers = append(o.subscribers, c)
	o.mu.Unlock()
	return c
}

func (o *watchObjectBroadcast) shutdown() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := range o.subscribers {
		c := o.subscribers[i]
		close(c)
	}
}

func addAPIURLHandler(t *testing.T, mux *http.ServeMux, apiURL string, initObjects []byte, notifier *watchObjectBroadcast) {
	t.Helper()
	mux.HandleFunc(apiURL, func(w http.ResponseWriter, r *http.Request) {
		if needWatch := r.URL.Query().Get("watch"); len(needWatch) > 0 {
			// start watch handler
			w.WriteHeader(200)
			flusher := w.(http.Flusher)
			flusher.Flush()
			updateC := notifier.sub()
			for obj := range updateC {
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
