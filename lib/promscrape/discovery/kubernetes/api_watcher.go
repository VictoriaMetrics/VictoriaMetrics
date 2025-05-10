package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	apiServerTimeout      = flag.Duration("promscrape.kubernetes.apiServerTimeout", 30*time.Minute, "How frequently to reload the full state from Kubernetes API server")
	attachNodeMetadataAll = flag.Bool("promscrape.kubernetes.attachNodeMetadataAll", false, "Whether to set attach_metadata.node=true for all the kubernetes_sd_configs at -promscrape.config . "+
		"It is possible to set attach_metadata.node=false individually per each kubernetes_sd_configs . See https://docs.victoriametrics.com/victoriametrics/sd_configs/#kubernetes_sd_configs")
	useHTTP2Client = flag.Bool("promscrape.kubernetes.useHTTP2Client", false, "Whether to use HTTP/2 client for connection to Kubernetes API server."+
		" This may reduce amount of concurrent connections to API server when watching for a big number of Kubernetes objects.")
)

// WatchEvent is a watch event returned from API server endpoints if `watch=1` query arg is set.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
type WatchEvent struct {
	Type   string
	Object json.RawMessage
}

// object is any Kubernetes object.
type object interface {
	key() string

	// getTargetLabels must be called under gw.mu lock.
	getTargetLabels(gw *groupWatcher) []*promutil.Labels
}

// parseObjectFunc must parse object from the given data.
type parseObjectFunc func(data []byte) (object, error)

// parseObjectListFunc must parse objectList from the given r.
type parseObjectListFunc func(r io.Reader) (map[string]object, ListMeta, error)

// apiWatcher is used for watching for Kubernetes object changes and caching their latest states.
type apiWatcher struct {
	role string

	// Constructor for creating ScrapeWork objects from labels
	swcFunc ScrapeWorkConstructorFunc

	gw *groupWatcher

	// swosByURLWatcher contains per-urlWatcher maps of ScrapeWork objects for the given apiWatcher
	swosByURLWatcher     map[*urlWatcher]map[string][]any
	swosByURLWatcherLock sync.Mutex

	swosCount *metrics.Counter
}

