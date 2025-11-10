package promrelabel

import (
	"encoding/json"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/metricsql"
)

// IfExpression represents PromQL-like label filters such as `metric_name{filters...}`.
//
// It may contain either a single filter or multiple filters, which are executed with `or` operator.
//
// Examples:
//
// if: 'foo{bar="baz"}'
//
// if:
// - 'foo{bar="baz"}'
// - '{x=~"y"}'
type IfExpression struct {
	ies    []*ifExpression
	lfSize int
}

// Match returns true if labels match at least a single label filter inside ie.
//
// Match returns true for empty ie.
func (ie *IfExpression) Match(labels []prompb.Label) bool {
	if ie == nil || len(ie.ies) == 0 {
		return true
	}
	for _, ie := range ie.ies {
		if ie.Match(labels, nil) {
			return true
		}
	}
	return false
}

// MatchWithFilters returns true if labels match at least a single label filter inside ie.
// takes in bloom filter labels to speed up matching.
// MatchWithFilters returns true for empty ie.
func (ie *IfExpression) MatchWithFilters(labels []prompb.Label, lBf *BloomFilter) bool {
	if ie == nil || len(ie.ies) == 0 {
		return true
	}
	for _, ie := range ie.ies {
		if ie.Match(labels, lBf) {
			return true
		}
	}
	return false
}

// Parse parses ie from s.
func (ie *IfExpression) Parse(s string) error {
	ieLocal, err := newIfExpression(s)
	if err != nil {
		return err
	}
	ie.ies = []*ifExpression{ieLocal}
	return nil
}

// UnmarshalJSON unmarshals ie from JSON data.
func (ie *IfExpression) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	return ie.unmarshalFromInterface(v)
}

// MarshalJSON marshals ie to JSON.
func (ie *IfExpression) MarshalJSON() ([]byte, error) {
	if ie == nil || len(ie.ies) == 0 {
		return nil, nil
	}
	if len(ie.ies) == 1 {
		return json.Marshal(ie.ies[0])
	}
	return json.Marshal(ie.ies)
}

// UnmarshalYAML unmarshals ie from YAML passed to f.
func (ie *IfExpression) UnmarshalYAML(f func(any) error) error {
	var v any
	if err := f(&v); err != nil {
		return fmt.Errorf("cannot unmarshal `match` option: %w", err)
	}
	return ie.unmarshalFromInterface(v)
}

// Len returns the number of labelFilters in ie
func (ie *IfExpression) Len() int {
	if ie == nil {
		return 0
	}
	//we already computed it
	if len(ie.ies) != 0 && ie.lfSize != 0 {
		return ie.lfSize
	}
	total := 0
	for _, sie := range ie.ies {
		total += sie.Len()
	}
	ie.lfSize = total
	return total
}

func (ie *IfExpression) unmarshalFromInterface(v any) error {
	ies := ie.ies[:0]
	switch t := v.(type) {
	case string:
		ieLocal, err := newIfExpression(t)
		if err != nil {
			return fmt.Errorf("unexpected `match` option: %w", err)
		}
		ies = append(ies, ieLocal)
	case []any:
		for _, x := range t {
			s, ok := x.(string)
			if !ok {
				return fmt.Errorf("unexpected `match` item type; got %#v; want string", x)
			}
			ieLocal, err := newIfExpression(s)
			if err != nil {
				return fmt.Errorf("unexpected `match` item: %w", err)
			}
			ies = append(ies, ieLocal)
		}
	default:
		return fmt.Errorf("unexpected `match` type; got %#v; want string or an array of strings", t)
	}
	ie.ies = ies
	return nil
}

// MarshalYAML marshals ie to YAML
func (ie *IfExpression) MarshalYAML() (any, error) {
	if ie == nil || len(ie.ies) == 0 {
		return nil, nil
	}
	if len(ie.ies) == 1 {
		return ie.ies[0].MarshalYAML()
	}
	a := make([]string, 0, len(ie.ies))
	for _, ieLocal := range ie.ies {
		v, err := ieLocal.MarshalYAML()
		if err != nil {
			logger.Panicf("BUG: unexpected error: %s", err)
		}
		s := v.(string)
		a = append(a, s)
	}
	return a, nil
}

