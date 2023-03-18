package remotewrite

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/streamaggr"
	"github.com/VictoriaMetrics/metrics"
)

var (
	streamAggrConfig = flagutil.NewArrayString("remoteWrite.streamAggr.config", "Optional path to file with stream aggregation config. "+
		"See https://docs.victoriametrics.com/stream-aggregation.html . "+
		"See also -remoteWrite.streamAggr.keepInput and -remoteWrite.streamAggr.dedupInterval")
	streamAggrKeepInput = flagutil.NewArrayBool("remoteWrite.streamAggr.keepInput", "Whether to keep input samples after the aggregation with -remoteWrite.streamAggr.config. "+
		"By default the input is dropped after the aggregation, so only the aggregate data is sent to the -remoteWrite.url. "+
		"See https://docs.victoriametrics.com/stream-aggregation.html")
	streamAggrDedupInterval = flagutil.NewArrayDuration("remoteWrite.streamAggr.dedupInterval", "Input samples are de-duplicated with this interval before being aggregated. "+
		"Only the last sample per each time series per each interval is aggregated if the interval is greater than zero")
)

var (
	saCfgReloads   = metrics.NewCounter(`vmagent_streamaggr_config_reloads_total`)
	saCfgReloadErr = metrics.NewCounter(`vmagent_streamaggr_config_reloads_errors_total`)
	saCfgSuccess   = metrics.NewCounter(`vmagent_streamaggr_config_last_reload_successful`)
	saCfgTimestamp = metrics.NewCounter(`vmagent_streamaggr_config_last_reload_success_timestamp_seconds`)
)

type SaConfigRules = []*streamaggr.Config

type SaConfigLoader struct {
	files   []string
	chans   []chan SaConfigRules
	configs atomic.Pointer[[]saConfig]
}

// NewSaConfigLoader creates new SaConfigLoader for the given config files.
func NewSaConfigLoader(configFiles []string) (*SaConfigLoader, error) {
	var chans = make([]chan SaConfigRules, len(configFiles))
	for i := range chans {
		chans[i] = make(chan SaConfigRules)
	}
	result := &SaConfigLoader{
		chans: chans,
		files: configFiles,
	}
	// Initial load of configs.
	if err := result.ReloadConfigs(); err != nil {
		return nil, err
	}
	return result, nil
}

// ReloadConfigs reloads stream aggregation configs from the files given in constructor.
func (r *SaConfigLoader) ReloadConfigs() error {
	// Increment reloads counter if it is not the initial load.
	if r.configs.Load() != nil {
		saCfgReloads.Inc()
	}

	// Load all configs from files.
	var configs = make([]saConfig, len(r.files))
	for i, path := range r.files {
		if len(path) == 0 {
			// Skip empty stream aggregation config.
			continue
		}
		rules, hash, err := streamaggr.LoadConfigsFromFile(path)
		if err != nil {
			saCfgSuccess.Set(0)
			saCfgReloadErr.Inc()
			return fmt.Errorf("cannot load stream aggregation config from %q: %w", path, err)
		}
		configs[i] = saConfig{
			path:  path,
			hash:  hash,
			rules: rules,
		}
	}

	// Update configs.
	old := r.configs.Swap(&configs)
	// If it is the initial load, then don't notify the consumers about the configs.
	if old != nil {
		for i, ch := range r.chans {
			// Skip sending configs to ch if they haven't changed.
			if (*old)[i].hash == configs[i].hash {
				continue
			}
			// Notify the consumer about the new configs.
			ch <- configs[i].rules
		}
	}

	saCfgSuccess.Set(1)
	saCfgTimestamp.Set(fasttime.UnixTimestamp())
	return nil
}

// GetCurrentConfig returns the current stream aggregation config with the given idx.
func (r *SaConfigLoader) GetCurrentConfig(idx int) SaConfigRules {
	all := r.configs.Load()
	if all == nil {
		panic("BUG: SaConfigLoader.GetCurrentConfig called before SaConfigLoader.ReloadConfigs")
	}
	cfg := (*all)[idx]
	return cfg.rules
}

// UpdatesCh returns channel for receiving updates for the stream aggregation config with the given idx.
func (r *SaConfigLoader) UpdatesCh(idx int) chan SaConfigRules {
	return r.chans[idx]
}

type saConfig struct {
	path  string
	hash  uint64
	rules SaConfigRules
}

// CheckStreamAggConfigs checks -remoteWrite.streamAggr.config.
func CheckStreamAggConfigs() error {
	_, err := NewSaConfigLoader(*streamAggrConfig)
	return err
}
