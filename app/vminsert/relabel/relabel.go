package relabel

import (
	"flag"
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
	relabelDebug = flag.Bool("relabelDebug", false, "Whether to log metrics before and after relabeling with -relabelConfig. If the -relabelDebug is enabled, "+
		"then the metrics aren't sent to storage. This is useful for debugging the relabeling configs")
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
	if len(*relabelConfig) == 0 {
		return
	}
	go func() {
		for range sighupCh {
			logger.Infof("received SIGHUP; reloading -relabelConfig=%q...", *relabelConfig)
			pcs, err := loadRelabelConfig()
			if err != nil {
				logger.Errorf("cannot load the updated relabelConfig: %s; preserving the previous config", err)
				continue
			}
			pcsGlobal.Store(pcs)
			logger.Infof("successfully reloaded -relabelConfig=%q", *relabelConfig)
		}
	}()
}

var pcsGlobal atomic.Value

func loadRelabelConfig() (*promrelabel.ParsedConfigs, error) {
	if len(*relabelConfig) == 0 {
		return nil, nil
	}
	pcs, err := promrelabel.LoadRelabelConfigs(*relabelConfig, *relabelDebug)
	if err != nil {
		return nil, fmt.Errorf("error when reading -relabelConfig=%q: %w", *relabelConfig, err)
	}
	return pcs, nil
}

// HasRelabeling returns true if there is global relabeling.
func HasRelabeling() bool {
	pcs := pcsGlobal.Load().(*promrelabel.ParsedConfigs)
	return pcs.Len() > 0
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
	pcs := pcsGlobal.Load().(*promrelabel.ParsedConfigs)
	if pcs.Len() == 0 {
		// There are no relabeling rules.
		return labels
	}
	// Convert src to prompbmarshal.Label format suitable for relabeling.
	tmpLabels := ctx.tmpLabels[:0]
	for _, label := range labels {
		name := bytesutil.ToUnsafeString(label.Name)
		if len(name) == 0 {
			name = "__name__"
		}
		value := bytesutil.ToUnsafeString(label.Value)
		tmpLabels = append(tmpLabels, prompbmarshal.Label{
			Name:  name,
			Value: value,
		})
	}

	// Apply relabeling
	tmpLabels = pcs.Apply(tmpLabels, 0, true)
	ctx.tmpLabels = tmpLabels
	if len(tmpLabels) == 0 {
		metricsDropped.Inc()
	}

	// Return back labels to the desired format.
	dst := labels[:0]
	for _, label := range tmpLabels {
		name := bytesutil.ToUnsafeBytes(label.Name)
		if label.Name == "__name__" {
			name = nil
		}
		value := bytesutil.ToUnsafeBytes(label.Value)
		dst = append(dst, prompb.Label{
			Name:  name,
			Value: value,
		})
	}
	return dst
}

var metricsDropped = metrics.NewCounter(`vm_relabel_metrics_dropped_total`)
