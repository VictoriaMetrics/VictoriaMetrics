package consul

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

type serviceWatch struct {
	stopCh       chan struct{}
	serviceNodes []ServiceNode
}

// watcher for consul api, updates targets in background with long-polling.
type consulWatcher struct {
	baseQueryArgs       string
	client              *discoveryutils.Client
	lastAccessTime      atomic.Value
	nodeMeta            string
	shouldWatchServices []string
	shouldWatchTags     []string
	// guards services
	servicesLock sync.Mutex
	services     map[string]serviceWatch
	stopCh       chan struct{}
}

// init new watcher and start background service discovery for Consul.
func newConsulWatcher(client *discoveryutils.Client, sdc *SDConfig, datacenter string) (*consulWatcher, error) {
	baseQueryArgs := fmt.Sprintf("?sdc=%s", url.QueryEscape(datacenter))
	var nodeMeta string
	if len(sdc.NodeMeta) > 0 {
		for k, v := range sdc.NodeMeta {
			nodeMeta += fmt.Sprintf("&node-meta=%s", url.QueryEscape(k+":"+v))
		}
	}
	if sdc.AllowStale {
		baseQueryArgs += "&stale"
	}
	cw := consulWatcher{
		client:              client,
		baseQueryArgs:       baseQueryArgs,
		shouldWatchServices: sdc.Services,
		shouldWatchTags:     sdc.Tags,
		services:            make(map[string]serviceWatch),
	}

	watchServiceNames, _, err := cw.getServiceNames(0)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	cw.servicesLock.Lock()
	for serviceName := range watchServiceNames {
		stopCh := make(chan struct{})
		cw.services[serviceName] = serviceWatch{stopCh: stopCh}
		wg.Add(1)
		go func(serviceName string) {
			defer wg.Done()
			cw.watchForServiceUpdates(serviceName, stopCh)
		}(serviceName)
	}
	cw.servicesLock.Unlock()
	// wait for first init.
	wg.Wait()
	go cw.watchForServices()
	return &cw, nil
}

// stops all service watchers.
func (cw *consulWatcher) stopServiceWatchersAll() {
	cw.servicesLock.Lock()
	for _, sw := range cw.services {
		close(sw.stopCh)
	}
	cw.servicesLock.Unlock()
}

// getServiceNames returns serviceNames and index version.
func (cw *consulWatcher) getServiceNames(index uint64) (map[string]struct{}, uint64, error) {
	sns := make(map[string]struct{})
	path := fmt.Sprintf("/v1/catalog/services%s", cw.baseQueryArgs)
	if len(cw.nodeMeta) > 0 {
		path += cw.nodeMeta
	}
	data, newIndex, err := getBlockingAPIResponse(cw.client, path, index)
	if err != nil {
		return nil, index, err
	}
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, index, fmt.Errorf("cannot parse services response=%q, err=%w", data, err)
	}
	for k, tags := range m {
		if !shouldCollectServiceByName(cw.shouldWatchServices, k) {
			continue
		}
		if !shouldCollectServiceByTags(cw.shouldWatchTags, tags) {
			continue
		}
		sns[k] = struct{}{}
	}
	return sns, newIndex, nil
}

// listen for new services and update it.
func (cw *consulWatcher) watchForServices() {
	ticker := time.NewTicker(*SDCheckInterval)
	defer ticker.Stop()
	var index uint64
	for {
		select {
		case <-cw.stopCh:
			cw.stopServiceWatchersAll()
			return
		case <-ticker.C:
			if time.Since(cw.lastAccessTime.Load().(time.Time)) > *SDCheckInterval*2 {
				// exit watch and stop all background watchers.
				cw.stopServiceWatchersAll()
				return
			}
			m, newIndex, err := cw.getServiceNames(index)
			if err != nil {
				logger.Errorf("failed get serviceNames from consul api: err=%v", err)
				continue
			}
			// nothing changed.
			if index == newIndex {
				continue
			}
			cw.servicesLock.Lock()
			// start new services watchers.
			for svc := range m {
				if _, ok := cw.services[svc]; !ok {
					stopCh := make(chan struct{})
					cw.services[svc] = serviceWatch{stopCh: stopCh}
					go cw.watchForServiceUpdates(svc, stopCh)
				}
			}
			// stop watch for removed services.
			for svc, s := range cw.services {
				if _, ok := m[svc]; !ok {
					close(s.stopCh)
					delete(cw.services, svc)
				}
			}
			cw.servicesLock.Unlock()
			index = newIndex
		}
	}

}

// start watching for consul service changes.
func (cw *consulWatcher) watchForServiceUpdates(svc string, stopCh chan struct{}) {
	ticker := time.NewTicker(*SDCheckInterval)
	defer ticker.Stop()
	updateServiceState := func(index uint64) uint64 {
		sns, newIndex, err := getServiceState(cw.client, svc, cw.baseQueryArgs, index)
		if err != nil {
			logger.Errorf("failed update service state, service_name=%q, err=%v", svc, err)
			return index
		}
		if newIndex == index {
			return index
		}
		cw.servicesLock.Lock()
		s := cw.services[svc]
		s.serviceNodes = sns
		cw.services[svc] = s
		cw.servicesLock.Unlock()
		return newIndex
	}
	watchIndex := updateServiceState(0)
	go func() {
		for {
			select {
			case <-ticker.C:
				watchIndex = updateServiceState(watchIndex)
			case <-stopCh:
				return
			}
		}
	}()
}

// returns ServiceNodes.
func (cw *consulWatcher) getServiceNodes() []ServiceNode {
	var sns []ServiceNode
	cw.servicesLock.Lock()
	for _, v := range cw.services {
		sns = append(sns, v.serviceNodes...)
	}
	cw.servicesLock.Unlock()
	cw.lastAccessTime.Store(time.Now())
	return sns
}

func shouldCollectServiceByName(filterServices []string, service string) bool {
	if len(filterServices) == 0 {
		return true
	}
	for _, filterService := range filterServices {
		if filterService == service {
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
