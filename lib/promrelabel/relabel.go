package promrelabel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/cespare/xxhash/v2"
)

// parsedRelabelConfig contains parsed `relabel_config`.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type parsedRelabelConfig struct {
	// ruleOriginal contains the original relabeling rule for the given parsedRelabelConfig.
	ruleOriginal string

	SourceLabels  []string
	Separator     string
	TargetLabel   string
	RegexAnchored *regexp.Regexp
	Modulus       uint64
	Replacement   string
	Action        string
	If            *IfExpression

	graphiteMatchTemplate *graphiteMatchTemplate
	graphiteLabelRules    []graphiteLabelRule

	regex         *regexutil.PromRegex
	regexOriginal *regexp.Regexp

	hasCaptureGroupInTargetLabel   bool
	hasCaptureGroupInReplacement   bool
	hasLabelReferenceInReplacement bool

	stringReplacer   *bytesutil.FastStringTransformer
	submatchReplacer *bytesutil.FastStringTransformer
}

// DebugStep contains debug information about a single relabeling rule step
type DebugStep struct {
	// Rule contains string representation of the rule step
	Rule string

	// In contains the input labels before the execution of the rule step
	In string

	// Out contains the output labels after the execution of the rule step
	Out string
}

// String returns human-readable representation for ds
func (ds DebugStep) String() string {
	return fmt.Sprintf("rule=%q, in=%s, out=%s", ds.Rule, ds.In, ds.Out)
}

// String returns human-readable representation for prc.
func (prc *parsedRelabelConfig) String() string {
	return prc.ruleOriginal
}

// ApplyDebug applies pcs to labels in debug mode.
//
// It returns DebugStep list - one entry per each applied relabeling step.
func (pcs *ParsedConfigs) ApplyDebug(labels []prompbmarshal.Label) ([]prompbmarshal.Label, []DebugStep) {
	// Protect from overwriting labels between len(labels) and cap(labels) by limiting labels capacity to its length.
	labels = labels[:len(labels):len(labels)]

	inStr := LabelsToString(labels)
	var dss []DebugStep
	if pcs != nil {
		for _, prc := range pcs.prcs {
			labels = prc.apply(labels, 0)
			outStr := LabelsToString(labels)
			dss = append(dss, DebugStep{
				Rule: prc.String(),
				In:   inStr,
				Out:  outStr,
			})
			inStr = outStr
			if len(labels) == 0 {
				// All the labels have been removed.
				return labels, dss
			}
		}
	}

	labels = removeEmptyLabels(labels, 0)
	outStr := LabelsToString(labels)
	if outStr != inStr {
		dss = append(dss, DebugStep{
			Rule: "remove empty labels",
			In:   inStr,
			Out:  outStr,
		})
	}
	return labels, dss
}

// Apply applies pcs to labels starting from the labelsOffset.
//
// This function may add additional labels after the len(labels), so make sure it doesn't corrupt in-use labels
// stored between len(labels) and cap(labels).
func (pcs *ParsedConfigs) Apply(labels []prompbmarshal.Label, labelsOffset int) []prompbmarshal.Label {
	if pcs != nil {
		for _, prc := range pcs.prcs {
			labels = prc.apply(labels, labelsOffset)
			if len(labels) == labelsOffset {
				// All the labels have been removed.
				return labels
			}
		}
	}
	labels = removeEmptyLabels(labels, labelsOffset)
	return labels
}

func removeEmptyLabels(labels []prompbmarshal.Label, labelsOffset int) []prompbmarshal.Label {
	src := labels[labelsOffset:]
	needsRemoval := false
	for i := range src {
		label := &src[i]
		if label.Name == "" || label.Value == "" {
			needsRemoval = true
			break
		}
	}
	if !needsRemoval {
		return labels
	}
	dst := labels[:labelsOffset]
	for i := range src {
		label := &src[i]
		if label.Name != "" && label.Value != "" {
			dst = append(dst, *label)
		}
	}
	return dst
}

// FinalizeLabels removes labels with "__" in the beginning (except of "__name__").
func FinalizeLabels(dst, src []prompbmarshal.Label) []prompbmarshal.Label {
	for _, label := range src {
		name := label.Name
		if strings.HasPrefix(name, "__") && name != "__name__" {
			continue
		}
		dst = append(dst, label)
	}
	return dst
}

