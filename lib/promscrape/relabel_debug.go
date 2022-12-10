package promscrape

import (
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

// WriteMetricRelabelDebug serves requests to /metric-relabel-debug page
func WriteMetricRelabelDebug(w http.ResponseWriter, r *http.Request) {
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")

	if metric == "" {
		metric = "{}"
	}
	labels, err := promutils.NewLabelsFromString(metric)
	if err != nil {
		err = fmt.Errorf("cannot parse metric: %s", err)
		WriteMetricRelabelDebugSteps(w, nil, metric, relabelConfigs, err)
		return
	}
	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(relabelConfigs))
	if err != nil {
		err = fmt.Errorf("cannot parse relabel configs: %s", err)
		WriteMetricRelabelDebugSteps(w, nil, metric, relabelConfigs, err)
		return
	}

	dss := newDebugRelabelSteps(pcs, labels, false)
	WriteMetricRelabelDebugSteps(w, dss, metric, relabelConfigs, nil)
}

// WriteTargetRelabelDebug generates response for /target-relabel-debug page
func WriteTargetRelabelDebug(w http.ResponseWriter, r *http.Request) {
	targetID := r.FormValue("id")
	metric := r.FormValue("metric")
	relabelConfigs := r.FormValue("relabel_configs")

	if metric == "" && relabelConfigs == "" {
		if targetID == "" {
			metric = "{}"
			WriteTargetRelabelDebugSteps(w, targetID, nil, metric, relabelConfigs, nil)
			return
		}
		pcs, labels, ok := getRelabelContextByTargetID(targetID)
		if !ok {
			err := fmt.Errorf("cannot find target for id=%s", targetID)
			targetID = ""
			WriteTargetRelabelDebugSteps(w, targetID, nil, metric, relabelConfigs, err)
			return
		}
		metric = labels.String()
		relabelConfigs = pcs.String()
		dss := newDebugRelabelSteps(pcs, labels, true)
		WriteTargetRelabelDebugSteps(w, targetID, dss, metric, relabelConfigs, nil)
		return
	}

	if metric == "" {
		metric = "{}"
	}
	labels, err := promutils.NewLabelsFromString(metric)
	if err != nil {
		err = fmt.Errorf("cannot parse metric: %s", err)
		WriteTargetRelabelDebugSteps(w, targetID, nil, metric, relabelConfigs, err)
		return
	}
	pcs, err := promrelabel.ParseRelabelConfigsData([]byte(relabelConfigs))
	if err != nil {
		err = fmt.Errorf("cannot parse relabel configs: %s", err)
		WriteTargetRelabelDebugSteps(w, targetID, nil, metric, relabelConfigs, err)
		return
	}
	dss := newDebugRelabelSteps(pcs, labels, true)
	WriteTargetRelabelDebugSteps(w, targetID, dss, metric, relabelConfigs, nil)
}

func newDebugRelabelSteps(pcs *promrelabel.ParsedConfigs, labels *promutils.Labels, isTargetRelabel bool) []promrelabel.DebugStep {
	// The target relabeling below must be in sync with the code at scrapeWorkConfig.getScrapeWork if isTragetRelabeling=true
	// and with the code at scrapeWork.addRowToTimeseries when isTargetRelabeling=false

	// Prevent from modifying the original labels
	labels = labels.Clone()

	// Apply relabeling
	labelsResult, dss := pcs.ApplyDebug(labels.GetLabels())
	labels.Labels = labelsResult
	outStr := promrelabel.LabelsToString(labels.GetLabels())

	// Add missing instance label
	if isTargetRelabel && labels.Get("instance") == "" {
		address := labels.Get("__address__")
		if address != "" {
			inStr := outStr
			labels.Add("instance", address)
			outStr = promrelabel.LabelsToString(labels.GetLabels())
			dss = append(dss, promrelabel.DebugStep{
				Rule: "add missing instance label from __address__ label",
				In:   inStr,
				Out:  outStr,
			})
		}
	}

	// Remove labels with __ prefix
	inStr := outStr
	labels.RemoveLabelsWithDoubleUnderscorePrefix()
	outStr = promrelabel.LabelsToString(labels.GetLabels())
	if inStr != outStr {
		dss = append(dss, promrelabel.DebugStep{
			Rule: "remove labels with __ prefix",
			In:   inStr,
			Out:  outStr,
		})
	}

	// There is no need in labels' sorting, since promrelabel.LabelsToString() automatically sorts labels.
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
