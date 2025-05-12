package consulagent

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/metrics"
)

// SDCheckInterval is check interval for Consul Agent service discovery.
var SDCheckInterval = flag.Duration("promscrape.consulagentSDCheckInterval", 30*time.Second, "Interval for checking for changes in Consul Agent. "+
	"This works only if consulagent_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#consulagent_sd_configs for details")

// consulAgentWatcher is a watcher for consul api, updates services map in background with long-polling.
type consulAgentWatcher struct {
	client *discoveryutil.Client

	servicesQueryArgs string

	watchServices   []string
	watchDatacenter string

	// servicesLock protects services
	servicesLock sync.Mutex
	services     map[string]*serviceWatcher

	stoppedCh chan struct{}
}

type serviceWatcher struct {
	serviceName  string
	serviceNodes []consul.ServiceNode

	stoppedCh chan struct{}

	requestCtx    context.Context
	requestCancel context.CancelFunc
}

// newConsulAgentWatcher creates new watcher and starts background service discovery for Consul.
func newConsulAgentWatcher(client *discoveryutil.Client, sdc *SDConfig, datacenter, namespace string) *consulAgentWatcher {
	var qv = url.Values{}
	if namespace != "" {
		qv.Set("ns", namespace)
	}
	if len(sdc.Filter) > 0 {
		qv.Set("filter", sdc.Filter)
	}

	cw := &consulAgentWatcher{
		client:            client,
		servicesQueryArgs: "?" + qv.Encode(),
		watchServices:     sdc.Services,
		watchDatacenter:   datacenter,
		services:          make(map[string]*serviceWatcher),
		stoppedCh:         make(chan struct{}),
	}
	initCh := make(chan struct{})
	go func() {
		cw.watchForServicesUpdates(initCh)
		close(cw.stoppedCh)
	}()
	// wait for initialization to complete
	<-initCh
	return cw
}

func (cw *consulAgentWatcher) mustStop() {
	cw.client.Stop()
	<-cw.stoppedCh
}

func (cw *consulAgentWatcher) updateServices(serviceNames []string) {
	var initWG sync.WaitGroup

	// Start watchers for new services.
	cw.servicesLock.Lock()
	for _, serviceName := range serviceNames {
		if _, ok := cw.services[serviceName]; ok {
			// The watcher for serviceName already exists.
			continue
		}
		ctx, cancel := context.WithCancel(cw.client.Context())
		sw := &serviceWatcher{
			serviceName:   serviceName,
			stoppedCh:     make(chan struct{}),
			requestCtx:    ctx,
			requestCancel: cancel,
		}
		cw.services[serviceName] = sw
		serviceWatchersCreated.Inc()
		initWG.Add(1)
		go func() {
			serviceWatchersCount.Inc()
			sw.watchForServiceNodesUpdates(cw, &initWG)
			serviceWatchersCount.Dec()
			close(sw.stoppedCh)
		}()
	}

	// Stop watchers for removed services.
	newServiceNamesMap := make(map[string]struct{}, len(serviceNames))
	for _, serviceName := range serviceNames {
		newServiceNamesMap[serviceName] = struct{}{}
	}
	var swsStopped []*serviceWatcher
	for serviceName, sw := range cw.services {
		if _, ok := newServiceNamesMap[serviceName]; ok {
			continue
		}
		sw.requestCancel()
		delete(cw.services, serviceName)
		swsStopped = append(swsStopped, sw)
	}
	cw.servicesLock.Unlock()

	// Wait until deleted service watchers are stopped.
	for _, sw := range swsStopped {
		<-sw.stoppedCh
		serviceWatchersStopped.Inc()
	}

	// Wait until added service watchers are initialized.
	initWG.Wait()
}

