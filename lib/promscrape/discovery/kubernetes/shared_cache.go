package kubernetes

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// SharedKubernetesCache holds cache of kubernetes objects for current config.
type SharedKubernetesCache struct {
	Endpoints       *sync.Map
	EndpointsSlices *sync.Map
	Pods            *sync.Map
	Services        *sync.Map
}

// NewSharedKubernetesCache returns new cache.
func NewSharedKubernetesCache() *SharedKubernetesCache {
	return &SharedKubernetesCache{
		Endpoints:       new(sync.Map),
		EndpointsSlices: new(sync.Map),
		Pods:            new(sync.Map),
		Services:        new(sync.Map),
	}
}

func updatePodCache(cache *sync.Map, p *Pod, action string) {
	switch action {
	case "ADDED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "MODIFIED":
		cache.Store(p.key(), p)
	case "ERROR":
	default:
		logger.Warnf("unexpected action: %s", action)
	}
}

func updateServiceCache(cache *sync.Map, p *Service, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "ERROR":
	default:
		logger.Warnf("unexpected action: %s", action)
	}

}

func updateEndpointsCache(cache *sync.Map, p *Endpoints, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "ERROR":
	default:
		logger.Warnf("unexpected action: %s", action)
	}
}

func updateEndpointsSliceCache(cache *sync.Map, p *EndpointSlice, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	default:
		logger.Infof("unexpected action: %s", action)
	}
}
