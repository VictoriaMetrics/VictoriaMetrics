package remotewrite

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

var (
	unparsedLabelsGlobal = flagutil.NewArrayString("remoteWrite.label", "Optional label in the form 'name=value' to add to all the metrics before sending them to -remoteWrite.url. "+
		"Pass multiple -remoteWrite.label flags in order to add multiple labels to metrics before sending them to remote storage")
	relabelConfigPathGlobal = flag.String("remoteWrite.relabelConfig", "", "Optional path to file with relabeling configs, which are applied "+
		"to all the metrics before sending them to -remoteWrite.url. See also -remoteWrite.urlRelabelConfig. "+
		"The path can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/victoriametrics/relabeling/")
	relabelConfigPaths = flagutil.NewArrayString("remoteWrite.urlRelabelConfig", "Optional path to relabel configs for the corresponding -remoteWrite.url. "+
		"See also -remoteWrite.relabelConfig. The path can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/victoriametrics/relabeling/")

	usePromCompatibleNaming = flag.Bool("usePromCompatibleNaming", false, "Whether to replace characters unsupported by Prometheus with underscores "+
		"in the ingested metric names and label names. For example, foo.bar{a.b='c'} is transformed into foo_bar{a_b='c'} during data ingestion if this flag is set. "+
		"See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels")
)

var (
	labelsGlobal []prompb.Label

	remoteWriteRelabelConfigData    atomic.Pointer[[]byte]
	remoteWriteURLRelabelConfigData atomic.Pointer[[]interface{}]

	relabelConfigReloads      *metrics.Counter
	relabelConfigReloadErrors *metrics.Counter
	relabelConfigSuccess      *metrics.Gauge
	relabelConfigTimestamp    *metrics.Counter
)

func initRelabelMetrics() {
	relabelConfigReloads = metrics.NewCounter(`vmagent_relabel_config_reloads_total`)
	relabelConfigReloadErrors = metrics.NewCounter(`vmagent_relabel_config_reloads_errors_total`)
	relabelConfigSuccess = metrics.NewGauge(`vmagent_relabel_config_last_reload_successful`, nil)
	relabelConfigTimestamp = metrics.NewCounter(`vmagent_relabel_config_last_reload_success_timestamp_seconds`)
}

// CheckRelabelConfigs checks -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig.
func CheckRelabelConfigs() error {
	_, err := loadRelabelConfigs()
	return err
}

func initRelabelConfigs() {
	rcs, err := loadRelabelConfigs()
	if err != nil {
		logger.Fatalf("cannot initialize relabel configs: %s", err)
	}
	allRelabelConfigs.Store(rcs)
	if rcs.isSet() {
		initRelabelMetrics()
		relabelConfigSuccess.Set(1)
		relabelConfigTimestamp.Set(fasttime.UnixTimestamp())
	}
}

// WriteRelabelConfigData writes -remoteWrite.relabelConfig contents to w
func WriteRelabelConfigData(w io.Writer) {
	p := remoteWriteRelabelConfigData.Load()
	if p == nil {
		// Nothing to write to w
		return
	}
	_, _ = w.Write(*p)
}

// WriteURLRelabelConfigData writes -remoteWrite.urlRelabelConfig contents to w
func WriteURLRelabelConfigData(w io.Writer) {
	p := remoteWriteURLRelabelConfigData.Load()
	if p == nil {
		// Nothing to write to w
		return
	}
	type urlRelabelCfg struct {
		Url           string      `yaml:"url"`
		RelabelConfig interface{} `yaml:"relabel_config"`
	}
	var cs []urlRelabelCfg
	for i, url := range *remoteWriteURLs {
		cfgData := (*p)[i]
		if !*showRemoteWriteURL {
			url = fmt.Sprintf("%d:secret-url", i+1)
		}
		cs = append(cs, urlRelabelCfg{
			Url:           url,
			RelabelConfig: cfgData,
		})
	}
	d, _ := yaml.Marshal(cs)
	_, _ = w.Write(d)
}

func reloadRelabelConfigs() {
	rcs := allRelabelConfigs.Load()
	if !rcs.isSet() {
		return
	}
	relabelConfigReloads.Inc()
	logger.Infof("reloading relabel configs pointed by -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig")
	rcs, err := loadRelabelConfigs()
	if err != nil {
		relabelConfigReloadErrors.Inc()
		relabelConfigSuccess.Set(0)
		logger.Errorf("cannot reload relabel configs; preserving the previous configs; error: %s", err)
		return
	}
	allRelabelConfigs.Store(rcs)
	relabelConfigSuccess.Set(1)
	relabelConfigTimestamp.Set(fasttime.UnixTimestamp())
	logger.Infof("successfully reloaded relabel configs")
}

