package remotewrite

import (
	"flag"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var (
	extraLabelsUnparsed = flagutil.NewArray("remoteWrite.label", "Optional label in the form 'name=value' to add to all the metrics before sending them to -remoteWrite.url. "+
		"Pass multiple -remoteWrite.label flags in order to add multiple flags to metrics before sending them to remote storage")
	relabelConfigPath = flag.String("remoteWrite.relabelConfig", "", "Optional path to file with relabel_config entries. These entries are applied to all the metrics "+
		"before sending them to -remoteWrite.url. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config for details")
)

var extraLabels []prompbmarshal.Label
var prcs []promrelabel.ParsedRelabelConfig

// initRelabel must be called after parsing command-line flags.
func initRelabel() {
	// Init extraLabels
	for _, s := range *extraLabelsUnparsed {
		n := strings.IndexByte(s, '=')
		if n < 0 {
			logger.Panicf("FATAL: missing '=' in `-remoteWrite.label`. It must contain label in the form `name=value`; got %q", s)
		}
		extraLabels = append(extraLabels, prompbmarshal.Label{
			Name:  s[:n],
			Value: s[n+1:],
		})
	}

	// Init prcs
	if len(*relabelConfigPath) > 0 {
		var err error
		prcs, err = promrelabel.LoadRelabelConfigs(*relabelConfigPath)
		if err != nil {
			logger.Panicf("FATAL: cannot load relabel configs from -remoteWrite.relabelConfig=%q: %s", *relabelConfigPath, err)
		}
	}
}

func resetRelabel() {
	extraLabels = nil
	prcs = nil
}

func (rctx *relabelCtx) applyRelabeling(tss []prompbmarshal.TimeSeries) []prompbmarshal.TimeSeries {
	if len(extraLabels) == 0 && len(prcs) == 0 {
		// Nothing to change.
		return tss
	}
	tssDst := tss[:0]
	labels := rctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		// extraLabels must be added before applying relabeling according to https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write
		for j := range extraLabels {
			extraLabel := &extraLabels[j]
			tmp := promrelabel.GetLabelByName(labels[labelsLen:], extraLabel.Name)
			if tmp != nil {
				tmp.Value = extraLabel.Value
			} else {
				labels = append(labels, *extraLabel)
			}
		}
		labels = promrelabel.ApplyRelabelConfigs(labels, labelsLen, prcs, true)
		if len(labels) == labelsLen {
			// Drop the current time series, since relabeling removed all the labels.
			continue
		}
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: ts.Samples,
		})
	}
	rctx.labels = labels
	return tssDst
}

type relabelCtx struct {
	// pool for labels, which are used during the relabeling.
	labels []prompbmarshal.Label
}

func (rctx *relabelCtx) reset() {
	labels := rctx.labels
	for i := range labels {
		label := &labels[i]
		label.Name = ""
		label.Value = ""
	}
	rctx.labels = rctx.labels[:0]
}

var relabelCtxPool = &sync.Pool{
	New: func() interface{} {
		return &relabelCtx{}
	},
}
