package nomad

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/metrics"
)

// SDCheckInterval is check interval for Nomad service discovery.
var SDCheckInterval = flag.Duration("promscrape.nomadSDCheckInterval", 30*time.Second, "Interval for checking for changes in Nomad. "+
	"This works only if nomad_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/sd_configs.html#nomad_sd_configs for details")

// nomadWatcher is a watcher for nomad api, updates services map in background with long-polling.
type nomadWatcher struct {
	client *discoveryutils.Client

	serviceNamesQueryArgs string
	watchServices         []string
	watchTags             []string

	// servicesLock protects services
	servicesLock sync.Mutex
	services     map[string]*serviceWatcher

	wg     sync.WaitGroup
	stopCh chan struct{}
}

type serviceWatcher struct {
	serviceName string
	services    []Service
	stopCh      chan struct{}
}

// newNomadWatcher creates new watcher and starts background service discovery for Nomad.
func newNomadWatcher(client *discoveryutils.Client, sdc *SDConfig, datacenter, namespace string) *nomadWatcher {
	baseQueryArgs := "?dc=" + url.QueryEscape(datacenter)
	if sdc.AllowStale == nil || *sdc.AllowStale {
		baseQueryArgs += "&stale"
	}
	if namespace != "" {
		baseQueryArgs += "&namespace=" + url.QueryEscape(namespace)
	}
	serviceNodesQueryArgs := baseQueryArgs

	cw := &nomadWatcher{
		client:                client,
		serviceNamesQueryArgs: serviceNodesQueryArgs,
		watchServices:         sdc.Services,
		watchTags:             sdc.Tags,
		services:              make(map[string]*serviceWatcher),
		stopCh:                make(chan struct{}),
	}
	initCh := make(chan struct{})
	go cw.watchForServicesUpdates(initCh)
	// wait for initialization to complete
	<-initCh
	return cw
}

func (cw *nomadWatcher) mustStop() {
	close(cw.stopCh)
	// Do not wait for the watcher to stop, since it may take
	// up to discoveryutils.BlockingClientReadTimeout to complete.
	// TODO: add ability to cancel blocking requests.
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
		sw := &serviceWatcher{
			serviceName: serviceName,
			stopCh:      make(chan struct{}),
		}
		cw.services[serviceName] = sw
		cw.wg.Add(1)
		serviceWatchersCreated.Inc()
		initWG.Add(1)
		go func() {
			serviceWatchersCount.Inc()
			sw.watchForServiceAddressUpdates(cw, &initWG)
			serviceWatchersCount.Dec()
			cw.wg.Done()
		}()
	}

	// Stop watchers for removed services.
	newServiceNamesMap := make(map[string]struct{}, len(serviceNames))
	for _, serviceName := range serviceNames {
		newServiceNamesMap[serviceName] = struct{}{}
	}
	for serviceName, sw := range cw.services {
		if _, ok := newServiceNamesMap[serviceName]; ok {
			continue
		}
		close(sw.stopCh)
		delete(cw.services, serviceName)
		serviceWatchersStopped.Inc()

		// Do not wait for the watcher goroutine to exit, since this may take for up to maxWaitTime
		// if it is blocked in Nomad API request.
	}
	cw.servicesLock.Unlock()

	// Wait for initialization to complete.
	initWG.Wait()
}

// watchForServicesUpdates watches for new services and updates it in cw.
//
// watchForServicesUpdates closes the initCh once the initialization is complete and first discovery iteration is done.
func (cw *nomadWatcher) watchForServicesUpdates(initCh chan struct{}) {
	index := int64(0)
	clientAddr := cw.client.Addr()
	f := func() {
		serviceNames, newIndex, err := cw.getBlockingServiceNames(index)
		if err != nil {
			logger.Errorf("cannot obtain Nomad serviceNames from %q: %s", clientAddr, err)
			return
		}
		if index == newIndex {
			// Nothing changed.
			return
		}
		cw.updateServices(serviceNames)
		index = newIndex
	}

	logger.Infof("started Nomad service watcher for %q", clientAddr)
	f()

	// send signal that initialization is complete
	close(initCh)

	checkInterval := getCheckInterval()
	ticker := time.NewTicker(checkInterval / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			f()
		case <-cw.stopCh:
			logger.Infof("stopping Nomad service watchers for %q", clientAddr)
			startTime := time.Now()
			cw.servicesLock.Lock()
			for _, sw := range cw.services {
				close(sw.stopCh)
			}
			cw.servicesLock.Unlock()
			cw.wg.Wait()
			logger.Infof("stopped Nomad service watcher for %q in %.3f seconds", clientAddr, time.Since(startTime).Seconds())
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
	data, newIndex, err := getBlockingAPIResponse(cw.client, path, index)
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
			if !shouldCollectServiceByName(cw.watchServices, s.ServiceName) {
				continue
			}
			if !shouldCollectServiceByTags(cw.watchTags, s.Tags) {
				continue
			}
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

// watchForServiceNodesUpdates watches for Nomad serviceNode changes for the given serviceName.
//
// watchForServiceNodesUpdates calls initWG.Done() once the initialization is complete and the first discovery iteration is done.
func (sw *serviceWatcher) watchForServiceAddressUpdates(nw *nomadWatcher, initWG *sync.WaitGroup) {
	clientAddr := nw.client.Addr()
	index := int64(0)
	// TODO: Maybe use a different query arg.
	path := "/v1/service/" + sw.serviceName + nw.serviceNamesQueryArgs
	f := func() {
		data, newIndex, err := getBlockingAPIResponse(nw.client, path, index)
		if err != nil {
			logger.Errorf("cannot obtain Nomad services for serviceName=%q from %q: %s", sw.serviceName, clientAddr, err)
			return
		}
		if index == newIndex {
			// Nothing changed.
			return
		}
		sns, err := parseServices(data)
		if err != nil {
			logger.Errorf("cannot parse Nomad services response for serviceName=%q from %q: %s", sw.serviceName, clientAddr, err)
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
	for {
		select {
		case <-ticker.C:
			f()
		case <-sw.stopCh:
			return
		}
	}
}

func shouldCollectServiceByName(filterServices []string, serviceName string) bool {
	if len(filterServices) == 0 {
		return true
	}
	for _, filterService := range filterServices {
		// Use case-insensitive comparison for service names according to https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1422
		if strings.EqualFold(filterService, serviceName) {
			return true
		}
	}
	return false
}

func shouldCollectServiceByTags(filterTags, tags []string) bool {
	if len(filterTags) == 0 {
		return true
	}
	for _, filterTag := range filterTags {
		hasTag := false
		for _, tag := range tags {
			if tag == filterTag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}
	return true
}

func getCheckInterval() time.Duration {
	d := *SDCheckInterval
	if d <= time.Second {
		return time.Second
	}
	return d
}
