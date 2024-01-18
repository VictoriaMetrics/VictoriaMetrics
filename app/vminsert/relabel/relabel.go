package relabel

import (
	"flag"
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/metrics"
)

var (
	relabelConfig = flag.String("relabelConfig", "", "Optional path to a file with relabeling rules, which are applied to all the ingested metrics. "+
		"The path can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/#relabeling for details. The config is reloaded on SIGHUP signal")

	usePromCompatibleNaming = flag.Bool("usePromCompatibleNaming", false, "Whether to replace characters unsupported by Prometheus with underscores "+
		"in the ingested metric names and label names. For example, foo.bar{a.b='c'} is transformed into foo_bar{a_b='c'} during data ingestion if this flag is set. "+
		"See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels")
)

// Init must be called after flag.Parse and before using the relabel package.
func Init() {
	// Register SIGHUP handler for config re-read just before loadRelabelConfig call.
	// This guarantees that the config will be re-read if the signal arrives during loadRelabelConfig call.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1240
	sighupCh := procutil.NewSighupChan()

	pcs, err := loadRelabelConfig()
	if err != nil {
		logger.Fatalf("cannot load relabelConfig: %s", err)
	}
	pcsGlobal.Store(pcs)
	configSuccess.Set(1)
	configTimestamp.Set(fasttime.UnixTimestamp())

	if len(*relabelConfig) == 0 {
		return
	}
	go func() {
		for range sighupCh {
			configReloads.Inc()
			logger.Infof("received SIGHUP; reloading -relabelConfig=%q...", *relabelConfig)
			pcs, err := loadRelabelConfig()
			if err != nil {
				configReloadErrors.Inc()
				configSuccess.Set(0)
				logger.Errorf("cannot load the updated relabelConfig: %s; preserving the previous config", err)
				continue
			}
			pcsGlobal.Store(pcs)
			configSuccess.Set(1)
			configTimestamp.Set(fasttime.UnixTimestamp())
			logger.Infof("successfully reloaded -relabelConfig=%q", *relabelConfig)
		}
	}()
}

var (
	configReloads      = metrics.NewCounter(`vm_relabel_config_reloads_total`)
	configReloadErrors = metrics.NewCounter(`vm_relabel_config_reloads_errors_total`)
	configSuccess      = metrics.NewGauge(`vm_relabel_config_last_reload_successful`, nil)
	configTimestamp    = metrics.NewCounter(`vm_relabel_config_last_reload_success_timestamp_seconds`)
)

var pcsGlobal atomic.Pointer[promrelabel.ParsedConfigs]

// CheckRelabelConfig checks config pointed by -relabelConfig
func CheckRelabelConfig() error {
	_, err := loadRelabelConfig()
	return err
}

func loadRelabelConfig() (*promrelabel.ParsedConfigs, error) {
	if len(*relabelConfig) == 0 {
		return nil, nil
	}
	pcs, err := promrelabel.LoadRelabelConfigs(*relabelConfig)
	if err != nil {
		return nil, fmt.Errorf("error when reading -relabelConfig=%q: %w", *relabelConfig, err)
	}
	return pcs, nil
}

// HasRelabeling returns true if there is global relabeling.
func HasRelabeling() bool {
	pcs := pcsGlobal.Load()
	return pcs.Len() > 0 || *usePromCompatibleNaming
}

// Ctx holds relabeling context.
type Ctx struct {
	// tmpLabels is used during ApplyRelabeling call.
	tmpLabels []prompbmarshal.Label
}

// Reset resets ctx.
func (ctx *Ctx) Reset() {
	promrelabel.CleanLabels(ctx.tmpLabels)
	ctx.tmpLabels = ctx.tmpLabels[:0]
}

// ApplyRelabeling applies relabeling to the given labels and returns the result.
//
// The returned labels are valid until the next call to ApplyRelabeling.
func (ctx *Ctx) ApplyRelabeling(labels []prompb.Label) []prompb.Label {
	pcs := pcsGlobal.Load()
	if pcs.Len() == 0 && !*usePromCompatibleNaming {
		// There are no relabeling rules.
		return labels
	}
	// Convert labels to prompbmarshal.Label format suitable for relabeling.
	tmpLabels := ctx.tmpLabels[:0]
	for _, label := range labels {
		name := label.Name
		if name == "" {
			name = "__name__"
		}
		value := label.Value
		tmpLabels = append(tmpLabels, prompbmarshal.Label{
			Name:  name,
			Value: value,
		})
	}

	if *usePromCompatibleNaming {
		// Replace unsupported Prometheus chars in label names and metric names with underscores.
		for i := range tmpLabels {
			label := &tmpLabels[i]
			if label.Name == "__name__" {
				label.Value = promrelabel.SanitizeMetricName(label.Value)
			} else {
				label.Name = promrelabel.SanitizeLabelName(label.Name)
			}
		}
	}

	if pcs.Len() > 0 {
		// Apply relabeling
		tmpLabels = pcs.Apply(tmpLabels, 0)
		tmpLabels = promrelabel.FinalizeLabels(tmpLabels[:0], tmpLabels)
		if len(tmpLabels) == 0 {
			metricsDropped.Inc()
		}
	}

	ctx.tmpLabels = tmpLabels

	// Return back labels to the desired format.
	dst := labels[:0]
	for _, label := range tmpLabels {
		name := label.Name
		if label.Name == "__name__" {
			name = ""
		}
		value := label.Value
		dst = append(dst, prompb.Label{
			Name:  name,
			Value: value,
		})
	}
	return dst
}

var metricsDropped = metrics.NewCounter(`vm_relabel_metrics_dropped_total`)
