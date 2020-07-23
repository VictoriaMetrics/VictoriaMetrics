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

var relabelConfig = flag.String("relabelConfig", "", "Optional path to a file with relabeling rules, which are applied to all the ingested metrics. "+
	"See https://victoriametrics.github.io/#relabeling for details")

// Init must be called after flag.Parse and before using the relabel package.
func Init() {
	prcs, err := loadRelabelConfig()
	if err != nil {
		logger.Fatalf("cannot load relabelConfig: %s", err)
	}
	prcsGlobal.Store(&prcs)
	if len(*relabelConfig) == 0 {
		return
	}
	sighupCh := procutil.NewSighupChan()
	go func() {
		for range sighupCh {
			logger.Infof("received SIGHUP; reloading -relabelConfig=%q...", *relabelConfig)
			prcs, err := loadRelabelConfig()
			if err != nil {
				logger.Errorf("cannot load the updated relabelConfig: %s; preserving the previous config", err)
				continue
			}
			prcsGlobal.Store(&prcs)
			logger.Infof("successfully reloaded -relabelConfig=%q", *relabelConfig)
		}
	}()
}

var prcsGlobal atomic.Value

func loadRelabelConfig() ([]promrelabel.ParsedRelabelConfig, error) {
	if len(*relabelConfig) == 0 {
		return nil, nil
	}
	prcs, err := promrelabel.LoadRelabelConfigs(*relabelConfig)
	if err != nil {
		return nil, fmt.Errorf("error when reading -relabelConfig=%q: %w", *relabelConfig, err)
	}
	return prcs, nil
}

// HasRelabeling returns true if there is global relabeling.
func HasRelabeling() bool {
	prcs := prcsGlobal.Load().(*[]promrelabel.ParsedRelabelConfig)
	return len(*prcs) > 0
}

// Ctx holds relabeling context.
type Ctx struct {
	// tmpLabels is used during ApplyRelabeling call.
	tmpLabels []prompbmarshal.Label
}

// Reset resets ctx.
func (ctx *Ctx) Reset() {
	labels := ctx.tmpLabels
	for i := range labels {
		label := &labels[i]
		label.Name = ""
		label.Value = ""
	}
	ctx.tmpLabels = ctx.tmpLabels[:0]
}

// ApplyRelabeling applies relabeling to the given labels and returns the result.
//
// The returned labels are valid until the next call to ApplyRelabeling.
func (ctx *Ctx) ApplyRelabeling(labels []prompb.Label) []prompb.Label {
	prcs := prcsGlobal.Load().(*[]promrelabel.ParsedRelabelConfig)
	if len(*prcs) == 0 {
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
	tmpLabels = promrelabel.ApplyRelabelConfigs(tmpLabels, 0, *prcs, true)
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
