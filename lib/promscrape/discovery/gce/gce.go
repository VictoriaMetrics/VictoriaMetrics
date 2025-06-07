package gce

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.gceSDCheckInterval", time.Minute, "Interval for checking for changes in gce. "+
	"This works only if gce_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#gce_sd_configs for details")

// SDConfig represents service discovery config for gce.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config
type SDConfig struct {
	Project string   `yaml:"project"`
	Zone    ZoneYAML `yaml:"zone"`
	Filter  string   `yaml:"filter,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.gceSDCheckInterval` command-line option.
	Port         *int    `yaml:"port,omitempty"`
	TagSeparator *string `yaml:"tag_separator,omitempty"`
}

// ZoneYAML holds info about zones.
type ZoneYAML struct {
	Zones []string
}

// UnmarshalYAML implements yaml.Unmarshaler
func (z *ZoneYAML) UnmarshalYAML(unmarshal func(any) error) error {
	var v any
	if err := unmarshal(&v); err != nil {
		return err
	}
	var zones []string
	switch t := v.(type) {
	case string:
		zones = []string{t}
	case []any:
		for _, vv := range t {
			zone, ok := vv.(string)
			if !ok {
				return fmt.Errorf("unexpected zone type detected: %T; contents: %#v", vv, vv)
			}
			zones = append(zones, zone)
		}
	default:
		return fmt.Errorf("unexpected type unmarshaled for ZoneYAML: %T; contents: %#v", v, v)
	}
	z.Zones = zones
	return nil
}

// MarshalYAML implements yaml.Marshaler
func (z ZoneYAML) MarshalYAML() (any, error) {
	return z.Zones, nil
}

// GetLabels returns gce labels according to sdc.
func (sdc *SDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	ms := getInstancesLabels(cfg)
	return ms, nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.CloseIdleConnections()
	}
}