func newAPIWatcher(apiServer string, ac *promauth.Config, sdc *SDConfig, swcFunc ScrapeWorkConstructorFunc) (*apiWatcher, error) {
	namespaces := sdc.Namespaces.Names
	if len(namespaces) == 0 {
		if sdc.Namespaces.OwnNamespace {
			namespace, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err != nil {
				return nil, fmt.Errorf("cannot determine namespace for the current pod according to `own_namespace: true` option in kubernetes_sd_config: %w", err)
			}
			namespaces = []string{string(namespace)}
		}
	}
	selectors := sdc.Selectors
	attachNodeMetadata := *attachNodeMetadataAll
	if sdc.AttachMetadata != nil {
		attachNodeMetadata = sdc.AttachMetadata.Node
	}
	proxyURL := sdc.ProxyURL.GetURL()
	gw := getGroupWatcher(apiServer, ac, namespaces, selectors, attachNodeMetadata, proxyURL)
	role := sdc.role()
	aw := &apiWatcher{
		role:             role,
		swcFunc:          swcFunc,
		gw:               gw,
		swosByURLWatcher: make(map[*urlWatcher]map[string][]any),
		swosCount:        metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_scrape_works{role=%q}`, role)),
	}
	return aw, nil
}

func (aw *apiWatcher) mustStart() {
	aw.gw.apiWatcherInflightStartCalls.Add(1)
	aw.gw.startWatchersForRole(aw.role, aw)
	aw.gw.apiWatcherInflightStartCalls.Add(-1)
}

func (aw *apiWatcher) updateSwosCount(multiplier int, swosByKey map[string][]any) {
	n := 0
	for _, swos := range swosByKey {
		n += len(swos)
	}
	n *= multiplier
	aw.swosCount.Add(n)
}

func (aw *apiWatcher) mustStop() {
	aw.gw.unsubscribeAPIWatcher(aw)
	aw.swosByURLWatcherLock.Lock()
	for _, swosByKey := range aw.swosByURLWatcher {
		aw.updateSwosCount(-1, swosByKey)
	}
	aw.swosByURLWatcher = make(map[*urlWatcher]map[string][]any)
	aw.swosByURLWatcherLock.Unlock()
}

func (aw *apiWatcher) replaceScrapeWorks(uw *urlWatcher, swosByKey map[string][]any) {
	aw.swosByURLWatcherLock.Lock()
	aw.updateSwosCount(-1, aw.swosByURLWatcher[uw])
	aw.updateSwosCount(1, swosByKey)
	aw.swosByURLWatcher[uw] = swosByKey
	aw.swosByURLWatcherLock.Unlock()
}

func (aw *apiWatcher) updateScrapeWorks(uw *urlWatcher, swosByKey map[string][]any) {
	aw.swosByURLWatcherLock.Lock()
	dst := aw.swosByURLWatcher[uw]
	if dst == nil {
		dst = make(map[string][]any)
		aw.swosByURLWatcher[uw] = dst
	}
	for key, swos := range swosByKey {
		aw.swosCount.Add(len(swos) - len(dst[key]))
		if len(swos) == 0 {
			delete(dst, key)
		} else {
			dst[key] = swos
		}
	}
	aw.swosByURLWatcherLock.Unlock()
}

func (aw *apiWatcher) setScrapeWorks(uw *urlWatcher, key string, labelss []*promutil.Labels) {
	swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labelss)
	aw.swosByURLWatcherLock.Lock()
	swosByKey := aw.swosByURLWatcher[uw]
	if swosByKey == nil {
		swosByKey = make(map[string][]any)
		aw.swosByURLWatcher[uw] = swosByKey
	}
	aw.swosCount.Add(len(swos) - len(swosByKey[key]))
	if len(swos) == 0 {
		delete(swosByKey, key)
	} else {
		swosByKey[key] = swos
	}
	aw.swosByURLWatcherLock.Unlock()
}

func (aw *apiWatcher) removeScrapeWorks(uw *urlWatcher, key string) {
	aw.swosByURLWatcherLock.Lock()
	swosByKey := aw.swosByURLWatcher[uw]
	if len(swosByKey) > 0 {
		aw.swosCount.Add(-len(swosByKey[key]))
		delete(swosByKey, key)
	}
	aw.swosByURLWatcherLock.Unlock()
}

func getScrapeWorkObjectsForLabels(swcFunc ScrapeWorkConstructorFunc, labelss []*promutil.Labels) []any {
	// Do not pre-allocate swos, since it is likely the swos will be empty because of relabeling
	var swos []any
	for _, labels := range labelss {
		swo := swcFunc(labels)
		// The reflect check is needed because of https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
		if swo != nil && !reflect.ValueOf(swo).IsNil() {
			swos = append(swos, swo)
		}
	}
	return swos
}

// getScrapeWorkObjects returns all the ScrapeWork objects for the given aw.
func (aw *apiWatcher) getScrapeWorkObjects() []any {
	aw.gw.registerPendingAPIWatchers()

	swos := make([]any, 0, aw.swosCount.Get())
	aw.swosByURLWatcherLock.Lock()
	for _, swosByKey := range aw.swosByURLWatcher {
		for _, swosLocal := range swosByKey {
			swos = append(swos, swosLocal...)
		}
	}
	aw.swosByURLWatcherLock.Unlock()
	return swos
}

// groupWatcher watches for Kubernetes objects on the given apiServer with the given namespaces,
// selectors and attachNodeMetadata using the given client.
type groupWatcher struct {
	// The number of in-flight apiWatcher.mustStart() calls for the given groupWatcher.
	// This field is used by groupWatchersCleaner() in order to determine when the given groupWatcher can be stopped.
	apiWatcherInflightStartCalls atomic.Int32

	// Old Kubernetes doesn't support /apis/networking.k8s.io/v1/, so /apis/networking.k8s.io/v1beta1/ must be used instead.
	// This flag is used for automatic substitution of v1 API path with v1beta1 API path during requests to apiServer.
	useNetworkingV1Beta1 atomic.Bool

	// Old Kubernetes doesn't support /apis/discovery.k8s.io/v1/, so discovery.k8s.io/v1beta1/ must be used instead.
	// This flag is used for automatic substitution of v1 API path with v1beta1 API path during requests to apiServer.
	useDiscoveryV1Beta1 atomic.Bool

	apiServer          string
	namespaces         []string
	selectors          []Selector
	attachNodeMetadata bool

	setHeaders func(req *http.Request) error
	client     *http.Client

	mu sync.Mutex
	m  map[string]*urlWatcher

	// cancel is used for stopping all the urlWatcher instances inside m,
	// which must check for ctx.Done() when performing their background watch work.
	ctx    context.Context
	cancel context.CancelFunc

	// noAPIWatchers is set to true when there are no API watchers for the given groupWatcher.
	// This field is used for determining when it is safe to stop all the urlWatcher instances
	// for the given groupWatcher.
	noAPIWatchers bool
}

var (
	httpClientsCache = make(map[string]*http.Client)
	httpClientsLock  sync.Mutex
)

func getHTTPClient(ac *promauth.Config, proxyURL *url.URL) *http.Client {
	key := fmt.Sprintf("authConfig=%s, proxyURL=%s", ac.String(), proxyURL)
	httpClientsLock.Lock()
	if c, ok := httpClientsCache[key]; ok {
		httpClientsLock.Unlock()
		return c
	}

	tr := newHTTPTransport(*useHTTP2Client)
	if !*useHTTP2Client {
		// Proxy is not supported for http2 client.
		// See https://github.com/golang/go/issues/26479
		var proxy func(*http.Request) (*url.URL, error)
		if proxyURL != nil {
			proxy = http.ProxyURL(proxyURL)
		}
		tr.Proxy = proxy
	}
	c := &http.Client{
		Transport: ac.NewRoundTripper(tr),
		Timeout:   *apiServerTimeout,
	}
	httpClientsCache[key] = c
	httpClientsLock.Unlock()
	return c
}

func newHTTPTransport(enableHTTP2 bool) *http.Transport {
	tr := httputil.NewTransport(enableHTTP2, "vm_promscrape_discovery_kubernetes")
	tr.DialContext = netutil.Dialer.DialContext
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.IdleConnTimeout = *apiServerTimeout
	tr.MaxIdleConnsPerHost = 100
	return tr
}

func newGroupWatcher(apiServer string, ac *promauth.Config, namespaces []string, selectors []Selector, attachNodeMetadata bool, proxyURL *url.URL) *groupWatcher {
	client := getHTTPClient(ac, proxyURL)
	ctx, cancel := context.WithCancel(context.Background())
	gw := &groupWatcher{
		apiServer:          apiServer,
		namespaces:         namespaces,
		selectors:          selectors,
		attachNodeMetadata: attachNodeMetadata,

		setHeaders: func(req *http.Request) error {
			return ac.SetHeaders(req, true)
		},
		client: client,
		m:      make(map[string]*urlWatcher),

		ctx:    ctx,
		cancel: cancel,
	}
	return gw
}

func getGroupWatcher(apiServer string, ac *promauth.Config, namespaces []string, selectors []Selector, attachNodeMetadata bool, proxyURL *url.URL) *groupWatcher {
	proxyURLStr := "<nil>"
	if proxyURL != nil {
		proxyURLStr = proxyURL.String()
	}
	key := fmt.Sprintf("apiServer=%s, namespaces=%s, selectors=%s, attachNodeMetadata=%v, proxyURL=%s, authConfig=%s",
		apiServer, namespaces, selectorsKey(selectors), attachNodeMetadata, proxyURLStr, ac.String())
	groupWatchersLock.Lock()
	gw := groupWatchers[key]
	if gw == nil {
		gw = newGroupWatcher(apiServer, ac, namespaces, selectors, attachNodeMetadata, proxyURL)
		groupWatchers[key] = gw
	}
	groupWatchersLock.Unlock()
	return gw
}

func selectorsKey(selectors []Selector) string {
	var sb strings.Builder
	for _, s := range selectors {
		fmt.Fprintf(&sb, "{role=%q, label=%q, field=%q}", s.Role, s.Label, s.Field)
	}
	return sb.String()
}

var (
	groupWatchersLock sync.Mutex
	groupWatchers     map[string]*groupWatcher

	_ = metrics.NewGauge(`vm_promscrape_discovery_kubernetes_group_watchers`, func() float64 {
		groupWatchersLock.Lock()
		n := len(groupWatchers)
		groupWatchersLock.Unlock()
		return float64(n)
	})
)

func init() {
	groupWatchers = make(map[string]*groupWatcher)
	go groupWatchersCleaner()
}

func groupWatchersCleaner() {
	for {
		time.Sleep(7 * time.Second)
		groupWatchersLock.Lock()
		for key, gw := range groupWatchers {
			gw.mu.Lock()
			// Calculate the number of apiWatcher instances subscribed to gw.
			awsTotal := 0
			for _, uw := range gw.m {
				awsTotal += len(uw.aws) + len(uw.awsPending)
			}

			if awsTotal == 0 && gw.apiWatcherInflightStartCalls.Load() == 0 {
				// There are no API watchers subscribed to gw and there are no in-flight apiWatcher.mustStart() calls.
				// Stop all the urlWatcher instances at gw and drop gw from groupWatchers in this case,
				// but do it only on the second iteration in order to reduce urlWatcher churn
				// during scrape config reloads.
				if gw.noAPIWatchers {
					gw.cancel()
					delete(groupWatchers, key)
				} else {
					gw.noAPIWatchers = true
				}
			} else {
				gw.noAPIWatchers = false
			}
			gw.mu.Unlock()
		}
		groupWatchersLock.Unlock()
	}
}

type swosByKeyWithLock struct {
	mu        sync.Mutex
	swosByKey map[string][]any
}

func (gw *groupWatcher) getScrapeWorkObjectsByAPIWatcherLocked(objectsByKey map[string]object, awsMap map[*apiWatcher]struct{}) map[*apiWatcher]*swosByKeyWithLock {
	if len(awsMap) == 0 {
		return nil
	}
	swosByAPIWatcher := make(map[*apiWatcher]*swosByKeyWithLock, len(awsMap))
	for aw := range awsMap {
		swosByAPIWatcher[aw] = &swosByKeyWithLock{
			swosByKey: make(map[string][]any),
		}
	}

	// Generate ScrapeWork objects in parallel on available CPU cores.
	// This should reduce the time needed for their generation on systems with many CPU cores.
	var wg sync.WaitGroup
	limiterCh := make(chan struct{}, cgroup.AvailableCPUs())
	for key, o := range objectsByKey {
		labelss := o.getTargetLabels(gw)
		wg.Add(1)
		limiterCh <- struct{}{}
		go func(key string, labelss []*promutil.Labels) {
			for aw, e := range swosByAPIWatcher {
				swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labelss)
				e.mu.Lock()
				e.swosByKey[key] = swos
				e.mu.Unlock()
			}
			putLabelssToPool(labelss)
			wg.Done()
			<-limiterCh
		}(key, labelss)
	}
	wg.Wait()
	return swosByAPIWatcher
}

func putLabelssToPool(labelss []*promutil.Labels) {
	for _, labels := range labelss {
		promutil.PutLabels(labels)
	}
}

func (gw *groupWatcher) getObjectByRoleLocked(role, namespace, name string) object {
	if role == "node" {
		// Node objects have no namespace
		namespace = ""
	}
	key := namespace + "/" + name
	for _, uw := range gw.m {
		if uw.role != role {
			// Role mismatch
			continue
		}
		if namespace != "" && uw.namespace != "" && uw.namespace != namespace {
			// Namespace mismatch
			continue
		}
		if o := uw.objectsByKey[key]; o != nil {
			return o
		}
	}
	return nil
}

func (gw *groupWatcher) startWatchersForRole(role string, aw *apiWatcher) {
	if role == "endpoints" || role == "endpointslice" {
		// endpoints and endpointslice watchers query pod and service objects. So start watchers for these roles as well.
		gw.startWatchersForRole("pod", nil)
		gw.startWatchersForRole("service", nil)
	}
	if gw.attachNodeMetadata && (role == "pod" || role == "endpoints" || role == "endpointslice") {
		gw.startWatchersForRole("node", nil)
	}

	paths := getAPIPathsWithNamespaces(role, gw.namespaces, gw.selectors)
	for _, path := range paths {
		apiURL := gw.apiServer + path
		gw.mu.Lock()
		uw := gw.m[apiURL]
		needStart := uw == nil
		if needStart {
			uw = newURLWatcher(role, apiURL, gw)
			gw.m[apiURL] = uw
		}
		if aw != nil {
			uw.subscribeAPIWatcherLocked(aw)
		}
		gw.mu.Unlock()
		if needStart {
			uw.reloadObjects()
			go uw.watchForUpdates()
			if role == "endpoints" || role == "endpointslice" || (gw.attachNodeMetadata && role == "pod") {
				// Refresh targets in background, since they depend on other object types such as pod, service or node.
				// This should guarantee that the ScrapeWork objects for these objects are properly updated
				// as soon as the objects they depend on are updated.
				// This should fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240 .
				go uw.recreateScrapeWorks()
			}
		}
	}
}

// doRequest performs http request to the given requestURL.
func (gw *groupWatcher) doRequest(ctx context.Context, requestURL string) (*http.Response, error) {
	if strings.Contains(requestURL, "/apis/networking.k8s.io/v1/") && gw.useNetworkingV1Beta1.Load() {
		// Update networking URL for old Kubernetes API, which supports only v1beta1 path.
		requestURL = strings.Replace(requestURL, "/apis/networking.k8s.io/v1/", "/apis/networking.k8s.io/v1beta1/", 1)
	}
	if strings.Contains(requestURL, "/apis/discovery.k8s.io/v1/") && gw.useDiscoveryV1Beta1.Load() {
		// Update discovery URL for old Kubernetes API, which supports only v1beta1 path.
		requestURL = strings.Replace(requestURL, "/apis/discovery.k8s.io/v1/", "/apis/discovery.k8s.io/v1beta1/", 1)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		logger.Panicf("FATAL: cannot create a request for %q: %s", requestURL, err)
	}
	if err := gw.setHeaders(req); err != nil {
		return nil, fmt.Errorf("cannot set request headers: %w", err)
	}
	resp, err := gw.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		if strings.Contains(requestURL, "/apis/networking.k8s.io/v1/") && !gw.useNetworkingV1Beta1.Load() {
			gw.useNetworkingV1Beta1.Store(true)
			return gw.doRequest(ctx, requestURL)
		}
		if strings.Contains(requestURL, "/apis/discovery.k8s.io/v1/") && !gw.useDiscoveryV1Beta1.Load() {
			gw.useDiscoveryV1Beta1.Store(true)
			return gw.doRequest(ctx, requestURL)
		}
	}
	return resp, nil
}

func (gw *groupWatcher) registerPendingAPIWatchers() {
	gw.mu.Lock()
	for _, uw := range gw.m {
		uw.registerPendingAPIWatchersLocked()
	}
	gw.mu.Unlock()
}

func (gw *groupWatcher) unsubscribeAPIWatcher(aw *apiWatcher) {
	gw.mu.Lock()
	for _, uw := range gw.m {
		uw.unsubscribeAPIWatcherLocked(aw)
	}
	gw.mu.Unlock()
}

// urlWatcher watches for an apiURL and updates object states in objectsByKey.
//
// urlWatcher fields must be accessed under gw.mu lock.
type urlWatcher struct {
	role      string
	namespace string
	apiURL    string
	gw        *groupWatcher

	parseObject     parseObjectFunc
	parseObjectList parseObjectListFunc

	// awsPending contains pending apiWatcher objects, which are registered in a batch.
	// Batch registering saves CPU time needed for registering big number of Kubernetes objects
	// shared among big number of scrape jobs, since per-object labels are generated only once
	// for all the scrape jobs (each scrape job is associated with a single apiWatcher).
	// See registerPendingAPIWatchersLocked for details.
	awsPending map[*apiWatcher]struct{}

	// aws contains registered apiWatcher objects
	aws map[*apiWatcher]struct{}

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey map[string]object

	needRecreateScrapeWorks bool

	resourceVersion string

	objectsCount          *metrics.Counter
	objectsAdded          *metrics.Counter
	objectsRemoved        *metrics.Counter
	objectsUpdated        *metrics.Counter
	staleResourceVersions *metrics.Counter
}

func newURLWatcher(role, apiURL string, gw *groupWatcher) *urlWatcher {
	parseObject, parseObjectList := getObjectParsersForRole(role)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers{role=%q}`, role)).Inc()

	uw := &urlWatcher{
		role:   role,
		apiURL: apiURL,
		gw:     gw,

		parseObject:     parseObject,
		parseObjectList: parseObjectList,

		awsPending:   make(map[*apiWatcher]struct{}),
		aws:          make(map[*apiWatcher]struct{}),
		objectsByKey: make(map[string]object),

		objectsCount:          metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects{role=%q}`, role)),
		objectsAdded:          metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_added_total{role=%q}`, role)),
		objectsRemoved:        metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_removed_total{role=%q}`, role)),
		objectsUpdated:        metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_updated_total{role=%q}`, role)),
		staleResourceVersions: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_stale_resource_versions_total{role=%q}`, role)),
	}
	logger.Infof("started %s watcher for %q", uw.role, uw.apiURL)
	return uw
}

func (uw *urlWatcher) recreateScrapeWorks() {
	minSleepTime := timeutil.AddJitterToDuration(5 * time.Second)
	sleepTime := minSleepTime
	gw := uw.gw
	stopCh := gw.ctx.Done()
	for {
		t := timerpool.Get(sleepTime)
		select {
		case <-stopCh:
			timerpool.Put(t)
			return
		case <-t.C:
			timerpool.Put(t)
		}
		startTime := time.Now()
		gw.mu.Lock()
		if uw.needRecreateScrapeWorks {
			uw.needRecreateScrapeWorks = false
			uw.recreateScrapeWorksLocked(uw.objectsByKey, uw.aws)
			sleepTime = time.Since(startTime)
			if sleepTime < minSleepTime {
				sleepTime = minSleepTime
			}
		}
		gw.mu.Unlock()
	}
}

func (uw *urlWatcher) subscribeAPIWatcherLocked(aw *apiWatcher) {
	if _, ok := uw.aws[aw]; !ok {
		if _, ok := uw.awsPending[aw]; !ok {
			uw.awsPending[aw] = struct{}{}
			metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Inc()
		}
	}
}

func (uw *urlWatcher) registerPendingAPIWatchersLocked() {
	if len(uw.awsPending) == 0 {
		return
	}
	uw.recreateScrapeWorksLocked(uw.objectsByKey, uw.awsPending)
	for aw := range uw.awsPending {
		uw.aws[aw] = struct{}{}
	}
	awsPendingLen := len(uw.awsPending)
	uw.awsPending = make(map[*apiWatcher]struct{})
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="working"}`, uw.role)).Add(awsPendingLen)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Add(-awsPendingLen)
}

