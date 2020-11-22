package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

type serviceWatch struct {
	cancel       context.CancelFunc
	serviceNodes []ServiceNode
}

// watcher for consul api, updates targets in background with long-polling.
type watchConsul struct {
	baseQueryArgs  string
	cancel         context.CancelFunc
	client         *discoveryutils.Client
	lastAccessTime uint64
	// guards services
	mu                  sync.Mutex
	nodeMeta            string
	shouldWatchServices []string
	shouldWatchTags     []string
	services            map[string]serviceWatch
}

// init new watcher and start bachground discovery.
func newWatchConsul(client *discoveryutils.Client, sdc *SDConfig, dc string) (*watchConsul, error) {
	// wait time must be less, then fasthttp client deadline - its 1 minute.
	baseQueryArgs := fmt.Sprintf("?sdc=%s", url.QueryEscape(dc))
	var nodeMeta string
	if len(sdc.NodeMeta) > 0 {
		for k, v := range sdc.NodeMeta {
			nodeMeta += fmt.Sprintf("&node-meta=%s", url.QueryEscape(k+":"+v))
		}
	}
	if sdc.AllowStale {
		baseQueryArgs += "&stale"
	}
	wc := watchConsul{
		client:              client,
		baseQueryArgs:       baseQueryArgs,
		shouldWatchServices: sdc.Services,
		shouldWatchTags:     sdc.Tags,
		services:            make(map[string]serviceWatch),
	}

	watchServiceNames, _, err := wc.getServiceNames(0)
	if err != nil {
		return nil, err
	}
	// global context
	ctx, cancel := context.WithCancel(context.Background())
	wc.cancel = cancel
	var syncWait sync.WaitGroup
	for serviceName := range watchServiceNames {
		ctx, cancel := context.WithCancel(ctx)
		syncWait.Add(1)
		go wc.startWatchService(ctx, serviceName, &syncWait)
		wc.services[serviceName] = serviceWatch{cancel: cancel}
	}
	// wait for first init.
	syncWait.Wait()
	go wc.watchForServices(ctx)
	return &wc, nil
}

// stops all service watchers.
func (w *watchConsul) stopAll() {
	w.mu.Lock()
	for _, sw := range w.services {
		sw.cancel()
	}
	w.mu.Unlock()
}

// getServiceNames returns serviceNames and index version.
func (w *watchConsul) getServiceNames(index uint64) (map[string]struct{}, uint64, error) {
	sns := make(map[string]struct{})
	path := fmt.Sprintf("/v1/catalog/services%s", w.baseQueryArgs)
	if len(w.nodeMeta) > 0 {
		path += w.nodeMeta
	}
	data, newIndex, err := getAPIResponse(w.client, path, index)
	if err != nil {
		return nil, index, err
	}
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, index, fmt.Errorf("cannot parse services response=%q, err=%w", data, err)
	}
	for k, tags := range m {
		if !shouldCollectServiceByName(w.shouldWatchServices, k) {
			continue
		}
		if !shouldCollectServiceByTags(w.shouldWatchTags, tags) {
			continue
		}
		sns[k] = struct{}{}
	}
	return sns, newIndex, nil
}

// listen for new services and update it.
func (w *watchConsul) watchForServices(ctx context.Context) {
	ticker := time.NewTicker(*SDCheckInterval)
	defer ticker.Stop()
	var index uint64
	for {
		select {
		case <-ctx.Done():
			w.stopAll()
			return
		case <-ticker.C:
			if fasttime.UnixTimestamp()-atomic.LoadUint64(&w.lastAccessTime) > uint64(SDCheckInterval.Seconds())*2 {
				// exit watch and stop all background watchers.
				w.stopAll()
				return
			}
			m, newIndex, err := w.getServiceNames(index)
			if err != nil {
				logger.Errorf("failed get serviceNames from consul api: err=%v", err)
				continue
			}
			// nothing changed.
			if index == newIndex {
				continue
			}
			w.mu.Lock()
			// start new services watchers.
			for svc := range m {
				if _, ok := w.services[svc]; !ok {
					ctx, cancel := context.WithCancel(ctx)
					go w.startWatchService(ctx, svc, nil)
					w.services[svc] = serviceWatch{cancel: cancel}
				}
			}
			// stop watch for removed services.
			for svc, s := range w.services {
				if _, ok := m[svc]; !ok {
					s.cancel()
					delete(w.services, svc)
				}
			}
			w.mu.Unlock()
			index = newIndex
		}
	}

}

// start watch for consul service changes.
func (w *watchConsul) startWatchService(ctx context.Context, svc string, initWait *sync.WaitGroup) {
	ticker := time.NewTicker(*SDCheckInterval)
	defer ticker.Stop()
	updateServiceState := func(index uint64) uint64 {
		sns, newIndex, err := getServiceState(w.client, svc, w.baseQueryArgs, index)
		if err != nil {
			logger.Errorf("failed update service state err=%v", err)
			return index
		}
		if newIndex == index {
			return index
		}
		w.mu.Lock()
		s := w.services[svc]
		s.serviceNodes = sns
		w.services[svc] = s
		w.mu.Unlock()
		return newIndex
	}
	watchIndex := updateServiceState(0)
	// report after first sync if needed.
	if initWait != nil {
		initWait.Done()
	}
	for {
		select {
		case <-ticker.C:
			watchIndex = updateServiceState(watchIndex)
		case <-ctx.Done():
			return
		}
	}
}

// returns combined ServiceNodes.
func (w *watchConsul) getSNS() []ServiceNode {
	var sns []ServiceNode
	w.mu.Lock()
	for _, v := range w.services {
		sns = append(sns, v.serviceNodes...)
	}
	w.mu.Unlock()
	atomic.StoreUint64(&w.lastAccessTime, fasttime.UnixTimestamp())
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