// watchForServicesUpdates watches for new services and updates it in cw.
//
// watchForServicesUpdates closes the initCh once the initialization is complete and first discovery iteration is done.
func (cw *consulAgentWatcher) watchForServicesUpdates(initCh chan struct{}) {
	apiServer := cw.client.APIServer()
	f := func() {
		serviceNames, err := cw.getServiceNames()
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot obtain Consul Agent services from %q: %s", apiServer, err)
			}
			return
		}
		cw.updateServices(serviceNames)
	}

	logger.Infof("started Consul Agent service watcher for %q", apiServer)
	f()

	// send signal that initialization is complete
	close(initCh)

	ticker := time.NewTicker(getCheckInterval())
	defer ticker.Stop()
	stopCh := cw.client.Context().Done()
	for {
		select {
		case <-ticker.C:
			f()
		case <-stopCh:
			logger.Infof("stopping Consul Agent service watchers for %q", apiServer)
			startTime := time.Now()
			var swsStopped []*serviceWatcher

			cw.servicesLock.Lock()
			for _, sw := range cw.services {
				sw.requestCancel()
				swsStopped = append(swsStopped, sw)
			}
			cw.servicesLock.Unlock()

			for _, sw := range swsStopped {
				<-sw.stoppedCh
				serviceWatchersStopped.Inc()
			}
			logger.Infof("stopped Consul Agent service watcher for %q in %.3f seconds", apiServer, time.Since(startTime).Seconds())
			return
		}
	}
}

var (
	serviceWatchersCreated = metrics.NewCounter("vm_promscrape_discovery_consulagent_service_watchers_created_total")
	serviceWatchersStopped = metrics.NewCounter("vm_promscrape_discovery_consulagent_service_watchers_stopped_total")
	serviceWatchersCount   = metrics.NewCounter("vm_promscrape_discovery_consulagent_service_watchers")
)

// getServiceNames obtains serviceNames via request to Consul Agent.
//
// It returns an empty serviceNames list if response contains the same index.
func (cw *consulAgentWatcher) getServiceNames() ([]string, error) {
	path := "/v1/agent/services" + cw.servicesQueryArgs
	data, err := cw.client.GetAPIResponse(path)
	if err != nil {
		return nil, fmt.Errorf("cannot perform Consul Agent API request at %q: %w", path, err)
	}

	var m map[string]consul.Service
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("cannot parse response from %q: %w; data=%q", path, err, data)
	}
	serviceNames := make([]string, 0, len(m))
	for _, service := range m {
		serviceName := service.Service
		if service.Datacenter != cw.watchDatacenter {
			continue
		}
		if !consul.ShouldCollectServiceByName(cw.watchServices, serviceName) {
			continue
		}
		serviceNames = append(serviceNames, serviceName)
	}
	return serviceNames, nil
}

// watchForServiceNodesUpdates watches for Consul serviceNode changes for the given serviceName.
//
// watchForServiceNodesUpdates calls initWG.Done() once the initialization is complete and the first discovery iteration is done.
func (sw *serviceWatcher) watchForServiceNodesUpdates(cw *consulAgentWatcher, initWG *sync.WaitGroup) {
	apiServer := cw.client.APIServer()
	path := "/v1/agent/health/service/name/" + sw.serviceName
	f := func() {
		data, err := cw.client.GetAPIResponseWithParamsCtx(sw.requestCtx, path, nil, nil)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot obtain Consul Agent serviceNodes for serviceName=%q from %q: %s", sw.serviceName, apiServer, err)
			}
			return
		}
		sns, err := consul.ParseServiceNodes(data)
		if err != nil {
			logger.Errorf("cannot parse Consul Agent serviceNodes response for serviceName=%q from %q: %s", sw.serviceName, apiServer, err)
			return
		}

		cw.servicesLock.Lock()
		sw.serviceNodes = sns
		cw.servicesLock.Unlock()
	}

	f()
	// Notify caller that initialization is complete
	initWG.Done()

	ticker := time.NewTicker(getCheckInterval())
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

// getServiceNodesSnapshot returns a snapshot of discovered ServiceNodes.
func (cw *consulAgentWatcher) getServiceNodesSnapshot() map[string][]consul.ServiceNode {
	cw.servicesLock.Lock()
	sns := make(map[string][]consul.ServiceNode, len(cw.services))
	for svc, sw := range cw.services {
		sns[svc] = sw.serviceNodes
	}
	cw.servicesLock.Unlock()
	return sns
}

func getCheckInterval() time.Duration {
	d := *SDCheckInterval
	if d <= time.Second {
		return time.Second
	}
	return d
}