// apply applies relabeling according to prc.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
func (prc *parsedRelabelConfig) apply(labels []prompbmarshal.Label, labelsOffset int) []prompbmarshal.Label {
	src := labels[labelsOffset:]
	if !prc.If.Match(src) {
		if prc.Action == "keep" {
			// Drop the target on `if` mismatch for `action: keep`
			return labels[:labelsOffset]
		}
		// Do not apply prc actions on `if` mismatch.
		return labels
	}
	switch prc.Action {
	case "graphite":
		metricName := getLabelValue(src, "__name__")
		gm := graphiteMatchesPool.Get().(*graphiteMatches)
		var ok bool
		gm.a, ok = prc.graphiteMatchTemplate.Match(gm.a[:0], metricName)
		if !ok {
			// Fast path - name mismatch
			graphiteMatchesPool.Put(gm)
			return labels
		}
		// Slow path - extract labels from graphite metric name
		bb := relabelBufPool.Get()
		for _, gl := range prc.graphiteLabelRules {
			bb.B = gl.grt.Expand(bb.B[:0], gm.a)
			valueStr := bytesutil.InternBytes(bb.B)
			labels = setLabelValue(labels, labelsOffset, gl.targetLabel, valueStr)
		}
		relabelBufPool.Put(bb)
		graphiteMatchesPool.Put(gm)
		return labels
	case "replace":
		// Store `replacement` at `target_label` if the `regex` matches `source_labels` joined with `separator`
		replacement := prc.Replacement
		bb := relabelBufPool.Get()
		if prc.hasLabelReferenceInReplacement {
			// Fill {{labelName}} references in the replacement
			bb.B = fillLabelReferences(bb.B[:0], replacement, labels[labelsOffset:])
			replacement = bytesutil.InternBytes(bb.B)
		}
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		if prc.RegexAnchored == defaultRegexForRelabelConfig && !prc.hasCaptureGroupInTargetLabel {
			if replacement == "$1" {
				// Fast path for the rule that copies source label values to destination:
				// - source_labels: [...]
				//   target_label: foobar
				valueStr := bytesutil.InternBytes(bb.B)
				relabelBufPool.Put(bb)
				return setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
			}
			if !prc.hasCaptureGroupInReplacement {
				// Fast path for the rule that sets label value:
				// - target_label: foobar
				//   replacement: something-here
				relabelBufPool.Put(bb)
				labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, replacement)
				return labels
			}
		}
		sourceStr := bytesutil.ToUnsafeString(bb.B)
		if !prc.regex.MatchString(sourceStr) {
			// Fast path - regexp mismatch.
			relabelBufPool.Put(bb)
			return labels
		}
		var valueStr string
		if replacement == prc.Replacement {
			// Fast path - the replacement wasn't modified, so it is safe calling stringReplacer.Transform.
			valueStr = prc.stringReplacer.Transform(sourceStr)
		} else {
			// Slow path - the replacement has been modified, so the valueStr must be calculated
			// from scratch based on the new replacement value.
			match := prc.RegexAnchored.FindSubmatchIndex(bb.B)
			valueStr = prc.expandCaptureGroups(replacement, sourceStr, match)
		}
		nameStr := prc.TargetLabel
		if prc.hasCaptureGroupInTargetLabel {
			// Slow path - target_label contains regex capture groups, so the target_label
			// must be calculated from the regex match.
			match := prc.RegexAnchored.FindSubmatchIndex(bb.B)
			nameStr = prc.expandCaptureGroups(nameStr, sourceStr, match)
		}
		relabelBufPool.Put(bb)
		return setLabelValue(labels, labelsOffset, nameStr, valueStr)
	case "replace_all":
		// Replace all the occurrences of `regex` at `source_labels` joined with `separator` with the `replacement`
		// and store the result at `target_label`
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		sourceStr := bytesutil.InternBytes(bb.B)
		relabelBufPool.Put(bb)
		valueStr := prc.replaceStringSubmatchesFast(sourceStr)
		if valueStr != sourceStr {
			labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
		}
		return labels
	case "keep_if_contains":
		// Keep the entry if target_label contains all the label values listed in source_labels.
		// For example, the following relabeling rule would leave the entry if __meta_consul_tags
		// contains values of __meta_required_tag1 and __meta_required_tag2:
		//
		//   - action: keep_if_contains
		//     target_label: __meta_consul_tags
		//     source_labels: [__meta_required_tag1, __meta_required_tag2]
		//
		if containsAllLabelValues(src, prc.TargetLabel, prc.SourceLabels) {
			return labels
		}
		return labels[:labelsOffset]
	case "drop_if_contains":
		// Drop the entry if target_label contains all the label values listed in source_labels.
		// For example, the following relabeling rule would drop the entry if __meta_consul_tags
		// contains values of __meta_required_tag1 and __meta_required_tag2:
		//
		//   - action: drop_if_contains
		//     target_label: __meta_consul_tags
		//     source_labels: [__meta_required_tag1, __meta_required_tag2]
		//
		if containsAllLabelValues(src, prc.TargetLabel, prc.SourceLabels) {
			return labels[:labelsOffset]
		}
		return labels
	case "keep_if_equal":
		// Keep the entry if all the label values in source_labels are equal.
		// For example:
		//
		//   - source_labels: [foo, bar]
		//     action: keep_if_equal
		//
		// Would leave the entry if `foo` value equals `bar` value
		if areEqualLabelValues(src, prc.SourceLabels) {
			return labels
		}
		return labels[:labelsOffset]
	case "drop_if_equal":
		// Drop the entry if all the label values in source_labels are equal.
		// For example:
		//
		//   - source_labels: [foo, bar]
		//     action: drop_if_equal
		//
		// Would drop the entry if `foo` value equals `bar` value.
		if areEqualLabelValues(src, prc.SourceLabels) {
			return labels[:labelsOffset]
		}
		return labels
	case "keepequal":
		// Keep the entry if `source_labels` joined with `separator` matches `target_label`
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		targetValue := getLabelValue(labels[labelsOffset:], prc.TargetLabel)
		keep := string(bb.B) == targetValue
		relabelBufPool.Put(bb)
		if keep {
			return labels
		}
		return labels[:labelsOffset]
	case "dropequal":
		// Drop the entry if `source_labels` joined with `separator` doesn't match `target_label`
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		targetValue := getLabelValue(labels[labelsOffset:], prc.TargetLabel)
		drop := string(bb.B) == targetValue
		relabelBufPool.Put(bb)
		if !drop {
			return labels
		}
		return labels[:labelsOffset]
	case "keep":
		// Keep the target if `source_labels` joined with `separator` match the `regex`.
		if prc.RegexAnchored == defaultRegexForRelabelConfig {
			// Fast path for the case with `if` and without explicitly set `regex`:
			//
			// - action: keep
			//   if: 'some{label=~"filters"}'
			//
			return labels
		}
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		keep := prc.regex.MatchString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		if !keep {
			return labels[:labelsOffset]
		}
		return labels
	case "drop":
		// Drop the target if `source_labels` joined with `separator` don't match the `regex`.
		if prc.RegexAnchored == defaultRegexForRelabelConfig {
			// Fast path for the case with `if` and without explicitly set `regex`:
			//
			// - action: drop
			//   if: 'some{label=~"filters"}'
			//
			return labels[:labelsOffset]
		}
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		drop := prc.regex.MatchString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		if drop {
			return labels[:labelsOffset]
		}
		return labels
	case "hashmod":
		// Calculate the `modulus` from the hash of `source_labels` joined with `separator` and store it at `target_label`
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		h := xxhash.Sum64(bb.B) % prc.Modulus
		value := strconv.Itoa(int(h))
		relabelBufPool.Put(bb)
		return setLabelValue(labels, labelsOffset, prc.TargetLabel, value)
	case "labelmap":
		// Replace label names with the `replacement` if they match `regex`
		for _, label := range src {
			labelName := prc.replaceFullStringFast(label.Name)
			if labelName != label.Name {
				labels = setLabelValue(labels, labelsOffset, labelName, label.Value)
			}
		}
		return labels
	case "labelmap_all":
		// Replace all the occurrences of `regex` at label names with `replacement`
		for i := range src {
			label := &src[i]
			label.Name = prc.replaceStringSubmatchesFast(label.Name)
		}
		return labels
	case "labeldrop":
		// Drop labels with names matching the `regex`
		dst := labels[:labelsOffset]
		re := prc.regex
		for _, label := range src {
			if !re.MatchString(label.Name) {
				dst = append(dst, label)
			}
		}
		return dst
	case "labelkeep":
		// Keep labels with names matching the `regex`
		dst := labels[:labelsOffset]
		re := prc.regex
		for _, label := range src {
			if re.MatchString(label.Name) {
				dst = append(dst, label)
			}
		}
		return dst
	case "uppercase":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		valueStr := bytesutil.InternBytes(bb.B)
		relabelBufPool.Put(bb)
		valueStr = strings.ToUpper(valueStr)
		labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
		return labels
	case "lowercase":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		valueStr := bytesutil.InternBytes(bb.B)
		relabelBufPool.Put(bb)
		valueStr = strings.ToLower(valueStr)
		labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
		return labels
	default:
		logger.Panicf("BUG: unknown `action`: %q", prc.Action)
		return labels
	}
}