func (uw *urlWatcher) recreateScrapeWorksLocked(objectsByKey map[string]object, awsMap map[*apiWatcher]struct{}) {
	es := uw.gw.getScrapeWorkObjectsByAPIWatcherLocked(objectsByKey, awsMap)
	for aw, e := range es {
		swosByKey := e.swosByKey
		for key, swos := range swosByKey {
			if len(swos) == 0 {
				delete(swosByKey, key)
			}
		}
		aw.replaceScrapeWorks(uw, swosByKey)
	}
}

func (uw *urlWatcher) updateScrapeWorksLocked(objectsByKey map[string]object, awsMap map[*apiWatcher]struct{}) {
	es := uw.gw.getScrapeWorkObjectsByAPIWatcherLocked(objectsByKey, awsMap)
	for aw, e := range es {
		aw.updateScrapeWorks(uw, e.swosByKey)
	}
}

func (uw *urlWatcher) removeScrapeWorksLocked(keys []string) {
	for _, key := range keys {
		for aw := range uw.aws {
			aw.removeScrapeWorks(uw, key)
		}
	}
}

func (uw *urlWatcher) unsubscribeAPIWatcherLocked(aw *apiWatcher) {
	if _, ok := uw.awsPending[aw]; ok {
		delete(uw.awsPending, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Dec()
	}
	if _, ok := uw.aws[aw]; ok {
		delete(uw.aws, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="working"}`, uw.role)).Dec()
	}
}

// reloadObjects reloads objects to the latest state and returns resourceVersion for the latest state.
func (uw *urlWatcher) reloadObjects() string {
	if uw.resourceVersion != "" {
		// Fast path - there is no need in reloading the objects.
		return uw.resourceVersion
	}

	gw := uw.gw
	startTime := time.Now()
	apiURL := uw.apiURL

	// Set resourceVersion to 0 in order to reduce load on Kubernetes control plane.
	// See https://kubernetes.io/docs/reference/using-api/api-concepts/#semantics-for-get-and-list
	// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4855 .
	delimiter := getQueryArgsDelimiter(apiURL)
	requestURL := apiURL + delimiter + "resourceVersion=0&resourceVersionMatch=NotOlderThan"
	resp, err := gw.doRequest(gw.ctx, requestURL)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Errorf("cannot perform request to %q: %s", requestURL, err)
		}
		return ""
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
		return ""
	}
	objectsByKey, metadata, err := uw.parseObjectList(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		logger.Errorf("cannot parse objects from %q: %s", requestURL, err)
		return ""
	}

	gw.mu.Lock()
	objectsAdded := make(map[string]object)
	objectsUpdated := make(map[string]object)
	var objectsRemoved []string
	for key, oPrev := range uw.objectsByKey {
		o, ok := objectsByKey[key]
		if ok {
			if !reflect.DeepEqual(oPrev, o) {
				objectsUpdated[key] = o
			}
			// Overwrite oPrev with o even if these objects are equal.
			// This should free up memory associated with oPrev.
			uw.objectsByKey[key] = o
		} else {
			objectsRemoved = append(objectsRemoved, key)
			delete(uw.objectsByKey, key)
		}
	}
	for key, o := range objectsByKey {
		if _, ok := uw.objectsByKey[key]; !ok {
			objectsAdded[key] = o
			uw.objectsByKey[key] = o
		}
	}
	uw.removeScrapeWorksLocked(objectsRemoved)
	uw.updateScrapeWorksLocked(objectsUpdated, uw.aws)
	uw.updateScrapeWorksLocked(objectsAdded, uw.aws)
	uw.needRecreateScrapeWorks = false
	if len(objectsRemoved) > 0 || len(objectsUpdated) > 0 || len(objectsAdded) > 0 {
		uw.maybeUpdateDependedScrapeWorksLocked()
	}
	gw.mu.Unlock()

	uw.objectsUpdated.Add(len(objectsUpdated))
	uw.objectsRemoved.Add(len(objectsRemoved))
	uw.objectsAdded.Add(len(objectsAdded))
	uw.objectsCount.Add(len(objectsAdded) - len(objectsRemoved))
	uw.resourceVersion = metadata.ResourceVersion

	logger.Infof("reloaded %d objects from %q in %.3fs; updated=%d, removed=%d, added=%d, resourceVersion=%q",
		len(objectsByKey), requestURL, time.Since(startTime).Seconds(), len(objectsUpdated), len(objectsRemoved), len(objectsAdded), uw.resourceVersion)
	return uw.resourceVersion
}

// watchForUpdates watches for object updates starting from uw.resourceVersion and updates the corresponding objects to the latest state.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
func (uw *urlWatcher) watchForUpdates() {
	gw := uw.gw
	stopCh := gw.ctx.Done()
	minBackoffDelay := timeutil.AddJitterToDuration(time.Second)
	maxBackoffDelay := timeutil.AddJitterToDuration(time.Second * 30)
	backoffDelay := minBackoffDelay
	backoffSleep := func() {
		t := timerpool.Get(backoffDelay)
		select {
		case <-stopCh:
			timerpool.Put(t)
			return
		case <-t.C:
			timerpool.Put(t)
		}
		backoffDelay *= 2
		if backoffDelay > maxBackoffDelay {
			backoffDelay = maxBackoffDelay
		}
	}
	apiURL := uw.apiURL
	delimiter := getQueryArgsDelimiter(apiURL)
	timeoutSeconds := time.Duration(0.9 * float64(gw.client.Timeout)).Seconds()
	apiURL += delimiter + "watch=1&allowWatchBookmarks=true&timeoutSeconds=" + strconv.Itoa(int(timeoutSeconds))
	for {
		select {
		case <-stopCh:
			metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers{role=%q}`, uw.role)).Dec()
			logger.Infof("stopped %s watcher for %q", uw.role, uw.apiURL)
			return
		default:
		}

		resourceVersion := uw.reloadObjects()
		if resourceVersion == "" {
			backoffSleep()
			continue
		}
		requestURL := apiURL + "&resourceVersion=" + url.QueryEscape(resourceVersion)
		resp, err := gw.doRequest(gw.ctx, requestURL)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot perform request to %q: %s", requestURL, err)
				backoffSleep()
			}
			continue
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == 410 {
				// There is no need for sleep on 410 error. See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				backoffDelay = minBackoffDelay
				uw.staleResourceVersions.Inc()
				uw.resourceVersion = ""
			} else {
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
				backoffSleep()
			}
			continue
		}
		backoffDelay = minBackoffDelay
		err = uw.readObjectUpdateStream(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
				logger.Errorf("error when reading WatchEvent stream from %q: %s", requestURL, err)
				uw.resourceVersion = ""
			}
			backoffSleep()
			continue
		}
	}
}

