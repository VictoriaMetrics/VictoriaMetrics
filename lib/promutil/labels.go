package promutil

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

// Labels contains Prometheus labels.
type Labels struct {
	Labels []prompbmarshal.Label
}

// NewLabels returns Labels with the given capacity.
func NewLabels(capacity int) *Labels {
	return &Labels{
		Labels: make([]prompbmarshal.Label, 0, capacity),
	}
}

// NewLabelsFromMap returns Labels generated from m.
func NewLabelsFromMap(m map[string]string) *Labels {
	var x Labels
	x.InitFromMap(m)
	return &x
}

// MarshalYAML implements yaml.Marshaler interface.
func (x *Labels) MarshalYAML() (any, error) {
	m := x.ToMap()
	return m, nil
}

// UnmarshalYAML implements yaml.Unmarshaler interface.
func (x *Labels) UnmarshalYAML(unmarshal func(any) error) error {
	var m map[string]string
	if err := unmarshal(&m); err != nil {
		return err
	}
	x.InitFromMap(m)
	return nil
}

// MarshalJSON returns JSON representation for x.
func (x *Labels) MarshalJSON() ([]byte, error) {
	m := x.ToMap()
	return json.Marshal(m)
}

// UnmarshalJSON unmarshals JSON from data.
func (x *Labels) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	x.InitFromMap(m)
	return nil
}

// InitFromMap initializes x from m.
func (x *Labels) InitFromMap(m map[string]string) {
	x.Reset()
	for name, value := range m {
		x.Add(name, value)
	}
	x.Sort()
}

// ToMap returns a map for the given labels x.
func (x *Labels) ToMap() map[string]string {
	labels := x.GetLabels()
	m := make(map[string]string, len(labels))
	for _, label := range labels {
		m[label.Name] = label.Value
	}
	return m
}

// GetLabels returns the list of labels from x.
func (x *Labels) GetLabels() []prompbmarshal.Label {
	if x == nil {
		return nil
	}
	return x.Labels
}

// String returns string representation of x.
func (x *Labels) String() string {
	labels := x.GetLabels()
	// Calculate the required memory for storing serialized labels.
	n := 2 // for `{...}`
	for _, label := range labels {
		n += len(label.Name) + len(label.Value)
		n += 4 // for `="...",`
	}
	b := make([]byte, 0, n)
	b = append(b, '{')
	for i, label := range labels {
		b = append(b, label.Name...)
		b = append(b, '=')
		b = strconv.AppendQuote(b, label.Value)
		if i+1 < len(labels) {
			b = append(b, ',')
		}
	}
	b = append(b, '}')
	return bytesutil.ToUnsafeString(b)
}

// Reset resets x.
func (x *Labels) Reset() {
	clear(x.Labels)
	x.Labels = x.Labels[:0]
}

// Clone returns a clone of x.
func (x *Labels) Clone() *Labels {
	srcLabels := x.GetLabels()
	labels := append([]prompbmarshal.Label{}, srcLabels...)
	return &Labels{
		Labels: labels,
	}
}

// Sort sorts x labels in alphabetical order of their names.
func (x *Labels) Sort() {
	if !sort.IsSorted(x) {
		sort.Sort(x)
	}
}

// SortStable sorts x labels in alphabetical order of their name using stable sort.
func (x *Labels) SortStable() {
	if !sort.IsSorted(x) {
		sort.Stable(x)
	}
}

// Len returns the number of labels in x.
func (x *Labels) Len() int {
	labels := x.GetLabels()
	return len(labels)
}

// Less compares label names at i and j index.
func (x *Labels) Less(i, j int) bool {
	labels := x.Labels
	return labels[i].Name < labels[j].Name
}

// Swap swaps labels at i and j index.
func (x *Labels) Swap(i, j int) {
	labels := x.Labels
	labels[i], labels[j] = labels[j], labels[i]
}

// Add adds name=value label to x.
func (x *Labels) Add(name, value string) {
	x.Labels = append(x.Labels, prompbmarshal.Label{
		Name:  name,
		Value: value,
	})
}

// AddFrom adds src labels to x.
func (x *Labels) AddFrom(src *Labels) {
	for _, label := range src.GetLabels() {
		x.Add(label.Name, label.Value)
	}
}

// Get returns value for label with the given name.
func (x *Labels) Get(name string) string {
	labels := x.GetLabels()
	for _, label := range labels {
		if label.Name == name {
			return label.Value
		}
	}
	return ""
}

