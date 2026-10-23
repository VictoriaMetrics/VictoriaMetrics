package netstorage

import (
	"fmt"
	"slices"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func getStorageNodesForTagFilterss(qt *querytracer.Tracer, tfss [][]storage.TagFilter) []*storageNode {
	snb := getStorageNodesBucket()
	if len(snb.labelIndexes) == 0 || len(tfss) == 0 {
		return snb.sns
	}
	tfmss := newTagFilterMatchers(tfss)
	sns := make([]*storageNode, 0, len(snb.sns))
	for i, sn := range snb.sns {
		snli := snb.labelIndexes[i]
		if snli == nil || snli.mayMatchFilterss(tfmss) {
			sns = append(sns, sn)
			continue
		}
		qt.Printf("skip vmstorage %s, since its -storageNodeLabelIndex cannot match the query", sn.connPool.Addr())
	}
	return sns
}

type storageNodeLabelIndex map[string][]string

// mustParseStorageNodeLabelIndexes returns -storageNodeLabelIndex values keyed by the corresponding -storageNode addrs.
func mustParseStorageNodeLabelIndexes(addrs, labelIndexes []string) map[string]storageNodeLabelIndex {
	if len(labelIndexes) == 0 {
		return nil
	}
	if len(labelIndexes) != len(addrs) {
		logger.Fatalf("the number of -storageNodeLabelIndex args must match the number of -storageNode args; got %d vs %d", len(labelIndexes), len(addrs))
	}
	snlis := make(map[string]storageNodeLabelIndex, len(addrs))
	for i, addr := range addrs {
		key := normalizeStorageNodeAddr(addr)
		if _, ok := snlis[key]; ok {
			logger.Fatalf("duplicate -storageNode=%q cannot be used with -storageNodeLabelIndex", addr)
		}
		s := labelIndexes[i]
		if s == "" {
			snlis[key] = nil
			continue
		}
		snli, err := parseStorageNodeLabelIndex(s)
		if err != nil {
			logger.Fatalf("cannot parse -storageNodeLabelIndex=%q for -storageNode=%q: %s", s, addr, err)
		}
		snlis[key] = snli
	}
	return snlis
}

// normalizeStorageNodeAddr adds the default port to addr, so -storageNode=host and the discovered host:8401 have the same key.
func normalizeStorageNodeAddr(addr string) string {
	groupName, hostPort := netutil.ParseGroupAddr(addr)
	normalizedAddr, err := netutil.NormalizeAddr(hostPort, 8401)
	if err != nil {
		// newStorageNode reports the invalid addr
		return addr
	}
	if groupName == "" {
		return normalizedAddr
	}
	return groupName + "/" + normalizedAddr
}

// getStorageNodeLabelIndexes returns snlis items for addrs in the same order as addrs.
//
// It returns nil if snlis contains no indexes, so queries aren't checked against indexes.
func getStorageNodeLabelIndexes(addrs []string, snlis map[string]storageNodeLabelIndex) []storageNodeLabelIndex {
	var result []storageNodeLabelIndex
	for i, addr := range addrs {
		snli := snlis[normalizeStorageNodeAddr(addr)]
		if snli == nil {
			continue
		}
		if result == nil {
			result = make([]storageNodeLabelIndex, len(addrs))
		}
		result[i] = snli
	}
	return result
}

func parseStorageNodeLabelIndex(s string) (storageNodeLabelIndex, error) {
	snli := make(storageNodeLabelIndex)
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
		snli[name] = append(snli[name], value)
	}
	return snli, nil
}

func (snli storageNodeLabelIndex) mayMatchFilterss(tfmss [][]tagFilterMatcher) bool {
	return slices.ContainsFunc(tfmss, snli.mayMatchFilters)
}

func (snli storageNodeLabelIndex) mayMatchFilters(tfms []tagFilterMatcher) bool {
	for i := range tfms {
		tfm := &tfms[i]
		values, ok := snli[string(tfm.tf.Key)]
		if ok && !slices.ContainsFunc(values, tfm.match) {
			return false
		}
	}
	return true
}

type tagFilterMatcher struct {
	tf *storage.TagFilter

	// re is compiled from tf.Value on the first match and is reused for all storage nodes in the query.
	// It stays nil for non-regexp filters and for filters on labels without index.
	re *regexutil.PromRegex
}

func newTagFilterMatchers(tfss [][]storage.TagFilter) [][]tagFilterMatcher {
	tfmss := make([][]tagFilterMatcher, len(tfss))
	for i, tfs := range tfss {
		tfms := make([]tagFilterMatcher, len(tfs))
		for j := range tfs {
			tfms[j].tf = &tfs[j]
		}
		tfmss[i] = tfms
	}
	return tfmss
}

func (tfm *tagFilterMatcher) match(v string) bool {
	tf := tfm.tf
	if !tf.IsRegexp {
		return (v == string(tf.Value)) != tf.IsNegative
	}
	if tfm.re == nil {
		re, err := regexutil.NewPromRegex(string(tf.Value))
		if err != nil {
			// let the storage node return the error
			return true
		}
		tfm.re = re
	}
	return tfm.re.MatchString(v) != tf.IsNegative
}