// readObjectUpdateStream reads Kubernetes watch events from r and updates locally cached objects according to the received events.
func (uw *urlWatcher) readObjectUpdateStream(r io.Reader) error {
	gw := uw.gw
	d := json.NewDecoder(r)
	var we WatchEvent
	for {
		if err := d.Decode(&we); err != nil {
			return fmt.Errorf("cannot parse WatchEvent json response: %w", err)
		}
		switch we.Type {
		case "ADDED", "MODIFIED":
			o, err := uw.parseObject(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse %s object: %w", we.Type, err)
			}
			key := o.key()
			gw.mu.Lock()
			uw.updateObjectLocked(key, o)
			gw.mu.Unlock()
		case "DELETED":
			o, err := uw.parseObject(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse %s object: %w", we.Type, err)
			}
			key := o.key()
			gw.mu.Lock()
			uw.removeObjectLocked(key)
			gw.mu.Unlock()
		case "BOOKMARK":
			// See https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
			bm, err := parseBookmark(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse bookmark from %q: %w", we.Object, err)
			}
			uw.resourceVersion = bm.Metadata.ResourceVersion
		case "ERROR":
			em, err := parseError(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse error message from %q: %w", we.Object, err)
			}
			if em.Code == 410 {
				// See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				uw.staleResourceVersions.Inc()
				uw.resourceVersion = ""
				return nil
			}
			return fmt.Errorf("unexpected error message: %q", we.Object)
		default:
			return fmt.Errorf("unexpected WatchEvent type %q: %q", we.Type, we.Object)
		}
	}
}