// replaceFullStringFast replaces s with the replacement if s matches '^regex$'.
//
// s is returned as is if it doesn't match '^regex$'.
func (prc *parsedRelabelConfig) replaceFullStringFast(s string) string {
	prefix, complete := prc.regexOriginal.LiteralPrefix()
	replacement := prc.Replacement
	if complete && !prc.hasCaptureGroupInReplacement {
		if s == prefix {
			// Fast path - s matches literal regex
			return replacement
		}
		// Fast path - s doesn't match literal regex
		return s
	}
	if !strings.HasPrefix(s, prefix) {
		// Fast path - s doesn't match literal prefix from regex
		return s
	}
	if replacement == "$1" {
		// Fast path for commonly used rule for deleting label prefixes such as:
		//
		// - action: labelmap
		//   regex: __meta_kubernetes_node_label_(.+)
		//
		reStr := prc.regexOriginal.String()
		if strings.HasPrefix(reStr, prefix) {
			suffix := s[len(prefix):]
			reSuffix := reStr[len(prefix):]
			switch reSuffix {
			case "(.*)":
				return suffix
			case "(.+)":
				if len(suffix) > 0 {
					return suffix
				}
				return s
			}
		}
	}
	if !prc.regex.MatchString(s) {
		// Fast path - regex mismatch
		return s
	}
	// Slow path - handle the rest of cases.
	return prc.stringReplacer.Transform(s)
}

