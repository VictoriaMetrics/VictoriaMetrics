package nacos

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type V3Response struct {
	Code    int        `json:"code"`
	Message string     `json:"message"`
	Data    []Instance `json:"data"`
}

type Instance struct {
	Ip                        string            `json:"ip"`
	Port                      int               `json:"port"`
	Weight                    float64           `json:"weight"`
	Healthy                   bool              `json:"healthy"`
	Enabled                   bool              `json:"enabled"`
	Ephemeral                 bool              `json:"ephemeral"`
	ClusterName               string            `json:"clusterName"`
	ServiceName               string            `json:"serviceName"`
	Metadata                  map[string]string `json:"metadata"`
	InstanceHeartBeatInterval int               `json:"instanceHeartBeatInterval"`
	InstanceHeartBeatTimeOut  int               `json:"instanceHeartBeatTimeOut"`
	IpDeleteTimeout           int               `json:"ipDeleteTimeout"`
}

func (i *Instance) appendTargetLabels(ms []*promutil.Labels, svc, namespace, group string) []*promutil.Labels {
	var addr = discoveryutil.JoinHostPort(i.Ip, i.Port)
	m := promutil.NewLabels(16)
	m.Add("__address__", addr)
	m.Add("__meta_nacos_service", svc)
	m.Add("__meta_nacos_service_address", i.Ip)
	m.Add("__meta_nacos_service_healthy", strconv.FormatBool(i.Healthy))
	m.Add("__meta_nacos_service_ephemeral", strconv.FormatBool(i.Ephemeral))
	m.Add("__meta_nacos_service_cluster", i.ClusterName)
	m.Add("__meta_nacos_service_enabled", strconv.FormatBool(i.Enabled))
	m.Add("__meta_nacos_service_port", strconv.Itoa(i.Port))
	m.Add("__meta_nacos_service_namespace", namespace)
	m.Add("__meta_nacos_service_group", group)
	for k, v := range i.Metadata {
		m.Add(discoveryutil.SanitizeLabelName("__meta_nacos_metadata_"+k), v)
	}
	ms = append(ms, m)
	return ms
}

// GetInstance return parsed slice of ServiceNode by data.
func GetInstance(data []byte) ([]Instance, error) {
	var response V3Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ServiceNodes from %q: %w", data, err)
	}
	return response.Data, nil
}

func getServiceInstancesLabels(cfg *apiConfig) []*promutil.Labels {
	sis := cfg.nacosWatcher.getServiceInstanceSnapshot()
	var ms []*promutil.Labels
	for svc, sn := range sis {
		for i := range sn {
			ms = sn[i].appendTargetLabels(ms, svc, cfg.nacosWatcher.namespace, cfg.nacosWatcher.group)
		}
	}
	return ms
}
