package promrelabel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"gopkg.in/yaml.v2"
)

// RelabelConfig represents relabel config.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config
type RelabelConfig struct {
	If           *IfExpression   `yaml:"if,omitempty"`
	Action       string          `yaml:"action,omitempty"`
	SourceLabels []string        `yaml:"source_labels,flow,omitempty"`
	Separator    *string         `yaml:"separator,omitempty"`
	TargetLabel  string          `yaml:"target_label,omitempty"`
	Regex        *MultiLineRegex `yaml:"regex,omitempty"`
	Modulus      uint64          `yaml:"modulus,omitempty"`
	Replacement  *string         `yaml:"replacement,omitempty"`

	// Match is used together with Labels for `action: graphite`. For example:
	// - action: graphite
	//   match: 'foo.*.*.bar'
	//   labels:
	//     job: '$1'
	//     instance: '${2}:8080'
	Match string `yaml:"match,omitempty"`

	// Labels is used together with Match for `action: graphite`. For example:
	// - action: graphite
	//   match: 'foo.*.*.bar'
	//   labels:
	//     job: '$1'
	//     instance: '${2}:8080'
	Labels map[string]string `yaml:"labels,omitempty"`
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
	S string
}

// UnmarshalYAML unmarshals mlr from YAML passed to f.
func (mlr *MultiLineRegex) UnmarshalYAML(f func(any) error) error {
	var v any
	if err := f(&v); err != nil {
		return fmt.Errorf("cannot parse multiline regex: %w", err)
	}
	s, err := stringValue(v)
	if err != nil {
		return err
	}
	mlr.S = s
	return nil
}

func stringValue(v any) (string, error) {
	if v == nil {
		return "null", nil
	}
	switch x := v.(type) {
	case []any:
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
func (mlr *MultiLineRegex) MarshalYAML() (any, error) {
	if strings.ContainsAny(mlr.S, "([") {
		// The mlr.S contains groups. Fall back to returning the regexp as is without splitting it into parts.
		// This fixes https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2928 .
		return mlr.S, nil
	}
	a := strings.Split(mlr.S, "|")
	if len(a) == 1 {
		return a[0], nil
	}
	return a, nil
}

// ParsedConfigs represents parsed relabel configs.
type ParsedConfigs struct {
	prcs []*parsedRelabelConfig
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
	var a []string
	for _, prc := range pcs.prcs {
		s := prc.String()
		lines := strings.Split(s, "\n")
		lines[0] = "- " + lines[0]
		for i := range lines[1:] {
			line := &lines[1+i]
			if len(*line) > 0 {
				*line = "  " + *line
			}
		}
		s = strings.Join(lines, "\n")
		a = append(a, s)
	}
	return strings.Join(a, "")
}

// LoadRelabelConfigs loads relabel configs from the given path.
func LoadRelabelConfigs(path string) (*ParsedConfigs, error) {
	data, err := fscore.ReadFileOrHTTP(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read `relabel_configs` from %q: %w", path, err)
	}
	data, err = envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment vars at %q: %w", path, err)
	}
	pcs, err := ParseRelabelConfigsData(data)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal `relabel_configs` from %q: %w", path, err)
	}
	return pcs, nil
}

// ParseRelabelConfigsData parses relabel configs from the given data.
func ParseRelabelConfigsData(data []byte) (*ParsedConfigs, error) {
	var rcs []RelabelConfig
	if err := yaml.UnmarshalStrict(data, &rcs); err != nil {
		return nil, err
	}
	return ParseRelabelConfigs(rcs)
}

// ParseRelabelConfigs parses rcs to dst.
func ParseRelabelConfigs(rcs []RelabelConfig) (*ParsedConfigs, error) {
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
		prcs: prcs,
	}, nil
}

var (
	defaultOriginalRegexForRelabelConfig = regexp.MustCompile(".*")
	defaultRegexForRelabelConfig         = regexp.MustCompile("^(.*)$")
	defaultPromRegex                     = func() *regexutil.PromRegex {
		pr, err := regexutil.NewPromRegex(".*")
		if err != nil {
			panic(fmt.Errorf("BUG: unexpected error: %w", err))
		}
		return pr
	}()
)

