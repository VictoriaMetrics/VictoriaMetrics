package kubernetes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/easyproto"
)

var mp easyproto.MarshalerPool

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
	bm, err := parseBookmark([]byte(data), contentTypeJSON)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	expectedResourceVersion := "12746"
	if bm.Metadata.ResourceVersion != expectedResourceVersion {
		t.Fatalf("unexpected resourceVersion; got %q; want %q", bm.Metadata.ResourceVersion, expectedResourceVersion)
	}
}

type marshallable interface {
	marshalProtobuf(*easyproto.MessageMarshaler)
}

func TestGetScrapeWorkObjects(t *testing.T) {
	type testCase struct {
		name                 string
		sdc                  *SDConfig
		expectedTargetsLen   int
		initAPIObjectsByRole map[string]marshallable
		// will be added for watching api.
		watchAPIMustAddObjectsByRole map[string][]marshallable
	}
	cases := []testCase{
		{
			name: "simple 1 pod with update 1",
			sdc: &SDConfig{
				Role: "pod",
			},
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string]marshallable{
				"pod": &PodList{
					Metadata: ListMeta{ResourceVersion: "72425"},
					Items: []Pod{
						{
							Metadata: ObjectMeta{
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/instance", Value: "stack"},
										{Name: "pod-template-hash", Value: "5b9c6cf775"},
									},
								},
								Name:      "stack-name-1",
								Namespace: "default",
							},
							Spec: PodSpec{
								Containers: []Container{
									{Name: "generic-pod"},
								},
							},
							Status: PodStatus{
								PodIP: "10.10.2.2",
								Phase: "Running",
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"pod": {
					&Pod{
						Metadata: ObjectMeta{
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "app.kubernetes.io/instance", Value: "stack"},
									{Name: "pod-template-hash", Value: "5b9c6cf775"},
								},
							},
							Name:      "stack-next-2",
							Namespace: "default",
						},
						Spec: PodSpec{
							Containers: []Container{
								{Name: "generic-pod"},
							},
						},
						Status: PodStatus{
							PodIP: "10.10.2.5",
							Phase: "Running",
						},
					},
				},
			},
		},
		{
			name: "endpoints with service update",
			sdc: &SDConfig{
				Role: "endpoints",
			},
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string]marshallable{
				"service": &ServiceList{
					Metadata: ListMeta{ResourceVersion: "72425"},
				},
				"endpoints": &EndpointsList{
					Metadata: ListMeta{ResourceVersion: "72425"},
					Items: []Endpoints{
						{
							Metadata: ObjectMeta{
								Annotations: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "endpoints.kubernetes.io/last-change-trigger-time", Value: "2021-04-27T02:06:55Z"},
									},
								},
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/managed-by", Value: "Helm"},
									},
								},
								Name:      "stack-kube-state-metrics",
								Namespace: "default",
							},
							Subsets: []EndpointSubset{
								{
									Addresses: []EndpointAddress{
										{
											IP:       "10.244.0.5",
											NodeName: "kind-control-plane",
											TargetRef: ObjectReference{
												Kind:      "Pod",
												Name:      "stack-kube-state-metrics-db5879bf8-bg78p",
												Namespace: "default",
											},
										},
									},
									Ports: []EndpointPort{
										{
											Name:     "http",
											Port:     8080,
											Protocol: "TCP",
										},
									},
								},
							},
						},
					},
				},
				"pod": &PodList{
					Metadata: ListMeta{ResourceVersion: "72425"},
					Items: []Pod{
						{
							Metadata: ObjectMeta{
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/instance", Value: "stack"},
									},
								},
								Name:      "stack-kube-state-metrics-db5879bf8-bg78p",
								Namespace: "default",
							},
							Spec: PodSpec{
								Containers: []Container{
									{
										Image: "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v1.9.8",
										Name:  "kube-state-metrics",
										Ports: []ContainerPort{
											{
												ContainerPort: 8080,
												Protocol:      "TCP",
											},
										},
									},
									{
										Image: "k8s.gcr.io/kube-state-metrics/kube-state-metrics:v1.9.8",
										Name:  "kube-state-metrics-2",
										Ports: []ContainerPort{
											{
												ContainerPort: 8085,
												Protocol:      "TCP",
											},
										},
									},
								},
							},
							Status: PodStatus{
								Phase: "Running",
								PodIP: "10.244.0.5",
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"service": {
					&Service{
						Metadata: ObjectMeta{
							Annotations: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "meta.helm.sh/release-name", Value: "stack"},
								},
							},
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "app.kubernetes.io/managed-by", Value: "Helm"},
									{Name: "app.kubernetes.io/name", Value: "kube-state-metrics"},
								},
							},
							Name:      "stack-kube-state-metrics",
							Namespace: "default",
						},
						Spec: ServiceSpec{
							ClusterIP: "10.97.109.249",
							Ports: []ServicePort{
								{
									Name:     "http",
									Port:     8080,
									Protocol: "TCP",
								},
							},
							Type: "ClusterIP",
						},
					},
				},
			},
		},
		{
			name:               "get nodes",
			sdc:                &SDConfig{Role: "node"},
			expectedTargetsLen: 2,
			initAPIObjectsByRole: map[string]marshallable{
				"node": &NodeList{
					Metadata: ListMeta{ResourceVersion: "22627"},
					Items: []Node{
						{
							Metadata: ObjectMeta{
								Annotations: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/managed-by", Value: "Helm"},
										{Name: "app.kubernetes.io/name", Value: "kube-state-metrics"},
									},
								},
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/arch", Value: "amd64"},
										{Name: "app.kubernetes.io/os", Value: "linux"},
									},
								},
								Name: "kind-control-plane-new",
							},
							Status: NodeStatus{
								Addresses: []NodeAddress{
									{
										Address: "10.10.2.5",
										Type:    "InternalIP",
									},
									{
										Address: "kind-control-plane",
										Type:    "Hostname",
									},
								},
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"node": {
					&Node{
						Metadata: ObjectMeta{
							Annotations: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "kubeadm.alpha.kubernetes.io/cri-socket", Value: "/run/containerd/containerd.sock"},
								},
							},
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "beta.kubernetes.io/arch", Value: "amd64"},
									{Name: "beta.kubernetes.io/os", Value: "linux"},
								},
							},
							Name: "kind-control-plane",
						},
						Status: NodeStatus{
							Addresses: []NodeAddress{
								{
									Address: "10.10.2.2",
									Type:    "InternalIP",
								},
								{
									Address: "kind-control-plane",
									Type:    "Hostname",
								},
							},
						},
					},
				},
			},
		},
		{
			name:               "2 service with 2 added",
			sdc:                &SDConfig{Role: "service"},
			expectedTargetsLen: 4,
			initAPIObjectsByRole: map[string]marshallable{
				"service": &ServiceList{
					Metadata: ListMeta{ResourceVersion: "60485"},
					Items: []Service{
						{
							Metadata: ObjectMeta{
								Name:      "kube-dns",
								Namespace: "kube-system",
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "k8s-app", Value: "kube-dns"},
									},
								},
							},
							Spec: ServiceSpec{
								Ports: []ServicePort{
									{
										Name:     "dns",
										Protocol: "UDP",
										Port:     53,
									},
									{
										Name:     "dns-tcp",
										Protocol: "TCP",
										Port:     53,
									},
								},
								ClusterIP: "10.96.0.10",
								Type:      "ClusterIP",
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"service": {
					&Service{
						Metadata: ObjectMeta{
							Name:      "another-service-1",
							Namespace: "default",
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "k8s-app", Value: "kube-dns"},
								},
							},
						},
						Spec: ServiceSpec{
							Ports: []ServicePort{
								{
									Name:     "some-app-1-tcp",
									Protocol: "TCP",
									Port:     1053,
								},
							},
							ClusterIP: "10.96.0.10",
							Type:      "ClusterIP",
						},
					},
					&Service{
						Metadata: ObjectMeta{
							Name:      "another-service-2",
							Namespace: "default",
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "k8s-app", Value: "kube-dns"},
								},
							},
						},
						Spec: ServiceSpec{
							Ports: []ServicePort{
								{
									Name:     "some-app-2-tcp",
									Protocol: "TCP",
									Port:     1053,
								},
							},
							ClusterIP: "10.96.0.15",
							Type:      "ClusterIP",
						},
					},
				},
			},
		},
		{
			name:               "1 ingress with 2 add",
			expectedTargetsLen: 3,
			sdc: &SDConfig{
				Role: "ingress",
			},
			initAPIObjectsByRole: map[string]marshallable{
				"ingress": &IngressList{
					Metadata: ListMeta{ResourceVersion: "351452"},
					Items: []Ingress{
						{
							Metadata: ObjectMeta{
								Name:      "test-ingress",
								Namespace: "default",
							},
							Spec: IngressSpec{
								Rules: []IngressRule{
									{
										Host: "foobar",
									},
								},
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"ingress": {
					&Ingress{
						Metadata: ObjectMeta{
							Name:      "test-ingress-1",
							Namespace: "default",
						},
						Spec: IngressSpec{
							Rules: []IngressRule{
								{
									Host: "foobar",
								},
							},
						},
					},
					&Ingress{
						Metadata: ObjectMeta{
							Name:      "test-ingress-2",
							Namespace: "default",
						},
						Spec: IngressSpec{
							Rules: []IngressRule{
								{
									Host: "foobar",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "7 endpointslices slice with 1 service update",
			sdc: &SDConfig{
				Role: "endpointslice",
			},
			expectedTargetsLen: 7,
			initAPIObjectsByRole: map[string]marshallable{
				"endpointslice": &EndpointSliceList{
					Metadata: ListMeta{ResourceVersion: "1177"},
					Items: []EndpointSlice{
						{
							Metadata: ObjectMeta{
								Name:      "kubernetes",
								Namespace: "default",
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "kubernetes.io/service-name", Value: "kubernetes"},
									},
								},
							},
							AddressType: "IPv4",
							Endpoints: []Endpoint{
								{
									Addresses: []string{
										"172.18.0.2",
									},
									Conditions: EndpointConditions{
										Ready: true,
									},
								},
							},
							Ports: []EndpointPort{
								{
									Name:     "https",
									Protocol: "TCP",
									Port:     6443,
								},
							},
						},
						{
							Metadata: ObjectMeta{
								Name:      "kube-dns",
								Namespace: "kube-system",
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "kubernetes.io/service-name", Value: "kube-dns"},
									},
								},
							},
							AddressType: "IPv4",
							Endpoints: []Endpoint{
								{
									Addresses: []string{
										"10.244.0.3",
									},
									Conditions: EndpointConditions{
										Ready: true,
									},
									TargetRef: ObjectReference{
										Kind:      "Pod",
										Namespace: "kube-system",
										Name:      "coredns-66bff467f8-z8czk",
									},
									NodeName: "kind-control-plane",
								},
								{
									Addresses: []string{
										"10.244.0.4",
									},
									Conditions: EndpointConditions{
										Ready: true,
									},
									TargetRef: ObjectReference{
										Kind:      "Pod",
										Namespace: "kube-system",
										Name:      "coredns-66bff467f8-kpbhk",
									},
									NodeName: "kind-control-plane",
								},
							},
							Ports: []EndpointPort{
								{
									Name:     "dns-tcp",
									Protocol: "TCP",
									Port:     53,
								},
								{
									Name:     "metrics",
									Protocol: "TCP",
									Port:     9153,
								},
								{
									Name:     "dns",
									Protocol: "UDP",
									Port:     53,
								},
							},
						},
					},
				},
				"pod": &PodList{
					Metadata: ListMeta{ResourceVersion: "72425"},
					Items: []Pod{
						{
							Metadata: ObjectMeta{
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/instance", Value: "stack"},
										{Name: "pod-template-hash", Value: "5b9c6cf775"},
									},
								},
								Name:      "coredns-66bff467f8-kpbhk",
								Namespace: "kube-system",
							},
							Spec: PodSpec{
								Containers: []Container{
									{
										Name: "generic-pod",
									},
								},
							},
							Status: PodStatus{
								PodIP: "10.10.2.2",
								Phase: "Running",
							},
						},
						{
							Metadata: ObjectMeta{
								Namespace: "kube-system",
							},
							Spec: PodSpec{
								Containers: []Container{
									{
										Name: "generic-pod",
									},
								},
							},
							Status: PodStatus{
								PodIP: "10.10.2.2",
								Phase: "Running",
							},
						},
						{
							Metadata: ObjectMeta{
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "app.kubernetes.io/instance", Value: "stack"},
										{Name: "pod-template-hash", Value: "5b9c6cf775"},
									},
								},
								Name:      "coredns-66bff467f8-z8czk",
								Namespace: "kube-system",
							},
							Spec: PodSpec{
								Containers: []Container{
									{
										Name: "generic-pod",
									},
								},
							},
							Status: PodStatus{
								PodIP: "10.10.2.3",
								Phase: "Running",
							},
						},
					},
				},
				"service": &ServiceList{
					Metadata: ListMeta{ResourceVersion: "60485"},
					Items: []Service{
						{
							Metadata: ObjectMeta{
								Name:      "kube-dns",
								Namespace: "kube-system",
								Labels: &promutil.Labels{
									Labels: []prompbmarshal.Label{
										{Name: "k8s-app", Value: "kube-dns"},
									},
								},
							},
							Spec: ServiceSpec{
								Ports: []ServicePort{
									{
										Name:     "dns",
										Protocol: "UDP",
										Port:     53,
									},
									{
										Name:     "dns-tcp",
										Protocol: "TCP",
										Port:     53,
									},
								},
								ClusterIP: "10.96.0.10",
								Type:      "ClusterIP",
							},
						},
					},
				},
			},
			watchAPIMustAddObjectsByRole: map[string][]marshallable{
				"service": {
					&Service{
						Metadata: ObjectMeta{
							Name:      "kube-dns",
							Namespace: "kube-system",
							Labels: &promutil.Labels{
								Labels: []prompbmarshal.Label{
									{Name: "k8s-app", Value: "kube-dns"},
									{Name: "some-new", Value: "label-value"},
								},
							},
						},
						Spec: ServiceSpec{
							Ports: []ServicePort{
								{
									Name:     "dns-tcp",
									Protocol: "TCP",
									Port:     53,
								},
							},
							ClusterIP: "10.96.0.10",
							Type:      "ClusterIP",
						},
					},
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
			ac, err := newAPIConfig(tc.sdc, "", func(metaLabels *promutil.Labels) any {
				var res []any
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
	subscribers []chan marshallable
}

func (o *watchObjectBroadcast) pub(msg marshallable) {
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

func (o *watchObjectBroadcast) sub() <-chan marshallable {
	c := make(chan marshallable, 5)
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

func addAPIURLHandler(t *testing.T, mux *http.ServeMux, apiURL string, initObjects marshallable, notifier *watchObjectBroadcast) {
	t.Helper()
	var buf []byte
	var objData []byte
	mux.HandleFunc(apiURL, func(w http.ResponseWriter, r *http.Request) {
		acceptType := r.Header.Get("Accept")
		var contentType string
		switch {
		case strings.HasPrefix(acceptType, contentTypeJSON):
			contentType = contentTypeJSON
		case strings.HasPrefix(acceptType, contentTypeProtobuf):
			contentType = contentTypeProtobuf
		}
		if needWatch := r.URL.Query().Get("watch"); len(needWatch) > 0 {
			w.Header().Set("Content-Type", contentType)
			// start watch handler
			w.WriteHeader(200)
			flusher := w.(http.Flusher)
			flusher.Flush()
			updateC := notifier.sub()
			for obj := range updateC {
				switch contentType {
				case contentTypeJSON:
					objData, err := json.Marshal(obj)
					if err != nil {
						t.Fatalf("cannot serialize obj: %v", err)
					}
					we := WatchEvent{
						Type:   "ADDED",
						Object: objData,
					}
					szd, err := json.Marshal(we)
					if err != nil {
						t.Fatalf("cannot serialize event: %v", err)
					}
					_, _ = w.Write(szd)
				case contentTypeProtobuf:
					m := mp.Get()
					obj.marshalProtobuf(m.MessageMarshaler())
					objData = m.Marshal(objData[:0])
					mp.Put(m)
					we := WatchEvent{
						Type:   "ADDED",
						Object: objData,
					}
					m = mp.Get()
					we.marshalProtobuf(m.MessageMarshaler())
					buf = m.MarshalWithLen(buf[:0])
					mp.Put(m)
					_, _ = w.Write(buf)
				}
				flusher.Flush()
			}
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(200)
		switch contentType {
		case contentTypeJSON:
			szd, err := json.Marshal(initObjects)
			if err != nil {
				t.Fatalf("cannot serialize: %v", err)
			}
			_, _ = w.Write(szd)
		case contentTypeProtobuf:
			m := mp.Get()
			initObjects.marshalProtobuf(m.MessageMarshaler())
			buf = m.Marshal(buf[:0])
			mp.Put(m)
			_, _ = w.Write(buf)
		}
	})
}
