package netstorage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// storageNodeLabelIndex contains all the values of a label, which the series at a storage node have.
type storageNodeLabelIndex struct {
	s string

	// key is the label name in the form used by storage.TagFilter.Key.
	key string

	values []string
}

// parseStorageNodeLabelIndex parses s in the form `label=value1^^...^^valueN`.
func parseStorageNodeLabelIndex(s string) (*storageNodeLabelIndex, error) {
	label, valuesStr, ok := strings.Cut(s, "=")
	if !ok {
		return nil, fmt.Errorf("missing `=` between label name and values")
	}
	if label == "" {
		return nil, fmt.Errorf("label name cannot be empty")
	}
	values := strings.Split(valuesStr, "^^")
	for i, v := range values {
		if v == "" {
			return nil, fmt.Errorf("label value cannot be empty")
		}
		if slices.Contains(values[:i], v) {
			return nil, fmt.Errorf("duplicate label value %q", v)
		}
	}
	key := label
	if key == "__name__" {
		// storage.TagFilter uses an empty key for the metric name.
		key = ""
	}
	return &storageNodeLabelIndex{
		s:      s,
		key:    key,
		values: values,
	}, nil
}

// mayMatch returns false if the series at the storage node cannot match qfss.
func (li *storageNodeLabelIndex) mayMatch(qfss [][]queryFilter) bool {
	for _, qfs := range qfss {
		for _, v := range li.values {
			if li.matchQueryFilters(v, qfs) {
				return true
			}
		}
	}
	return false
}

// matchQueryFilters returns true if the series with the label value v may match qfs.
func (li *storageNodeLabelIndex) matchQueryFilters(v string, qfs []queryFilter) bool {
	for i := range qfs {
		qf := &qfs[i]
		// Filters for other labels are ignored, since the series at the storage node may have any values for them.
		if string(qf.tf.Key) == li.key && !qf.match(v) {
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