func loadRelabelConfigs() (*relabelConfigs, error) {
	var rcs relabelConfigs
	if *relabelConfigPathGlobal != "" {
		global, rawCfg, err := promrelabel.LoadRelabelConfigs(*relabelConfigPathGlobal)
		if err != nil {
			return nil, fmt.Errorf("cannot load -remoteWrite.relabelConfig=%q: %w", *relabelConfigPathGlobal, err)
		}
		remoteWriteRelabelConfigData.Store(&rawCfg)
		rcs.global = global
	}

	if len(*relabelConfigPaths) > len(*remoteWriteURLs) {
		return nil, fmt.Errorf("too many -remoteWrite.urlRelabelConfig args: %d; it mustn't exceed the number of -remoteWrite.url args: %d",
			len(*relabelConfigPaths), (len(*remoteWriteURLs)))
	}

	var urlRelabelCfgs []interface{}
	rcs.perURL = make([]*promrelabel.ParsedConfigs, len(*remoteWriteURLs))
	for i, path := range *relabelConfigPaths {
		if len(path) == 0 {
			urlRelabelCfgs = append(urlRelabelCfgs, nil)
			continue
		}
		prc, rawCfg, err := promrelabel.LoadRelabelConfigs(path)
		if err != nil {
			return nil, fmt.Errorf("cannot load relabel configs from -remoteWrite.urlRelabelConfig=%q: %w", path, err)
		}
		rcs.perURL[i] = prc

		var parsedCfg interface{}
		_ = yaml.Unmarshal(rawCfg, &parsedCfg)
		urlRelabelCfgs = append(urlRelabelCfgs, parsedCfg)
	}
	if len(*remoteWriteURLs) > len(*relabelConfigPaths) {
		// fill the urlRelabelCfgs with empty relabel configs if not set
		for i := len(*relabelConfigPaths); i < len(*remoteWriteURLs); i++ {
			urlRelabelCfgs = append(urlRelabelCfgs, nil)
		}
	}
	remoteWriteURLRelabelConfigData.Store(&urlRelabelCfgs)
	return &rcs, nil
}

type relabelConfigs struct {
	global *promrelabel.ParsedConfigs
	perURL []*promrelabel.ParsedConfigs
}

// isSet indicates whether (global or per-URL) command-line flags is set
func (rcs *relabelConfigs) isSet() bool {
	return *relabelConfigPathGlobal != "" || len(*relabelConfigPaths) > 0
}

// initLabelsGlobal must be called after parsing command-line flags.
func initLabelsGlobal() {
	labelsGlobal = nil
	for _, s := range *unparsedLabelsGlobal {
		if len(s) == 0 {
			continue
		}
		n := strings.IndexByte(s, '=')
		if n < 0 {
			logger.Fatalf("missing '=' in `-remoteWrite.label`. It must contain label in the form `name=value`; got %q", s)
		}
		labelsGlobal = append(labelsGlobal, prompb.Label{
			Name:  s[:n],
			Value: s[n+1:],
		})
	}
}

func (rctx *relabelCtx) applyRelabeling(tss []prompb.TimeSeries, pcs *promrelabel.ParsedConfigs) []prompb.TimeSeries {
	if pcs.Len() == 0 && !*usePromCompatibleNaming {
		// Nothing to change.
		return tss
	}
	rctx.reset()
	tssDst := tss[:0]
	labels := rctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		labels = pcs.Apply(labels, labelsLen)
		labels = promrelabel.FinalizeLabels(labels[:labelsLen], labels[labelsLen:])
		if len(labels) == labelsLen {
			// Drop the current time series, since relabeling removed all the labels.
			continue
		}
		if *usePromCompatibleNaming {
			fixPromCompatibleNaming(labels[labelsLen:])
		}
		tssDst = append(tssDst, prompb.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: ts.Samples,
		})
	}
	rctx.labels = labels
	return tssDst
}

func (rctx *relabelCtx) appendExtraLabels(tss []prompb.TimeSeries, extraLabels []prompb.Label) {
	if len(extraLabels) == 0 {
		return
	}
	rctx.reset()
	labels := rctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labelsLen := len(labels)
		labels = append(labels, ts.Labels...)
		for j := range extraLabels {
			extraLabel := extraLabels[j]
			tmp := promrelabel.GetLabelByName(labels[labelsLen:], extraLabel.Name)
			if tmp != nil {
				tmp.Value = extraLabel.Value
			} else {
				labels = append(labels, extraLabel)
			}
		}
		ts.Labels = labels[labelsLen:]
	}
	rctx.labels = labels
}

func (rctx *relabelCtx) tenantToLabels(tss []prompb.TimeSeries, accountID, projectID uint32) {
	rctx.reset()
	accountIDStr := strconv.FormatUint(uint64(accountID), 10)
	projectIDStr := strconv.FormatUint(uint64(projectID), 10)
	labels := rctx.labels[:0]
	for i := range tss {
		ts := &tss[i]
		labelsLen := len(labels)
		for _, label := range ts.Labels {
			labelName := label.Name
			if labelName == "vm_account_id" || labelName == "vm_project_id" {
				continue
			}
			labels = append(labels, label)
		}
		labels = append(labels, prompb.Label{
			Name:  "vm_account_id",
			Value: accountIDStr,
		})
		labels = append(labels, prompb.Label{
			Name:  "vm_project_id",
			Value: projectIDStr,
		})
		ts.Labels = labels[labelsLen:]
	}
	rctx.labels = labels
}

type relabelCtx struct {
	// pool for labels, which are used during the relabeling.
	labels []prompb.Label
}

func (rctx *relabelCtx) reset() {
	promrelabel.CleanLabels(rctx.labels)
	rctx.labels = rctx.labels[:0]
}

var relabelCtxPool = &sync.Pool{
	New: func() any {
		return &relabelCtx{}
	},
}

func getRelabelCtx() *relabelCtx {
	return relabelCtxPool.Get().(*relabelCtx)
}

func putRelabelCtx(rctx *relabelCtx) {
	rctx.reset()
	relabelCtxPool.Put(rctx)
}

func fixPromCompatibleNaming(labels []prompb.Label) {
	// Replace unsupported Prometheus chars in label names and metric names with underscores.
	for i := range labels {
		label := &labels[i]
		if label.Name == "__name__" {
			label.Value = promrelabel.SanitizeMetricName(label.Value)
		} else {
			label.Name = promrelabel.SanitizeLabelName(label.Name)
		}
	}
}