func (uw *urlWatcher) updateObjectLocked(key string, o object) {
	oPrev, ok := uw.objectsByKey[key]
	// Overwrite oPrev with o even if these objects are equal.
	// This should free up memory associated with oPrev.
	uw.objectsByKey[key] = o
	if !ok {
		uw.objectsCount.Inc()
		uw.objectsAdded.Inc()
	} else {
		if reflect.DeepEqual(oPrev, o) {
			// Nothing to do, since the new object is equal to the previous one.
			return
		}
		uw.objectsUpdated.Inc()
	}
	if len(uw.aws) > 0 {
		labelss := o.getTargetLabels(uw.gw)
		for aw := range uw.aws {
			aw.setScrapeWorks(uw, key, labelss)
		}
		putLabelssToPool(labelss)
	}
	uw.maybeUpdateDependedScrapeWorksLocked()
}

func (uw *urlWatcher) removeObjectLocked(key string) {
	if _, ok := uw.objectsByKey[key]; !ok {
		return
	}
	uw.objectsCount.Dec()
	uw.objectsRemoved.Inc()
	delete(uw.objectsByKey, key)
	for aw := range uw.aws {
		aw.removeScrapeWorks(uw, key)
	}
	uw.maybeUpdateDependedScrapeWorksLocked()
}

