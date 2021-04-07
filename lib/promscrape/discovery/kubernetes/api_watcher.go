package kubernetes

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/metrics"
)

var apiServerTimeout = flag.Duration("promscrape.kubernetes.apiServerTimeout", 30*time.Minute, "How frequently to reload the full state from Kuberntes API server")

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
	getTargetLabels(gw *groupWatcher) []map[string]string
}

// parseObjectFunc must parse object from the given data.
type parseObjectFunc func(data []byte) (object, error)

// parseObjectListFunc must parse objectList from the given r.
type parseObjectListFunc func(r io.Reader) (map[string]object, ListMeta, error)

// apiWatcher is used for watching for Kuberntes object changes and caching their latest states.
type apiWatcher struct {
	role string

	// Constructor for creating ScrapeWork objects from labels
	swcFunc ScrapeWorkConstructorFunc

	gw *groupWatcher

	// swos contains per-namepsace maps of ScrapeWork objects for the given apiWatcher
	swosByNamespace     map[string]map[string][]interface{}
	swosByNamespaceLock sync.Mutex

	swosCount *metrics.Counter
}

func newAPIWatcher(apiServer string, ac *promauth.Config, sdc *SDConfig, swcFunc ScrapeWorkConstructorFunc) *apiWatcher {
	namespaces := sdc.Namespaces.Names
	selectors := sdc.Selectors
	proxyURL := sdc.ProxyURL.URL()
	gw := getGroupWatcher(apiServer, ac, namespaces, selectors, proxyURL)
	return &apiWatcher{
		role:            sdc.Role,
		swcFunc:         swcFunc,
		gw:              gw,
		swosByNamespace: make(map[string]map[string][]interface{}),
		swosCount:       metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_scrape_works{role=%q}`, sdc.Role)),
	}
}

func (aw *apiWatcher) mustStart() {
	aw.gw.startWatchersForRole(aw.role, aw)
}

func (aw *apiWatcher) mustStop() {
	aw.gw.unsubscribeAPIWatcher(aw)
	aw.swosByNamespaceLock.Lock()
	aw.swosByNamespace = make(map[string]map[string][]interface{})
	aw.swosByNamespaceLock.Unlock()
}

func (aw *apiWatcher) reloadScrapeWorks(namespace string, swosByKey map[string][]interface{}) {
	aw.swosByNamespaceLock.Lock()
	aw.swosCount.Add(len(swosByKey) - len(aw.swosByNamespace[namespace]))
	aw.swosByNamespace[namespace] = swosByKey
	aw.swosByNamespaceLock.Unlock()
}

func (aw *apiWatcher) setScrapeWorks(namespace, key string, labels []map[string]string) {
	swos := aw.getScrapeWorkObjectsForLabels(labels)
	aw.swosByNamespaceLock.Lock()
	swosByKey := aw.swosByNamespace[namespace]
	if swosByKey == nil {
		swosByKey = make(map[string][]interface{})
		aw.swosByNamespace[namespace] = swosByKey
	}
	if len(swos) > 0 {
		aw.swosCount.Add(len(swos) - len(swosByKey[key]))
		swosByKey[key] = swos
	} else {
		aw.swosCount.Add(-len(swosByKey[key]))
		delete(swosByKey, key)
	}
	aw.swosByNamespaceLock.Unlock()
}

func (aw *apiWatcher) removeScrapeWorks(namespace, key string) {
	aw.swosByNamespaceLock.Lock()
	swosByKey := aw.swosByNamespace[namespace]
	if len(swosByKey) > 0 {
		aw.swosCount.Add(-len(swosByKey[key]))
		delete(swosByKey, key)
	}
	aw.swosByNamespaceLock.Unlock()
}

func (aw *apiWatcher) getScrapeWorkObjectsForLabels(labelss []map[string]string) []interface{} {
	swos := make([]interface{}, 0, len(labelss))
	for _, labels := range labelss {
		swo := aw.swcFunc(labels)
		// The reflect check is needed because of https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
		if swo != nil && !reflect.ValueOf(swo).IsNil() {
			swos = append(swos, swo)
		}
	}
	return swos
}

// getScrapeWorkObjects returns all the ScrapeWork objects for the given aw.
func (aw *apiWatcher) getScrapeWorkObjects() []interface{} {
	aw.gw.registerPendingAPIWatchers()

	aw.swosByNamespaceLock.Lock()
	defer aw.swosByNamespaceLock.Unlock()

	size := 0
	for _, swosByKey := range aw.swosByNamespace {
		for _, swosLocal := range swosByKey {
			size += len(swosLocal)
		}
	}
	swos := make([]interface{}, 0, size)
	for _, swosByKey := range aw.swosByNamespace {
		for _, swosLocal := range swosByKey {
			swos = append(swos, swosLocal...)
		}
	}
	return swos
}

// groupWatcher watches for Kubernetes objects on the given apiServer with the given namespaces,
// selectors and authorization using the given client.
type groupWatcher struct {
	apiServer     string
	namespaces    []string
	selectors     []Selector
	authorization string
	client        *http.Client

	mu sync.Mutex
	m  map[string]*urlWatcher
}

func newGroupWatcher(apiServer string, ac *promauth.Config, namespaces []string, selectors []Selector, proxyURL *url.URL) *groupWatcher {
	var proxy func(*http.Request) (*url.URL, error)
	if proxyURL != nil {
		proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     ac.NewTLSConfig(),
			Proxy:               proxy,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     *apiServerTimeout,
		},
		Timeout: *apiServerTimeout,
	}
	return &groupWatcher{
		apiServer:     apiServer,
		authorization: ac.Authorization,
		namespaces:    namespaces,
		selectors:     selectors,
		client:        client,
		m:             make(map[string]*urlWatcher),
	}
}

func getGroupWatcher(apiServer string, ac *promauth.Config, namespaces []string, selectors []Selector, proxyURL *url.URL) *groupWatcher {
	proxyURLStr := "<nil>"
	if proxyURL != nil {
		proxyURLStr = proxyURL.String()
	}
	key := fmt.Sprintf("apiServer=%s, namespaces=%s, selectors=%s, proxyURL=%s, authConfig=%s",
		apiServer, namespaces, selectorsKey(selectors), proxyURLStr, ac.String())
	groupWatchersLock.Lock()
	gw := groupWatchers[key]
	if gw == nil {
		gw = newGroupWatcher(apiServer, ac, namespaces, selectors, proxyURL)
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
	groupWatchers     = make(map[string]*groupWatcher)

	_ = metrics.NewGauge(`vm_promscrape_discovery_kubernetes_group_watchers`, func() float64 {
		groupWatchersLock.Lock()
		n := len(groupWatchers)
		groupWatchersLock.Unlock()
		return float64(n)
	})
)

// getObjectByRole returns an object with the given (namespace, name) key and the given role.
func (gw *groupWatcher) getObjectByRole(role, namespace, name string) object {
	if gw == nil {
		// this is needed for testing
		return nil
	}
	o := gw.getCachedObjectByRole(role, namespace, name)
	if o != nil {
		// Fast path: the object has been found in the cache.
		return o
	}

	// The object wasn't found in the cache. Try querying it directly from API server.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1182#issuecomment-813353359 for details.
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_loads_total{role=%q}`, role)).Inc()
	objectType := getObjectTypeByRole(role)
	path := getAPIPath(objectType, namespace, "")
	path += "/" + name
	requestURL := gw.apiServer + path
	resp, err := gw.doRequest(requestURL)
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_load_errors_total{role=%q}`, role)).Inc()
		logger.Errorf("cannot obtain data for object %s (namespace=%q, name=%q): %s", role, namespace, name, err)
		return nil
	}
	data, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_load_errors_total{role=%q}`, role)).Inc()
		logger.Errorf("cannot read response from %q: %s", requestURL, err)
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_load_misses_total{role=%q}`, role)).Inc()
			return nil
		}
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_load_errors_total{role=%q}`, role)).Inc()
		logger.Errorf("unexpected status code when reading response from %q; got %d; want %d; response body: %q", requestURL, resp.StatusCode, http.StatusOK, data)
		return nil
	}
	parseObject, _ := getObjectParsersForRole(role)
	o, err = parseObject(data)
	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_direct_object_load_errors_total{role=%q}`, role)).Inc()
		logger.Errorf("cannot parse object obtained from %q: %s; response body: %q", requestURL, err, data)
		return nil
	}
	// There is no need in storing the object in urlWatcher cache, since it should be eventually populated there by urlWatcher itself.
	return o
}

func (gw *groupWatcher) getCachedObjectByRole(role, namespace, name string) object {
	key := namespace + "/" + name
	gw.startWatchersForRole(role, nil)
	uws := gw.getURLWatchers()
	for _, uw := range uws {
		if uw.role != role {
			// Role mismatch
			continue
		}
		if uw.namespace != "" && uw.namespace != namespace {
			// Namespace mismatch
			continue
		}
		uw.mu.Lock()
		o := uw.objectsByKey[key]
		uw.mu.Unlock()
		if o != nil {
			return o
		}
	}
	return nil
}

func (gw *groupWatcher) startWatchersForRole(role string, aw *apiWatcher) {
	paths, namespaces := getAPIPathsWithNamespaces(role, gw.namespaces, gw.selectors)
	for i, path := range paths {
		apiURL := gw.apiServer + path
		gw.mu.Lock()
		uw := gw.m[apiURL]
		needStart := uw == nil
		if needStart {
			uw = newURLWatcher(role, namespaces[i], apiURL, gw)
			gw.m[apiURL] = uw
		}
		gw.mu.Unlock()
		if needStart {
			uw.reloadObjects()
			go uw.watchForUpdates()
		}
		if aw != nil {
			uw.subscribeAPIWatcher(aw)
		}
	}
}

// doRequest performs http request to the given requestURL.
func (gw *groupWatcher) doRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		logger.Fatalf("cannot create a request for %q: %s", requestURL, err)
	}
	if gw.authorization != "" {
		req.Header.Set("Authorization", gw.authorization)
	}
	return gw.client.Do(req)
}

func (gw *groupWatcher) registerPendingAPIWatchers() {
	uws := gw.getURLWatchers()
	for _, uw := range uws {
		uw.registerPendingAPIWatchers()
	}
}

func (gw *groupWatcher) getURLWatchers() []*urlWatcher {
	gw.mu.Lock()
	uws := make([]*urlWatcher, 0, len(gw.m))
	for _, uw := range gw.m {
		uws = append(uws, uw)
	}
	gw.mu.Unlock()
	return uws
}

func (gw *groupWatcher) unsubscribeAPIWatcher(aw *apiWatcher) {
	uws := gw.getURLWatchers()
	for _, uw := range uws {
		uw.unsubscribeAPIWatcher(aw)
	}
}

// urlWatcher watches for an apiURL and updates object states in objectsByKey.
type urlWatcher struct {
	role      string
	namespace string
	apiURL    string
	gw        *groupWatcher

	parseObject     parseObjectFunc
	parseObjectList parseObjectListFunc

	// mu protects aws, awsPending, objectsByKey and resourceVersion
	mu sync.Mutex

	// awsPending contains pending apiWatcher objects, which are registered in a batch.
	// Batch registering saves CPU time needed for registering big number of Kubernetes objects
	// shared among big number of scrape jobs, since per-object labels are generated only once
	// for all the scrape jobs (each scrape job is associated with a single apiWatcher).
	// See reloadScrapeWorksForAPIWatchers for details.
	awsPending map[*apiWatcher]struct{}

	// aws contains registered apiWatcher objects
	aws map[*apiWatcher]struct{}

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey map[string]object

	resourceVersion string

	objectsCount          *metrics.Counter
	objectsAdded          *metrics.Counter
	objectsRemoved        *metrics.Counter
	objectsUpdated        *metrics.Counter
	staleResourceVersions *metrics.Counter
}

func newURLWatcher(role, namespace, apiURL string, gw *groupWatcher) *urlWatcher {
	parseObject, parseObjectList := getObjectParsersForRole(role)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers{role=%q}`, role)).Inc()
	uw := &urlWatcher{
		role:      role,
		namespace: namespace,
		apiURL:    apiURL,
		gw:        gw,

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

func (uw *urlWatcher) subscribeAPIWatcher(aw *apiWatcher) {
	uw.mu.Lock()
	if _, ok := uw.aws[aw]; !ok {
		if _, ok := uw.awsPending[aw]; !ok {
			uw.awsPending[aw] = struct{}{}
			metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Inc()
		}
	}
	uw.mu.Unlock()
}

func (uw *urlWatcher) registerPendingAPIWatchers() {
	uw.mu.Lock()
	awsPending := make([]*apiWatcher, 0, len(uw.awsPending))
	for aw := range uw.awsPending {
		awsPending = append(awsPending, aw)
		delete(uw.awsPending, aw)
		uw.aws[aw] = struct{}{}
	}
	uw.reloadScrapeWorksForAPIWatchers(awsPending, uw.objectsByKey)
	uw.mu.Unlock()
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="working"}`, uw.role)).Add(len(awsPending))
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Add(-len(awsPending))
}

func (uw *urlWatcher) unsubscribeAPIWatcher(aw *apiWatcher) {
	uw.mu.Lock()
	if _, ok := uw.awsPending[aw]; ok {
		delete(uw.awsPending, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="pending"}`, uw.role)).Dec()
	}
	if _, ok := uw.aws[aw]; ok {
		delete(uw.aws, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscribers{role=%q,status="working"}`, uw.role)).Dec()
	}
	uw.mu.Unlock()
}

func (uw *urlWatcher) setResourceVersion(resourceVersion string) {
	uw.mu.Lock()
	uw.resourceVersion = resourceVersion
	uw.mu.Unlock()
}

// reloadObjects reloads objects to the latest state and returns resourceVersion for the latest state.
func (uw *urlWatcher) reloadObjects() string {
	uw.mu.Lock()
	resourceVersion := uw.resourceVersion
	uw.mu.Unlock()
	if resourceVersion != "" {
		// Fast path - there is no need in reloading the objects.
		return resourceVersion
	}

	requestURL := uw.apiURL
	resp, err := uw.gw.doRequest(requestURL)
	if err != nil {
		logger.Errorf("cannot perform request to %q: %s", requestURL, err)
		return ""
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
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

	uw.mu.Lock()
	var updated, removed, added int
	for key := range uw.objectsByKey {
		if o, ok := objectsByKey[key]; ok {
			uw.objectsByKey[key] = o
			updated++
		} else {
			delete(uw.objectsByKey, key)
			removed++
		}
	}
	for key, o := range objectsByKey {
		if _, ok := uw.objectsByKey[key]; !ok {
			uw.objectsByKey[key] = o
			added++
		}
	}
	uw.objectsUpdated.Add(updated)
	uw.objectsRemoved.Add(removed)
	uw.objectsAdded.Add(added)
	uw.objectsCount.Add(added - removed)
	uw.resourceVersion = metadata.ResourceVersion
	aws := getAPIWatchers(uw.aws)
	uw.mu.Unlock()

	uw.reloadScrapeWorksForAPIWatchers(aws, objectsByKey)
	logger.Infof("reloaded %d objects from %q", len(objectsByKey), requestURL)
	return uw.resourceVersion
}

func (uw *urlWatcher) reloadScrapeWorksForAPIWatchers(aws []*apiWatcher, objectsByKey map[string]object) {
	if len(aws) == 0 {
		return
	}
	swosByKey := make([]map[string][]interface{}, len(aws))
	for i := range aws {
		swosByKey[i] = make(map[string][]interface{})
	}
	for key, o := range objectsByKey {
		labels := o.getTargetLabels(uw.gw)
		for i, aw := range aws {
			swos := aw.getScrapeWorkObjectsForLabels(labels)
			if len(swos) > 0 {
				swosByKey[i][key] = swos
			}
		}
	}
	for i, aw := range aws {
		aw.reloadScrapeWorks(uw.namespace, swosByKey[i])
	}
}

func getAPIWatchers(awsMap map[*apiWatcher]struct{}) []*apiWatcher {
	aws := make([]*apiWatcher, 0, len(awsMap))
	for aw := range awsMap {
		aws = append(aws, aw)
	}
	return aws
}

// watchForUpdates watches for object updates starting from uw.resourceVersion and updates the corresponding objects to the latest state.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
func (uw *urlWatcher) watchForUpdates() {
	backoffDelay := time.Second
	maxBackoffDelay := 30 * time.Second
	backoffSleep := func() {
		time.Sleep(backoffDelay)
		backoffDelay *= 2
		if backoffDelay > maxBackoffDelay {
			backoffDelay = maxBackoffDelay
		}
	}
	apiURL := uw.apiURL
	delimiter := "?"
	if strings.Contains(apiURL, "?") {
		delimiter = "&"
	}
	timeoutSeconds := time.Duration(0.9 * float64(uw.gw.client.Timeout)).Seconds()
	apiURL += delimiter + "watch=1&allowWatchBookmarks=true&timeoutSeconds=" + strconv.Itoa(int(timeoutSeconds))
	for {
		resourceVersion := uw.reloadObjects()
		if resourceVersion == "" {
			backoffSleep()
			continue
		}
		requestURL := apiURL + "&resourceVersion=" + url.QueryEscape(resourceVersion)
		resp, err := uw.gw.doRequest(requestURL)
		if err != nil {
			logger.Errorf("cannot perform request to %q: %s", requestURL, err)
			backoffSleep()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == 410 {
				// There is no need for sleep on 410 error. See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				backoffDelay = time.Second
				uw.staleResourceVersions.Inc()
				uw.setResourceVersion("")
			} else {
				body, _ := ioutil.ReadAll(resp.Body)
				_ = resp.Body.Close()
				logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
				backoffSleep()
			}
			continue
		}
		backoffDelay = time.Second
		err = uw.readObjectUpdateStream(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Errorf("error when reading WatchEvent stream from %q: %s", requestURL, err)
			}
			backoffSleep()
			continue
		}
	}
}

// readObjectUpdateStream reads Kuberntes watch events from r and updates locally cached objects according to the received events.
func (uw *urlWatcher) readObjectUpdateStream(r io.Reader) error {
	d := json.NewDecoder(r)
	var we WatchEvent
	for {
		if err := d.Decode(&we); err != nil {
			return err
		}
		switch we.Type {
		case "ADDED", "MODIFIED":
			o, err := uw.parseObject(we.Object)
			if err != nil {
				return err
			}
			key := o.key()
			uw.mu.Lock()
			if _, ok := uw.objectsByKey[key]; !ok {
				uw.objectsCount.Inc()
				uw.objectsAdded.Inc()
			} else {
				uw.objectsUpdated.Inc()
			}
			uw.objectsByKey[key] = o
			aws := getAPIWatchers(uw.aws)
			uw.mu.Unlock()
			labels := o.getTargetLabels(uw.gw)
			for _, aw := range aws {
				aw.setScrapeWorks(uw.namespace, key, labels)
			}
		case "DELETED":
			o, err := uw.parseObject(we.Object)
			if err != nil {
				return err
			}
			key := o.key()
			uw.mu.Lock()
			if _, ok := uw.objectsByKey[key]; ok {
				uw.objectsCount.Dec()
				uw.objectsRemoved.Inc()
				delete(uw.objectsByKey, key)
			}
			aws := getAPIWatchers(uw.aws)
			uw.mu.Unlock()
			for _, aw := range aws {
				aw.removeScrapeWorks(uw.namespace, key)
			}
		case "BOOKMARK":
			// See https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
			bm, err := parseBookmark(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse bookmark from %q: %w", we.Object, err)
			}
			uw.setResourceVersion(bm.Metadata.ResourceVersion)
		case "ERROR":
			em, err := parseError(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse error message from %q: %w", we.Object, err)
			}
			if em.Code == 410 {
				// See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				uw.staleResourceVersions.Inc()
				uw.setResourceVersion("")
				return nil
			}
			return fmt.Errorf("unexpected error message: %q", we.Object)
		default:
			return fmt.Errorf("unexpected WatchEvent type %q: %q", we.Type, we.Object)
		}
	}
}

// Bookmark is a bookmark message from Kubernetes Watch API.
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
type Bookmark struct {
	Metadata struct {
		ResourceVersion string
	}
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

func getAPIPathsWithNamespaces(role string, namespaces []string, selectors []Selector) ([]string, []string) {
	objectType := getObjectTypeByRole(role)
	if objectType == "nodes" || len(namespaces) == 0 {
		query := joinSelectors(role, selectors)
		path := getAPIPath(objectType, "", query)
		return []string{path}, []string{""}
	}
	query := joinSelectors(role, selectors)
	paths := make([]string, len(namespaces))
	for i, namespace := range namespaces {
		paths[i] = getAPIPath(objectType, namespace, query)
	}
	return paths, namespaces
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
		return "/apis/networking.k8s.io/v1beta1/" + suffix
	}
	if objectType == "endpointslices" {
		return "/apis/discovery.k8s.io/v1beta1/" + suffix
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
	case "endpointslices":
		return "endpointslices"
	case "ingress":
		return "ingresses"
	default:
		logger.Panicf("BUG: unknonw role=%q", role)
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
	case "endpointslices":
		return parseEndpointSlice, parseEndpointSliceList
	case "ingress":
		return parseIngress, parseIngressList
	default:
		logger.Panicf("BUG: unsupported role=%q", role)
		return nil, nil
	}
}
