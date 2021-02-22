package kubernetes

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// we have to store at cache:
// 1) all pods grouped by map[string]Pod -> where string = pod.Name + pod.Namespace
// 2) all services by map[string]Service -> where string = service.Name + service.Namespace.

// watch should be executed for each namespace ?
// multi namespace ?

var servicesCache, podsCache, endpointsCache, endpointSlicesCache sync.Map

func updatePodCache(cache *sync.Map, p *Pod, action string) {
	switch action {
	case "ADDED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	case "MODIFIED":
		cache.Store(p.key(), p)
	default:
		logger.Infof("unexpected action: %s", action)
	}
}

func processService(cfg *apiConfig, svc *Service, action string) {
	key := "service/" + cfg.setName + svc.key()
	switch action {
	case "ADDED", "MODIFIED":
		cfg.targetChan <- K8sSyncEvent{
			Labels:           svc.appendTargetLabels(nil),
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	case "DELETED":
		cfg.targetChan <- K8sSyncEvent{
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	}
}

func updateServiceCache(cache *sync.Map, p *Service, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	default:
		logger.Infof("default action: %s", action)
	}
}

func updateEndpointsCache(cache *sync.Map, p *Endpoints, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	default:
		logger.Infof("default action: %s", action)
	}
}

func updateEndpointsSliceCache(cache *sync.Map, p *EndpointSlice, action string) {
	switch action {
	case "ADDED", "MODIFIED":
		cache.Store(p.key(), p)
	case "DELETED":
		cache.Delete(p.key())
	default:
		logger.Infof("default action: %s", action)
	}
}

func processIngress(cfg *apiConfig, p *Ingress, action string) {

	switch action {
	case "ADDED", "MODIFIED":
		cfg.targetChan <- K8sSyncEvent{
			Labels:           p.appendTargetLabels(nil),
			Key:              p.key(),
			ConfigSectionSet: cfg.setName,
		}
	case "DELETED":
		cfg.targetChan <- K8sSyncEvent{
			Key:              p.key(),
			ConfigSectionSet: cfg.setName,
		}
	default:
		logger.Infof("default action: %s", action)
	}
}

func processNode(cfg *apiConfig, n *Node, action string) {
	key := "node/" + cfg.setName + n.key()
	switch action {
	case "ADDED", "MODIFIED":
		lbs := n.appendTargetLabels(nil)
		cfg.targetChan <- K8sSyncEvent{
			Labels:           lbs,
			ConfigSectionSet: cfg.setName,
			Key:              key,
		}
	case "DELETED":
		cfg.targetChan <- K8sSyncEvent{
			ConfigSectionSet: cfg.setName,
			Key:              key,
		}
	default:
		logger.Infof("default action: %s", action)
	}
}

func processEndpoints(cfg *apiConfig, p *Endpoints, action string) {

	key := "endpoint/" + cfg.setName + p.key()
	switch action {
	case "ADDED", "MODIFIED":
		lbs := p.appendTargetLabels(nil, &podsCache, &servicesCache)
		cfg.targetChan <- K8sSyncEvent{
			Labels:           lbs,
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	case "DELETED":
		cfg.targetChan <- K8sSyncEvent{
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	default:
		logger.Infof("default action: %s", action)
	}
}

func processEndpointSlices(cfg *apiConfig, p *EndpointSlice, action string) {
	key := "endpointslice/" + cfg.setName + p.key()
	switch action {
	case "ADDED", "MODIFIED":
		cfg.targetChan <- K8sSyncEvent{
			Labels:           p.appendTargetLabels(nil, &podsCache, &servicesCache),
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	case "DELETED":
		cfg.targetChan <- K8sSyncEvent{
			Key:              key,
			ConfigSectionSet: cfg.setName,
		}
	default:
		logger.Infof("default action: %s", action)
	}
}
