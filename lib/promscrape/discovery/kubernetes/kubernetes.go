package kubernetes

import (
	"fmt"
)

// GetLabels returns labels for the given k8s role and the given cfg.
func GetLabels(cfg *APIConfig, role string) ([]map[string]string, error) {
	switch role {
	case "node":
		return getNodesLabels(cfg)
	case "service":
		return getServicesLabels(cfg)
	case "pod":
		return getPodsLabels(cfg)
	case "endpoints":
		return getEndpointsLabels(cfg)
	case "ingress":
		return getIngressesLabels(cfg)
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `node`, `service`, `pod`, `endpoints` or `ingress`; skipping it", role)
	}
}
