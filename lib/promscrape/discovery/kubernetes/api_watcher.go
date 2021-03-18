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

	// swos contains a map of ScrapeWork objects for the given apiWatcher
	swosByKey     map[string][]interface{}
	swosByKeyLock sync.Mutex

	swosCount *metrics.Counter
}

func newAPIWatcher(apiServer string, ac *promauth.Config, sdc *SDConfig, swcFunc ScrapeWorkConstructorFunc) *apiWatcher {
	namespaces := sdc.Namespaces.Names
	selectors := sdc.Selectors
	proxyURL := sdc.ProxyURL.URL()
	gw := getGroupWatcher(apiServer, ac, namespaces, selectors, proxyURL)
	return &apiWatcher{
		role:      sdc.Role,
		swcFunc:   swcFunc,
		gw:        gw,
		swosByKey: make(map[string][]interface{}),
		swosCount: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_scrape_works{role=%q}`, sdc.Role)),
	}
}

func (aw *apiWatcher) mustStop() {
	aw.gw.unsubscribeAPIWatcher(aw)
	aw.reloadScrapeWorks(make(map[string][]interface{}))
}

func (aw *apiWatcher) reloadScrapeWorks(swosByKey map[string][]interface{}) {
	aw.swosByKeyLock.Lock()
	aw.swosCount.Add(len(swosByKey) - len(aw.swosByKey))
	aw.swosByKey = swosByKey
	aw.swosByKeyLock.Unlock()
}

func (aw *apiWatcher) setScrapeWorks(key string, labels []map[string]string) {
	swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
	aw.swosByKeyLock.Lock()
	if len(swos) > 0 {
		aw.swosCount.Add(len(swos) - len(aw.swosByKey[key]))
		aw.swosByKey[key] = swos
	} else {
		aw.swosCount.Add(-len(aw.swosByKey[key]))
		delete(aw.swosByKey, key)
	}
	aw.swosByKeyLock.Unlock()
}

func (aw *apiWatcher) removeScrapeWorks(key string) {
	aw.swosByKeyLock.Lock()
	aw.swosCount.Add(-len(aw.swosByKey[key]))
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
	aw.gw.startWatchersForRole(aw.role, aw)
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
	key := fmt.Sprintf("apiServer=%s, namespaces=%s, selectors=%s, proxyURL=%v, authConfig=%s",
		apiServer, namespaces, selectorsKey(selectors), proxyURL, ac.String())
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
	key := namespace + "/" + name
	gw.startWatchersForRole(role, nil)
	gw.mu.Lock()
	defer gw.mu.Unlock()

	for _, uw := range gw.m {
		if uw.role != role {
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
	paths := getAPIPaths(role, gw.namespaces, gw.selectors)
	for _, path := range paths {
		apiURL := gw.apiServer + path
		gw.mu.Lock()
		uw := gw.m[apiURL]
		if uw == nil {
			uw = newURLWatcher(role, apiURL, gw)
			gw.m[apiURL] = uw
		}
		gw.mu.Unlock()
		uw.subscribeAPIWatcher(aw)
	}
}

func (gw *groupWatcher) reloadScrapeWorksForAPIWatchers(aws []*apiWatcher, objectsByKey map[string]object) {
	if len(aws) == 0 {
		return
	}
	swosByKey := make([]map[string][]interface{}, len(aws))
	for i := range aws {
		swosByKey[i] = make(map[string][]interface{})
	}
	for key, o := range objectsByKey {
		labels := o.getTargetLabels(gw)
		for i, aw := range aws {
			swos := getScrapeWorkObjectsForLabels(aw.swcFunc, labels)
			if len(swos) > 0 {
				swosByKey[i][key] = swos
			}
		}
	}
	for i, aw := range aws {
		aw.reloadScrapeWorks(swosByKey[i])
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

func (gw *groupWatcher) unsubscribeAPIWatcher(aw *apiWatcher) {
	gw.mu.Lock()
	for _, uw := range gw.m {
		uw.unsubscribeAPIWatcher(aw)
	}
	gw.mu.Unlock()
}

// urlWatcher watches for an apiURL and updates object states in objectsByKey.
type urlWatcher struct {
	role   string
	apiURL string
	gw     *groupWatcher

	parseObject     parseObjectFunc
	parseObjectList parseObjectListFunc

	// mu protects aws, awsPending, objectsByKey and resourceVersion
	mu sync.Mutex

	// aws contains registered apiWatcher objects
	aws map[*apiWatcher]struct{}

	// awsPending contains pending apiWatcher objects, which must be moved to aws in a batch
	awsPending map[*apiWatcher]struct{}

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey map[string]object

	resourceVersion string

	objectsCount   *metrics.Counter
	objectsAdded   *metrics.Counter
	objectsRemoved *metrics.Counter
	objectsUpdated *metrics.Counter
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

		aws:          make(map[*apiWatcher]struct{}),
		awsPending:   make(map[*apiWatcher]struct{}),
		objectsByKey: make(map[string]object),

		objectsCount:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects{role=%q}`, role)),
		objectsAdded:   metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_added_total{role=%q}`, role)),
		objectsRemoved: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_removed_total{role=%q}`, role)),
		objectsUpdated: metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_objects_updated_total{role=%q}`, role)),
	}
	logger.Infof("started %s watcher for %q", uw.role, uw.apiURL)
	go uw.watchForUpdates()
	go uw.processPendingSubscribers()
	return uw
}

func (uw *urlWatcher) subscribeAPIWatcher(aw *apiWatcher) {
	if aw == nil {
		return
	}
	uw.mu.Lock()
	if _, ok := uw.aws[aw]; !ok {
		if _, ok := uw.awsPending[aw]; !ok {
			uw.awsPending[aw] = struct{}{}
			metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscibers{role=%q,type="pending"}`, uw.role)).Inc()
		}
	}
	uw.mu.Unlock()
}