func (uw *urlWatcher) maybeUpdateDependedScrapeWorksLocked() {
	role := uw.role
	attachNodeMetadata := uw.gw.attachNodeMetadata
	if !(role == "pod" || role == "service" || (attachNodeMetadata && role == "node")) {
		// Nothing to update
		return
	}
	namespace := uw.namespace
	for _, uwx := range uw.gw.m {
		if namespace != "" && uwx.namespace != "" && uwx.namespace != namespace {
			// Namespace mismatch
			continue
		}
		if (role == "pod" || role == "service") && (uwx.role == "endpoints" || uwx.role == "endpointslice") {
			// endpoints and endpointslice objects depend on pods and service objects
			uwx.needRecreateScrapeWorks = true
			continue
		}
		if attachNodeMetadata && role == "node" && (uwx.role == "pod" || uwx.role == "endpoints" || uwx.role == "endpointslice") {
			// pod, endpoints and enpointslices objects depend on node objects if attachNodeMetadata is set
			uwx.needRecreateScrapeWorks = true
			continue
		}
	}
}

// Bookmark is a bookmark message from Kubernetes Watch API.
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
type Bookmark struct {
	Metadata BookmarkMetadata
}

// BookmarkMetadata is metadata for Bookmark
type BookmarkMetadata struct {
	ResourceVersion string
}

