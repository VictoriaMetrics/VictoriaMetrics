package config

import (
	"crypto/md5"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Group contains list of Rules grouped into
// entity with one name and evaluation interval
type Group struct {
	Type        datasource.Type `yaml:"type,omitempty"`
	File        string
	Name        string             `yaml:"name"`
	Interval    utils.PromDuration `yaml:"interval"`
	Rules       []Rule             `yaml:"rules"`
	Concurrency int                `yaml:"concurrency"`
	// ExtraFilterLabels is a list label filters applied to every rule
	// request withing a group. Is compatible only with VM datasources.
	// See https://docs.victoriametrics.com#prometheus-querying-api-enhancements
	// DEPRECATED: use Params field instead
	ExtraFilterLabels map[string]string `yaml:"extra_filter_labels"`
	// Labels is a set of label value pairs, that will be added to every rule.
	// It has priority over the external labels.
	Labels map[string]string `yaml:"labels"`
	// Checksum stores the hash of yaml definition for this group.
	// May be used to detect any changes like rules re-ordering etc.
	Checksum string
	// Optional HTTP URL parameters added to each rule request
	Params url.Values `yaml:"params"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (g *Group) UnmarshalYAML(unmarshal func(interface{}) error) error {
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
		g.Type.Set(datasource.NewPrometheusType())
	}

	// backward compatibility with deprecated `ExtraFilterLabels` param
	if len(g.ExtraFilterLabels) > 0 {
		if g.Params == nil {
			g.Params = url.Values{}
		}
		// Sort extraFilters for consistent order for query args across runs.
		extraFilters := make([]string, 0, len(g.ExtraFilterLabels))
		for k, v := range g.ExtraFilterLabels {
			extraFilters = append(extraFilters, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(extraFilters)
		for _, extraFilter := range extraFilters {
			g.Params.Add("extra_label", extraFilter)
		}
	}

	h := md5.New()
	h.Write(b)
	g.Checksum = fmt.Sprintf("%x", h.Sum(nil))
	return nil
}

// Validate check for internal Group or Rule configuration errors
func (g *Group) Validate(validateAnnotations, validateExpressions bool) error {
	if g.Name == "" {
		return fmt.Errorf("group name must be set")
	}

	uniqueRules := map[uint64]struct{}{}
	for _, r := range g.Rules {
		ruleName := r.Record
		if r.Alert != "" {
			ruleName = r.Alert
		}
		if _, ok := uniqueRules[r.ID]; ok {
			return fmt.Errorf("rule %q duplicate", ruleName)
		}
		uniqueRules[r.ID] = struct{}{}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid rule %q.%q: %w", g.Name, ruleName, err)
		}
		if validateExpressions {
			// its needed only for tests.
			// because correct types must be inherited after unmarshalling.
			exprValidator := g.Type.ValidateExpr
			if err := exprValidator(r.Expr); err != nil {
				return fmt.Errorf("invalid expression for rule %q.%q: %w", g.Name, ruleName, err)
			}
		}
		if validateAnnotations {
			if err := notifier.ValidateTemplates(r.Annotations); err != nil {
				return fmt.Errorf("invalid annotations for rule %q.%q: %w", g.Name, ruleName, err)
			}
			if err := notifier.ValidateTemplates(r.Labels); err != nil {
				return fmt.Errorf("invalid labels for rule %q.%q: %w", g.Name, ruleName, err)
			}
		}
	}
	return checkOverflow(g.XXX, fmt.Sprintf("group %q", g.Name))
}

// Rule describes entity that represent either
// recording rule or alerting rule.
type Rule struct {
	ID          uint64
	Record      string             `yaml:"record,omitempty"`
	Alert       string             `yaml:"alert,omitempty"`
	Expr        string             `yaml:"expr"`
	For         utils.PromDuration `yaml:"for"`
	Labels      map[string]string  `yaml:"labels,omitempty"`
	Annotations map[string]string  `yaml:"annotations,omitempty"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (r *Rule) UnmarshalYAML(unmarshal func(interface{}) error) error {
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

// Parse parses rule configs from given file patterns
func Parse(pathPatterns []string, validateAnnotations, validateExpressions bool) ([]Group, error) {
	var fp []string
	for _, pattern := range pathPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("error reading file pattern %s: %w", pattern, err)
		}
		fp = append(fp, matches...)
	}
	errGroup := new(utils.ErrGroup)
	var isExtraFilterLabelsUsed bool
	var groups []Group
	for _, file := range fp {
		uniqueGroups := map[string]struct{}{}
		gr, err := parseFile(file)
		if err != nil {
			errGroup.Add(fmt.Errorf("failed to parse file %q: %w", file, err))
			continue
		}
		for _, g := range gr {
			if err := g.Validate(validateAnnotations, validateExpressions); err != nil {
				errGroup.Add(fmt.Errorf("invalid group %q in file %q: %w", g.Name, file, err))
				continue
			}
			if _, ok := uniqueGroups[g.Name]; ok {
				errGroup.Add(fmt.Errorf("group name %q duplicate in file %q", g.Name, file))
				continue
			}
			uniqueGroups[g.Name] = struct{}{}
			g.File = file
			if len(g.ExtraFilterLabels) > 0 {
				isExtraFilterLabelsUsed = true
			}
			groups = append(groups, g)
		}
	}
	if err := errGroup.Err(); err != nil {
		return nil, err
	}
	if len(groups) < 1 {
		logger.Warnf("no groups found in %s", strings.Join(pathPatterns, ";"))
	}
	if isExtraFilterLabelsUsed {
		logger.Warnf("field `extra_filter_labels` is deprecated - use `params` instead")
	}
	return groups, nil
}

func parseFile(path string) ([]Group, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading alert rule file: %w", err)
	}
	data = envtemplate.Replace(data)
	g := struct {
		Groups []Group `yaml:"groups"`
		// Catches all undefined fields and must be empty after parsing.
		XXX map[string]interface{} `yaml:",inline"`
	}{}
	err = yaml.Unmarshal(data, &g)
	if err != nil {
		return nil, err
	}
	return g.Groups, checkOverflow(g.XXX, "config")
}

func checkOverflow(m map[string]interface{}, ctx string) error {
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
