package kubernetes

import "sync"

// we have to store at cache:
// 1) all pods grouped by map[string]Pod -> where string = pod.Name + pod.Namespace
// 2) all services by map[string]Service -> where string = service.Name + service.Namespace.

// watch should be executed for each namespace ?
// multi namespace ?

var servicesCache, nodesCache, endpointsCache sync.Map