func parseBookmark(data []byte) (*Bookmark, error) {
	var bm Bookmark
	if err := json.Unmarshal(data, &bm); err != nil {
		return nil, err
	}
	return &bm, nil
}

// Error is an error message from Kubernetes Watch API.
type Error struct {
	Code int
}

func parseError(data []byte) (*Error, error) {
	var em Error
	if err := json.Unmarshal(data, &em); err != nil {
		return nil, err
	}
	return &em, nil
}

func getAPIPathsWithNamespaces(role string, namespaces []string, selectors []Selector) []string {
	objectType := getObjectTypeByRole(role)
	if objectType == "nodes" || len(namespaces) == 0 {
		query := joinSelectors(role, selectors)
		path := getAPIPath(objectType, "", query)
		return []string{path}
	}
	query := joinSelectors(role, selectors)
	paths := make([]string, len(namespaces))
	for i, namespace := range namespaces {
		paths[i] = getAPIPath(objectType, namespace, query)
	}
	return paths
}

func getAPIPath(objectType, namespace, query string) string {
	suffix := objectType
	if namespace != "" {
		suffix = "namespaces/" + namespace + "/" + objectType
	}
	if len(query) > 0 {
		suffix += "?" + query
	}
	if objectType == "ingresses" {
		return "/apis/networking.k8s.io/v1/" + suffix
	}
	if objectType == "endpointslices" {
		return "/apis/discovery.k8s.io/v1/" + suffix
	}
	return "/api/v1/" + suffix
}

