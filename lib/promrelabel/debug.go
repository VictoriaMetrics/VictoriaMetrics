package promrelabel

import (
	"fmt"
	"io"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// WriteMetricRelabelDebug writes /metric-relabel-debug page to w with the corresponding args.
func WriteMetricRelabelDebug(w io.Writer, targetID, metric, relabelConfigs string, err error) {
	writeRelabelDebug(w, false, targetID, metric, relabelConfigs, err)
}

// WriteTargetRelabelDebug writes /target-relabel-debug page to w with the corresponding args.
func WriteTargetRelabelDebug(w io.Writer, targetID, metric, relabelConfigs string, err error) {
	writeRelabelDebug(w, true, targetID, metric, relabelConfigs, err)
}

func writeRelabelDebug(w io.Writer, isTargetRelabel bool, targetID, metric, relabelConfigs string, err error) {
	if metric == "" {
		metric = "{}"
	}
	if err != nil {
		WriteRelabelDebugSteps(w, isTargetRelabel, targetID, nil, metric, relabelConfigs, err)
		return
	}
	labels, err := promutils.NewLabelsFromString(metric)
	if err != nil {
		err = fmt.Errorf("cannot parse metric: %s", err)
		WriteRelabelDebugSteps(w, isTargetRelabel, targetID, nil, metric, relabelConfigs, err)
		return
	}
	pcs, err := ParseRelabelConfigsData([]byte(relabelConfigs))
	if err != nil {
		err = fmt.Errorf("cannot parse relabel configs: %s", err)
		WriteRelabelDebugSteps(w, isTargetRelabel, targetID, nil, metric, relabelConfigs, err)
		return
	}

	dss := newDebugRelabelSteps(pcs, labels, isTargetRelabel)
	WriteRelabelDebugSteps(w, isTargetRelabel, targetID, dss, metric, relabelConfigs, nil)
}

func newDebugRelabelSteps(pcs *ParsedConfigs, labels *promutils.Labels, isTargetRelabel bool) []DebugStep {
	// The target relabeling below must be in sync with the code at scrapeWorkConfig.getScrapeWork if isTragetRelabeling=true
	// and with the code at scrapeWork.addRowToTimeseries when isTargetRelabeling=false

	// Prevent from modifying the original labels
	labels = labels.Clone()

	// Apply relabeling
	labelsResult, dss := pcs.ApplyDebug(labels.GetLabels())
	labels.Labels = labelsResult
	outStr := LabelsToString(labels.GetLabels())

	if isTargetRelabel {
		// Add missing instance label
		if labels.Get("instance") == "" {
			address := labels.Get("__address__")
			if address != "" {
				inStr := outStr
				labels.Add("instance", address)
				outStr = LabelsToString(labels.GetLabels())
				dss = append(dss, DebugStep{
					Rule: "add missing instance label from __address__ label",
					In:   inStr,
					Out:  outStr,
				})
			}
		}

		// Remove labels with __ prefix
		inStr := outStr
		labels.RemoveLabelsWithDoubleUnderscorePrefix()
		outStr = LabelsToString(labels.GetLabels())
		if inStr != outStr {
			dss = append(dss, DebugStep{
				Rule: "remove labels with __ prefix",
				In:   inStr,
				Out:  outStr,
			})
		}
	} else {
		// Remove labels with __ prefix except of __name__
		inStr := outStr
		labels.Labels = FinalizeLabels(labels.Labels[:0], labels.Labels)
		outStr = LabelsToString(labels.GetLabels())
		if inStr != outStr {
			dss = append(dss, DebugStep{
				Rule: "remove labels with __ prefix except of __name__",
				In:   inStr,
				Out:  outStr,
			})
		}
	}

	// There is no need in labels' sorting, since LabelsToString() automatically sorts labels.
	return dss
}

func getChangedLabelNames(in, out *promutils.Labels) map[string]struct{} {
	inMap := in.ToMap()
	outMap := out.ToMap()
	changed := make(map[string]struct{})
	for k, v := range outMap {
		inV, ok := inMap[k]
		if !ok || inV != v {
			changed[k] = struct{}{}
		}
	}
	for k, v := range inMap {
		outV, ok := outMap[k]
		if !ok || outV != v {
			changed[k] = struct{}{}
		}
	}
	return changed
}
