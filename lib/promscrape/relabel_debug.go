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

	// The metric relabeling below must be in sync with the code at scrapeWork.addRowToTimeseries

	// Apply relabeling
	labelsResult, dss := pcs.ApplyDebug(labels.GetLabels())

	// Remove labels with __ prefix
	inStr := promrelabel.LabelsToString(labelsResult)
	labelsResult = promrelabel.FinalizeLabels(labelsResult[:0], labelsResult)
	outStr := promrelabel.LabelsToString(labelsResult)
	if inStr != outStr {
		dss = append(dss, promrelabel.DebugStep{
			Rule: "remove labels with __ prefix",
			In:   inStr,
			Out:  outStr,
		})
	}

	// There is no need in labels' sorting, since promrelabel.LabelsToString() automatically sorts labels.

	WriteMetricRelabelDebugSteps(w, dss, metric, relabelConfigs, nil)
}

// WriteTargetRelabelDebug generates response for /target-relabel-debug page
func WriteTargetRelabelDebug(w http.ResponseWriter, r *http.Request) error {
	targetID := r.FormValue("id")
	relabelConfigs, labels, ok := getRelabelContextByTargetID(targetID)
	if !ok {
		return fmt.Errorf("cannot find target for id=%s", targetID)
	}

	// The target relabeling below must be in sync with the code at scrapeWorkConfig.getScrapeWork

	// Prevent from modifying the original labels
	labels = labels.Clone()

	// Apply relabeling
	labelsResult, dss := relabelConfigs.ApplyDebug(labels.GetLabels())

	// Remove labels with __meta_ prefix
	inStr := promrelabel.LabelsToString(labelsResult)
	labels.Labels = labelsResult
	labels.RemoveMetaLabels()
	outStr := promrelabel.LabelsToString(labels.Labels)
	if inStr != outStr {
		dss = append(dss, promrelabel.DebugStep{
			Rule: "remove labels with __meta_ prefix",
			In:   inStr,
			Out:  outStr,
		})
	}

	// Add missing instance label
	if labels.Get("instance") == "" {
		address := labels.Get("__address__")
		if address != "" {
			inStr = outStr
			labels.Add("instance", address)
			outStr = promrelabel.LabelsToString(labels.Labels)
			dss = append(dss, promrelabel.DebugStep{
				Rule: "add missing instance label from __address__ label",
				In:   inStr,
				Out:  outStr,
			})
		}
	}

	// Remove labels with __ prefix
	inStr = outStr
	labels.RemoveLabelsWithDoubleUnderscorePrefix()
	outStr = promrelabel.LabelsToString(labels.Labels)
	if inStr != outStr {
		dss = append(dss, promrelabel.DebugStep{
			Rule: "remove labels with __ prefix",
			In:   inStr,
			Out:  outStr,
		})
	}

	// There is no need in labels' sorting, since promrelabel.LabelsToString() automatically sorts labels.

	WriteTargetRelabelDebugSteps(w, dss)
	return nil
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
