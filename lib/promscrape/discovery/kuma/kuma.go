package kuma

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.kumaSDCheckInterval", time.Minute, "Interval for checking for changes in kuma service discovery. "+
	"This works only if kuma_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/sd_configs.html#kuma_sd_configs for details")

// SDConfig represents service discovery config for Kuma Service Mesh.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kuma_sd_config
type SDConfig struct {
	Server            string                     `yaml:"server"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
}

type kumaTarget struct {
	Mesh         string            `json:"mesh"`
	ControlPlane string            `json:"controlplane"`
	Service      string            `json:"service"`
	DataPlane    string            `json:"dataplane"`
	Instance     string            `json:"instance"`
	Scheme       string            `json:"scheme"`
	Address      string            `json:"address"`
	MetricsPath  string            `json:"metrics_path"`
	Labels       map[string]string `json:"labels"`
}

// GetLabels returns kuma service discovery labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutils.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config for kuma_sd: %w", err)
	}
	targets, err := cfg.getTargets()
	if err != nil {
		return nil, err
	}
	return kumaTargetsToLabels(targets, sdc.Server), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		// v can be nil if GetLabels wasn't called yet.
		cfg := v.(*apiConfig)
		cfg.mustStop()
	}
}

func kumaTargetsToLabels(src []kumaTarget, sourceURL string) []*promutils.Labels {
	ms := make([]*promutils.Labels, 0, len(src))
	for _, target := range src {
		m := promutils.NewLabels(8 + len(target.Labels))

		m.Add("instance", target.Instance)
		m.Add("__address__", target.Address)
		m.Add("__scheme__", target.Scheme)
		m.Add("__metrics_path__", target.MetricsPath)
		m.Add("__meta_server", sourceURL)
		m.Add("__meta_kuma_mesh", target.Mesh)
		m.Add("__meta_kuma_service", target.Service)
		m.Add("__meta_kuma_dataplane", target.DataPlane)
		for k, v := range target.Labels {
			m.Add("__meta_kuma_label_"+k, v)
		}

		m.RemoveDuplicates()
		ms = append(ms, m)
	}
	return ms
}

func parseKumaTargets(response discoveryResponse) []kumaTarget {
	result := make([]kumaTarget, 0, len(response.Resources))

	for _, resource := range response.Resources {
		for _, target := range resource.Targets {
			labels := make(map[string]string)
			for label, value := range resource.Labels {
				labels[label] = value
			}
			for label, value := range target.Labels {
				labels[label] = value
			}
			result = append(result, kumaTarget{
				Mesh:         resource.Mesh,
				ControlPlane: response.ControlPlane.Identifier,
				Service:      resource.Service,
				DataPlane:    target.Name,
				Instance:     target.Name,
				Scheme:       target.Scheme,
				Address:      target.Address,
				MetricsPath:  target.MetricsPath,
				Labels:       labels,
			})
		}
	}

	return result
}
