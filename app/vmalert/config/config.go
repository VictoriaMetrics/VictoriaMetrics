package config

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"hash/fnv"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config/log"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"gopkg.in/yaml.v2"
)

// Group contains list of Rules grouped into
// entity with one name and evaluation interval
type Group struct {
	Type       Type `yaml:"type,omitempty"`
	File       string
	Name       string              `yaml:"name"`
	Interval   *promutils.Duration `yaml:"interval,omitempty"`
	EvalOffset *promutils.Duration `yaml:"eval_offset,omitempty"`
	// EvalDelay will adjust the `time` parameter of rule evaluation requests to compensate intentional query delay from datasource.
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5155
	EvalDelay   *promutils.Duration `yaml:"eval_delay,omitempty"`
	Limit       int                 `yaml:"limit,omitempty"`
	Rules       []Rule              `yaml:"rules"`
	Concurrency int                 `yaml:"concurrency"`
	// Labels is a set of label value pairs, that will be added to every rule.
	// It has priority over the external labels.
	Labels map[string]string `yaml:"labels"`
	// Checksum stores the hash of yaml definition for this group.
	// May be used to detect any changes like rules re-ordering etc.
	Checksum string
	// Optional HTTP URL parameters added to each rule request
	Params url.Values `yaml:"params"`
	// Headers contains optional HTTP headers added to each rule request
	Headers []Header `yaml:"headers,omitempty"`
	// NotifierHeaders contains optional HTTP headers sent to notifiers for generated notifications
	NotifierHeaders []Header `yaml:"notifier_headers,omitempty"`
	// EvalAlignment will make the timestamp of group query requests be aligned with interval
	EvalAlignment *bool `yaml:"eval_alignment,omitempty"`
	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]any `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (g *Group) UnmarshalYAML(unmarshal func(any) error) error {
	type group Group
	if err := unmarshal((*group)(g)); err != nil {
		return err
	}
	b, err := yaml.Marshal(g)
	if err != nil {
		return fmt.Errorf("failed to marshal group configuration for checksum: %w", err)
	}
	// change default value to prometheus datasource.
	if g.Type.Get() == "" {
		g.Type.Set(NewPrometheusType())
	}

	h := md5.New()
	h.Write(b)
	g.Checksum = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

// Validate checks configuration errors for group and internal rules
func (g *Group) Validate(validateTplFn ValidateTplFn, validateExpressions bool) error {
	if g.Name == "" {
		return fmt.Errorf("group name must be set")
	}
	if g.Interval.Duration() < 0 {
		return fmt.Errorf("interval shouldn't be lower than 0")
	}
	if g.EvalOffset.Duration() < 0 {
		return fmt.Errorf("eval_offset shouldn't be lower than 0")
	}
	// if `eval_offset` is set, interval won't use global evaluationInterval flag and must bigger than offset.
	if g.EvalOffset.Duration() > g.Interval.Duration() {
		return fmt.Errorf("eval_offset should be smaller than interval; now eval_offset: %v, interval: %v", g.EvalOffset.Duration(), g.Interval.Duration())
	}
	if g.Limit < 0 {
		return fmt.Errorf("invalid limit %d, shouldn't be less than 0", g.Limit)
	}
	if g.Concurrency < 0 {
		return fmt.Errorf("invalid concurrency %d, shouldn't be less than 0", g.Concurrency)
	}

	uniqueRules := map[uint64]struct{}{}
	for _, r := range g.Rules {
		ruleName := r.Record
		if r.Alert != "" {
			ruleName = r.Alert
		}
		if _, ok := uniqueRules[r.ID]; ok {
			return fmt.Errorf("%q is a duplicate in group", r.String())
		}
		uniqueRules[r.ID] = struct{}{}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid rule %q: %w", ruleName, err)
		}
		if validateExpressions {
			// its needed only for tests.
			// because correct types must be inherited after unmarshalling.
			exprValidator := g.Type.ValidateExpr
			if err := exprValidator(r.Expr); err != nil {
				return fmt.Errorf("invalid expression for rule  %q: %w", ruleName, err)
			}
		}
		if validateTplFn != nil {
			if err := validateTplFn(r.Annotations); err != nil {
				return fmt.Errorf("invalid annotations for rule  %q: %w", ruleName, err)
			}
			if err := validateTplFn(r.Labels); err != nil {
				return fmt.Errorf("invalid labels for rule  %q: %w", ruleName, err)
			}
		}
	}
	return checkOverflow(g.XXX, fmt.Sprintf("group %q", g.Name))
}

// Rule describes entity that represent either
// recording rule or alerting rule.
type Rule struct {
	ID     uint64
	Record string              `yaml:"record,omitempty"`
	Alert  string              `yaml:"alert,omitempty"`
	Expr   string              `yaml:"expr"`
	For    *promutils.Duration `yaml:"for,omitempty"`
	// Alert will continue firing for this long even when the alerting expression no longer has results.
	KeepFiringFor *promutils.Duration `yaml:"keep_firing_for,omitempty"`
	Labels        map[string]string   `yaml:"labels,omitempty"`
	Annotations   map[string]string   `yaml:"annotations,omitempty"`
	Debug         bool                `yaml:"debug,omitempty"`
	// UpdateEntriesLimit defines max number of rule's state updates stored in memory.
	// Overrides `-rule.updateEntriesLimit`.
	UpdateEntriesLimit *int `yaml:"update_entries_limit,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]any `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Rule) UnmarshalYAML(unmarshal func(any) error) error {
	type rule Rule
	if err := unmarshal((*rule)(r)); err != nil {
		return err
	}
	r.ID = HashRule(*r)
	return nil
}

