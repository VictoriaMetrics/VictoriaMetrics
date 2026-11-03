package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func newKubernetesTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	server := httptest.NewUnstartedServer(handler)
	server.Config.BaseContext = func(net.Listener) context.Context { return ctx }
	server.Start()
	t.Cleanup(func() {
		// Unblock handlers even if discovery startup timed out before registering its cleanup.
		cancel()
		server.Close()
	})
	return server
}

func startTestSDConfig(t *testing.T, sdc *SDConfig) {
	t.Helper()
	started := make(chan struct{})
	// A regression to synchronous LISTs should fail with a timeout instead of hanging the test.
	go func() {
		sdc.MustStart("", func(labels *promutil.Labels) any { return labels.Clone() })
		close(started)
	}()
	receiveWithTimeout(t, started)
	if sdc.cfg == nil {
		t.Fatalf("cannot start discovery: %v", sdc.startErr)
	}
	t.Cleanup(func() {
		sdc.MustStop()
		sdc.cfg.aw.gw.cancel()
	})
}

func receiveWithTimeout[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Kubernetes discovery")
		var zero T
		return zero
	}
}

func getTestScrapeWorksAsync(t *testing.T, sdc *SDConfig) <-chan []any {
	result := make(chan []any, 1)
	go func() {
		swos, err := sdc.GetScrapeWorkObjects()
		if err != nil {
			t.Errorf("cannot get scrape works: %v", err)
		}
		result <- swos
	}()
	return result
}