func newIfExpression(s string) (*ifExpression, error) {
	var ie ifExpression
	if err := ie.Parse(s); err != nil {
		return nil, err
	}
	return &ie, nil
}

// String returns string representation of ie.
func (ie *IfExpression) String() string {
	if ie == nil {
		return "{}"
	}
	if len(ie.ies) == 1 {
		return ie.ies[0].String()
	}

	b := append([]byte{}, ie.ies[0].String()...)
	for _, e := range ie.ies[1:] {
		b = append(b, ',')
		b = append(b, e.String()...)
	}
	return string(b)
}

type ifExpression struct {
	s      string
	lfss   [][]*labelFilter
	lfsshv [][]uint64
	lfsshl [][]uint64
}

func (ie *ifExpression) String() string {
	if ie == nil {
		return ""
	}
	return ie.s
}

func (ie *ifExpression) Parse(s string) error {
	expr, err := metricsql.Parse(s)
	if err != nil {
		return err
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return fmt.Errorf("expecting series selector; got %q", expr.AppendString(nil))
	}
	lfss, err := metricExprToLabelFilterss(me)
	if err != nil {
		return fmt.Errorf("cannot parse series selector: %w", err)
	}

	lfsshv := make([][]uint64, len(lfss))
	lfsshl := make([][]uint64, len(lfss))
	for i, lfs := range lfss {
		var lh []uint64
		var vh []uint64
		for _, lf := range lfs {
			lh = append(lh, lf.labelHash...)
			vh = append(vh, lf.valueHash...)
		}
		lfsshv[i] = vh
		lfsshl[i] = lh
	}
	ie.s = s
	ie.lfss = lfss
	ie.lfsshv = lfsshv
	ie.lfsshl = lfsshl
	return nil
}

// UnmarshalJSON unmarshals ie from JSON data.
func (ie *ifExpression) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return ie.Parse(s)
}

// MarshalJSON marshals ie to JSON.
func (ie *ifExpression) MarshalJSON() ([]byte, error) {
	return json.Marshal(ie.s)
}

// UnmarshalYAML unmarshals ie from YAML passed to f.
func (ie *ifExpression) UnmarshalYAML(f func(any) error) error {
	var s string
	if err := f(&s); err != nil {
		return fmt.Errorf("cannot unmarshal `if` option: %w", err)
	}
	if err := ie.Parse(s); err != nil {
		return fmt.Errorf("cannot parse `if` series selector: %w", err)
	}
	return nil
}

// MarshalYAML marshals ie to YAML.
func (ie *ifExpression) MarshalYAML() (any, error) {
	return ie.s, nil
}

// Match returns true if ie matches the given labels.
func (ie *ifExpression) Match(labels []prompb.Label, lbf *BloomFilter) bool {
	if ie == nil {
		return true
	}
	for i, lfs := range ie.lfss {
		//if we don't contain all hashes for this set of label filters, skip it
		if lbf != nil {
			//check value hashes first since they're more specific
			if !lbf.ContainsAll(ie.lfsshv[i]) {
				continue
			}
			if !lbf.ContainsAll(ie.lfsshl[i]) {
				continue
			}
		}
		if matchLabelFilters(lfs, labels) {
			return true
		}
	}
	return false
}

// Len returns the number of labelFilters in ie
func (ie *ifExpression) Len() int {
	if ie == nil {
		return 0
	}
	total := 0
	for _, lfs := range ie.lfss {
		total += len(lfs)
	}
	return total
}

func matchLabelFilters(lfs []*labelFilter, labels []prompb.Label) bool {
	for _, lf := range lfs {
		if !lf.match(labels) {
			return false
		}
	}
	return true
}

