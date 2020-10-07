package dockerswarm

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// https://docs.docker.com/engine/api/v1.40/#tag/Service
type service struct {
	ID   string
	Spec struct {
		Labels       map[string]string
		Name         string
		TaskTemplate struct {
			ContainerSpec struct {
				Hostname string
				Image    string
			}
		}
		Mode struct {
			Global     interface{}
			Replicated interface{}
		}
	}
	UpdateStatus *struct {
		State string
	}
	Endpoint struct {
		Ports      []portConfig
		VirtualIPs []struct {
			NetworkID string
			Addr      string
		}
	}
}

type portConfig struct {
	Protocol      string
	Name          string
	PublishMode   string
	PublishedPort int
}

func getServicesLabels(cfg *apiConfig) ([]map[string]string, error) {
	services, err := getServices(cfg)
	if err != nil {
		return nil, err
	}
	networksLabels, err := getNetworksLabels(cfg)
	if err != nil {
		return nil, err
	}
	return addServicesLabels(services, networksLabels, cfg.port), nil
}

func getServices(cfg *apiConfig) ([]service, error) {
	data, err := cfg.client.GetAPIResponse("/services")
	if err != nil {
		return nil, fmt.Errorf("cannot query dockerswarm api for services: %w", err)
	}
	return parseServicesResponse(data)
}

func parseServicesResponse(data []byte) ([]service, error) {
	var services []service
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("cannot parse services: %w", err)
	}
	return services, nil
}

func getServiceMode(svc service) string {
	if svc.Spec.Mode.Global != nil {
		return "global"
	}
	if svc.Spec.Mode.Replicated != nil {
		return "replicated"
	}
	return ""
}

func addServicesLabels(services []service, networksLabels []map[string]string, port int) []map[string]string {
	var ms []map[string]string
	for _, service := range services {
		m := map[string]string{
			"__meta_dockerswarm_service_id":                      service.ID,
			"__meta_dockerswarm_service_name":                    service.Spec.Name,
			"__meta_dockerswarm_service_task_container_hostname": service.Spec.TaskTemplate.ContainerSpec.Hostname,
			"__meta_dockerswarm_service_task_container_image":    service.Spec.TaskTemplate.ContainerSpec.Image,
			"__meta_dockerswarm_service_mode":                    getServiceMode(service),
		}
		if service.UpdateStatus != nil {
			m["__meta_dockerswarm_service_updating_status"] = service.UpdateStatus.State
		}
		for k, v := range service.Spec.Labels {
			m["__meta_dockerswarm_service_label_"+discoveryutils.SanitizeLabelName(k)] = v
		}
		for _, vip := range service.Endpoint.VirtualIPs {
			var added bool
			ip, _, err := net.ParseCIDR(vip.Addr)
			if err != nil {
				logger.Errorf("cannot parse: %q as cidr for service label add, err: %v", vip.Addr, err)
				continue
			}
			for _, ep := range service.Endpoint.Ports {
				if ep.Protocol != "tcp" {
					continue
				}
				lbls := map[string]string{
					"__meta_dockerswarm_service_endpoint_port_name":         ep.Name,
					"__meta_dockerswarm_service_endpoint_port_publish_mode": ep.PublishMode,
					"__address__": discoveryutils.JoinHostPort(ip.String(), ep.PublishedPort),
				}
				for k, v := range m {
					lbls[k] = v
				}
				lbls = joinLabels(networksLabels, lbls, "__meta_dockerswarm_network_id", vip.NetworkID)
				added = true
				ms = append(ms, lbls)
			}
			if !added {
				lbls := make(map[string]string, len(m))
				for k, v := range m {
					lbls[k] = v
				}
				lbls = joinLabels(networksLabels, lbls, "__meta_dockerswarm_network_id", vip.NetworkID)
				lbls["__address__"] = discoveryutils.JoinHostPort(ip.String(), port)
				ms = append(ms, lbls)
			}
		}

	}
	return ms
}
