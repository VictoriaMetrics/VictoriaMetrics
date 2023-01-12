package dockerswarm

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
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

func getNodesLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
	nodes, err := getNodes(cfg)
	if err != nil {
		return nil, err
	}
	return addNodeLabels(nodes, cfg.port), nil
}

func getNodes(cfg *apiConfig) ([]node, error) {
	filtersQueryArg := ""
	if cfg.role == "nodes" {
		filtersQueryArg = cfg.filtersQueryArg
	}
	resp, err := cfg.getAPIResponse("/nodes", filtersQueryArg)
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

func addNodeLabels(nodes []node, port int) []*promutils.Labels {
	var ms []*promutils.Labels
	for _, node := range nodes {
		m := promutils.NewLabels(16)
		m.Add("__address__", discoveryutils.JoinHostPort(node.Status.Addr, port))
		m.Add("__meta_dockerswarm_node_address", node.Status.Addr)
		m.Add("__meta_dockerswarm_node_availability", node.Spec.Availability)
		m.Add("__meta_dockerswarm_node_engine_version", node.Description.Engine.EngineVersion)
		m.Add("__meta_dockerswarm_node_hostname", node.Description.Hostname)
		m.Add("__meta_dockerswarm_node_id", node.ID)
		m.Add("__meta_dockerswarm_node_manager_address", node.ManagerStatus.Addr)
		m.Add("__meta_dockerswarm_node_manager_leader", fmt.Sprintf("%t", node.ManagerStatus.Leader))
		m.Add("__meta_dockerswarm_node_manager_reachability", node.ManagerStatus.Reachability)
		m.Add("__meta_dockerswarm_node_platform_architecture", node.Description.Platform.Architecture)
		m.Add("__meta_dockerswarm_node_platform_os", node.Description.Platform.OS)
		m.Add("__meta_dockerswarm_node_role", node.Spec.Role)
		m.Add("__meta_dockerswarm_node_status", node.Status.State)
		for k, v := range node.Spec.Labels {
			m.Add(discoveryutils.SanitizeLabelName("__meta_dockerswarm_node_label_"+k), v)
		}
		ms = append(ms, m)
	}
	return ms
}