func joinSelectors(role string, selectors []Selector) string {
	var labelSelectors, fieldSelectors []string
	for _, s := range selectors {
		if s.Role != role {
			continue
		}
		if s.Label != "" {
			labelSelectors = append(labelSelectors, s.Label)
		}
		if s.Field != "" {
			fieldSelectors = append(fieldSelectors, s.Field)
		}
	}
	var args []string
	if len(labelSelectors) > 0 {
		args = append(args, "labelSelector="+url.QueryEscape(strings.Join(labelSelectors, ",")))
	}
	if len(fieldSelectors) > 0 {
		args = append(args, "fieldSelector="+url.QueryEscape(strings.Join(fieldSelectors, ",")))
	}
	return strings.Join(args, "&")
}

func getObjectTypeByRole(role string) string {
	switch role {
	case "node":
		return "nodes"
	case "pod":
		return "pods"
	case "service":
		return "services"
	case "endpoints":
		return "endpoints"
	case "endpointslice":
		return "endpointslices"
	case "ingress":
		return "ingresses"
	default:
		logger.Panicf("BUG: unknown role=%q", role)
		return ""
	}
}

func getObjectParsersForRole(role string) (parseObjectFunc, parseObjectListFunc) {
	switch role {
	case "node":
		return parseNode, parseNodeList
	case "pod":
		return parsePod, parsePodList
	case "service":
		return parseService, parseServiceList
	case "endpoints":
		return parseEndpoints, parseEndpointsList
	case "endpointslice":
		return parseEndpointSlice, parseEndpointSliceList
	case "ingress":
		return parseIngress, parseIngressList
	default:
		logger.Panicf("BUG: unsupported role=%q", role)
		return nil, nil
	}
}

func getQueryArgsDelimiter(apiURL string) string {
	if strings.Contains(apiURL, "?") {
		return "&"
	}
	return "?"
}