// replaceFullStringSlow replaces s with the replacement if s matches '^regex$'.
//
// s is returned as is if it doesn't match '^regex$'.
func (prc *parsedRelabelConfig) replaceFullStringSlow(s string) string {
	// Slow path - regexp processing
	match := prc.RegexAnchored.FindStringSubmatchIndex(s)
	if match == nil {
		return s
	}
	return prc.expandCaptureGroups(prc.Replacement, s, match)
}

// replaceStringSubmatchesFast replaces all the regex matches with the replacement in s.
func (prc *parsedRelabelConfig) replaceStringSubmatchesFast(s string) string {
	prefix, complete := prc.regexOriginal.LiteralPrefix()
	if complete && !prc.hasCaptureGroupInReplacement && !strings.Contains(s, prefix) {
		// Fast path - zero regex matches in s.
		return s
	}
	// Slow path - replace all the regex matches in s with the replacement.
	return prc.submatchReplacer.Transform(s)
}

// replaceStringSubmatchesSlow replaces all the regex matches with the replacement in s.
func (prc *parsedRelabelConfig) replaceStringSubmatchesSlow(s string) string {
	return prc.regexOriginal.ReplaceAllString(s, prc.Replacement)
}

func (prc *parsedRelabelConfig) expandCaptureGroups(template, source string, match []int) string {
	bb := relabelBufPool.Get()
	bb.B = prc.RegexAnchored.ExpandString(bb.B[:0], template, source, match)
	s := bytesutil.InternBytes(bb.B)
	relabelBufPool.Put(bb)
	return s
}

var relabelBufPool bytesutil.ByteBufferPool

func containsAllLabelValues(labels []prompbmarshal.Label, targetLabel string, sourceLabels []string) bool {
	targetLabelValue := getLabelValue(labels, targetLabel)
	for _, sourceLabel := range sourceLabels {
		v := getLabelValue(labels, sourceLabel)
		if !strings.Contains(targetLabelValue, v) {
			return false
		}
	}
	return true
}

func areEqualLabelValues(labels []prompbmarshal.Label, labelNames []string) bool {
	if len(labelNames) < 2 {
		logger.Panicf("BUG: expecting at least 2 labelNames; got %d", len(labelNames))
		return false
	}
	labelValue := getLabelValue(labels, labelNames[0])
	for _, labelName := range labelNames[1:] {
		v := getLabelValue(labels, labelName)
		if v != labelValue {
			return false
		}
	}
	return true
}

