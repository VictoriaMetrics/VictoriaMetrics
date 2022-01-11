package dockerswarm

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// See https://docs.docker.com/engine/api/v1.40/#tag/Task
type task struct {
	ID                  string
	ServiceID           string
	NodeID              string
	Labels              map[string]string
	DesiredState        string
	NetworksAttachments []struct {
		Addresses []string
		Network   struct {
			ID string
		}
	}
	Status struct {
		State           string
		ContainerStatus struct {
			ContainerID string
		}
		PortStatus struct {
			Ports []portConfig
		}
	}
	Slot int
}

func getTasksLabels(cfg *apiConfig) ([]map[string]string, error) {
	tasks, err := getTasks(cfg)
	if err != nil {
		return nil, err
	}
	services, err := getServices(cfg)
	if err != nil {
		return nil, err
	}
	networkLabels, err := getNetworksLabelsByNetworkID(cfg)
	if err != nil {
		return nil, err
	}
	svcLabels := addServicesLabels(services, networkLabels, cfg.port)
	nodeLabels, err := getNodesLabels(cfg)
	if err != nil {
		return nil, err
	}
	return addTasksLabels(tasks, nodeLabels, svcLabels, networkLabels, services, cfg.port), nil
}

func getTasks(cfg *apiConfig) ([]task, error) {
	resp, err := cfg.getAPIResponse("/tasks")
	if err != nil {
		return nil, fmt.Errorf("cannot query dockerswarm api for tasks: %w", err)
	}
	return parseTasks(resp)
}

func parseTasks(data []byte) ([]task, error) {
	var tasks []task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("cannot parse tasks: %w", err)
	}
	return tasks, nil
}

func addTasksLabels(tasks []task, nodesLabels, servicesLabels []map[string]string, networksLabels map[string]map[string]string, services []service, port int) []map[string]string {
	var ms []map[string]string
	for _, task := range tasks {
		commonLabels := map[string]string{
			"__meta_dockerswarm_task_id":            task.ID,
			"__meta_dockerswarm_task_container_id":  task.Status.ContainerStatus.ContainerID,
			"__meta_dockerswarm_task_desired_state": task.DesiredState,
			"__meta_dockerswarm_task_slot":          strconv.Itoa(task.Slot),
			"__meta_dockerswarm_task_state":         task.Status.State,
		}
		for k, v := range task.Labels {
			commonLabels["__meta_dockerswarm_task_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		var svcPorts []portConfig
		for i, v := range services {
			if v.ID == task.ServiceID {
				svcPorts = services[i].Endpoint.Ports
				break
			}
		}
		addLabels(commonLabels, servicesLabels, "__meta_dockerswarm_service_id", task.ServiceID)
		addLabels(commonLabels, nodesLabels, "__meta_dockerswarm_node_id", task.NodeID)

		for _, port := range task.Status.PortStatus.Ports {
			if port.Protocol != "tcp" {
				continue
			}
			m := make(map[string]string, len(commonLabels)+2)
			for k, v := range commonLabels {
				m[k] = v
			}
			m["__address__"] = discoveryutils.JoinHostPort(commonLabels["__meta_dockerswarm_node_address"], port.PublishedPort)
			m["__meta_dockerswarm_task_port_publish_mode"] = port.PublishMode
			ms = append(ms, m)
		}
		for _, na := range task.NetworksAttachments {
			networkLabels := networksLabels[na.Network.ID]
			for _, address := range na.Addresses {
				ip, _, err := net.ParseCIDR(address)
				if err != nil {
					logger.Errorf("cannot parse task network attachments address: %s as net CIDR: %v", address, err)
					continue
				}
				added := false
				for _, ep := range svcPorts {
					if ep.Protocol != "tcp" {
						continue
					}
					m := make(map[string]string, len(commonLabels)+len(networkLabels)+2)
					for k, v := range commonLabels {
						m[k] = v
					}
					for k, v := range networkLabels {
						m[k] = v
					}
					m["__address__"] = discoveryutils.JoinHostPort(ip.String(), ep.PublishedPort)
					m["__meta_dockerswarm_task_port_publish_mode"] = ep.PublishMode
					ms = append(ms, m)
					added = true
				}
				if !added {
					m := make(map[string]string, len(commonLabels)+len(networkLabels)+1)
					for k, v := range commonLabels {
						m[k] = v
					}
					for k, v := range networkLabels {
						m[k] = v
					}
					m["__address__"] = discoveryutils.JoinHostPort(ip.String(), port)
					ms = append(ms, m)
				}
			}
		}
	}
	return ms
}

// addLabels adds lables from src to dst if they contain the given `key: value` pair.
func addLabels(dst map[string]string, src []map[string]string, key, value string) {
	for _, m := range src {
		if m[key] != value {
			continue
		}
		for k, v := range m {
			dst[k] = v
		}
		return
	}
}