func (uw *urlWatcher) unsubscribeAPIWatcher(aw *apiWatcher) {
	uw.mu.Lock()
	if _, ok := uw.aws[aw]; ok {
		delete(uw.aws, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscibers{role=%q,type="permanent"}`, uw.role)).Dec()
	} else if _, ok := uw.awsPending[aw]; ok {
		delete(uw.awsPending, aw)
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscibers{role=%q,type="pending"}`, uw.role)).Dec()
	}
	uw.mu.Unlock()
}

func (uw *urlWatcher) processPendingSubscribers() {
	t := time.NewTicker(time.Second)
	for range t.C {
		var awsPending []*apiWatcher
		var objectsByKey map[string]object

		uw.mu.Lock()
		if len(uw.awsPending) > 0 {
			awsPending = getAPIWatchers(uw.awsPending)
			for _, aw := range awsPending {
				if _, ok := uw.aws[aw]; ok {
					logger.Panicf("BUG: aw=%p already exists in uw.aws", aw)
				}
				uw.aws[aw] = struct{}{}
				delete(uw.awsPending, aw)
			}
			objectsByKey = make(map[string]object, len(uw.objectsByKey))
			for key, o := range uw.objectsByKey {
				objectsByKey[key] = o
			}
		}
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscibers{role=%q,type="pending"}`, uw.role)).Add(-len(awsPending))
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_discovery_kubernetes_subscibers{role=%q,type="permanent"}`, uw.role)).Add(len(awsPending))
		uw.mu.Unlock()

		uw.gw.reloadScrapeWorksForAPIWatchers(awsPending, objectsByKey)
	}
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

	uw.gw.reloadScrapeWorksForAPIWatchers(aws, objectsByKey)
	logger.Infof("reloaded %d objects from %q", len(objectsByKey), requestURL)
	return metadata.ResourceVersion
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
			aws := getAPIWatchers(uw.aws)
			uw.mu.Unlock()
			labels := o.getTargetLabels(uw.gw)
			for _, aw := range aws {
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
			aws := getAPIWatchers(uw.aws)
			uw.mu.Unlock()
			for _, aw := range aws {
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
