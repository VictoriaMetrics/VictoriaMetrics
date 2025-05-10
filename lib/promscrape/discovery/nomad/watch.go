package nomad

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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/metrics"
)

// SDCheckInterval is check interval for Nomad service discovery.
var SDCheckInterval = flag.Duration("promscrape.nomadSDCheckInterval", 30*time.Second, "Interval for checking for changes in Nomad. "+
	"This works only if nomad_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#nomad_sd_configs for details")

// nomadWatcher is a watcher for nomad api, updates services map in background with long-polling.
type nomadWatcher struct {
	client *discoveryutil.Client

	serviceNamesQueryArgs string

	// servicesLock protects services
	servicesLock sync.Mutex
	services     map[string]*serviceWatcher

	stoppedCh chan struct{}
}

type serviceWatcher struct {
	serviceName string
	services    []Service

	stoppedCh chan struct{}

	requestCtx    context.Context
	requestCancel context.CancelFunc
}

// newNomadWatcher creates new watcher and starts background service discovery for Nomad.
func newNomadWatcher(client *discoveryutil.Client, sdc *SDConfig, namespace, region string) *nomadWatcher {
	qa := url.Values{}
	if sdc.AllowStale == nil || *sdc.AllowStale {
		qa.Set("stale", "")
	}
	if namespace != "" {
		qa.Set("namespace", namespace)
	}
	if region != "" {
		qa.Set("region", region)
	}
	queryArgs := qa.Encode()
	if queryArgs != "" {
		queryArgs = "?" + queryArgs
	}

	cw := &nomadWatcher{
		client:                client,
		serviceNamesQueryArgs: queryArgs,
		services:              make(map[string]*serviceWatcher),
		stoppedCh:             make(chan struct{}),
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

func (cw *nomadWatcher) mustStop() {
	cw.client.Stop()
	<-cw.stoppedCh
}

func (cw *nomadWatcher) updateServices(serviceNames []string) {
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
			sw.watchForServiceAddressUpdates(cw, &initWG)
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
func (cw *nomadWatcher) watchForServicesUpdates(initCh chan struct{}) {
	index := int64(0)
	apiServer := cw.client.APIServer()
	f := func() {
		serviceNames, newIndex, err := cw.getBlockingServiceNames(index)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot obtain Nomad serviceNames from %q: %s", apiServer, err)
			}
			return
		}
		if index == newIndex {
			// Nothing changed.
			return
		}
		cw.updateServices(serviceNames)
		index = newIndex
	}

	logger.Infof("started Nomad service watcher for %q", apiServer)
	f()

	// send signal that initialization is complete
	close(initCh)

	checkInterval := getCheckInterval()
	ticker := time.NewTicker(checkInterval / 2)
	defer ticker.Stop()
	stopCh := cw.client.Context().Done()
	for {
		select {
		case <-ticker.C:
			f()
		case <-stopCh:
			logger.Infof("stopping Nomad service watchers for %q", apiServer)
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
			logger.Infof("stopped Nomad service watcher for %q in %.3f seconds", apiServer, time.Since(startTime).Seconds())
			return
		}
	}
}

var (
	serviceWatchersCreated = metrics.NewCounter("vm_promscrape_discovery_nomad_service_watchers_created_total")
	serviceWatchersStopped = metrics.NewCounter("vm_promscrape_discovery_nomad_service_watchers_stopped_total")
	serviceWatchersCount   = metrics.NewCounter("vm_promscrape_discovery_nomad_service_watchers")
)

// getBlockingServiceNames obtains service names via blocking request to Nomad.
//
// It returns an empty serviceNames list if response contains the same index.
func (cw *nomadWatcher) getBlockingServiceNames(index int64) ([]string, int64, error) {
	path := "/v1/services" + cw.serviceNamesQueryArgs
	data, newIndex, err := getBlockingAPIResponse(cw.client.Context(), cw.client, path, index)
	if err != nil {
		return nil, index, err
	}
	if index == newIndex {
		// Nothing changed - return an empty serviceNames list.
		return nil, index, nil
	}

	var svcs []ServiceList
	if err := json.Unmarshal(data, &svcs); err != nil {
		return nil, index, fmt.Errorf("cannot parse response from %q: %w; data=%q", path, err, data)
	}

	serviceNames := make([]string, 0, len(svcs))
	for _, svc := range svcs {
		for _, s := range svc.Services {
			serviceNames = append(serviceNames, s.ServiceName)
		}
	}

	return serviceNames, newIndex, nil
}

// getServiceSnapshot returns a snapshot of discovered Services.
func (cw *nomadWatcher) getServiceSnapshot() map[string][]Service {
	cw.servicesLock.Lock()
	sns := make(map[string][]Service, len(cw.services))
	for svc, sw := range cw.services {
		sns[svc] = sw.services
	}
	cw.servicesLock.Unlock()
	return sns
}

// watchForServiceAddressUpdates watches for Nomad serviceNode changes for the given serviceName.
//
// watchForServiceNodesUpdates calls initWG.Done() once the initialization is complete and the first discovery iteration is done.
func (sw *serviceWatcher) watchForServiceAddressUpdates(nw *nomadWatcher, initWG *sync.WaitGroup) {
	apiServer := nw.client.APIServer()
	index := int64(0)
	// TODO: Maybe use a different query arg.
	path := "/v1/service/" + sw.serviceName + nw.serviceNamesQueryArgs
	f := func() {
		data, newIndex, err := getBlockingAPIResponse(sw.requestCtx, nw.client, path, index)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logger.Errorf("cannot obtain Nomad services for serviceName=%q from %q: %s", sw.serviceName, apiServer, err)
			}
			return
		}
		if index == newIndex {
			// Nothing changed.
			return
		}
		sns, err := parseServices(data)
		if err != nil {
			logger.Errorf("cannot parse Nomad services response for serviceName=%q from %q: %s", sw.serviceName, apiServer, err)
			return
		}

		nw.servicesLock.Lock()
		sw.services = sns
		nw.servicesLock.Unlock()

		index = newIndex
	}

	f()
	// Notify caller that initialization is complete
	initWG.Done()

	checkInterval := getCheckInterval()
	ticker := time.NewTicker(checkInterval / 2)
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

func getCheckInterval() time.Duration {
	d := *SDCheckInterval
	if d <= time.Second {
		return time.Second
	}
	return d
}