// Set label value for label with given name
// If the label with the given name doesn't exist, it adds as the new label
func (x *Labels) Set(name, value string) {
	if name == "" || value == "" {
		return
	}
	labels := x.GetLabels()
	for i, label := range labels {
		if label.Name == name {
			labels[i].Value = value
			return
		}
	}
	x.Add(name, value)
}

// InternStrings interns all the strings used in x labels.
func (x *Labels) InternStrings() {
	labels := x.GetLabels()
	for _, label := range labels {
		label.Name = bytesutil.InternString(label.Name)
		label.Value = bytesutil.InternString(label.Value)
	}
}

// RemoveDuplicates removes labels with duplicate names.
func (x *Labels) RemoveDuplicates() {
	if x.Len() < 2 {
		return
	}
	// Remove duplicate labels if any.
	// Stable sorting is needed in order to preserve the order for labels with identical names.
	// This is needed in order to remove labels with duplicate names other than the last one.
	x.SortStable()
	labels := x.Labels
	prevName := labels[0].Name
	hasDuplicateLabels := false
	for _, label := range labels[1:] {
		if label.Name == prevName {
			hasDuplicateLabels = true
			break
		}
		prevName = label.Name
	}
	if !hasDuplicateLabels {
		return
	}
	prevName = labels[0].Name
	tmp := labels[:1]
	for _, label := range labels[1:] {
		if label.Name == prevName {
			tmp[len(tmp)-1] = label
		} else {
			tmp = append(tmp, label)
			prevName = label.Name
		}
	}
	clear(labels[len(tmp):])
	x.Labels = tmp
}

// RemoveMetaLabels removes all the `__meta_` labels from x.
//
// See https://www.robustperception.io/life-of-a-label for details.
func (x *Labels) RemoveMetaLabels() {
	src := x.Labels
	dst := x.Labels[:0]
	for _, label := range src {
		if strings.HasPrefix(label.Name, "__meta_") {
			continue
		}
		dst = append(dst, label)
	}
	clear(src[len(dst):])
	x.Labels = dst
}

// RemoveLabelsWithDoubleUnderscorePrefix removes labels with "__" prefix from x.
func (x *Labels) RemoveLabelsWithDoubleUnderscorePrefix() {
	src := x.Labels
	dst := x.Labels[:0]
	for _, label := range src {
		name := label.Name
		if strings.HasPrefix(name, "__") {
			continue
		}
		dst = append(dst, label)
	}
	clear(src[len(dst):])
	x.Labels = dst
}

// GetLabels returns and empty Labels instance from the pool.
//
// The returned Labels instance must be returned to pool via PutLabels() when no longer needed.
func GetLabels() *Labels {
	v := labelsPool.Get()
	if v == nil {
		return &Labels{}
	}
	return v.(*Labels)
}

// PutLabels returns x, which has been obtained via GetLabels(), to the pool.
//
// The x mustn't be used after returning to the pool.
func PutLabels(x *Labels) {
	x.Reset()
	labelsPool.Put(x)
}

var labelsPool sync.Pool

// MustNewLabelsFromString creates labels from s, which can have the form `metric{labels}`.
//
// This function must be used only in tests. Use NewLabelsFromString in production code.
func MustNewLabelsFromString(metricWithLabels string) *Labels {
	labels, err := NewLabelsFromString(metricWithLabels)
	if err != nil {
		logger.Panicf("BUG: cannot parse %q: %s", metricWithLabels, err)
	}
	return labels
}

// NewLabelsFromString creates labels from s, which can have the form `metric{labels}`.
//
// This function must be used only in non performance-critical code, since it allocates too much
func NewLabelsFromString(metricWithLabels string) (*Labels, error) {
	stripDummyMetric := false
	if strings.HasPrefix(metricWithLabels, "{") {
		// Add a dummy metric name, since the parser needs it
		metricWithLabels = "dummy_metric" + metricWithLabels
		stripDummyMetric = true
	}
	// add a value to metricWithLabels, so it could be parsed by prometheus protocol parser.
	s := metricWithLabels + " 123"
	var rows prometheus.Rows
	var err error
	rows.UnmarshalWithErrLogger(s, func(s string) {
		err = fmt.Errorf("error during metric parse: %s", s)
	})
	if err != nil {
		return nil, err
	}
	if len(rows.Rows) != 1 {
		return nil, fmt.Errorf("unexpected number of rows parsed; got %d; want 1", len(rows.Rows))
	}
	r := rows.Rows[0]
	var x Labels
	if !stripDummyMetric {
		x.Add("__name__", r.Metric)
	}
	for _, tag := range r.Tags {
		x.Add(tag.Key, tag.Value)
	}
	return &x, nil
}
