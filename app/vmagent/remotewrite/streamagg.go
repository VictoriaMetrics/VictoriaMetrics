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

// saConfigRules - type alias for unmarshalled stream aggregation config
type saConfigRules = []*streamaggr.Config

// saConfigsLoader loads stream aggregation configs from the given files.
type saConfigsLoader struct {
	files   []string
	configs atomic.Pointer[[]saConfig]
}

// newSaConfigsLoader creates new saConfigsLoader for the given config files.
func newSaConfigsLoader(configFiles []string) (*saConfigsLoader, error) {
	result := &saConfigsLoader{
		files: configFiles,
	}
	// Initial load of configs.
	if err := result.reloadConfigs(); err != nil {
		return nil, err
	}
	return result, nil
}

// reloadConfigs reloads stream aggregation configs from the files given in constructor.
func (r *saConfigsLoader) reloadConfigs() error {
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
	r.configs.Store(&configs)

	saCfgSuccess.Set(1)
	saCfgTimestamp.Set(fasttime.UnixTimestamp())
	return nil
}

// getCurrentConfig returns the current stream aggregation config with the given idx.
func (r *saConfigsLoader) getCurrentConfig(idx int) (saConfigRules, uint64) {
	all := r.configs.Load()
	if all == nil {
		return nil, 0
	}
	cfgs := *all
	if len(cfgs) == 0 {
		return nil, 0
	}
	if idx >= len(cfgs) {
		if len(cfgs) == 1 {
			cfg := cfgs[0]
			return cfg.rules, cfg.hash
		}
		return nil, 0
	}
	cfg := cfgs[idx]
	return cfg.rules, cfg.hash
}

type saConfig struct {
	path  string
	hash  uint64
	rules saConfigRules
}

// CheckStreamAggConfigs checks -remoteWrite.streamAggr.config.
func CheckStreamAggConfigs() error {
	_, err := newSaConfigsLoader(*streamAggrConfig)
	return err
}