// Name returns Rule name according to its type
func (r *Rule) Name() string {
	if r.Record != "" {
		return r.Record
	}
	return r.Alert
}

// String implements Stringer interface
func (r *Rule) String() string {
	ruleType := "recording"
	if r.Alert != "" {
		ruleType = "alerting"
	}
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("%s rule %q", ruleType, r.Name()))
	b.WriteString(fmt.Sprintf("; expr: %q", r.Expr))

	kv := sortMap(r.Labels)
	for i := range kv {
		if i == 0 {
			b.WriteString("; labels:")
		}
		b.WriteString(" ")
		b.WriteString(kv[i].key)
		b.WriteString("=")
		b.WriteString(kv[i].value)
		if i < len(kv)-1 {
			b.WriteString(",")
		}
	}
	return b.String()
}

// HashRule hashes significant Rule fields into
// unique hash that supposed to define Rule uniqueness
func HashRule(r Rule) uint64 {
	h := fnv.New64a()
	h.Write([]byte(r.Expr))
	if r.Record != "" {
		h.Write([]byte("recording"))
		h.Write([]byte(r.Record))
	} else {
		h.Write([]byte("alerting"))
		h.Write([]byte(r.Alert))
	}
	kv := sortMap(r.Labels)
	for _, i := range kv {
		h.Write([]byte(i.key))
		h.Write([]byte(i.value))
		h.Write([]byte("\xff"))
	}
	return h.Sum64()
}

// Validate check for Rule configuration errors
func (r *Rule) Validate() error {
	if (r.Record == "" && r.Alert == "") || (r.Record != "" && r.Alert != "") {
		return fmt.Errorf("either `record` or `alert` must be set")
	}
	if r.Expr == "" {
		return fmt.Errorf("expression can't be empty")
	}
	return checkOverflow(r.XXX, "rule")
}

// ValidateTplFn must validate the given annotations
type ValidateTplFn func(annotations map[string]string) error

// cLogger is a logger with support of logs suppressing.
// it is used when logs emitted by config package needs
// to be suppressed.
var cLogger = &log.Logger{}

// ParseSilent parses rule configs from given file patterns without emitting logs
func ParseSilent(pathPatterns []string, validateTplFn ValidateTplFn, validateExpressions bool) ([]Group, error) {
	cLogger.Suppress(true)
	defer cLogger.Suppress(false)

	files, err := ReadFromFS(pathPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to read from the config: %w", err)
	}
	return parse(files, validateTplFn, validateExpressions)
}

// Parse parses rule configs from given file patterns
func Parse(pathPatterns []string, validateTplFn ValidateTplFn, validateExpressions bool) ([]Group, error) {
	files, err := ReadFromFS(pathPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to read from the config: %w", err)
	}
	groups, err := parse(files, validateTplFn, validateExpressions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", pathPatterns, err)
	}
	if len(groups) < 1 {
		cLogger.Warnf("no groups found in %s", strings.Join(pathPatterns, ";"))
	}
	return groups, nil
}

func parse(files map[string][]byte, validateTplFn ValidateTplFn, validateExpressions bool) ([]Group, error) {
	errGroup := new(utils.ErrGroup)
	var groups []Group
	for file, data := range files {
		uniqueGroups := map[string]struct{}{}
		gr, err := parseConfig(data)
		if err != nil {
			errGroup.Add(fmt.Errorf("failed to parse file %q: %w", file, err))
			continue
		}
		for _, g := range gr {
			if err := g.Validate(validateTplFn, validateExpressions); err != nil {
				errGroup.Add(fmt.Errorf("invalid group %q in file %q: %w", g.Name, file, err))
				continue
			}
			if _, ok := uniqueGroups[g.Name]; ok {
				errGroup.Add(fmt.Errorf("group name %q duplicate in file %q", g.Name, file))
				continue
			}
			uniqueGroups[g.Name] = struct{}{}
			g.File = file
			groups = append(groups, g)
		}
	}
	if err := errGroup.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].File != groups[j].File {
			return groups[i].File < groups[j].File
		}
		return groups[i].Name < groups[j].Name
	})
	return groups, nil
}

func parseConfig(data []byte) ([]Group, error) {
	data, err := envtemplate.ReplaceBytes(data)
	if err != nil {
		return nil, fmt.Errorf("cannot expand environment vars: %w", err)
	}

	var result []Group
	type cfgFile struct {
		Groups []Group `yaml:"groups"`
		// Catches all undefined fields and must be empty after parsing.
		XXX map[string]any `yaml:",inline"`
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var cf cfgFile
		if err = decoder.Decode(&cf); err != nil {
			if err == io.EOF { // EOF indicates no more documents to read
				break
			}
			return nil, err
		}
		if err = checkOverflow(cf.XXX, "config"); err != nil {
			return nil, err
		}
		result = append(result, cf.Groups...)
	}

	return result, nil
}

func checkOverflow(m map[string]any, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields in %s: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}

type item struct {
	key, value string
}

func sortMap(m map[string]string) []item {
	var kv []item
	for k, v := range m {
		kv = append(kv, item{key: k, value: v})
	}
	sort.Slice(kv, func(i, j int) bool {
		return kv[i].key < kv[j].key
	})
	return kv
}
