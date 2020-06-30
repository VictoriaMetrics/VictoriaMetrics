package promrelabel

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

// RelabelConfig represents relabel config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type RelabelConfig struct {
	SourceLabels []string `yaml:"source_labels"`
	Separator    *string  `yaml:"separator"`
	TargetLabel  string   `yaml:"target_label"`
	Regex        *string  `yaml:"regex"`
	Modulus      uint64   `yaml:"modulus"`
	Replacement  *string  `yaml:"replacement"`
	Action       string   `yaml:"action"`
}

// LoadRelabelConfigs loads relabel configs from the given path.
func LoadRelabelConfigs(path string) ([]ParsedRelabelConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `relabel_configs` from %q: %w", path, err)
	}
	var rcs []RelabelConfig
	if err := yaml.UnmarshalStrict(data, &rcs); err != nil {
		return nil, fmt.Errorf("cannot unmarshal `relabel_configs` from %q: %w", path, err)
	}
	return ParseRelabelConfigs(nil, rcs)
}

// ParseRelabelConfigs parses rcs to dst.
func ParseRelabelConfigs(dst []ParsedRelabelConfig, rcs []RelabelConfig) ([]ParsedRelabelConfig, error) {
	if len(rcs) == 0 {
		return dst, nil
	}
	for i := range rcs {
		var err error
		dst, err = parseRelabelConfig(dst, &rcs[i])
		if err != nil {
			return dst, fmt.Errorf("error when parsing `relabel_config` #%d: %w", i+1, err)
		}
	}
	return dst, nil
}

var defaultRegexForRelabelConfig = regexp.MustCompile("^(.*)$")

func parseRelabelConfig(dst []ParsedRelabelConfig, rc *RelabelConfig) ([]ParsedRelabelConfig, error) {
	sourceLabels := rc.SourceLabels
	separator := ";"
	if rc.Separator != nil {
		separator = *rc.Separator
	}
	targetLabel := rc.TargetLabel
	regexCompiled := defaultRegexForRelabelConfig
	if rc.Regex != nil {
		regex := *rc.Regex
		if rc.Action != "replace_all" && rc.Action != "labelmap_all" {
			regex = "^(?:" + *rc.Regex + ")$"
		}
		re, err := regexp.Compile(regex)
		if err != nil {
			return dst, fmt.Errorf("cannot parse `regex` %q: %w", regex, err)
		}
		regexCompiled = re
	}
	modulus := rc.Modulus
	replacement := "$1"
	if rc.Replacement != nil {
		replacement = *rc.Replacement
	}
	action := rc.Action
	if action == "" {
		action = "replace"
	}
	switch action {
	case "replace":
		if targetLabel == "" {
			return dst, fmt.Errorf("missing `target_label` for `action=replace`")
		}
	case "replace_all":
		if len(sourceLabels) == 0 {
			return dst, fmt.Errorf("missing `source_labels` for `action=replace_all`")
		}
		if targetLabel == "" {
			return dst, fmt.Errorf("missing `target_label` for `action=replace`")
		}
	case "keep_if_equal":
		if len(sourceLabels) < 2 {
			return dst, fmt.Errorf("`source_labels` must contain at least two entries for `action=keep_if_equal`; got %q", sourceLabels)
		}
	case "drop_if_equal":
		if len(sourceLabels) < 2 {
			return dst, fmt.Errorf("`source_labels` must contain at least two entries for `action=drop_if_equal`; got %q", sourceLabels)
		}
	case "keep":
		if len(sourceLabels) == 0 {
			return dst, fmt.Errorf("missing `source_labels` for `action=keep`")
		}
	case "drop":
		if len(sourceLabels) == 0 {
			return dst, fmt.Errorf("missing `source_labels` for `action=drop`")
		}
	case "hashmod":
		if len(sourceLabels) == 0 {
			return dst, fmt.Errorf("missing `source_labels` for `action=hashmod`")
		}
		if targetLabel == "" {
			return dst, fmt.Errorf("missing `target_label` for `action=hashmod`")
		}
		if modulus < 1 {
			return dst, fmt.Errorf("unexpected `modulus` for `action=hashmod`: %d; must be greater than 0", modulus)
		}
	case "labelmap":
	case "labelmap_all":
	case "labeldrop":
	case "labelkeep":
	default:
		return dst, fmt.Errorf("unknown `action` %q", action)
	}
	dst = append(dst, ParsedRelabelConfig{
		SourceLabels: sourceLabels,
		Separator:    separator,
		TargetLabel:  targetLabel,
		Regex:        regexCompiled,
		Modulus:      modulus,
		Replacement:  replacement,
		Action:       action,

		hasCaptureGroupInTargetLabel: strings.Contains(targetLabel, "$"),
		hasCaptureGroupInReplacement: strings.Contains(replacement, "$"),
	})
	return dst, nil
}
