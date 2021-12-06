package promrelabel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"gopkg.in/yaml.v2"
)

// RelabelConfig represents relabel config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type RelabelConfig struct {
	SourceLabels []string        `yaml:"source_labels,flow,omitempty"`
	Separator    *string         `yaml:"separator,omitempty"`
	TargetLabel  string          `yaml:"target_label,omitempty"`
	Regex        *MultiLineRegex `yaml:"regex,omitempty"`
	Modulus      uint64          `yaml:"modulus,omitempty"`
	Replacement  *string         `yaml:"replacement,omitempty"`
	Action       string          `yaml:"action,omitempty"`
}

// MultiLineRegex contains a regex, which can be split into multiple lines.
//
// These lines are joined with "|" then.
// For example:
//
// regex:
// - foo
// - bar
//
// is equivalent to:
//
// regex: "foo|bar"
type MultiLineRegex struct {
	s string
}

// UnmarshalYAML unmarshals mlr from YAML passed to f.
func (mlr *MultiLineRegex) UnmarshalYAML(f func(interface{}) error) error {
	var v interface{}
	if err := f(&v); err != nil {
		return err
	}
	s, err := stringValue(v)
	if err != nil {
		return err
	}
	mlr.s = s
	return nil
}

func stringValue(v interface{}) (string, error) {
	if v == nil {
		return "null", nil
	}
	switch x := v.(type) {
	case []interface{}:
		a := make([]string, len(x))
		for i, xx := range x {
			s, err := stringValue(xx)
			if err != nil {
				return "", err
			}
			a[i] = s
		}
		return strings.Join(a, "|"), nil
	case string:
		return x, nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(x), nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("unexpected type for `regex`: %T; want string or []string", v)
	}
}

// MarshalYAML marshals mlr to YAML.
func (mlr *MultiLineRegex) MarshalYAML() (interface{}, error) {
	a := strings.Split(mlr.s, "|")
	if len(a) == 1 {
		return a[0], nil
	}
	return a, nil
}

// ParsedConfigs represents parsed relabel configs.
type ParsedConfigs struct {
	prcs         []*parsedRelabelConfig
	relabelDebug bool
}

// Len returns the number of relabel configs in pcs.
func (pcs *ParsedConfigs) Len() int {
	if pcs == nil {
		return 0
	}
	return len(pcs.prcs)
}

// String returns human-readabale representation for pcs.
func (pcs *ParsedConfigs) String() string {
	if pcs == nil {
		return ""
	}
	var sb strings.Builder
	for _, prc := range pcs.prcs {
		fmt.Fprintf(&sb, "%s,", prc.String())
	}
	fmt.Fprintf(&sb, "relabelDebug=%v", pcs.relabelDebug)
	return sb.String()
}

// LoadRelabelConfigs loads relabel configs from the given path.
func LoadRelabelConfigs(path string, relabelDebug bool) (*ParsedConfigs, error) {
	data, err := fs.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `relabel_configs` from %q: %w", path, err)
	}
	data = envtemplate.Replace(data)
	pcs, err := ParseRelabelConfigsData(data, relabelDebug)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal `relabel_configs` from %q: %w", path, err)
	}
	return pcs, nil
}

// ParseRelabelConfigsData parses relabel configs from the given data.
func ParseRelabelConfigsData(data []byte, relabelDebug bool) (*ParsedConfigs, error) {
	var rcs []RelabelConfig
	if err := yaml.UnmarshalStrict(data, &rcs); err != nil {
		return nil, err
	}
	return ParseRelabelConfigs(rcs, relabelDebug)
}

// ParseRelabelConfigs parses rcs to dst.
func ParseRelabelConfigs(rcs []RelabelConfig, relabelDebug bool) (*ParsedConfigs, error) {
	if len(rcs) == 0 {
		return nil, nil
	}
	prcs := make([]*parsedRelabelConfig, len(rcs))
	for i := range rcs {
		prc, err := parseRelabelConfig(&rcs[i])
		if err != nil {
			return nil, fmt.Errorf("error when parsing `relabel_config` #%d: %w", i+1, err)
		}
		prcs[i] = prc
	}
	return &ParsedConfigs{
		prcs:         prcs,
		relabelDebug: relabelDebug,
	}, nil
}

