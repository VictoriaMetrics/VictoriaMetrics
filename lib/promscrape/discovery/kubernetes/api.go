package kubernetes

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var apiServerTimeout = flag.Duration("promscrape.kubernetes.apiServerTimeout", 10*time.Minute, "How frequently to reload the full state from Kuberntes API server")

// apiConfig contains config for API server
type apiConfig struct {
	aw *apiWatcher
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	ac, err := promauth.NewConfig(baseDir, sdc.BasicAuth, sdc.BearerToken, sdc.BearerTokenFile, sdc.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.APIServer
	if len(apiServer) == 0 {
		// Assume we run at k8s pod.
		// Discover apiServer and auth config according to k8s docs.
		// See https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#service-account-admission-controller
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_HOST env var; it must be defined when running in k8s; " +
				"probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?")
		}
		if len(port) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_PORT env var; it must be defined when running in k8s; "+
				"KUBERNETES_SERVICE_HOST=%q", host)
		}
		apiServer = "https://" + net.JoinHostPort(host, port)
		tlsConfig := promauth.TLSConfig{
			CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		}
		acNew, err := promauth.NewConfig(".", nil, "", "/var/run/secrets/kubernetes.io/serviceaccount/token", &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %w; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
	}
	if !strings.Contains(apiServer, "://") {
		proto := "http"
		if sdc.TLSConfig != nil {
			proto = "https"
		}
		apiServer = proto + "://" + apiServer
	}
	for strings.HasSuffix(apiServer, "/") {
		apiServer = apiServer[:len(apiServer)-1]
	}
	var proxy func(*http.Request) (*url.URL, error)
	if proxyURL := sdc.ProxyURL.URL(); proxyURL != nil {
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
	aw := newAPIWatcher(client, apiServer, ac.Authorization, sdc.Namespaces.Names, sdc.Selectors)
	cfg := &apiConfig{
		aw: aw,
	}
	return cfg, nil
}

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

	// mu protects watchersByURL and lastAccessTime
	mu sync.Mutex

	// a map of watchers keyed by request paths
	watchersByURL map[string]*urlWatcher

	// The last time the apiWatcher was queried for cached objects.
	// It is used for stopping unused watchers.
	lastAccessTime uint64
}

func newAPIWatcher(client *http.Client, apiServer, authorization string, namespaces []string, selectors []Selector) *apiWatcher {
	return &apiWatcher{
		apiServer:     apiServer,
		authorization: authorization,
		client:        client,
		namespaces:    namespaces,
		selectors:     selectors,

		watchersByURL: make(map[string]*urlWatcher),

		lastAccessTime: fasttime.UnixTimestamp(),
	}
}

// getLabelsForRole returns all the sets of labels for the given role.
func (aw *apiWatcher) getLabelsForRole(role string) []map[string]string {
	aw.startWatchersForRole(role)
	var ms []map[string]string
	aw.mu.Lock()
	for _, uw := range aw.watchersByURL {
		if uw.role != role {
			continue
		}
		uw.mu.Lock()
		for _, labels := range uw.labelsByKey {
			ms = append(ms, labels...)
		}
		uw.mu.Unlock()
	}
	aw.lastAccessTime = fasttime.UnixTimestamp()
	aw.mu.Unlock()
	return ms
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
		uw.mu.Lock()
		o = uw.objectsByKey[key]
		uw.mu.Unlock()
		if o != nil {
			break
		}
	}
	aw.lastAccessTime = fasttime.UnixTimestamp()
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

	resourceVersion := uw.reloadObjects()
	go func() {
		uw.watchForUpdates(resourceVersion)
		aw.mu.Lock()
		delete(aw.watchersByURL, apiURL)
		aw.mu.Unlock()
	}()
}

// needStop returns true if aw wasn't used for long time.
func (aw *apiWatcher) needStop() bool {
	aw.mu.Lock()
	defer aw.mu.Unlock()
	return fasttime.UnixTimestamp() > aw.lastAccessTime+5*60
}

