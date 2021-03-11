package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/VictoriaMetrics/metrics"
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
	getTargetLabels(aw *apiWatcher) []map[string]string
}

// parseObjectFunc must parse object from the given data.
type parseObjectFunc func(data []byte) (object, error)

// parseObjectListFunc must parse objectList from the given data.
type parseObjectListFunc func(data []byte) (map[string]object, ListMeta, error)

// apiWatcher is used for watching for Kuberntes object changes and caching their latest states.
type apiWatcher struct {
	// The client used for watching for object changes
	client *http.Client

	// Kubenetes API server address in the form http://api-server
	apiServer string

	// The contents for `Authorization` HTTP request header
	authorization string

	// Namespaces to watch
	namespaces []string

	// Selectors to apply during watch
	selectors []Selector

	// Constructor for creating ScrapeWork objects from labels.
	swcFunc ScrapeWorkConstructorFunc

	// mu protects watchersByURL
	mu sync.Mutex

	// a map of watchers keyed by request urls
	watchersByURL map[string]*urlWatcher

	stopFunc func()
	stopCtx  context.Context
	wg       sync.WaitGroup
}

func (aw *apiWatcher) mustStop() {
	aw.stopFunc()
	aw.wg.Wait()
}

func newAPIWatcher(client *http.Client, apiServer, authorization string, namespaces []string, selectors []Selector, swcFunc ScrapeWorkConstructorFunc) *apiWatcher {
	stopCtx, stopFunc := context.WithCancel(context.Background())
	return &apiWatcher{
		apiServer:     apiServer,
		authorization: authorization,
		client:        client,
		namespaces:    namespaces,
		selectors:     selectors,
		swcFunc:       swcFunc,

		watchersByURL: make(map[string]*urlWatcher),

		stopFunc: stopFunc,
		stopCtx:  stopCtx,
	}
}

// getScrapeWorkObjectsForRole returns all the ScrapeWork objects for the given role.
func (aw *apiWatcher) getScrapeWorkObjectsForRole(role string) []interface{} {
	aw.startWatchersForRole(role)
	var swos []interface{}
	aw.mu.Lock()
	for _, uw := range aw.watchersByURL {
		if uw.role != role {
			continue
		}
		uw.mu.Lock()
		for _, swosLocal := range uw.swosByKey {
			swos = append(swos, swosLocal...)
		}
		uw.mu.Unlock()
	}
	aw.mu.Unlock()
	return swos
}

// getObjectByRole returns an object with the given (namespace, name) key and the given role.
func (aw *apiWatcher) getObjectByRole(role, namespace, name string) object {
	if aw == nil {
		return nil
	}
	key := namespace + "/" + name
	aw.startWatchersForRole(role)
	var o object
	aw.mu.Lock()
	for _, uw := range aw.watchersByURL {
		if uw.role != role {
			continue
		}
		o = uw.objectsByKey.get(key)
		if o != nil {
			break
		}
	}
	aw.mu.Unlock()
	return o
}

func (aw *apiWatcher) startWatchersForRole(role string) {
	parseObject, parseObjectList := getObjectParsersForRole(role)
	paths := getAPIPaths(role, aw.namespaces, aw.selectors)
	for _, path := range paths {
		apiURL := aw.apiServer + path
		aw.startWatcherForURL(role, apiURL, parseObject, parseObjectList)
	}
}

func (aw *apiWatcher) startWatcherForURL(role, apiURL string, parseObject parseObjectFunc, parseObjectList parseObjectListFunc) {
	aw.mu.Lock()
	if aw.watchersByURL[apiURL] != nil {
		// Watcher for the given path already exists.
		aw.mu.Unlock()
		return
	}
	uw := aw.newURLWatcher(role, apiURL, parseObject, parseObjectList)
	aw.watchersByURL[apiURL] = uw
	aw.mu.Unlock()

	uw.watchersCount.Inc()
	uw.watchersCreated.Inc()
	uw.reloadObjects()
	aw.wg.Add(1)
	go func() {
		defer aw.wg.Done()
		uw.watchForUpdates()
		uw.objectsByKey.decRef()

		aw.mu.Lock()
		delete(aw.watchersByURL, apiURL)
		aw.mu.Unlock()
		uw.watchersCount.Dec()
		uw.watchersStopped.Inc()
	}()
}

// needStop returns true if aw must be stopped.
func (aw *apiWatcher) needStop() bool {
	select {
	case <-aw.stopCtx.Done():
		return true
	default:
		return false
	}
}

// doRequest performs http request to the given requestURL.
func (aw *apiWatcher) doRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(aw.stopCtx, "GET", requestURL, nil)
	if err != nil {
		logger.Fatalf("cannot create a request for %q: %s", requestURL, err)
	}
	if aw.authorization != "" {
		req.Header.Set("Authorization", aw.authorization)
	}
	return aw.client.Do(req)
}

