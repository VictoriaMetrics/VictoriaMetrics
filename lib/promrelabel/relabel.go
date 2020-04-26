package promrelabel

import (
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

// FinalizeLabels finalizes labels according to relabel_config rules.
//
// It renames `__address__` to `instance` and removes labels with "__" in the beginning.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
func FinalizeLabels(dst, src []prompbmarshal.Label) []prompbmarshal.Label {
	for i := range src {
		label := &src[i]
		name := label.Name
		if !strings.HasPrefix(name, "__") || name == "__name__" {
			dst = append(dst, *label)
			continue
		}
		if name == "__address__" {
			if GetLabelByName(src, "instance") != nil {
				// The `instance` label is already set. Skip `__address__` label.
				continue
			}
			// Rename `__address__` label to `instance`.
			labelCopy := *label
			labelCopy.Name = "instance"
			dst = append(dst, labelCopy)
		}
	}
	return dst
}

// applyRelabelConfig applies relabeling according to cfg.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
func applyRelabelConfig(labels []prompbmarshal.Label, labelsOffset int, cfg *ParsedRelabelConfig) []prompbmarshal.Label {
	src := labels[labelsOffset:]
	switch cfg.Action {
	case "replace":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, cfg.SourceLabels, cfg.Separator)
		if len(bb.B) == 0 && cfg.Regex == defaultRegexForRelabelConfig && !strings.Contains(cfg.Replacement, "$") {
			// Fast path for the following rule that just sets label value:
			// - target_label: foobar
			//   replacement: something-here
			relabelBufPool.Put(bb)
			return setLabelValue(labels, labelsOffset, cfg.TargetLabel, cfg.Replacement)
		}
		match := cfg.Regex.FindSubmatchIndex(bb.B)
		if match == nil {
			// Fast path - nothing to replace.
			relabelBufPool.Put(bb)
			return labels
		}
		sourceStr := bytesutil.ToUnsafeString(bb.B)
		value := relabelBufPool.Get()
		value.B = cfg.Regex.ExpandString(value.B[:0], cfg.Replacement, sourceStr, match)
		relabelBufPool.Put(bb)
		valueStr := string(value.B)
		relabelBufPool.Put(value)
		return setLabelValue(labels, labelsOffset, cfg.TargetLabel, valueStr)
	case "replace_all":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, cfg.SourceLabels, cfg.Separator)
		if !cfg.Regex.Match(bb.B) {
			// Fast path - nothing to replace.
			relabelBufPool.Put(bb)
			return labels
		}
		sourceStr := string(bb.B) // Make a copy of bb, since it can be returned from ReplaceAllString
		relabelBufPool.Put(bb)
		valueStr := cfg.Regex.ReplaceAllString(sourceStr, cfg.Replacement)
		return setLabelValue(labels, labelsOffset, cfg.TargetLabel, valueStr)
	case "keep":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, cfg.SourceLabels, cfg.Separator)
		keep := cfg.Regex.Match(bb.B)
		relabelBufPool.Put(bb)
		if !keep {
			return labels[:labelsOffset]
		}
		return labels
	case "drop":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, cfg.SourceLabels, cfg.Separator)
		drop := cfg.Regex.Match(bb.B)
		relabelBufPool.Put(bb)
		if drop {
			return labels[:labelsOffset]
		}
		return labels
	case "hashmod":
		bb := relabelBufPool.Get()
		bb.B = concatLabelValues(bb.B[:0], src, cfg.SourceLabels, cfg.Separator)
		h := xxhash.Sum64(bb.B) % cfg.Modulus
		value := strconv.Itoa(int(h))
		relabelBufPool.Put(bb)
		return setLabelValue(labels, labelsOffset, cfg.TargetLabel, value)
	case "labelmap":
		for i := range src {
			label := &src[i]
			match := cfg.Regex.FindStringSubmatchIndex(label.Name)
			if match == nil {
				continue
			}
			value := relabelBufPool.Get()
			value.B = cfg.Regex.ExpandString(value.B[:0], cfg.Replacement, label.Name, match)
			label.Name = string(value.B)
			relabelBufPool.Put(value)
		}
		return labels
	case "labelmap_all":
		for i := range src {
			label := &src[i]
			if !cfg.Regex.MatchString(label.Name) {
				continue
			}
			label.Name = cfg.Regex.ReplaceAllString(label.Name, cfg.Replacement)
		}
		return labels
	case "labeldrop":
		keepSrc := true
		for i := range src {
			if cfg.Regex.MatchString(src[i].Name) {
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
			if !cfg.Regex.MatchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	case "labelkeep":
		keepSrc := true
		for i := range src {
			if !cfg.Regex.MatchString(src[i].Name) {
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
			if cfg.Regex.MatchString(label.Name) {
				dst = append(dst, *label)
			}
		}
		return dst
	default:
		logger.Panicf("BUG: unknown `action`: %q", cfg.Action)
		return labels
	}
}

var relabelBufPool bytesutil.ByteBufferPool

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