func concatLabelValues(dst []byte, labels []prompbmarshal.Label, labelNames []string, separator string) []byte {
	if len(labelNames) == 0 {
		return dst
	}
	for _, labelName := range labelNames {
		labelValue := getLabelValue(labels, labelName)
		dst = append(dst, labelValue...)
		dst = append(dst, separator...)
	}
	return dst[:len(dst)-len(separator)]
}

func setLabelValue(labels []prompbmarshal.Label, labelsOffset int, name, value string) []prompbmarshal.Label {
	if label := GetLabelByName(labels[labelsOffset:], name); label != nil {
		label.Value = value
		return labels
	}
	labels = append(labels, prompbmarshal.Label{
		Name:  name,
		Value: value,
	})
	return labels
}

func getLabelValue(labels []prompbmarshal.Label, name string) string {
	for _, label := range labels {
		if label.Name == name {
			return label.Value
		}
	}
	return ""
}

// GetLabelByName returns label with the given name from labels.
func GetLabelByName(labels []prompbmarshal.Label, name string) *prompbmarshal.Label {
	for i := range labels {
		label := &labels[i]
		if label.Name == name {
			return label
		}
	}
	return nil
}

// CleanLabels sets label.Name and label.Value to an empty string for all the labels.
//
// This should help GC cleaning up label.Name and label.Value strings.
func CleanLabels(labels []prompbmarshal.Label) {
	clear(labels)
}

// LabelsToString returns Prometheus string representation for the given labels.
//
// Labels in the returned string are sorted by name,
// while the __name__ label is put in front of {} labels.
func LabelsToString(labels []prompbmarshal.Label) string {
	labelsCopy := append([]prompbmarshal.Label{}, labels...)
	SortLabels(labelsCopy)
	mname := ""
	for i, label := range labelsCopy {
		if label.Name == "__name__" {
			mname = label.Value
			labelsCopy = append(labelsCopy[:i], labelsCopy[i+1:]...)
			break
		}
	}
	if mname != "" && len(labelsCopy) == 0 {
		return mname
	}
	b := []byte(mname)
	b = append(b, '{')
	for i, label := range labelsCopy {
		b = append(b, label.Name...)
		b = append(b, '=')
		b = strconv.AppendQuote(b, label.Value)
		if i+1 < len(labelsCopy) {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return string(b)
}

// SortLabels sorts labels in alphabetical order.
func SortLabels(labels []prompbmarshal.Label) {
	x := promutil.GetLabels()
	labelsOrig := x.Labels
	x.Labels = labels
	x.Sort()
	x.Labels = labelsOrig
	promutil.PutLabels(x)
}

func fillLabelReferences(dst []byte, replacement string, labels []prompbmarshal.Label) []byte {
	s := replacement
	for len(s) > 0 {
		n := strings.Index(s, "{{")
		if n < 0 {
			return append(dst, s...)
		}
		dst = append(dst, s[:n]...)
		s = s[n+2:]
		n = strings.Index(s, "}}")
		if n < 0 {
			dst = append(dst, "{{"...)
			return append(dst, s...)
		}
		labelName := s[:n]
		s = s[n+2:]
		labelValue := getLabelValue(labels, labelName)
		dst = append(dst, labelValue...)
	}
	return dst
}

// SanitizeLabelName replaces unsupported by Prometheus chars in label names with _.
//
// See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func SanitizeLabelName(name string) string {
	return labelNameSanitizer.Transform(name)
}

// SplitMetricNameToTokens returns tokens generated from metric name divided by unsupported Prometheus characters
//
// See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func SplitMetricNameToTokens(name string) []string {
	return nonAlphaNumChars.Split(name, -1)
}

var nonAlphaNumChars = regexp.MustCompile(`[^a-zA-Z0-9]`)

var labelNameSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	return unsupportedLabelNameChars.ReplaceAllLiteralString(s, "_")
})

var unsupportedLabelNameChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// SanitizeMetricName replaces unsupported by Prometheus chars in metric names with _.
//
// See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func SanitizeMetricName(value string) string {
	return metricNameSanitizer.Transform(value)
}

var metricNameSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	return unsupportedMetricNameChars.ReplaceAllLiteralString(s, "_")
})

var unsupportedMetricNameChars = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