func metricExprToLabelFilterss(me *metricsql.MetricExpr) ([][]*labelFilter, error) {
	lfssNew := make([][]*labelFilter, len(me.LabelFilterss))
	for i, lfs := range me.LabelFilterss {
		lfsNew := make([]*labelFilter, len(lfs))
		for j := range lfs {
			lf, err := newLabelFilter(&lfs[j])
			if err != nil {
				return nil, fmt.Errorf("cannot parse %s: %w", me.AppendString(nil), err)
			}
			lfsNew[j] = lf
		}
		lfssNew[i] = lfsNew
	}
	return lfssNew, nil
}

// labelFilter contains PromQL filter for `{label op "value"}`
type labelFilter struct {
	label     string
	op        string
	value     string
	labelHash []uint64 //pre-computed hashes for use in bloom filter
	valueHash []uint64

	// re contains compiled regexp for `=~` and `!~` op.
	re *regexutil.PromRegex
}

func newLabelFilter(mlf *metricsql.LabelFilter) (*labelFilter, error) {
	lf := &labelFilter{
		label: toCanonicalLabelName(mlf.Label),
		op:    getFilterOp(mlf),
		value: mlf.Value,
	}
	if lf.op == "=~" || lf.op == "!~" {
		re, err := regexutil.NewPromRegex(lf.value)
		if err != nil {
			return nil, fmt.Errorf("cannot parse regexp for %s: %w", mlf.AppendString(nil), err)
		}
		lf.re = re
	}

	labelNameForHash := mlf.Label
	//direct equality, for a present value, we can require the exact values
	if lf.op == "=" && lf.value != "" {
		lf.labelHash = AppendTokensHashes(lf.labelHash, []string{labelNameForHash})
		lf.valueHash = AppendTokensHashes(lf.valueHash, []string{lf.value})
	}
	//regex equality we can require some components
	// label name is required if the regex doesn't match empty string
	// can also require the prefix if the regex is only the prefix
	if lf.op == "=~" && lf.value != "" {
		lf.valueHash = AppendTokensHashes(lf.valueHash, lf.re.GetHashableStrings())
		// also require label name if the regex doesn't match empty string
		if !lf.re.MatchesEmpty() {
			lf.labelHash = AppendTokensHashes(lf.labelHash, []string{labelNameForHash})
		}
	}
	return lf, nil
}

func (lf *labelFilter) match(labels []prompb.Label) bool {
	switch lf.op {
	case "=":
		return lf.equalValue(labels)
	case "!=":
		return !lf.equalValue(labels)
	case "=~":
		return lf.matchRegexp(labels)
	case "!~":
		return !lf.matchRegexp(labels)
	default:
		logger.Panicf("BUG: unexpected operation for label filter: %s", lf.op)
	}
	return false
}

func (lf *labelFilter) equalNameValue(labels []prompb.Label) bool {
	for _, label := range labels {
		if label.Name == "__name__" {
			return label.Value == lf.value
		}
	}
	return false
}

func (lf *labelFilter) equalValue(labels []prompb.Label) bool {
	if lf.label == "" {
		return lf.equalNameValue(labels)
	}
	labelNameMatches := 0
	for _, label := range labels {
		if label.Name != lf.label {
			continue
		}
		labelNameMatches++
		if label.Value == lf.value {
			return true
		}
	}
	if labelNameMatches == 0 {
		// Special case for {non_existing_label=""}, which matches anything except of non-empty non_existing_label
		return lf.value == ""
	}
	return false
}

func (lf *labelFilter) matchRegexp(labels []prompb.Label) bool {
	labelNameMatches := 0
	for _, label := range labels {
		if toCanonicalLabelName(label.Name) != lf.label {
			continue
		}
		labelNameMatches++
		if lf.re.MatchString(label.Value) {
			return true
		}
	}
	if labelNameMatches == 0 {
		// Special case for {non_existing_label=~"something|"}, which matches empty non_existing_label
		return lf.re.MatchString("")
	}
	return false
}

func toCanonicalLabelName(labelName string) string {
	if labelName == "__name__" {
		return ""
	}
	return labelName
}

func getFilterOp(mlf *metricsql.LabelFilter) string {
	if mlf.IsNegative {
		if mlf.IsRegexp {
			return "!~"
		}
		return "!="
	}
	if mlf.IsRegexp {
		return "=~"
	}
	return "="
}
