package dockerswarm

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// https://docs.docker.com/engine/api/v1.40/#tag/Service
type service struct {
	ID           string
	Spec         serviceSpec
	UpdateStatus serviceUpdateStatus
	Endpoint     serviceEndpoint
}

type serviceSpec struct {
	Labels       map[string]string
	Name         string
	TaskTemplate taskTemplate
	Mode         serviceSpecMode
}

type taskTemplate struct {
	ContainerSpec containerSpec
}

type containerSpec struct {
	Hostname string
	Image    string
}

type serviceSpecMode struct {
	Global     any
	Replicated any
}

type serviceUpdateStatus struct {
	State string
}

type serviceEndpoint struct {
	Ports      []portConfig
	VirtualIPs []virtualIP
}

type virtualIP struct {
	NetworkID string
	Addr      string
}

type portConfig struct {
	Protocol      string
	Name          string
	PublishMode   string
	PublishedPort int
}

func getServicesLabels(cfg *apiConfig) ([]*promutil.Labels, error) {
	services, err := getServices(cfg)
	if err != nil {
		return nil, err
	}
	networksLabels, err := getNetworksLabelsByNetworkID(cfg)
	if err != nil {
		return nil, err
	}
	return addServicesLabels(services, networksLabels, cfg.port), nil
}

func getServices(cfg *apiConfig) ([]service, error) {
	filtersQueryArg := ""
	if cfg.role == "services" {
		filtersQueryArg = cfg.filtersQueryArg
	}
	data, err := cfg.getAPIResponse("/services", filtersQueryArg)
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

func addServicesLabels(services []service, networksLabels map[string]*promutil.Labels, port int) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, service := range services {
		commonLabels := promutil.NewLabels(10)
		commonLabels.Add("__meta_dockerswarm_service_id", service.ID)
		commonLabels.Add("__meta_dockerswarm_service_name", service.Spec.Name)
		commonLabels.Add("__meta_dockerswarm_service_mode", getServiceMode(service))
		commonLabels.Add("__meta_dockerswarm_service_task_container_hostname", service.Spec.TaskTemplate.ContainerSpec.Hostname)
		commonLabels.Add("__meta_dockerswarm_service_task_container_image", service.Spec.TaskTemplate.ContainerSpec.Image)
		commonLabels.Add("__meta_dockerswarm_service_updating_status", service.UpdateStatus.State)
		for k, v := range service.Spec.Labels {
			commonLabels.Add(discoveryutil.SanitizeLabelName("__meta_dockerswarm_service_label_"+k), v)
		}
		for _, vip := range service.Endpoint.VirtualIPs {
			// skip services without virtual address.
			// usually its host services.
			if vip.Addr == "" {
				continue
			}
			ip, _, err := net.ParseCIDR(vip.Addr)
			if err != nil {
				logger.Errorf("cannot parse: %q as cidr for service label add, err: %v", vip.Addr, err)
				continue
			}
			added := false
			for _, ep := range service.Endpoint.Ports {
				if ep.Protocol != "tcp" {
					continue
				}
				m := promutil.NewLabels(24)
				m.Add("__address__", discoveryutil.JoinHostPort(ip.String(), ep.PublishedPort))
				m.Add("__meta_dockerswarm_service_endpoint_port_name", ep.Name)
				m.Add("__meta_dockerswarm_service_endpoint_port_publish_mode", ep.PublishMode)
				m.AddFrom(commonLabels)
				m.AddFrom(networksLabels[vip.NetworkID])
				// Remove possible duplicate labels, which can appear after AddFrom() calls
				m.RemoveDuplicates()
				added = true
				ms = append(ms, m)
			}
			if !added {
				m := promutil.NewLabels(24)
				m.Add("__address__", discoveryutil.JoinHostPort(ip.String(), port))
				m.AddFrom(commonLabels)
				m.AddFrom(networksLabels[vip.NetworkID])
				// Remove possible duplicate labels, which can appear after AddFrom() calls
				m.RemoveDuplicates()
				ms = append(ms, m)
			}
		}
	}
	return ms
}
