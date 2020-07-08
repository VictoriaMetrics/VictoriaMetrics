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

// ParsedRelabelConfig contains parsed `relabel_config`.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type ParsedRelabelConfig struct {
	SourceLabels []string
	Separator    string
	TargetLabel  string
	Regex        *regexp.Regexp
	Modulus      uint64
	Replacement  string
	Action       string

	hasCaptureGroupInTargetLabel bool
	hasCaptureGroupInReplacement bool
}

// String returns human-readable representation for prc.
func (prc *ParsedRelabelConfig) String() string {
	return fmt.Sprintf("SourceLabels=%s, Separator=%s, TargetLabel=%s, Regex=%s, Modulus=%d, Replacement=%s, Action=%s",
		prc.SourceLabels, prc.Separator, prc.TargetLabel, prc.Regex.String(), prc.Modulus, prc.Replacement, prc.Action)
}

// ApplyRelabelConfigs applies prcs to labels starting from the labelsOffset.
//
// If isFinalize is set, then FinalizeLabels is called on the labels[labelsOffset:].
//
// The returned labels at labels[labelsOffset:] are sorted.
func ApplyRelabelConfigs(labels []prompbmarshal.Label, labelsOffset int, prcs []ParsedRelabelConfig, isFinalize bool) []prompbmarshal.Label {
	for i := range prcs {
		tmp := applyRelabelConfig(labels, labelsOffset, &prcs[i])
		if len(tmp) == labelsOffset {
			// All the labels have been removed.
			return tmp
		}
		labels = tmp
	}
	labels = removeEmptyLabels(labels, labelsOffset)
	if isFinalize {
		labels = FinalizeLabels(labels[:labelsOffset], labels[labelsOffset:])
	}
	SortLabels(labels[labelsOffset:])
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

// applyRelabelConfig applies relabeling according to prc.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
func applyRelabelConfig(labels []prompbmarshal.Label, labelsOffset int, prc *ParsedRelabelConfig) []prompbmarshal.Label {
	src := labels[labelsOffset:]
	switch prc.Action {
	case "replace":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		if len(bb.B) == 0 && prc.Regex == defaultRegexForRelabelConfig && !prc.hasCaptureGroupInReplacement && !prc.hasCaptureGroupInTargetLabel {
			// Fast path for the following rule that just sets label value:
			// - target_label: foobar
			//   replacement: something-here
			relabelBufPool.Put(bb)
			return setLabelValue(labels, labelsOffset, prc.TargetLabel, prc.Replacement)
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
		if !prc.Regex.Match(bb.B) {
			// Fast path - nothing to replace.
			relabelBufPool.Put(bb)
			return labels
		}
		sourceStr := string(bb.B) // Make a copy of bb, since it can be returned from ReplaceAllString
		relabelBufPool.Put(bb)
		valueStr := prc.Regex.ReplaceAllString(sourceStr, prc.Replacement)
		return setLabelValue(labels, labelsOffset, prc.TargetLabel, valueStr)
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
		keep := prc.Regex.Match(bb.B)
		relabelBufPool.Put(bb)
		if !keep {
			return labels[:labelsOffset]
		}
		return labels
	case "drop":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, prc.SourceLabels, prc.Separator)
		drop := prc.Regex.Match(bb.B)
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
			match := prc.Regex.FindStringSubmatchIndex(label.Name)
			if match == nil {
				continue
			}
			value := relabelBufPool.Get()
			value.B = prc.Regex.ExpandString(value.B[:0], prc.Replacement, label.Name, match)
			label.Name = string(value.B)
			relabelBufPool.Put(value)
		}
		return labels
	case "labelmap_all":
		for i := range src {
			label := &src[i]
			if !prc.Regex.MatchString(label.Name) {
				continue
			}
			label.Name = prc.Regex.ReplaceAllString(label.Name, prc.Replacement)
		}
		return labels
	case "labeldrop":
		keepSrc := true
		for i := range src {
			if prc.Regex.MatchString(src[i].Name) {
				keepSrc = false
				break
			}
		}
		if keepSrc {
			return labels
		}
		dst := labels[:labelsOffset]
		for i := range src {
			label := &src[i]
			if !prc.Regex.MatchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	case "labelkeep":
		keepSrc := true
		for i := range src {
			if !prc.Regex.MatchString(src[i].Name) {
				keepSrc = false
				break
			}
		}
		if keepSrc {
			return labels
		}
		dst := labels[:labelsOffset]
		for i := range src {
			label := &src[i]
			if prc.Regex.MatchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	default:
		logger.Panicf("BUG: unknown `action`: %q", prc.Action)
		return labels
	}
}

func (prc *ParsedRelabelConfig) expandCaptureGroups(template, source string, match []int) string {
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
