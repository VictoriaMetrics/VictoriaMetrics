package promrelabel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	xxhash "github.com/cespare/xxhash/v2"
)

// parsedRelabelConfig contains parsed `relabel_config`.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type parsedRelabelConfig struct {
	SourceLabels []string
	Separator    string
	TargetLabel  string
	Regex        *regexp.Regexp
	Modulus      uint64
	Replacement  string
	Action       string

	regexOriginal                *regexp.Regexp
	hasCaptureGroupInTargetLabel bool
	hasCaptureGroupInReplacement bool
}

// String returns human-readable representation for prc.
func (prc *parsedRelabelConfig) String() string {
	return fmt.Sprintf("SourceLabels=%s, Separator=%s, TargetLabel=%s, Regex=%s, Modulus=%d, Replacement=%s, Action=%s",
		prc.SourceLabels, prc.Separator, prc.TargetLabel, prc.Regex.String(), prc.Modulus, prc.Replacement, prc.Action)
}

// Apply applies pcs to labels starting from the labelsOffset.
//
// If isFinalize is set, then FinalizeLabels is called on the labels[labelsOffset:].
//
// The returned labels at labels[labelsOffset:] are sorted.
func (pcs *ParsedConfigs) Apply(labels []prompbmarshal.Label, labelsOffset int, isFinalize bool) []prompbmarshal.Label {
	var inStr string
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
	labels = removeEmptyLabels(labels, labelsOffset)
	if isFinalize {
		labels = FinalizeLabels(labels[:labelsOffset], labels[labelsOffset:])
	}
	SortLabels(labels[labelsOffset:])
	if relabelDebug {
		if len(labels) == labelsOffset {
			logger.Infof("\nRelabel  In: %s\nRelabel Out: DROPPED - all labels removed", inStr)
			return labels
		}
		outStr := labelsToString(labels[labelsOffset:])
		if inStr == outStr {
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
	for i := range src {
		label := &src[i]
		if strings.HasPrefix(label.Name, "__meta_") {
			continue
		}
		dst = append(dst, *label)
	}
	return dst
}

// FinalizeLabels removes labels with "__" in the beginning (except of "__name__").
func FinalizeLabels(dst, src []prompbmarshal.Label) []prompbmarshal.Label {
	for i := range src {
		label := &src[i]
		name := label.Name
		if strings.HasPrefix(name, "__") && name != "__name__" {
			continue
		}
		dst = append(dst, *label)
	}
	return dst
}

// apply applies relabeling according to prc.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
func (prc *parsedRelabelConfig) apply(labels []prompbmarshal.Label, labelsOffset int) []prompbmarshal.Label {
	src := labels[labelsOffset:]
	switch prc.Action {
	case "replace":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		if prc.Regex == defaultRegexForRelabelConfig && !prc.hasCaptureGroupInTargetLabel {
			if prc.Replacement == "$1" {
				// Fast path for the rule that copies source label values to destination:
				// - source_labels: [...]
				//   target_label: foobar
				valueStr := string(bb.B)
				relabelBufPool.Put(bb)
				return setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
			}
			if !prc.hasCaptureGroupInReplacement {
				// Fast path for the rule that sets label value:
				// - target_label: foobar
				//   replacement: something-here
				relabelBufPool.Put(bb)
				labels = setLabelValue(labels, labelsOffset, prc.TargetLabel, prc.Replacement)
				return labels
			}
		}
		match := prc.Regex.FindSubmatchIndex(bb.B)
		if match == nil {
			// Fast path - nothing to replace.
			relabelBufPool.Put(bb)
			return labels
		}
		sourceStr := bytesutil.ToUnsafeString(bb.B)
		nameStr := prc.TargetLabel
		if prc.hasCaptureGroupInTargetLabel {
			nameStr = prc.expandCaptureGroups(nameStr, sourceStr, match)
		}
		valueStr := prc.expandCaptureGroups(prc.Replacement, sourceStr, match)
		relabelBufPool.Put(bb)
		return setLabelValue(labels, labelsOffset, nameStr, valueStr)
	case "replace_all":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		sourceStr := string(bb.B)
		relabelBufPool.Put(bb)
		valueStr, ok := prc.replaceStringSubmatches(sourceStr, prc.Replacement, prc.hasCaptureGroupInReplacement)
		if ok {
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
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		keep := prc.matchString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		if !keep {
			return labels[:labelsOffset]
		}
		return labels
	case "drop":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		drop := prc.matchString(bytesutil.ToUnsafeString(bb.B))
		relabelBufPool.Put(bb)
		if drop {
			return labels[:labelsOffset]
		}
		return labels
	case "hashmod":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		h := xxhash.Sum64(bb.B) % prc.Modulus
		value := strconv.Itoa(int(h))
		relabelBufPool.Put(bb)
		return setLabelValue(labels, labelsOffset, prc.TargetLabel, value)
	case "labelmap":
		for i := range src {
			label := &src[i]
			labelName, ok := prc.replaceFullString(label.Name, prc.Replacement, prc.hasCaptureGroupInReplacement)
			if ok {
				labels = setLabelValue(labels, labelsOffset, labelName, label.Value)
			}
		}
		return labels
	case "labelmap_all":
		for i := range src {
			label := &src[i]
			label.Name, _ = prc.replaceStringSubmatches(label.Name, prc.Replacement, prc.hasCaptureGroupInReplacement)
		}
		return labels
	case "labeldrop":
		dst := labels[:labelsOffset]
		for i := range src {
			label := &src[i]
			if !prc.matchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	case "labelkeep":
		dst := labels[:labelsOffset]
		for i := range src {
			label := &src[i]
			if prc.matchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	default:
		logger.Panicf("BUG: unknown `action`: %q", prc.Action)
		return labels
	}
}

func (prc *parsedRelabelConfig) replaceFullString(s, replacement string, hasCaptureGroupInReplacement bool) (string, bool) {
	prefix, complete := prc.regexOriginal.LiteralPrefix()
	if complete && !hasCaptureGroupInReplacement {
		if s == prefix {
			return replacement, true
		}
		return s, false
	}
	if !strings.HasPrefix(s, prefix) {
		return s, false
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
				return suffix, true
			case "(.+)":
				if len(suffix) > 0 {
					return suffix, true
				}
				return s, false
			}
		}
	}
	// Slow path - regexp processing
	match := prc.Regex.FindStringSubmatchIndex(s)
	if match == nil {
		return s, false
	}
	bb := relabelBufPool.Get()
	bb.B = prc.Regex.ExpandString(bb.B[:0], replacement, s, match)
	result := string(bb.B)
	relabelBufPool.Put(bb)
	return result, true
}

func (prc *parsedRelabelConfig) replaceStringSubmatches(s, replacement string, hasCaptureGroupInReplacement bool) (string, bool) {
	re := prc.regexOriginal
	prefix, complete := re.LiteralPrefix()
	if complete && !hasCaptureGroupInReplacement {
		if !strings.Contains(s, prefix) {
			return s, false
		}
		return strings.ReplaceAll(s, prefix, replacement), true
	}
	if !re.MatchString(s) {
		return s, false
	}
	return re.ReplaceAllString(s, replacement), true
}

func (prc *parsedRelabelConfig) matchString(s string) bool {
	prefix, complete := prc.regexOriginal.LiteralPrefix()
	if complete {
		return prefix == s
	}
	if !strings.HasPrefix(s, prefix) {
		return false
	}
	reStr := prc.regexOriginal.String()
	if strings.HasPrefix(reStr, prefix) {
		// Fast path for `foo.*` and `bar.+` regexps
		reSuffix := reStr[len(prefix):]
		switch reSuffix {
		case ".+", "(.+)":
			return len(s) > len(prefix)
		case ".*", "(.*)":
			return true
		}
	}
	return prc.Regex.MatchString(s)
}

func (prc *parsedRelabelConfig) expandCaptureGroups(template, source string, match []int) string {
	bb := relabelBufPool.Get()
	bb.B = prc.Regex.ExpandString(bb.B[:0], template, source, match)
	s := string(bb.B)
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
