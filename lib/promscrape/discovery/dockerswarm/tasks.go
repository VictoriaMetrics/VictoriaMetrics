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
		ContainerStatus *struct {
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
	networkLabels, err := getNetworksLabels(cfg)
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
	resp, err := cfg.client.GetAPIResponse("/tasks")
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

func addTasksLabels(tasks []task, nodesLabels, servicesLabels, networksLabels []map[string]string, services []service, port int) []map[string]string {
	var ms []map[string]string
	for _, task := range tasks {
		m := map[string]string{
			"__meta_dockerswarm_task_id":            task.ID,
			"__meta_dockerswarm_task_desired_state": task.DesiredState,
			"__meta_dockerswarm_task_state":         task.Status.State,
			"__meta_dockerswarm_task_slot":          strconv.Itoa(task.Slot),
		}
		if task.Status.ContainerStatus != nil {
			m["__meta_dockerswarm_task_container_id"] = task.Status.ContainerStatus.ContainerID
		}
		for k, v := range task.Labels {
			m["__meta_dockerswarm_task_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		var svcPorts []portConfig
		for i, v := range services {
			if v.ID == task.ServiceID {
				svcPorts = services[i].Endpoint.Ports
				break
			}
		}
		m = joinLabels(servicesLabels, m, "__meta_dockerswarm_service_id", task.ServiceID)
		m = joinLabels(nodesLabels, m, "__meta_dockerswarm_node_id", task.NodeID)

		for _, port := range task.Status.PortStatus.Ports {
			if port.Protocol != "tcp" {
				continue
			}
			lbls := make(map[string]string, len(m))
			lbls["__meta_dockerswarm_task_port_publish_mode"] = port.PublishMode
			lbls["__address__"] = discoveryutils.JoinHostPort(m["__meta_dockerswarm_node_address"], port.PublishedPort)
			for k, v := range m {
				lbls[k] = v
			}
			ms = append(ms, lbls)
		}
		for _, na := range task.NetworksAttachments {
			for _, address := range na.Addresses {
				ip, _, err := net.ParseCIDR(address)
				if err != nil {
					logger.Errorf("cannot parse task network attachments address: %s as net CIDR: %v", address, err)
					continue
				}
				var added bool
				for _, v := range svcPorts {
					if v.Protocol != "tcp" {
						continue
					}
					lbls := make(map[string]string, len(m))
					for k, v := range m {
						lbls[k] = v
					}
					lbls = joinLabels(networksLabels, lbls, "__meta_dockerswarm_network_id", na.Network.ID)
					lbls["__address"] = discoveryutils.JoinHostPort(ip.String(), v.PublishedPort)
					lbls["__meta_dockerswarm_task_port_publish_mode"] = v.PublishMode
					ms = append(ms, lbls)
					added = true
				}

				if !added {
					lbls := make(map[string]string, len(m))
					for k, v := range m {
						lbls[k] = v
					}
					lbls = joinLabels(networksLabels, lbls, "__meta_dockerswarm_network_id", na.Network.ID)
					lbls["__address__"] = discoveryutils.JoinHostPort(ip.String(), port)
					ms = append(ms, lbls)
				}
			}
		}
	}
	return ms
}
