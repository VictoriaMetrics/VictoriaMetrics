package dockerswarm

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// See https://docs.docker.com/engine/api/v1.40/#tag/Node
type node struct {
	ID            string
	Spec          nodeSpec
	Description   nodeDescription
	Status        nodeStatus
	ManagerStatus nodeManagerStatus
}

type nodeSpec struct {
	Labels       map[string]string
	Role         string
	Availability string
}

type nodeDescription struct {
	Hostname string
	Platform nodePlatform
	Engine   nodeEngine
}

type nodePlatform struct {
	Architecture string
	OS           string
}

type nodeEngine struct {
	EngineVersion string
}

type nodeStatus struct {
	State   string
	Message string
	Addr    string
}

type nodeManagerStatus struct {
	Leader       bool
	Reachability string
	Addr         string
}

func getNodesLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
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

func addNodeLabels(nodes []node, port int) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, node := range nodes {
		m := promutil.NewLabels(16)
		m.Add("__address__", discoveryutil.JoinHostPort(node.Status.Addr, port))
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
			m.Add(discoveryutil.SanitizeLabelName("__meta_dockerswarm_node_label_"+k), v)
		}
		ms = append(ms, m)
	}
	return ms
}
