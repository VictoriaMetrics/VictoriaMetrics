package remotewrite

import (
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/metrics"
)

var (
	unparsedLabelsGlobal = flagutil.NewArrayString("remoteWrite.label", "Optional label in the form 'name=value' to add to all the metrics before sending them to -remoteWrite.url. "+
		"Pass multiple -remoteWrite.label flags in order to add multiple labels to metrics before sending them to remote storage")
	relabelConfigPathGlobal = flag.String("remoteWrite.relabelConfig", "", "Optional path to file with relabeling configs, which are applied "+
		"to all the metrics before sending them to -remoteWrite.url. See also -remoteWrite.urlRelabelConfig. "+
		"The path can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/vmagent/#relabeling")
	relabelConfigPaths = flagutil.NewDictValue("remoteWrite.urlRelabelConfig", "", '/', "Optional path to relabel configs for the corresponding -remoteWrite.url. "+
		"See also -remoteWrite.relabelConfig. The path can point either to local file or to http url. "+
		"See https://docs.victoriametrics.com/vmagent/#relabeling")

	usePromCompatibleNaming = flag.Bool("usePromCompatibleNaming", false, "Whether to replace characters unsupported by Prometheus with underscores "+
		"in the ingested metric names and label names. For example, foo.bar{a.b='c'} is transformed into foo_bar{a_b='c'} during data ingestion if this flag is set. "+
		"See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels")
)

var labelsGlobal []prompbmarshal.Label

// CheckRelabelConfigs checks -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig.
func CheckRelabelConfigs() error {
	rcs := &relabelConfigs{}
	if err := rcs.loadRelabelConfigs(); err != nil {
		return err
	}
	return nil
}

type relabelConfigs struct {
	global   atomic.Pointer[promrelabel.ParsedConfigs]
	perGroup []atomic.Pointer[promrelabel.ParsedConfigs]
	perCtx   []atomic.Pointer[promrelabel.ParsedConfigs]
}

func (rcs *relabelConfigs) reloadConfigs() {
	logger.Infof("reloading relabel configs pointed by -remoteWrite.relabelConfig and -remoteWrite.urlRelabelConfig")
	if err := rcs.loadRelabelConfigs(); err != nil {
		logger.Errorf("partually updated relabel configs; error: %s", err)
		return
	}
	logger.Infof("successfully reloaded relabel configs")
}

func (rcs *relabelConfigs) loadRelabelConfigs() error {
	if err := rcs.loadInternalRelabelConfigs(); err != nil {
		return err
	}
	return nil
}

func updateRelabelConfig(rc *atomic.Pointer[promrelabel.ParsedConfigs], path, group, flag string) error {
	if path != "" {
		if group != "" {
			flag = fmt.Sprintf(`-%s="%s/%s"`, flag, group, path)
		} else {
			flag = fmt.Sprintf("-%s=%q", flag, path)
		}
		metricLabels := fmt.Sprintf("path=%q", path)
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_relabel_config_reloads_total{%s}", metricLabels)).Inc()
		rcs, err := promrelabel.LoadRelabelConfigs(path)
		if err != nil {
			metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_relabel_config_reloads_errors_total{%s}", metricLabels)).Inc()
			metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_relabel_config_last_reload_successful{%s}", metricLabels)).Set(0)
			return fmt.Errorf("cannot load -%s: %w", flag, err)
		}
		rc.Store(rcs)
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_relabel_config_last_reload_successful{%s}", metricLabels)).Set(1)
		metrics.GetOrCreateCounter(fmt.Sprintf("vmagent_relabel_config_last_reload_success_timestamp_seconds{%s}", metricLabels)).Set(fasttime.UnixTimestamp())
	}
	return nil
}

func (rcs *relabelConfigs) loadInternalRelabelConfigs() error {
	flag := "remoteWrite.relabelConfig"
	if err := updateRelabelConfig(&rcs.global, *relabelConfigPathGlobal, "", flag); err != nil {
		return err
	}
	remoteWriteGroups := remoteWriteURLs.Keys()
	relabelConfigGroups := relabelConfigPaths.Keys()
	if len(rcs.perGroup) == 0 {
		rcs.perGroup = make([]atomic.Pointer[promrelabel.ParsedConfigs], len(remoteWriteGroups))
	}
	perCtxURLs := remoteWriteURLs.GetAll("")
	if len(rcs.perCtx) == 0 {
		rcs.perCtx = make([]atomic.Pointer[promrelabel.ParsedConfigs], len(perCtxURLs))
	}
	flag = "remoteWrite.urlRelabelConfig"
	for g, groupName := range relabelConfigGroups {
		if !slices.Contains(remoteWriteGroups, groupName) {
			return fmt.Errorf("no -remoteWrite.url args in group %q, which appeared in -%s", groupName, flag)
		}
		relabelConfigs := relabelConfigPaths.GetAll(groupName)
		if groupName != "" {
			if len(relabelConfigs) > 1 {
				return fmt.Errorf("no more than 1 -%s flag (defined %d) should be defined for group %q", flag, len(relabelConfigs), groupName)
			}
			path := relabelConfigPaths.GetOptionalArg(groupName, 0)
			if err := updateRelabelConfig(&rcs.perGroup[g], path, groupName, flag); err != nil {
				return err
			}
		} else {
			if len(relabelConfigs) > len(perCtxURLs) {
				return fmt.Errorf("too many ungrouped -%s args: %d; it mustn't exceed the number of -remoteWrite.url args: %d", flag,
					len(relabelConfigs), len(perCtxURLs))
			}
			for i, path := range relabelConfigs {
				if err := updateRelabelConfig(&rcs.perCtx[i], path, groupName, flag); err != nil {
					return err
				}
			}
		}
	}
	return nil
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
		labelsGlobal = append(labelsGlobal, prompbmarshal.Label{
			Name:  s[:n],
			Value: s[n+1:],
		})
	}
}

func (rctx *relabelCtx) applyRelabeling(tss []prompbmarshal.TimeSeries, pcs *promrelabel.ParsedConfigs) []prompbmarshal.TimeSeries {
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
		tssDst = append(tssDst, prompbmarshal.TimeSeries{
			Labels:  labels[labelsLen:],
			Samples: ts.Samples,
		})
	}
	rctx.labels = labels
	return tssDst
}

func (rctx *relabelCtx) appendExtraLabels(tss []prompbmarshal.TimeSeries, extraLabels []prompbmarshal.Label) {
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

func (rctx *relabelCtx) tenantToLabels(tss []prompbmarshal.TimeSeries, accountID, projectID uint32) {
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
		labels = append(labels, prompbmarshal.Label{
			Name:  "vm_account_id",
			Value: accountIDStr,
		})
		labels = append(labels, prompbmarshal.Label{
			Name:  "vm_project_id",
			Value: projectIDStr,
		})
		ts.Labels = labels[labelsLen:]
	}
	rctx.labels = labels
}

type relabelCtx struct {
	// pool for labels, which are used during the relabeling.
	labels []prompbmarshal.Label
}

func (rctx *relabelCtx) reset() {
	promrelabel.CleanLabels(rctx.labels)
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
	rctx.reset()
	relabelCtxPool.Put(rctx)
}

func fixPromCompatibleNaming(labels []prompbmarshal.Label) {
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