// The second v1 request receives its 404 after the first has enabled fallback.
// Both requests must retry v1beta1, and later requests must start with v1beta1.
func TestKubernetesConcurrentRequestsFallBackToV1Beta1(t *testing.T) {
	for _, tc := range []struct {
		apiGroup string
		resource string
	}{
		{apiGroup: "networking.k8s.io", resource: "ingresses"},
		{apiGroup: "discovery.k8s.io", resource: "endpointslices"},
	} {
		t.Run(tc.apiGroup, func(t *testing.T) {
			v1 := "/apis/" + tc.apiGroup + "/v1/namespaces/"
			beta := "/apis/" + tc.apiGroup + "/v1beta1/namespaces/"
			secondStarted := make(chan struct{})
			firstFallback := make(chan struct{})
			var betaRequests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case v1 + "a/" + tc.resource:
					// Both requests must use v1 before either enables fallback.
					select {
					case <-secondStarted:
					case <-r.Context().Done():
						return
					}
				case v1 + "b/" + tc.resource:
					close(secondStarted)
					// Return the second 404 after the shared fallback flag is set.
					select {
					case <-firstFallback:
					case <-r.Context().Done():
						return
					}
				case beta + "a/" + tc.resource:
					betaRequests.Add(1)
					close(firstFallback)
					fmt.Fprint(w, `{"metadata":{"resourceVersion":"7"},"items":[]}`)
					return
				case beta + "b/" + tc.resource:
					betaRequests.Add(1)
					fmt.Fprint(w, `{"metadata":{"resourceVersion":"7"},"items":[]}`)
					return
				case beta + "missing/" + tc.resource:
					betaRequests.Add(1)
				default:
					t.Errorf("unexpected request path: %q", r.URL.Path)
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			gw := &groupWatcher{
				client:     server.Client(),
				setHeaders: func(_ *http.Request) error { return nil },
			}
			statuses := make(chan int, 2)
			for _, namespace := range []string{"a", "b"} {
				go func() {
					resp, err := gw.doRequest(ctx, server.URL+v1+namespace+"/"+tc.resource)
					if err != nil {
						t.Errorf("cannot LIST namespace %q: %v", namespace, err)
						statuses <- 0
						return
					}
					resp.Body.Close()
					statuses <- resp.StatusCode
				}()
			}
			for range 2 {
				if status := receiveWithTimeout(t, statuses); status != http.StatusOK {
					t.Errorf("LIST status = %d; want 200", status)
				}
			}
			// Later requests must use the cached version, and a beta 404 must not retry.
			resp, err := gw.doRequest(ctx, server.URL+v1+"missing/"+tc.resource)
			if err != nil {
				t.Fatalf("cannot LIST missing namespace: %v", err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("missing namespace status = %d; want 404", resp.StatusCode)
			}
			if got := betaRequests.Load(); got != 3 {
				t.Errorf("got %d beta requests; want 3", got)
			}
		})
	}
}

// Finish the endpoint LIST while service and namespace LISTs remain blocked.
// Neither direct collection nor another role's batch registration may expose targets before their metadata is available.
func TestKubernetesTargetsWaitForMetadataDependencies(t *testing.T) {
	for _, role := range []string{"endpoints", "endpointslice"} {
		t.Run(role, func(t *testing.T) {
			lists := map[string]string{
				"nodes":          `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"node1","labels":{"zone":"west"}}}]}`,
				"namespaces":     `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"default","labels":{"team":"test"}}}]}`,
				"pods":           `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"pod1","namespace":"default"},"spec":{"nodeName":"node1"},"status":{"podIP":"10.0.0.1"}}]}`,
				"services":       `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"svc1","namespace":"default","labels":{"app":"demo"}}}]}`,
				"endpoints":      `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"svc1","namespace":"default"},"subsets":[{"addresses":[{"ip":"10.0.0.1","targetRef":{"kind":"Pod","namespace":"default","name":"pod1"}}],"ports":[{"port":80}]}]}]}`,
				"endpointslices": `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"slice1","namespace":"default","labels":{"kubernetes.io/service-name":"svc1"}},"endpoints":[{"addresses":["10.0.0.1"],"targetRef":{"kind":"Pod","namespace":"default","name":"pod1"}}],"ports":[{"port":80}]}]}`,
			}
			release := make(chan struct{})
			targetWatchStarted := make(chan struct{})
			server := newKubernetesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				resource := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
				if r.URL.Query().Get("watch") != "" {
					if resource == getObjectTypeByRole(role) {
						close(targetWatchStarted)
					}
					<-r.Context().Done()
					return
				}
				if resource == "services" || resource == "namespaces" {
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				fmt.Fprint(w, lists[resource])
			})
			sdc := &SDConfig{APIServer: server.URL, Role: role, AttachMetadata: &AttachMetadata{Node: true, Namespace: true}}
			startTestSDConfig(t, sdc)
			// The target LIST must finish even while dependency LISTs are blocked.
			receiveWithTimeout(t, targetWatchStarted)
			// Collecting nodes registers pending subscribers across the group; endpoints must remain pending.
			nodeSD := &SDConfig{APIServer: server.URL, Role: "node", AttachMetadata: sdc.AttachMetadata}
			startTestSDConfig(t, nodeSD)
			receiveWithTimeout(t, getTestScrapeWorksAsync(t, nodeSD))
			gw := sdc.cfg.aw.gw
			gw.mu.Lock()
			registered := false
			for _, uw := range gw.m {
				_, ok := uw.aws[sdc.cfg.aw]
				registered = registered || ok
			}
			gw.mu.Unlock()
			if registered {
				t.Fatal("registered targets before metadata dependencies finished")
			}
			result := getTestScrapeWorksAsync(t, sdc)
			select {
			case <-result:
				t.Fatal("getter returned before initial dependency LISTs finished")
			case <-time.After(50 * time.Millisecond):
			}
			close(release)
			swos := receiveWithTimeout(t, result)
			if len(swos) != 1 {
				t.Fatalf("got %d targets; want 1", len(swos))
			}
			labels := swos[0].(*promutil.Labels)
			for name, want := range map[string]string{
				"__meta_kubernetes_service_label_app":    "demo",
				"__meta_kubernetes_namespace_label_team": "test",
				"__meta_kubernetes_node_label_zone":      "west",
				"__meta_kubernetes_pod_name":             "pod1",
			} {
				if got := labels.Get(name); got != want {
					t.Errorf("%s = %q; want %q", name, got, want)
				}
			}
		})
	}
}