var (
	defaultOriginalRegexForRelabelConfig = regexp.MustCompile(".*")
	defaultRegexForRelabelConfig         = regexp.MustCompile("^(.*)$")
)

func parseRelabelConfig(rc *RelabelConfig) (*parsedRelabelConfig, error) {
	sourceLabels := rc.SourceLabels
	separator := ";"
	if rc.Separator != nil {
		separator = *rc.Separator
	}
	targetLabel := rc.TargetLabel
	regexCompiled := defaultRegexForRelabelConfig
	regexOriginalCompiled := defaultOriginalRegexForRelabelConfig
	if rc.Regex != nil {
		regex := rc.Regex.s
		regexOrig := regex
		if rc.Action != "replace_all" && rc.Action != "labelmap_all" {
			regex = "^(?:" + regex + ")$"
		}
		re, err := regexp.Compile(regex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `regex` %q: %w", regex, err)
		}
		regexCompiled = re
		reOriginal, err := regexp.Compile(regexOrig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `regex` %q: %w", regexOrig, err)
		}
		regexOriginalCompiled = reOriginal
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
			return nil, fmt.Errorf("missing `target_label` for `action=replace`")
		}
	case "replace_all":
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("missing `source_labels` for `action=replace_all`")
		}
		if targetLabel == "" {
			return nil, fmt.Errorf("missing `target_label` for `action=replace_all`")
		}
	case "keep_if_equal":
		if len(sourceLabels) < 2 {
			return nil, fmt.Errorf("`source_labels` must contain at least two entries for `action=keep_if_equal`; got %q", sourceLabels)
		}
	case "drop_if_equal":
		if len(sourceLabels) < 2 {
			return nil, fmt.Errorf("`source_labels` must contain at least two entries for `action=drop_if_equal`; got %q", sourceLabels)
		}
	case "keep":
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("missing `source_labels` for `action=keep`")
		}
	case "drop":
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("missing `source_labels` for `action=drop`")
		}
	case "hashmod":
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("missing `source_labels` for `action=hashmod`")
		}
		if targetLabel == "" {
			return nil, fmt.Errorf("missing `target_label` for `action=hashmod`")
		}
		if modulus < 1 {
			return nil, fmt.Errorf("unexpected `modulus` for `action=hashmod`: %d; must be greater than 0", modulus)
		}
	case "keep_metrics":
		if rc.Regex == nil || rc.Regex.s == "" {
			return nil, fmt.Errorf("`regex` must be non-empty for `action=keep_metrics`")
		}
		if len(sourceLabels) > 0 {
			return nil, fmt.Errorf("`source_labels` must be empty for `action=keep_metrics`; got %q", sourceLabels)
		}
		sourceLabels = []string{"__name__"}
		action = "keep"
	case "drop_metrics":
		if rc.Regex == nil || rc.Regex.s == "" {
			return nil, fmt.Errorf("`regex` must be non-empty for `action=drop_metrics`")
		}
		if len(sourceLabels) > 0 {
			return nil, fmt.Errorf("`source_labels` must be empty for `action=drop_metrics`; got %q", sourceLabels)
		}
		sourceLabels = []string{"__name__"}
		action = "drop"
	case "labelmap":
	case "labelmap_all":
	case "labeldrop":
	case "labelkeep":
	default:
		return nil, fmt.Errorf("unknown `action` %q", action)
	}
	return &parsedRelabelConfig{
		SourceLabels: sourceLabels,
		Separator:    separator,
		TargetLabel:  targetLabel,
		Regex:        regexCompiled,
		Modulus:      modulus,
		Replacement:  replacement,
		Action:       action,

		regexOriginal:                regexOriginalCompiled,
		hasCaptureGroupInTargetLabel: strings.Contains(targetLabel, "$"),
		hasCaptureGroupInReplacement: strings.Contains(replacement, "$"),
	}, nil
}