// urlWatcher watches for an apiURL and updates object states in objectsByKey.
type urlWatcher struct {
	role   string
	apiURL string

	parseObject     parseObjectFunc
	parseObjectList parseObjectListFunc

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey *objectsMap

	// mu protects swosByKey and resourceVersion
	mu              sync.Mutex
	swosByKey       map[string][]interface{}
	resourceVersion string

	// the parent apiWatcher
	aw *apiWatcher

	watchersCount   *metrics.Counter
	watchersCreated *metrics.Counter
	watchersStopped *metrics.Counter
}

func (aw *apiWatcher) newURLWatcher(role, apiURL string, parseObject parseObjectFunc, parseObjectList parseObjectListFunc) *urlWatcher {
	return &urlWatcher{
		role:   role,
		apiURL: apiURL,

		parseObject:     parseObject,
		parseObjectList: parseObjectList,

		objectsByKey: sharedObjectsGlobal.getByAPIURL(role, apiURL),
		swosByKey:    make(map[string][]interface{}),

		aw: aw,

		watchersCount:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers{role=%q}`, role)),
		watchersCreated: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers_created_total{role=%q}`, role)),
		watchersStopped: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers_stopped_total{role=%q}`, role)),
	}
}

// Limit the concurrency for per-role objects reloading to 1.
//
// This should reduce memory usage when big number of watchers simultaneously receive an update for objects of the same role.
var reloadObjectsLocksByRole = map[string]*sync.Mutex{
	"node":           {},
	"pod":            {},
	"service":        {},
	"endpoints":      {},
	"endpointslices": {},
	"ingress":        {},
}

func (uw *urlWatcher) setResourceVersion(resourceVersion string) {
	uw.mu.Lock()
	uw.resourceVersion = resourceVersion
	uw.mu.Unlock()
}

// reloadObjects reloads objects to the latest state and returns resourceVersion for the latest state.
func (uw *urlWatcher) reloadObjects() string {
	lock := reloadObjectsLocksByRole[uw.role]
	lock.Lock()
	defer lock.Unlock()

	uw.mu.Lock()
	resourceVersion := uw.resourceVersion
	uw.mu.Unlock()
	if resourceVersion != "" {
		// Fast path - objects have been already reloaded by concurrent goroutines.
		return resourceVersion
	}

	aw := uw.aw
	requestURL := uw.apiURL
	resp, err := aw.doRequest(requestURL)
	if err != nil {
		if !aw.needStop() {
			logger.Errorf("error when performing a request to %q: %s", requestURL, err)
		}
		return ""
	}
	body, _ := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
		return ""
	}
	objectsByKey, metadata, err := uw.parseObjectList(body)
	if err != nil {
		if !aw.needStop() {
			logger.Errorf("cannot parse response from %q: %s", requestURL, err)
		}
		return ""
	}
	logger.Infof("loaded %d objects from %q", len(objectsByKey), requestURL)
	uw.objectsByKey.reload(objectsByKey)
	swosByKey := make(map[string][]interface{})
	for k, o := range objectsByKey {
		labels := o.getTargetLabels(aw)
		swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
		if len(swos) > 0 {
			swosByKey[k] = swos
		}
	}
	uw.mu.Lock()
	uw.swosByKey = swosByKey
	uw.resourceVersion = metadata.ResourceVersion
	uw.mu.Unlock()

	return metadata.ResourceVersion
}

func getScrapeWorkObjectsForLabels(swcFunc ScrapeWorkConstructorFunc, labelss []map[string]string) []interface{} {
	swos := make([]interface{}, 0, len(labelss))
	for _, labels := range labelss {
		swo := swcFunc(labels)
		// The reflect check is needed because of https://mangatmodi.medium.com/go-check-nil-interface-the-right-way-d142776edef1
		if swo != nil && !reflect.ValueOf(swo).IsNil() {
			swos = append(swos, swo)
		}
	}
	return swos
}

// watchForUpdates watches for object updates starting from uw.resourceVersion and updates the corresponding objects to the latest state.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
func (uw *urlWatcher) watchForUpdates() {
	aw := uw.aw
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
	timeoutSeconds := time.Duration(0.9 * float64(aw.client.Timeout)).Seconds()
	apiURL += delimiter + "watch=1&allowWatchBookmarks=true&timeoutSeconds=" + strconv.Itoa(int(timeoutSeconds))
	for {
		if aw.needStop() {
			return
		}
		resourceVersion := uw.reloadObjects()
		requestURL := apiURL
		if resourceVersion != "" {
			requestURL += "&resourceVersion=" + url.QueryEscape(resourceVersion)
		}
		resp, err := aw.doRequest(requestURL)
		if err != nil {
			if aw.needStop() {
				return
			}
			logger.Errorf("error when performing a request to %q: %s", requestURL, err)
			backoffSleep()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
			if resp.StatusCode == 410 {
				// There is no need for sleep on 410 error. See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				backoffDelay = time.Second
				uw.setResourceVersion("")
			} else {
				backoffSleep()
			}
			continue
		}
		backoffDelay = time.Second
		err = uw.readObjectUpdateStream(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			if aw.needStop() {
				return
			}
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
	aw := uw.aw
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
			uw.objectsByKey.update(key, o)
			labels := o.getTargetLabels(aw)
			swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
			uw.mu.Lock()
			if len(swos) > 0 {
				uw.swosByKey[key] = swos
			} else {
				delete(uw.swosByKey, key)
			}
			uw.mu.Unlock()
		case "DELETED":
			o, err := uw.parseObject(we.Object)
			if err != nil {
				return err
			}
			key := o.key()
			uw.objectsByKey.remove(key)
			uw.mu.Lock()
			delete(uw.swosByKey, key)
			uw.mu.Unlock()
		case "BOOKMARK":
			// See https://kubernetes.io/docs/reference/using-api/api-concepts/#watch-bookmarks
			bm, err := parseBookmark(we.Object)
			if err != nil {
				return fmt.Errorf("cannot parse bookmark from %q: %w", we.Object, err)
			}
			uw.setResourceVersion(bm.Metadata.ResourceVersion)
		default:
			return fmt.Errorf("unexpected WatchEvent type %q for role %q", we.Type, uw.role)
		}
	}
}

// Bookmark is a bookmark from Kubernetes Watch API.
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

func getAPIPaths(role string, namespaces []string, selectors []Selector) []string {
	objectName := getObjectNameByRole(role)
	if objectName == "nodes" || len(namespaces) == 0 {
		query := joinSelectors(role, selectors)
		path := getAPIPath(objectName, "", query)
		return []string{path}
	}
	query := joinSelectors(role, selectors)
	paths := make([]string, len(namespaces))
	for i, namespace := range namespaces {
		paths[i] = getAPIPath(objectName, namespace, query)
	}
	return paths
}

func getAPIPath(objectName, namespace, query string) string {
	suffix := objectName
	if namespace != "" {
		suffix = "namespaces/" + namespace + "/" + objectName
	}
	if len(query) > 0 {
		suffix += "?" + query
	}
	if objectName == "ingresses" {
		return "/apis/networking.k8s.io/v1beta1/" + suffix
	}
	if objectName == "endpointslices" {
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

func getObjectNameByRole(role string) string {
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

type objectsMap struct {
	mu       sync.Mutex
	refCount int
	m        map[string]object

	objectsAdded   *metrics.Counter
	objectsRemoved *metrics.Counter
	objectsCount   *metrics.Counter
}

func (om *objectsMap) incRef() {
	om.mu.Lock()
	om.refCount++
	om.mu.Unlock()
}

func (om *objectsMap) decRef() {
	om.mu.Lock()
	om.refCount--
	if om.refCount < 0 {
		logger.Panicf("BUG: refCount cannot be smaller than 0; got %d", om.refCount)
	}
	if om.refCount == 0 {
		// Free up memory occupied by om.m
		om.objectsRemoved.Add(len(om.m))
		om.objectsCount.Add(-len(om.m))
		om.m = make(map[string]object)
	}
	om.mu.Unlock()
}

func (om *objectsMap) reload(m map[string]object) {
	om.mu.Lock()
	om.objectsAdded.Add(len(m))
	om.objectsRemoved.Add(len(om.m))
	om.objectsCount.Add(len(m) - len(om.m))
	for k := range om.m {
		delete(om.m, k)
	}
	for k, o := range m {
		om.m[k] = o
	}
	om.mu.Unlock()
}

func (om *objectsMap) update(key string, o object) {
	om.mu.Lock()
	if om.m[key] == nil {
		om.objectsAdded.Inc()
		om.objectsCount.Inc()
	}
	om.m[key] = o
	om.mu.Unlock()
}

func (om *objectsMap) remove(key string) {
	om.mu.Lock()
	if om.m[key] != nil {
		om.objectsRemoved.Inc()
		om.objectsCount.Dec()
		delete(om.m, key)
	}
	om.mu.Unlock()
}

func (om *objectsMap) get(key string) object {
	om.mu.Lock()
	o, ok := om.m[key]
	om.mu.Unlock()
	if !ok {
		return nil
	}
	return o
}

type sharedObjects struct {
	mu  sync.Mutex
	oms map[string]*objectsMap
}

func (so *sharedObjects) getByAPIURL(role, apiURL string) *objectsMap {
	so.mu.Lock()
	om := so.oms[apiURL]
	if om == nil {
		om = &objectsMap{
			m: make(map[string]object),

			objectsCount:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects{role=%q}`, role)),
			objectsAdded:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_added_total{role=%q}`, role)),
			objectsRemoved: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_removed_total{role=%q}`, role)),
		}
		so.oms[apiURL] = om
	}
	so.mu.Unlock()
	om.incRef()
	return om
}

var sharedObjectsGlobal = &sharedObjects{
	oms: make(map[string]*objectsMap),
}
