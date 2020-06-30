package remotewrite

import (
	"flag"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var (
	unparsedLabelsGlobal = flagutil.NewArray("remoteWrite.label", "Optional label in the form 'name=value' to add to all the metrics before sending them to -remoteWrite.url. "+
		"Pass multiple -remoteWrite.label flags in order to add multiple flags to metrics before sending them to remote storage")
	relabelConfigPathGlobal = flag.String("remoteWrite.relabelConfig", "", "Optional path to file with relabel_config entries. These entries are applied to all the metrics "+
		"before sending them to -remoteWrite.url. See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config for details")
	relabelConfigPaths = flagutil.NewArray("remoteWrite.urlRelabelConfig", "Optional path to relabel config for the corresponding -remoteWrite.url")
)

var labelsGlobal []prompbmarshal.Label

// CheckRelabelConfigs checks -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig.
func CheckRelabelConfigs() error {
	_, err := loadRelabelConfigs()
	return err
}

func loadRelabelConfigs() (*relabelConfigs, error) {
	var rcs relabelConfigs
	if *relabelConfigPathGlobal != "" {
		global, err := promrelabel.LoadRelabelConfigs(*relabelConfigPathGlobal)
		if err != nil {
			return nil, fmt.Errorf("cannot load -remoteWrite.relabelConfig=%q: %w", *relabelConfigPathGlobal, err)
		}
		rcs.global = global
	}
	if len(*relabelConfigPaths) > len(*remoteWriteURLs) {
		return nil, fmt.Errorf("too many -remoteWrite.urlRelabelConfig args: %d; it mustn't exceed the number of -remoteWrite.url args: %d",
			len(*relabelConfigPaths), len(*remoteWriteURLs))
	}
	rcs.perURL = make([][]promrelabel.ParsedRelabelConfig, len(*remoteWriteURLs))
	for i, path := range *relabelConfigPaths {
		prc, err := promrelabel.LoadRelabelConfigs(path)
		if err != nil {
			return nil, fmt.Errorf("cannot load relabel configs from -remoteWrite.urlRelabelConfig=%q: %w", path, err)
		}
		rcs.perURL[i] = prc
	}
	return &rcs, nil
}

type relabelConfigs struct {
	global []promrelabel.ParsedRelabelConfig
	perURL [][]promrelabel.ParsedRelabelConfig
}

// initLabelsGlobal must be called after parsing command-line flags.
func initLabelsGlobal() {
	// Init labelsGlobal
	labelsGlobal = nil
	for _, s := range *unparsedLabelsGlobal {
		n := strings.IndexByte(s, '=')
		if n < 0 {
			logger.Fatalf("missing '=' in `-remoteWrite.label`. It must contain label in the form `name=value`; got %q", s)
		}
		labelsGlobal = append(labelsGlobal, prompbmarshal.Label{
			Name:  s[:n],
			Value: s[n+1:],
		})
	}
}

func (rctx *relabelCtx) applyRelabeling(tss []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label, prcs []promrelabel.ParsedRelabelConfig) []prompbmarshal.TimeSeries {
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

func getRelabelCtx() *relabelCtx {
	return relabelCtxPool.Get().(*relabelCtx)
}

func putRelabelCtx(rctx *relabelCtx) {
	rctx.labels = rctx.labels[:0]
	relabelCtxPool.Put(rctx)
}
