package kubernetes

import (
	"encoding/json"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// we have to store at cache:
// 1) all pods grouped by map[string]Pod -> where string = pod.Name + pod.Namespace
// 2) all services by map[string]Service -> where string = service.Name + service.Namespace.

// watch should be executed for each namespace ?
// multi namespace ?

var servicesCache, podsCache sync.Map

type cacheKey interface {
	key() string
}

func updatePodCache(mainCache, sharedCache *sync.Map, p Pod, resp *watchResponse) {
	//var p Pod
	if err := json.Unmarshal(resp.Object, &p); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		if mainCache != nil {
			mainCache.Store(p.key(), p)
		}
		sharedCache.Store(p.key(), p)
	case "DELETED":
		if mainCache != nil {
			mainCache.Delete(p.key())
		}
		sharedCache.Delete(p.key())

	case "MODIFIED":
		if mainCache != nil {
			mainCache.Store(p.key(), p)
		}
		sharedCache.Store(p.key(), p)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}

func updateServiceCache(mainCache, sharedCache *sync.Map, p Service, resp *watchResponse) {
	//var p Pod
	if err := json.Unmarshal(resp.Object, &p); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		if mainCache != nil {
			mainCache.Store(p.key(), p)
		}
		sharedCache.Store(p.key(), p)
	case "DELETED":
		if mainCache != nil {
			mainCache.Delete(p.key())
		}
		sharedCache.Delete(p.key())

	case "MODIFIED":
		if mainCache != nil {
			mainCache.Store(p.key(), p)
		}
		sharedCache.Store(p.key(), p)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}

func updateIngressCache(cache *sync.Map, p Ingress, resp *watchResponse) {
	if err := json.Unmarshal(resp.Object, &p); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())

	case "MODIFIED":
		cache.Store(p.key(), p)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}

func nodesHandle(ac *apiConfig, resp *watchResponse) {
	//var p Pod
	var obj Node
	if err := json.Unmarshal(resp.Object, &obj); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		ac.watchCache.Store(obj.key(), obj)
	case "DELETED":
		ac.watchCache.Delete(obj.key())
	case "MODIFIED":
		ac.watchCache.Store(obj.key(), obj)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}

func updateEndpointsCache(cache *sync.Map, p Endpoints, resp *watchResponse) {
	if err := json.Unmarshal(resp.Object, &p); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "MODIFIED":
		cache.Store(p.key(), p)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}

func updateEndpointSlicesCache(cache *sync.Map, p EndpointSlice, resp *watchResponse) {
	if err := json.Unmarshal(resp.Object, &p); err != nil {
		logger.Errorf("cannot unmarshal object: %v", err)
		return
	}
	switch resp.Action {
	case "ADDED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "MODIFIED":
		cache.Store(p.key(), p)
	default:
		logger.Infof("default action: %s", resp.Action)
	}
}
