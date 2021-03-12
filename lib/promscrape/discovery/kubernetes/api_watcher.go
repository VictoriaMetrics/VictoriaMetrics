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
	getTargetLabels(aw *apiWatcher) []map[string]string
}

// parseObjectFunc must parse object from the given data.
type parseObjectFunc func(data []byte) (object, error)

// parseObjectListFunc must parse objectList from the given r.
type parseObjectListFunc func(r io.Reader) (map[string]object, ListMeta, error)

// apiWatcher is used for watching for Kuberntes object changes and caching their latest states.
type apiWatcher struct {
	// Kubenetes API server address in the form http://api-server
	apiServer string

	// ac contains auth config for communicating with apiServer
	ac *promauth.Config

	// sdc contains the related SDConfig
	sdc *SDConfig

	// Constructor for creating ScrapeWork objects from labels
	swcFunc ScrapeWorkConstructorFunc

	// swos contains a map of ScrapeWork objects for the given apiWatcher
	swosByKey     map[string][]interface{}
	swosByKeyLock sync.Mutex

	// a map of watchers keyed by role and request urls
	watchersURLByRole map[string]map[string]*urlWatcher
	//	watchersURLByRole sync.Mutex by role
	watcherURLByRoleLocks map[string]*sync.Mutex

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func (aw *apiWatcher) lockUWByRole(role string) {
	aw.watcherURLByRoleLocks[role].Lock()
}

func (aw *apiWatcher) unLockUWByRole(role string) {
	aw.watcherURLByRoleLocks[role].Unlock()
}

func newAPIWatcher(apiServer string, ac *promauth.Config, sdc *SDConfig, swcFunc ScrapeWorkConstructorFunc) *apiWatcher {
	return &apiWatcher{
		apiServer: apiServer,
		ac:        ac,
		sdc:       sdc,
		swcFunc:   swcFunc,

		swosByKey: make(map[string][]interface{}),
		watchersURLByRole: map[string]map[string]*urlWatcher{
			"pod":            {},
			"service":        {},
			"endpoints":      {},
			"ingress":        {},
			"endpointslices": {},
			"node":           {},
		},
		watcherURLByRoleLocks: map[string]*sync.Mutex{
			"pod":            {},
			"service":        {},
			"endpoints":      {},
			"ingress":        {},
			"endpointslices": {},
			"node":           {},
		},

		stopCh: make(chan struct{}),
	}
}

func (aw *apiWatcher) mustStop() {
	close(aw.stopCh)
	aw.wg.Wait()
}

func (aw *apiWatcher) reloadScrapeWorks(objectsByKey map[string]object) {
	swosByKey := make(map[string][]interface{})
	for key, o := range objectsByKey {
		labels := o.getTargetLabels(aw)
		swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
		if len(swos) > 0 {
			swosByKey[key] = swos
		}
	}
	aw.swosByKeyLock.Lock()
	aw.swosByKey = swosByKey
	aw.swosByKeyLock.Unlock()
}

func (aw *apiWatcher) setScrapeWorks(key string, labels []map[string]string) {
	swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
	aw.swosByKeyLock.Lock()
	if len(swos) > 0 {
		aw.swosByKey[key] = swos
	} else {
		delete(aw.swosByKey, key)
	}
	aw.swosByKeyLock.Unlock()
}

func (aw *apiWatcher) removeScrapeWorks(key string) {
	aw.swosByKeyLock.Lock()
	delete(aw.swosByKey, key)
	aw.swosByKeyLock.Unlock()
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

// getScrapeWorkObjects returns all the ScrapeWork objects for the given aw.
func (aw *apiWatcher) getScrapeWorkObjects() []interface{} {
	aw.startWatchersForRole(aw.sdc.Role)
	aw.swosByKeyLock.Lock()
	defer aw.swosByKeyLock.Unlock()

	size := 0
	for _, swosLocal := range aw.swosByKey {
		size += len(swosLocal)
	}
	swos := make([]interface{}, 0, size)
	for _, swosLocal := range aw.swosByKey {
		swos = append(swos, swosLocal...)
	}
	return swos
}

// getObjectByRole returns an object with the given (namespace, name) key and the given role.
func (aw *apiWatcher) getObjectByRole(role, namespace, name string) object {
	if aw == nil {
		// this is needed for testing
		return nil
	}
	key := namespace + "/" + name
	aw.startWatchersForRole(role)
	aw.lockUWByRole(role)
	defer aw.unLockUWByRole(role)

	for _, uw := range aw.watchersURLByRole[role] {
		uw.mu.Lock()
		o := uw.objectsByKey[key]
		uw.mu.Unlock()
		if o != nil {
			return o
		}
	}
	return nil
}

func (aw *apiWatcher) startWatchersForRole(role string) {
	paths := getAPIPaths(role, aw.sdc.Namespaces.Names, aw.sdc.Selectors)
	for _, path := range paths {
		apiURL := aw.apiServer + path
		aw.startWatcherForURL(role, apiURL)
	}
}

func (aw *apiWatcher) startWatcherForURL(role, apiURL string) {
	aw.lockUWByRole(role)
	if aw.watchersURLByRole[role][apiURL] != nil {
		aw.unLockUWByRole(role)
		return
	}

	uw := getURLWatcher(role, apiURL, aw.sdc.ProxyURL.URL(), aw.ac)
	uw.addAPIWatcher(aw)
	aw.watchersURLByRole[role][apiURL] = uw
	aw.unLockUWByRole(role)

	aw.wg.Add(1)
	go func() {
		defer aw.wg.Done()
		<-aw.stopCh
		aw.lockUWByRole(role)
		uw.removeAPIWatcher(aw)
		delete(aw.watchersURLByRole[role], apiURL)
		aw.unLockUWByRole(role)
	}()
}

func getURLWatcher(role, apiURL string, proxyURL *url.URL, ac *promauth.Config) *urlWatcher {
	key := fmt.Sprintf("url=%s, proxyURL=%v, authConfig=%s", apiURL, proxyURL, ac.String())
	urlWatchersLock.Lock()
	uw := urlWatchers[key]
	if uw == nil {
		uw = newURLWatcher(role, apiURL, proxyURL, ac)
		urlWatchers[key] = uw
	}
	urlWatchersLock.Unlock()
	return uw
}

var (
	urlWatchersLock sync.Mutex
	urlWatchers     = make(map[string]*urlWatcher)
)

// urlWatcher watches for an apiURL and updates object states in objectsByKey.
type urlWatcher struct {
	role          string
	apiURL        string
	authorization string
	client        *http.Client

	parseObject     parseObjectFunc
	parseObjectList parseObjectListFunc

	// mu protects aws, objectsByKey and resourceVersion
	mu sync.Mutex

	// aws contains registered apiWatcher objects
	aws map[*apiWatcher]struct{}

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey map[string]object

	resourceVersion string

	objectsCount   *metrics.Counter
	objectsAdded   *metrics.Counter
	objectsRemoved *metrics.Counter
	objectsUpdated *metrics.Counter
}

func newURLWatcher(role, apiURL string, proxyURL *url.URL, ac *promauth.Config) *urlWatcher {
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
	parseObject, parseObjectList := getObjectParsersForRole(role)
	metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_url_watchers{role=%q}`, role)).Inc()
	uw := &urlWatcher{
		role:          role,
		apiURL:        apiURL,
		authorization: ac.Authorization,
		client:        client,

		parseObject:     parseObject,
		parseObjectList: parseObjectList,

		aws:          make(map[*apiWatcher]struct{}),
		objectsByKey: make(map[string]object),

		objectsCount:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects{role=%q}`, role)),
		objectsAdded:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_added_total{role=%q}`, role)),
		objectsRemoved: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_removed_total{role=%q}`, role)),
		objectsUpdated: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_updated_total{role=%q}`, role)),
	}
	uw.reloadObjects()
	go uw.watchForUpdates()
	return uw
}

func (uw *urlWatcher) addAPIWatcher(aw *apiWatcher) {
	uw.mu.Lock()
	if _, ok := uw.aws[aw]; ok {
		logger.Panicf("BUG: aw=%p has been already added", aw)
	}
	uw.aws[aw] = struct{}{}
	aw.reloadScrapeWorks(uw.objectsByKey)
	uw.mu.Unlock()

}

func (uw *urlWatcher) removeAPIWatcher(aw *apiWatcher) {
	uw.mu.Lock()
	if _, ok := uw.aws[aw]; !ok {
		logger.Panicf("BUG: aw=%p is missing", aw)
	}
	delete(uw.aws, aw)
	uw.mu.Unlock()
}

// doRequest performs http request to the given requestURL.
func (uw *urlWatcher) doRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		logger.Fatalf("cannot create a request for %q: %s", requestURL, err)
	}
	if uw.authorization != "" {
		req.Header.Set("Authorization", uw.authorization)
	}
	return uw.client.Do(req)
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
	resp, err := uw.doRequest(requestURL)
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
	uw.mu.Unlock()

	for _, aw := range uw.getAPIWatchers() {
		aw.reloadScrapeWorks(objectsByKey)
	}
	logger.Infof("loaded %d objects from %q", len(objectsByKey), requestURL)
	return metadata.ResourceVersion
}

func (uw *urlWatcher) getAPIWatchers() []*apiWatcher {
	uw.mu.Lock()
	aws := make([]*apiWatcher, 0, len(uw.aws))
	for aw := range uw.aws {
		aws = append(aws, aw)
	}
	uw.mu.Unlock()
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
	timeoutSeconds := time.Duration(0.9 * float64(uw.client.Timeout)).Seconds()
	apiURL += delimiter + "watch=1&allowWatchBookmarks=true&timeoutSeconds=" + strconv.Itoa(int(timeoutSeconds))
	for {
		resourceVersion := uw.reloadObjects()
		requestURL := apiURL
		if resourceVersion != "" {
			requestURL += "&resourceVersion=" + url.QueryEscape(resourceVersion)
		}
		resp, err := uw.doRequest(requestURL)
		if err != nil {
			logger.Errorf("cannot performing request to %q: %s", requestURL, err)
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
			uw.mu.Unlock()
			for _, aw := range uw.getAPIWatchers() {
				labels := o.getTargetLabels(aw)
				aw.setScrapeWorks(key, labels)
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
			uw.mu.Unlock()
			for _, aw := range uw.getAPIWatchers() {
				aw.removeScrapeWorks(key)
			}
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
