package dockerswarm

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// See https://docs.docker.com/engine/api/v1.40/#tag/Task
type task struct {
	ID                  string
	ServiceID           string
	NodeID              string
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
	Spec struct {
		ContainerSpec struct {
			Labels map[string]string
		}
	}
	Slot int
}

func getTasksLabels(cfg *apiConfig) ([]*promutils.Labels, error) {
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
	filtersQueryArg := ""
	if cfg.role == "tasks" {
		filtersQueryArg = cfg.filtersQueryArg
	}
	resp, err := cfg.getAPIResponse("/tasks", filtersQueryArg)
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

func addTasksLabels(tasks []task, nodesLabels, servicesLabels []*promutils.Labels, networksLabels map[string]*promutils.Labels, services []service, port int) []*promutils.Labels {
	var ms []*promutils.Labels
	for _, task := range tasks {
		commonLabels := promutils.NewLabels(8)
		commonLabels.Add("__meta_dockerswarm_task_id", task.ID)
		commonLabels.Add("__meta_dockerswarm_task_container_id", task.Status.ContainerStatus.ContainerID)
		commonLabels.Add("__meta_dockerswarm_task_desired_state", task.DesiredState)
		commonLabels.Add("__meta_dockerswarm_task_slot", strconv.Itoa(task.Slot))
		commonLabels.Add("__meta_dockerswarm_task_state", task.Status.State)
		for k, v := range task.Spec.ContainerSpec.Labels {
			commonLabels.Add(discoveryutils.SanitizeLabelName("__meta_dockerswarm_container_label_"+k), v)
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
			m := promutils.NewLabels(10)
			m.AddFrom(commonLabels)
			m.Add("__address__", discoveryutils.JoinHostPort(commonLabels.Get("__meta_dockerswarm_node_address"), port.PublishedPort))
			m.Add("__meta_dockerswarm_task_port_publish_mode", port.PublishMode)
			// Remove possible duplicate labels, which can appear after AddFrom() call
			m.RemoveDuplicates()
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
					m := promutils.NewLabels(20)
					m.AddFrom(commonLabels)
					m.AddFrom(networkLabels)
					m.Add("__address__", discoveryutils.JoinHostPort(ip.String(), ep.PublishedPort))
					m.Add("__meta_dockerswarm_task_port_publish_mode", ep.PublishMode)
					// Remove possible duplicate labels, which can appear after AddFrom() calls
					m.RemoveDuplicates()
					ms = append(ms, m)
					added = true
				}
				if !added {
					m := promutils.NewLabels(20)
					m.AddFrom(commonLabels)
					m.AddFrom(networkLabels)
					m.Add("__address__", discoveryutils.JoinHostPort(ip.String(), port))
					// Remove possible duplicate labels, which can appear after AddFrom() calls
					m.RemoveDuplicates()
					ms = append(ms, m)
				}
			}
		}
	}
	return ms
}

// addLabels adds labels from src to dst if they contain the given `key: value` pair.
func addLabels(dst *promutils.Labels, src []*promutils.Labels, key, value string) {
	for _, m := range src {
		if m.Get(key) != value {
			continue
		}
		for _, label := range m.GetLabels() {
			dst.Add(label.Name, label.Value)
		}
		return
	}
}