// Two configs share a blocked LIST. Stopping one must unblock its getter,
// preserve discovery for the other, and let a replacement reuse the same LIST.
func TestKubernetesStopUnblocksGetterWithoutCancelingSharedList(t *testing.T) {
	release := make(chan struct{})
	var lists atomic.Int32
	listStarted := make(chan struct{}, 10)
	server := newKubernetesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") != "" {
			<-r.Context().Done()
			return
		}
		lists.Add(1)
		listStarted <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		fmt.Fprint(w, `{"metadata":{"resourceVersion":"7"},"items":[{"metadata":{"name":"svc1","namespace":"default"},"spec":{"ports":[{"port":80}]}}]}`)
	})
	sdc := &SDConfig{APIServer: server.URL, Role: "service"}
	startTestSDConfig(t, sdc)
	receiveWithTimeout(t, listStarted)
	peer := &SDConfig{APIServer: server.URL, Role: "service"}
	startTestSDConfig(t, peer)
	result := make(chan error, 1)
	go func() {
		_, err := sdc.GetScrapeWorkObjects()
		result <- err
	}()
	sdc.MustStop()
	// The getter must exit while the LIST needed by the other subscriber is still blocked.
	if err := receiveWithTimeout(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
	// A replacement config reuses the LIST already in flight, as on a config reload.
	replacement := &SDConfig{APIServer: server.URL, Role: "service"}
	startTestSDConfig(t, replacement)
	if replacement.cfg.aw.gw != sdc.cfg.aw.gw {
		t.Fatal("replacement config did not reuse the group watcher")
	}
	close(release)
	if got := receiveWithTimeout(t, getTestScrapeWorksAsync(t, peer)); len(got) != 1 {
		t.Fatalf("got %d targets for the other subscriber; want 1", len(got))
	}
	if got := receiveWithTimeout(t, getTestScrapeWorksAsync(t, replacement)); len(got) != 1 {
		t.Fatalf("got %d targets after reuse; want 1", len(got))
	}
	if got := lists.Load(); got != 1 {
		t.Fatalf("got %d LIST requests; want 1", got)
	}
	// Completing initialization must not subscribe the stopped watcher again.
	gw := sdc.cfg.aw.gw
	gw.mu.Lock()
	defer gw.mu.Unlock()
	for _, uw := range gw.m {
		if _, ok := uw.aws[sdc.cfg.aw]; ok {
			t.Fatal("stopped watcher was registered again")
		}
		if _, ok := uw.awsPending[sdc.cfg.aw]; ok {
			t.Fatal("stopped watcher was subscribed again")
		}
	}
}

// Block the retry after an initial 403; collection must return no targets without waiting for a successful retry.
func TestKubernetesFailedInitialListDoesNotBlockGetter(t *testing.T) {
	var requests atomic.Int32
	retryStarted := make(chan struct{})
	server := newKubernetesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "denied", http.StatusForbidden)
			return
		}
		close(retryStarted)
		<-r.Context().Done()
	})
	sdc := &SDConfig{APIServer: server.URL, Role: "pod"}
	startTestSDConfig(t, sdc)
	receiveWithTimeout(t, retryStarted)
	if got := receiveWithTimeout(t, getTestScrapeWorksAsync(t, sdc)); len(got) != 0 {
		t.Fatalf("got %d targets after failed LIST; want 0", len(got))
	}
}

