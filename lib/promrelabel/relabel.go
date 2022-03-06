package promrelabel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/cespare/xxhash/v2"
)

// parsedRelabelConfig contains parsed `relabel_config`.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type parsedRelabelConfig struct {
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

// String returns human-readable representation for prc.
func (prc *parsedRelabelConfig) String() string {
	return fmt.Sprintf("SourceLabels=%s, Separator=%s, TargetLabel=%s, Regex=%s, Modulus=%d, Replacement=%s, Action=%s, If=%s, graphiteMatchTemplate=%s, graphiteLabelRules=%s",
		prc.SourceLabels, prc.Separator, prc.TargetLabel, prc.regexOriginal, prc.Modulus, prc.Replacement,
		prc.Action, prc.If, prc.graphiteMatchTemplate, prc.graphiteLabelRules)
}

// Apply applies pcs to labels starting from the labelsOffset.
func (pcs *ParsedConfigs) Apply(labels []prompbmarshal.Label, labelsOffset int) []prompbmarshal.Label {
	var inStr, outStr string
	relabelDebug := false
	if pcs != nil {
		relabelDebug = pcs.relabelDebug
		if relabelDebug {
			inStr = labelsToString(labels[labelsOffset:])
		}
		for _, prc := range pcs.prcs {
			tmp := prc.apply(labels, labelsOffset)
			if len(tmp) == labelsOffset {
				// All the labels have been removed.
				if pcs.relabelDebug {
					logger.Infof("\nRelabel  In: %s\nRelabel Out: DROPPED - all labels removed", inStr)
				}
				return tmp
			}
			labels = tmp
		}
	}
	if relabelDebug {
		outStr = labelsToString(labels[labelsOffset:])
	}
	labels = removeEmptyLabels(labels, labelsOffset)
	if relabelDebug {
		if len(labels) == labelsOffset {
			logger.Infof("\nRelabel  In: %s\nRelabeled to DROP: %s", inStr, outStr)
			return labels
		} else if inStr == outStr {
			logger.Infof("\nRelabel  In: %s\nRelabel Out: KEPT AS IS - no change", inStr)
		} else {
			logger.Infof("\nRelabel  In: %s\nRelabel Out: %s", inStr, outStr)
		}
		// Drop labels
		labels = labels[:labelsOffset]
	}
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

// RemoveMetaLabels removes all the `__meta_` labels from src and puts the rest of labels to dst.
//
// See https://www.robustperception.io/life-of-a-label fo details.
func RemoveMetaLabels(dst, src []prompbmarshal.Label) []prompbmarshal.Label {
	for _, label := range src {
		if strings.HasPrefix(label.Name, "__meta_") {
			continue
		}
		dst = append(dst, label)
	}
	return dst
}

// RemoveLabelsWithDoubleDashPrefix removes labels with "__" prefix from src, appends the remaining lables to dst and returns the result.
func RemoveLabelsWithDoubleDashPrefix(dst, src []prompbmarshal.Label) []prompbmarshal.Label {
	for _, label := range src {
		name := label.Name
		// A hack: do not delete __vm_filepath label, since it is used by internal logic for FileSDConfig.
		if strings.HasPrefix(name, "__") && name != "__vm_filepath" {
			continue
		}
		dst = append(dst, label)
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
	if prc.If != nil && !prc.If.Match(labels) {
		if prc.Action == "keep" {
			// Drop the target on `if` mismatch for `action: keep`
			return labels[:labelsOffset]
		}
		// Do not apply prc actions on `if` mismatch.
		return labels
	}
	switch prc.Action {
	case "graphite":
		metricName := GetLabelValueByName(src, "__name__")
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
			valueStr := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
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
			replacement = bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
		}
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		if prc.RegexAnchored == defaultRegexForRelabelConfig && !prc.hasCaptureGroupInTargetLabel {
			if replacement == "$1" {
				// Fast path for the rule that copies source label values to destination:
				// - source_labels: [...]
				//   target_label: foobar
				valueStr := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
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
		sourceStr := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		valueStr := prc.replaceStringSubmatchesFast(sourceStr)
		if valueStr != sourceStr {
			labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
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
		valueStr := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		valueStr = strings.ToUpper(valueStr)
		labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
		return labels
	case "lowercase":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		valueStr := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
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
	s := bytesutil.InternString(bytesutil.ToUnsafeString(bb.B))
	relabelBufPool.Put(bb)
	return s
}

var relabelBufPool bytesutil.ByteBufferPool

func areEqualLabelValues(labels []prompbmarshal.Label, labelNames []string) bool {
	if len(labelNames) < 2 {
		logger.Panicf("BUG: expecting at least 2 labelNames; got %d", len(labelNames))
		return false
	}
	labelValue := GetLabelValueByName(labels, labelNames[0])
	for _, labelName := range labelNames[1:] {
		v := GetLabelValueByName(labels, labelName)
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
		label := GetLabelByName(labels, labelName)
		if label != nil {
			dst = append(dst, label.Value...)
		}
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

// GetLabelValueByName returns value for label with the given name from labels.
//
// It returns empty string for non-existing label.
func GetLabelValueByName(labels []prompbmarshal.Label, name string) string {
	label := GetLabelByName(labels, name)
	if label == nil {
		return ""
	}
	return label.Value
}

// CleanLabels sets label.Name and label.Value to an empty string for all the labels.
//
// This should help GC cleaning up label.Name and label.Value strings.
func CleanLabels(labels []prompbmarshal.Label) {
	for i := range labels {
		label := &labels[i]
		label.Name = ""
		label.Value = ""
	}
}

func labelsToString(labels []prompbmarshal.Label) string {
	labelsCopy := append([]prompbmarshal.Label{}, labels...)
	SortLabels(labelsCopy)
	mname := ""
	for _, label := range labelsCopy {
		if label.Name == "__name__" {
			mname = label.Value
			break
		}
	}
	if mname != "" && len(labelsCopy) <= 1 {
		return mname
	}
	b := []byte(mname)
	b = append(b, '{')
	for i, label := range labelsCopy {
		if label.Name == "__name__" {
			continue
		}
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
		labelValue := GetLabelValueByName(labels, labelName)
		dst = append(dst, labelValue...)
	}
	return dst
}

// SanitizeName replaces unsupported by Prometheus chars in metric names and label names with _.
//
// See https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels
func SanitizeName(name string) string {
	return promSanitizer.Transform(name)
}

var promSanitizer = bytesutil.NewFastStringTransformer(func(s string) string {
	return unsupportedPromChars.ReplaceAllString(s, "_")
})

var unsupportedPromChars = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
