package nacos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	serviceWatchersCreated = metrics.NewCounter("vm_promscrape_discovery_nacos_service_watchers_created_total")
	serviceWatchersStopped = metrics.NewCounter("vm_promscrape_discovery_nacos_service_watchers_stopped_total")
	serviceWatchersCount   = metrics.NewCounter("vm_promscrape_discovery_nacos_service_watchers")
)

type serviceWatcher struct {
	serviceName  string
	serviceNodes []Instance

	stoppedCh chan struct{}

	requestCtx    context.Context
	requestCancel context.CancelFunc
}

// nacosWatcher get instance by service from nacos server.
type nacosWatcher struct {
	client *discoveryutil.Client

	watchServices []string

	namespace string
	cluster   string
	group     string

	checkInterval time.Duration

	// authTokenLock protects authToken
	authTokenLock sync.RWMutex
	authToken     *authToken

	// servicesLock protects services
	servicesLock sync.Mutex
	services     map[string]*serviceWatcher
	stoppedCh    chan struct{}
	tokenReq     []byte
}

func newNacosWatcher(client *discoveryutil.Client, sdc *SDConfig) *nacosWatcher {
	initCh := make(chan struct{})
	nw := &nacosWatcher{
		client:        client,
		watchServices: sdc.Services,
		stoppedCh:     make(chan struct{}),
		checkInterval: sdc.RefreshInterval,
		services:      make(map[string]*serviceWatcher),
		namespace:     sdc.Namespace,
		cluster:       sdc.Cluster,
		group:         sdc.Group,
	}
	if sdc.Username != "" && sdc.Password != nil {
		nw.tokenReq = []byte(fmt.Sprintf("username=%s&password=%s", sdc.Username, sdc.Password))
	}
	go func() {
		nw.watchForServicesUpdates(initCh)
		close(nw.stoppedCh)
	}()
	<-initCh
	return nw
}

func (nw *nacosWatcher) updateServices(serviceNames []string) {
	var initWG sync.WaitGroup
	nw.servicesLock.Lock()
	for _, serviceName := range serviceNames {
		if _, ok := nw.services[serviceName]; ok {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		sw := &serviceWatcher{
			serviceName:   serviceName,
			stoppedCh:     make(chan struct{}),
			requestCtx:    ctx,
			requestCancel: cancel,
		}
		nw.services[serviceName] = sw
		serviceWatchersCreated.Inc()
		initWG.Add(1)
		go func() {
			serviceWatchersCount.Inc()
			sw.watchForServicesInstanceUpdates(nw, &initWG)
		}()
	}

	newServiceNamesMap := make(map[string]struct{}, len(serviceNames))
	for _, serviceName := range serviceNames {
		newServiceNamesMap[serviceName] = struct{}{}
	}

	var swsStopped []*serviceWatcher
	for serviceName, sw := range nw.services {
		if _, ok := newServiceNamesMap[serviceName]; ok {
			continue
		}
		sw.requestCancel()
		delete(nw.services, serviceName)
		swsStopped = append(swsStopped, sw)
	}
	nw.servicesLock.Unlock()

	// Wait until deleted service watchers are stopped.
	for _, sw := range swsStopped {
		<-sw.stoppedCh
		serviceWatchersStopped.Inc()
	}

	// Wait until added service watchers are initialized.
	initWG.Wait()
}

func (nw *nacosWatcher) watchForServicesUpdates(initCh chan struct{}) {
	apiServer := nw.client.APIServer()
	logger.Infof("started Nacos service watcher for %q", apiServer)
	// send signal that initialization is complete
	close(initCh)
	nw.updateServices(nw.watchServices)
	ticker := time.NewTicker(nw.checkInterval / 2)
	stopCh := nw.client.Context().Done()
	for {
		select {
		case <-ticker.C:
			nw.updateServices(nw.watchServices)
		case <-stopCh:
			logger.Infof("stopping Nacos service watchers for %q", apiServer)
			startTime := time.Now()
			var swsStopped []*serviceWatcher

			nw.servicesLock.Lock()
			for _, sw := range nw.services {
				sw.requestCancel()
				swsStopped = append(swsStopped, sw)
			}
			nw.servicesLock.Unlock()

			for _, sw := range swsStopped {
				<-sw.stoppedCh
				serviceWatchersStopped.Inc()
			}
			logger.Infof("stopped nacos service watcher for %q in %.3f seconds", apiServer, time.Since(startTime).Seconds())
			return
		}
	}
}

func (sw *serviceWatcher) watchForServicesInstanceUpdates(nw *nacosWatcher, initWG *sync.WaitGroup) {
	path := "/nacos/v3/client/ns/instance/list"
	path = fmt.Sprintf("%s?namespaceId=%s&groupName=%s&clusterName=%s&serviceName=%s", path, nw.namespace, nw.group, nw.cluster, sw.serviceName)
	apiServer := nw.client.APIServer()
	f := func() {
		data, err := nw.getBlockingAPIResponse(sw.requestCtx, nw.client, path)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot obtain nacos service instances for serviceName=%q from %q: %s", sw.serviceName, nw.client.APIServer(), err)
			}
			return
		}

		sis, err := GetInstance(data)
		if err != nil {
			logger.Errorf("cannot parse nacos serviceNodes response for serviceName=%q from %q: %s", sw.serviceName, apiServer, err)
			return
		}

		nw.servicesLock.Lock()
		sw.serviceNodes = sis
		nw.servicesLock.Unlock()
	}
	f()
	initWG.Done()

	ticker := time.NewTicker(nw.checkInterval / 2)
	defer ticker.Stop()
	stopCh := sw.requestCtx.Done()
	for {
		select {
		case <-ticker.C:
			f()
		case <-stopCh:
			return
		}
	}
}

