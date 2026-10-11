package netstorage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// storageNodeLabelIndex contains all the values per each indexed label, which the series at a storage node have.
type storageNodeLabelIndex struct {
	s string

	labels []indexedLabel
}

type indexedLabel struct {
	// key is the label name in the form used by storage.TagFilter.Key.
	key string

	values []string
}

// parseStorageNodeLabelIndex parses s in the form `label1=value1^^...^^labelN=valueN`.
func parseStorageNodeLabelIndex(s string) (*storageNodeLabelIndex, error) {
	var labels []indexedLabel
	for pair := range strings.SplitSeq(s, "^^") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("missing `=` between label name and value in %q", pair)
		}
		if name == "" {
			return nil, fmt.Errorf("label name cannot be empty in %q", pair)
		}
		if value == "" {
			return nil, fmt.Errorf("label value cannot be empty in %q", pair)
		}
		key := name
		if key == "__name__" {
			// storage.TagFilter uses an empty key for the metric name.
			key = ""
		}
		idx := slices.IndexFunc(labels, func(l indexedLabel) bool { return l.key == key })
		if idx < 0 {
			labels = append(labels, indexedLabel{key: key})
			idx = len(labels) - 1
		}
		l := &labels[idx]
		if slices.Contains(l.values, value) {
			return nil, fmt.Errorf("duplicate label pair %q", pair)
		}
		l.values = append(l.values, value)
	}
	return &storageNodeLabelIndex{
		s:      s,
		labels: labels,
	}, nil
}

// mayMatch returns false if the series at the storage node cannot match qfss.
func (li *storageNodeLabelIndex) mayMatch(qfss [][]queryFilter) bool {
	for _, qfs := range qfss {
		if li.mayMatchQueryFilters(qfs) {
			return true
		}
	}
	return false
}

// mayMatchQueryFilters returns true if every indexed label has a value, which matches qfs.
func (li *storageNodeLabelIndex) mayMatchQueryFilters(qfs []queryFilter) bool {
	for i := range li.labels {
		l := &li.labels[i]
		if !slices.ContainsFunc(l.values, func(v string) bool { return l.matchQueryFilters(v, qfs) }) {
			return false
		}
	}
	return true
}

// matchQueryFilters returns true if the series with the label value v may match qfs.
func (l *indexedLabel) matchQueryFilters(v string, qfs []queryFilter) bool {
	for i := range qfs {
		qf := &qfs[i]
		// Filters for other labels are ignored, since they are checked against other indexed labels.
		if string(qf.tf.Key) == l.key && !qf.match(v) {
			return false
		}
	}
	return true
}

// queryFilter is a storage.TagFilter from the search query with lazily compiled regexp.
type queryFilter struct {
	tf *storage.TagFilter

	// re is compiled on the first match, since most of query regexp filters are usually applied to labels other than the indexed one.
	re *regexutil.PromRegex
}

func (qf *queryFilter) match(v string) bool {
	tf := qf.tf
	if !tf.IsRegexp {
		return (v == string(tf.Value)) != tf.IsNegative
	}
	if qf.re == nil {
		re, err := regexutil.NewPromRegex(string(tf.Value))
		if err != nil {
			// Query the storage node, so it returns the error to the caller.
			return true
		}
		qf.re = re
	}
	return qf.re.MatchString(v) != tf.IsNegative
}

func newQueryFilters(tfss [][]storage.TagFilter) [][]queryFilter {
	qfss := make([][]queryFilter, len(tfss))
	for i, tfs := range tfss {
		qfs := make([]queryFilter, len(tfs))
		for j := range tfs {
			qfs[j].tf = &tfs[j]
		}
		qfss[i] = qfs
	}
	return qfss
}

// filterStorageNodes returns sns, which may contain series matching tfss according to their -storageNodeLabelIndex,
// and the number of skipped nodes per group.
func filterStorageNodes(qt *querytracer.Tracer, sns []*storageNode, tfss [][]storage.TagFilter) ([]*storageNode, map[*storageNodesGroup]int) {
	if len(tfss) == 0 || !slices.ContainsFunc(sns, func(sn *storageNode) bool { return sn.labelIndex != nil }) {
		return sns, nil
	}
	qfss := newQueryFilters(tfss)
	snsFiltered := make([]*storageNode, 0, len(sns))
	var skippedPerGroup map[*storageNodesGroup]int
	for _, sn := range sns {
		if sn.labelIndex == nil || sn.labelIndex.mayMatch(qfss) {
			snsFiltered = append(snsFiltered, sn)
			continue
		}
		qt.Printf("skip vmstorage %s, since -storageNodeLabelIndex=%s cannot match the query", sn.connPool.Addr(), sn.labelIndex.s)
		if skippedPerGroup == nil {
			skippedPerGroup = make(map[*storageNodesGroup]int)
		}
		skippedPerGroup[sn.group]++
	}
	return snsFiltered, skippedPerGroup
}