func parseRelabelConfig(rc *RelabelConfig) (*parsedRelabelConfig, error) {
	sourceLabels := rc.SourceLabels
	separator := ";"
	if rc.Separator != nil {
		separator = *rc.Separator
	}
	action := strings.ToLower(rc.Action)
	if action == "" {
		action = "replace"
	}
	targetLabel := rc.TargetLabel
	regexAnchored := defaultRegexForRelabelConfig
	regexOriginalCompiled := defaultOriginalRegexForRelabelConfig
	promRegex := defaultPromRegex
	if rc.Regex != nil && !isDefaultRegex(rc.Regex.S) {
		regex := rc.Regex.S
		regexOrig := regex
		if rc.Action != "replace_all" && rc.Action != "labelmap_all" {
			regex = regexutil.RemoveStartEndAnchors(regex)
			regexOrig = regex
			regex = "^(?:" + regex + ")$"
		}
		re, err := regexp.Compile(regex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `regex` %q: %w", regex, err)
		}
		regexAnchored = re
		reOriginal, err := regexp.Compile(regexOrig)
		if err != nil {
			return nil, fmt.Errorf("cannot parse `regex` %q: %w", regexOrig, err)
		}
		regexOriginalCompiled = reOriginal
		promRegex, err = regexutil.NewPromRegex(regexOrig)
		if err != nil {
			logger.Panicf("BUG: cannot parse already parsed regex %q: %s", regexOrig, err)
		}
	}
	modulus := rc.Modulus
	replacement := "$1"
	if rc.Replacement != nil {
		replacement = *rc.Replacement
	}
	var graphiteMatchTemplate *graphiteMatchTemplate
	if rc.Match != "" {
		graphiteMatchTemplate = newGraphiteMatchTemplate(rc.Match)
	}
	var graphiteLabelRules []graphiteLabelRule
	if rc.Labels != nil {
		graphiteLabelRules = newGraphiteLabelRules(rc.Labels)
	}
	switch action {
	case "graphite":
		if graphiteMatchTemplate == nil {
			return nil, fmt.Errorf("missing `match` for `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
		if len(graphiteLabelRules) == 0 {
			return nil, fmt.Errorf("missing `labels` for `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
		if len(rc.SourceLabels) > 0 {
			return nil, fmt.Errorf("`source_labels` cannot be used with `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
		if rc.TargetLabel != "" {
			return nil, fmt.Errorf("`target_label` cannot be used with `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
		if rc.Replacement != nil {
			return nil, fmt.Errorf("`replacement` cannot be used with `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=graphite`; see https://docs.victoriametrics.com/victoriametrics/vmagent/#graphite-relabeling")
		}
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
	case "keep_if_contains":
		if targetLabel == "" {
			return nil, fmt.Errorf("`target_label` must be set for `action=keep_if_contains`")
		}
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("`source_labels` must contain at least a single entry for `action=keep_if_contains`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=keep_if_contains`")
		}
	case "drop_if_contains":
		if targetLabel == "" {
			return nil, fmt.Errorf("`target_label` must be set for `action=drop_if_contains`")
		}
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("`source_labels` must contain at least a single entry for `action=drop_if_contains`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=drop_if_contains`")
		}
	case "keep_if_equal":
		if len(sourceLabels) < 2 {
			return nil, fmt.Errorf("`source_labels` must contain at least two entries for `action=keep_if_equal`; got %q", sourceLabels)
		}
		if targetLabel != "" {
			return nil, fmt.Errorf("`target_label` cannot be used for `action=keep_if_equal`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=keep_if_equal`")
		}
	case "drop_if_equal":
		if len(sourceLabels) < 2 {
			return nil, fmt.Errorf("`source_labels` must contain at least two entries for `action=drop_if_equal`; got %q", sourceLabels)
		}
		if targetLabel != "" {
			return nil, fmt.Errorf("`target_label` cannot be used for `action=drop_if_equal`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=drop_if_equal`")
		}
	case "keepequal":
		if targetLabel == "" {
			return nil, fmt.Errorf("missing `target_label` for `action=keepequal`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=keepequal`")
		}
	case "dropequal":
		if targetLabel == "" {
			return nil, fmt.Errorf("missing `target_label` for `action=dropequal`")
		}
		if rc.Regex != nil {
			return nil, fmt.Errorf("`regex` cannot be used for `action=dropequal`")
		}
	case "keep":
		if len(sourceLabels) == 0 && rc.If == nil {
			return nil, fmt.Errorf("missing `source_labels` for `action=keep`")
		}
	case "drop":
		if len(sourceLabels) == 0 && rc.If == nil {
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
		if (rc.Regex == nil || rc.Regex.S == "") && rc.If == nil {
			return nil, fmt.Errorf("`regex` must be non-empty for `action=keep_metrics`")
		}
		if len(sourceLabels) > 0 {
			return nil, fmt.Errorf("`source_labels` must be empty for `action=keep_metrics`; got %q", sourceLabels)
		}
		sourceLabels = []string{"__name__"}
		action = "keep"
	case "drop_metrics":
		if (rc.Regex == nil || rc.Regex.S == "") && rc.If == nil {
			return nil, fmt.Errorf("`regex` must be non-empty for `action=drop_metrics`")
		}
		if len(sourceLabels) > 0 {
			return nil, fmt.Errorf("`source_labels` must be empty for `action=drop_metrics`; got %q", sourceLabels)
		}
		sourceLabels = []string{"__name__"}
		action = "drop"
	case "uppercase", "lowercase":
		if len(sourceLabels) == 0 {
			return nil, fmt.Errorf("missing `source_labels` for `action=%s`", action)
		}
		if targetLabel == "" {
			return nil, fmt.Errorf("missing `target_label` for `action=%s`", action)
		}
	case "labelmap":
	case "labelmap_all":
	case "labeldrop":
	case "labelkeep":
	default:
		return nil, fmt.Errorf("unknown `action` %q", action)
	}
	if action != "graphite" {
		if graphiteMatchTemplate != nil {
			return nil, fmt.Errorf("`match` config cannot be applied to `action=%s`; it is applied only to `action=graphite`", action)
		}
		if len(graphiteLabelRules) > 0 {
			return nil, fmt.Errorf("`labels` config cannot be applied to `action=%s`; it is applied only to `action=graphite`", action)
		}
	}
	ruleOriginal, err := yaml.Marshal(rc)
	if err != nil {
		logger.Panicf("BUG: cannot marshal RelabelConfig: %s", err)
	}
	prc := &parsedRelabelConfig{
		ruleOriginal: string(ruleOriginal),

		SourceLabels:  sourceLabels,
		Separator:     separator,
		TargetLabel:   targetLabel,
		RegexAnchored: regexAnchored,
		Modulus:       modulus,
		Replacement:   replacement,
		Action:        action,
		If:            rc.If,

		graphiteMatchTemplate: graphiteMatchTemplate,
		graphiteLabelRules:    graphiteLabelRules,

		regex:         promRegex,
		regexOriginal: regexOriginalCompiled,

		hasCaptureGroupInTargetLabel:   strings.Contains(targetLabel, "$"),
		hasCaptureGroupInReplacement:   strings.Contains(replacement, "$"),
		hasLabelReferenceInReplacement: strings.Contains(replacement, "{{"),
	}
	prc.stringReplacer = bytesutil.NewFastStringTransformer(prc.replaceFullStringSlow)
	prc.submatchReplacer = bytesutil.NewFastStringTransformer(prc.replaceStringSubmatchesSlow)
	return prc, nil
}

func isDefaultRegex(expr string) bool {
	prefix, suffix := regexutil.SimplifyPromRegex(expr)
	if prefix != "" {
		return false
	}
	return suffix == "(?s:.*)"
}