// doRequest performs http request to the given requestURL.
func (aw *apiWatcher) doRequest(requestURL string) (*http.Response, error) {
	req, err := http.NewRequest("GET", requestURL, nil)
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

	// mu protects objectsByKey and labelsByKey
	mu sync.Mutex

	// objectsByKey contains the latest state for objects obtained from apiURL
	objectsByKey map[string]object
	labelsByKey  map[string][]map[string]string

	// the parent apiWatcher
	aw *apiWatcher
}

func (aw *apiWatcher) newURLWatcher(role, apiURL string, parseObject parseObjectFunc, parseObjectList parseObjectListFunc) *urlWatcher {
	return &urlWatcher{
		role:   role,
		apiURL: apiURL,

		parseObject:     parseObject,
		parseObjectList: parseObjectList,

		objectsByKey: make(map[string]object),
		labelsByKey:  make(map[string][]map[string]string),

		aw: aw,
	}
}

// reloadObjects reloads objects to the latest state and returns resourceVersion for the latest state.
func (uw *urlWatcher) reloadObjects() string {
	requestURL := uw.apiURL
	resp, err := uw.aw.doRequest(requestURL)
	if err != nil {
		logger.Errorf("error when performing a request to %q: %s", requestURL, err)
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
		logger.Errorf("cannot parse response from %q: %s", requestURL, err)
		return ""
	}
	labelsByKey := make(map[string][]map[string]string, len(objectsByKey))
	for k, o := range objectsByKey {
		labelsByKey[k] = o.getTargetLabels(uw.aw)
	}
	uw.mu.Lock()
	uw.objectsByKey = objectsByKey
	uw.labelsByKey = labelsByKey
	uw.mu.Unlock()
	return metadata.ResourceVersion
}

// watchForUpdates watches for object updates starting from resourceVersion and updates the corresponding objects to the latest state.
//
// See https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes
func (uw *urlWatcher) watchForUpdates(resourceVersion string) {
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
	apiURL += delimiter + "watch=1&timeoutSeconds=" + strconv.Itoa(int(timeoutSeconds))
	logger.Infof("started watcher for %q", apiURL)
	for {
		if aw.needStop() {
			logger.Infof("stopped unused watcher for %q", apiURL)
			return
		}
		requestURL := apiURL
		if resourceVersion != "" {
			requestURL += "&resourceVersion=" + url.QueryEscape(resourceVersion)
		}
		resp, err := aw.doRequest(requestURL)
		if err != nil {
			logger.Errorf("error when performing a request to %q: %s", requestURL, err)
			backoffSleep()
			resourceVersion = uw.reloadObjects()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(resp.Body)
			_ = resp.Body.Close()
			logger.Errorf("unexpected status code for request to %q: %d; want %d; response: %q", requestURL, resp.StatusCode, http.StatusOK, body)
			if resp.StatusCode == 410 {
				// There is no need for sleep on 410 error. See https://kubernetes.io/docs/reference/using-api/api-concepts/#410-gone-responses
				backoffDelay = time.Second
			} else {
				backoffSleep()
			}
			resourceVersion = uw.reloadObjects()
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
			resourceVersion = uw.reloadObjects()
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
		o, err := uw.parseObject(we.Object)
		if err != nil {
			return err
		}
		key := o.key()
		switch we.Type {
		case "ADDED", "MODIFIED":
			uw.mu.Lock()
			uw.objectsByKey[key] = o
			uw.mu.Unlock()
			labels := o.getTargetLabels(uw.aw)
			uw.mu.Lock()
			uw.labelsByKey[key] = labels
			uw.mu.Unlock()
		case "DELETED":
			uw.mu.Lock()
			delete(uw.objectsByKey, key)
			delete(uw.labelsByKey, key)
			uw.mu.Unlock()
		default:
			return fmt.Errorf("unexpected WatchEvent type %q for role %q", we.Type, uw.role)
		}
	}
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