func (nw *nacosWatcher) getToken(ctx context.Context, client *discoveryutil.Client) (*authToken, error) {
	nw.authTokenLock.Lock()
	defer nw.authTokenLock.Unlock()

	if nw.authToken != nil && time.Until(nw.authToken.ExpireAt) > 10*time.Second {
		return nw.authToken, nil
	}

	data, err := client.GetAPIResponseWithReqParams("/nacos/v3/auth/user/login", func(req *http.Request) {
		req.Method = http.MethodPost
		req.Header["Content-Type"] = []string{"application/x-www-form-urlencoded"}

		req.Body = io.NopCloser(bytes.NewReader(nw.tokenReq))
	})
	if err != nil {
		return nil, fmt.Errorf("cannot obtain nacos token from %s: %w", nw.client.APIServer(), err)
	}
	authResponse, err := parseAuthResponse(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse nacos token from %s: %w", nw.client.APIServer(), err)
	}
	nw.authToken = &authToken{AccessToken: authResponse.AccessToken, ExpireAt: time.Now().Add(time.Duration(authResponse.TokenTtl) * time.Second)}
	return nw.authToken, nil
}

func (nw *nacosWatcher) getBlockingAPIResponse(ctx context.Context, client *discoveryutil.Client, path string) ([]byte, error) {
	token, err := nw.getToken(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain nacos token from %s: %w", nw.client.APIServer(), err)
	}
	path = fmt.Sprintf("%s&accessToken=%s", path, token.AccessToken)
	data, err := client.GetAPIResponse(path)

	if err != nil {
		return nil, fmt.Errorf("cannot perform blocking Nacos API request at %q: %w", path, err)
	}
	return data, nil
}

func (nw *nacosWatcher) getServiceInstanceSnapshot() map[string][]Instance {
	nw.servicesLock.Lock()
	sis := make(map[string][]Instance, len(nw.services))
	for svc, sw := range nw.services {
		sis[svc] = sw.serviceNodes
	}
	nw.servicesLock.Unlock()
	return sis
}
