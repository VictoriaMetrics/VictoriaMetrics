package zookeeper

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.serversetSDCheckInterval", 30*time.Second, "Interval for checking for changes in ServerSet. "+
	"This works only if serverset_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#serverset_sd_configs for details")

// SDConfig represents service discovery config for ZooKeeper ServerSet.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#serverset_sd_config
type SDConfig struct {
	Servers []string       `yaml:"servers"`
	Paths   []string       `yaml:"paths"`
	Timeout *time.Duration `yaml:"timeout,omitempty"`
}

// GetLabels returns ServerSet labels according to sdc.
func (sdc *SDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
	if len(sdc.Servers) == 0 {
		return nil, fmt.Errorf("`servers` cannot be empty in `serverset_sd_config`")
	}
	if len(sdc.Paths) == 0 {
		return nil, fmt.Errorf("`paths` cannot be empty in `serverset_sd_config`")
	}
	cfg, err := getAPIConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config for serverset_sd_config: %w", err)
	}
	ms := getServersetLabels(cfg, sdc.Paths)
	return ms, nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.mustStop()
	}
}