// Configs using one server share a LIST limit, held until each response body is decoded.
// Exercise saturation, new arrivals, an independent server, and relists after WATCH expiry.
func TestKubernetesListConcurrencyLimitPerServer(t *testing.T) {
	limit := *listConcurrency
	const extra = 2
	listStarted := make(chan struct{}, 100)
	watchStarted := make(chan struct{}, 100)
	finishOneList := make(chan struct{})
	expireWatches := make(chan struct{})
	var active atomic.Int32
	var peak atomic.Int32
	server := newKubernetesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") != "" {
			if got := r.URL.Query().Get("resourceVersion"); got != "7" {
				t.Errorf("watch resourceVersion = %q; want 7", got)
			}
			watchStarted <- struct{}{}
			select {
			case <-expireWatches:
				fmt.Fprint(w, `{"type":"ERROR","object":{"code":410}}`)
			case <-r.Context().Done():
			}
			return
		}
		n := active.Add(1)
		defer active.Add(-1)
		for prev := peak.Load(); n > prev; prev = peak.Load() {
			if peak.CompareAndSwap(prev, n) {
				break
			}
		}
		// Headers alone must not release the permit; keep the response body incomplete.
		fmt.Fprint(w, `{"metadata":`)
		w.(http.Flusher).Flush()
		listStarted <- struct{}{}
		select {
		case <-finishOneList:
			fmt.Fprint(w, `{"resourceVersion":"7"},"items":[]}`)
		case <-r.Context().Done():
		}
	})
	// Different selectors create separate watchers, which must still share the server's limit.
	for i := 0; i < limit+extra; i++ {
		startTestSDConfig(t, &SDConfig{
			APIServer: server.URL, Role: "pod",
			Selectors: []Selector{{Role: "pod", Label: fmt.Sprintf("job=%d", i)}},
		})
	}
	assertListLimitReached := func() {
		t.Helper()
		for i := 0; i < limit; i++ {
			receiveWithTimeout(t, listStarted)
		}
		select {
		case <-listStarted:
			t.Fatal("LIST concurrency exceeded the per-server limit")
		case <-time.After(50 * time.Millisecond):
		}
	}
	assertListLimitReached()
	// Free one slot for a queued caller; a newly added job must still wait for another slot.
	select {
	case finishOneList <- struct{}{}:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out releasing LIST")
	}
	receiveWithTimeout(t, listStarted)
	receiveWithTimeout(t, watchStarted)
	startTestSDConfig(t, &SDConfig{
		APIServer: server.URL, Role: "pod",
		Selectors: []Selector{{Role: "pod", Label: "job=late"}},
	})
	select {
	case <-listStarted:
		t.Fatal("new request bypassed the shared LIST concurrency limit")
	case <-time.After(50 * time.Millisecond):
	}
	// Saturating one API server must not block another server.
	other := newKubernetesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("watch") != "" {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `{"metadata":{"resourceVersion":"7"},"items":[]}`)
	})
	otherSD := &SDConfig{APIServer: other.URL, Role: "pod"}
	startTestSDConfig(t, otherSD)
	receiveWithTimeout(t, getTestScrapeWorksAsync(t, otherSD))
	assertKubernetesListLimitReleased(t, other.URL)
	// Complete the remaining initial LISTs so every watcher can start WATCH.
	for i := 0; i < limit+extra; i++ {
		select {
		case finishOneList <- struct{}{}:
		case <-time.After(5 * time.Second):
			t.Fatal("timed out releasing LIST")
		}
	}
	for i := 0; i < extra; i++ {
		receiveWithTimeout(t, listStarted)
	}
	for i := 0; i < limit+extra; i++ {
		receiveWithTimeout(t, watchStarted)
	}
	assertKubernetesListLimitReleased(t, server.URL)
	// Expire all WATCH streams together; the resulting relists must obey the same limit.
	close(expireWatches)
	assertListLimitReached()
	if got := peak.Load(); got != int32(limit) {
		t.Fatalf("peak LIST concurrency = %d; want %d", got, limit)
	}
}

// A queued acquire must wake on cancellation and leave no limiter entry after the active requests finish.
func TestKubernetesListLimiterCancelsQueuedRequest(t *testing.T) {
	const apiServer = "https://limiter-cancellation.invalid"
	t.Cleanup(func() { assertKubernetesListLimitReleased(t, apiServer) })
	for i := 0; i < *listConcurrency; i++ {
		if err := listLimiter.Acquire(context.Background(), apiServer); err != nil {
			t.Fatalf("cannot acquire LIST slot: %v", err)
		}
		defer listLimiter.Release(apiServer)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		err := listLimiter.Acquire(ctx, apiServer)
		if err == nil {
			listLimiter.Release(apiServer)
		}
		result <- err
	}()
	// Cancel only after the extra caller is queued behind the occupied slots.
	deadline := time.Now().Add(5 * time.Second)
	for {
		listLimiter.mu.Lock()
		queued := listLimiter.limits[apiServer].users == *listConcurrency+1
		listLimiter.mu.Unlock()
		if queued {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for queued LIST")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := receiveWithTimeout(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v; want context.Canceled", err)
	}
}

func assertKubernetesListLimitReleased(t *testing.T, apiServer string) {
	t.Helper()
	listLimiter.mu.Lock()
	defer listLimiter.mu.Unlock()
	if _, ok := listLimiter.limits[apiServer]; ok {
		t.Errorf("LIST semaphore for %q was not released", apiServer)
	}
}

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
