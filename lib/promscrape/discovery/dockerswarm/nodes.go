package dockerswarm

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://docs.docker.com/engine/api/v1.40/#tag/Node
type node struct {
	ID   string
	Spec struct {
		Labels       map[string]string
		Role         string
		Availability string
	}
	Description struct {
		Hostname string
		Platform struct {
			Architecture string
			OS           string
		}
		Engine struct {
			EngineVersion string
		}
	}
	Status struct {
		State   string
		Message string
		Addr    string
	}
	ManagerStatus struct {
		Leader       bool
		Reachability string
		Addr         string
	}
}

func getNodesLabels(cfg *apiConfig) ([]map[string]string, error) {
	nodes, err := getNodes(cfg)
	if err != nil {
		return nil, err
	}
	return addNodeLabels(nodes, cfg.port), nil
}

func getNodes(cfg *apiConfig) ([]node, error) {
	resp, err := cfg.getAPIResponse("/nodes")
	if err != nil {
		return nil, fmt.Errorf("cannot query dockerswarm api for nodes: %w", err)
	}
	return parseNodes(resp)
}

func parseNodes(data []byte) ([]node, error) {
	var nodes []node
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("cannot parse nodes: %w", err)
	}
	return nodes, nil
}

func addNodeLabels(nodes []node, port int) []map[string]string {
	var ms []map[string]string
	for _, node := range nodes {
		m := map[string]string{
			"__address__":                                   discoveryutils.JoinHostPort(node.Status.Addr, port),
			"__meta_dockerswarm_node_address":               node.Status.Addr,
			"__meta_dockerswarm_node_availability":          node.Spec.Availability,
			"__meta_dockerswarm_node_engine_version":        node.Description.Engine.EngineVersion,
			"__meta_dockerswarm_node_hostname":              node.Description.Hostname,
			"__meta_dockerswarm_node_id":                    node.ID,
			"__meta_dockerswarm_node_manager_address":       node.ManagerStatus.Addr,
			"__meta_dockerswarm_node_manager_leader":        fmt.Sprintf("%t", node.ManagerStatus.Leader),
			"__meta_dockerswarm_node_manager_reachability":  node.ManagerStatus.Reachability,
			"__meta_dockerswarm_node_platform_architecture": node.Description.Platform.Architecture,
			"__meta_dockerswarm_node_platform_os":           node.Description.Platform.OS,
			"__meta_dockerswarm_node_role":                  node.Spec.Role,
			"__meta_dockerswarm_node_status":                node.Status.State,
		}
		for k, v := range node.Spec.Labels {
			m["__meta_dockerswarm_node_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		ms = append(ms, m)
	}
	return ms
}
