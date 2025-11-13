package promrelabel

import (
	"fmt"
	"io"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// WriteMetricRelabelDebug writes /metric-relabel-debug page to w with the corresponding args.
func WriteMetricRelabelDebug(w io.Writer, targetID, metric, relabelConfigs, format string, err error) {
	writeRelabelDebug(w, false, targetID, metric, relabelConfigs, format, err)
}

// WriteTargetRelabelDebug writes /target-relabel-debug page to w with the corresponding args.
func WriteTargetRelabelDebug(w io.Writer, targetID, metric, relabelConfigs, format string, err error) {
	writeRelabelDebug(w, true, targetID, metric, relabelConfigs, format, err)
}

func writeRelabelDebug(w io.Writer, isTargetRelabel bool, targetID, metric, relabelConfigs, format string, err error) {
	if metric == "" {
		metric = "{}"
	}
	targetURL := ""
	if err != nil {
		WriteRelabelDebugSteps(w, targetURL, targetID, format, nil, metric, relabelConfigs, err)
		return
	}

	metric, err = normalizeInputLabels(metric)
	if err != nil {
		err = fmt.Errorf("cannot parse metric: %w", err)
		WriteRelabelDebugSteps(w, targetURL, targetID, format, nil, metric, relabelConfigs, err)
		return
	}

	labels, err := promutil.NewLabelsFromString(metric)
	if err != nil {
		err = fmt.Errorf("cannot parse metric: %w", err)
		WriteRelabelDebugSteps(w, targetURL, targetID, format, nil, metric, relabelConfigs, err)
		return
	}
	pcs, err := ParseRelabelConfigsData([]byte(relabelConfigs))
	if err != nil {
		err = fmt.Errorf("cannot parse relabel configs: %w", err)
		WriteRelabelDebugSteps(w, targetURL, targetID, format, nil, metric, relabelConfigs, err)
		return
	}

	dss, targetURL := newDebugRelabelSteps(pcs, labels, isTargetRelabel)
	WriteRelabelDebugSteps(w, targetURL, targetID, format, dss, metric, relabelConfigs, nil)
}

func newDebugRelabelSteps(pcs *ParsedConfigs, labels *promutil.Labels, isTargetRelabel bool) ([]DebugStep, string) {
	// The target relabeling below must be in sync with the code at scrapeWorkConfig.getScrapeWork if isTargetRelabel=true
	// and with the code at scrapeWork.addRowToTimeseries when isTargetRelabeling=false
	targetURL := ""

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

		// Generate targetURL
		targetURL, _ = GetScrapeURL(labels, nil)

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
	return dss, targetURL
}

func getChangedLabelNames(in, out *promutil.Labels) map[string]struct{} {
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

// normalizeInputLabels does two things:
// 1. check if the input is (not) surrounded by braces. Inputs with or without braces are valid, but unclosed braces are not allowed.
// 2. add missing `{` and `}` to the input if needed.
//
// it does not handle complex edge cases like `{` or `}` appear multiple times. they're invalid and will not pass `NewLabelsFromString`.
func normalizeInputLabels(metric string) (string, error) {
	metric = strings.TrimSpace(metric)

	openBrace := strings.Contains(metric, `{`)
	closeBrace := strings.Contains(metric, `}`)

	if openBrace != closeBrace {
		// only either `{` or `}` exist, this must be an invalid expression.
		return "", fmt.Errorf("cannot unmarshal Prometheus line %q", metric)
	}

	if openBrace && closeBrace {
		return metric, nil
	}

	if strings.Contains(metric, `=`) {
		// special case for input like:
		// 1. __name__=metric_name, label1=value1, ...
		// 2. label1=value1, ...
		// 3. __name__=metric_name
		// add curly braces to turn it into a more common format that `NewLabelsFromString` can handle.
		//
		// see: https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8584 and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9900
		metric = `{` + metric + `}`
	}

	return metric, nil
}
