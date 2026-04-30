package netstorage

// This file provides HTTP-based communication with lower-level vmselect nodes
// in multi-level cluster setups. It replaces the legacy TCP-based RPC protocol
// for the top-level vmselect → lower-level vmselect communication layer,
// while keeping the vmselect → vmstorage TCP communication unchanged.
//
// Usage:
//
//	InitHTTPSelectNodes([]string{"vmselect-lower1:8481", "vmselect-lower2:8481"})
//
// After initialization, the HTTP select nodes are queried in parallel with the
// TCP-based storage nodes via the HTTP*() function variants.

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage/metricsmetadata"
)

var httpSelectNodesBucket atomic.Pointer[httpSelectNodesBucketType]

type httpSelectNodesBucketType struct {
	nodes []*httpSelectNode
	ms    *metrics.Set
}

// InitHTTPSelectNodes initializes HTTP-based vmselect node connections.
// addrs must be in "host:port" format pointing to lower-level vmselect instances.
// MustStopHTTPSelectNodes must be called when these connections are no longer needed.
func InitHTTPSelectNodes(addrs []string) {
	if len(addrs) == 0 {
		return
	}
	ms := metrics.NewSet()
	nodes := make([]*httpSelectNode, 0, len(addrs))
	for _, addr := range addrs {
		u := addr
		n := newHTTPSelectNode(ms, u)
		nodes = append(nodes, n)
	}
	metrics.RegisterSet(ms)
	httpSelectNodesBucket.Store(&httpSelectNodesBucketType{nodes: nodes, ms: ms})
}

// MustStopHTTPSelectNodes stops all HTTP select node connections.
func MustStopHTTPSelectNodes() {
	b := httpSelectNodesBucket.Load()
	if b == nil {
		return
	}
	metrics.UnregisterSet(b.ms, true)
	httpSelectNodesBucket.Store(nil)
}

func getHTTPSelectNodes() []*httpSelectNode {
	b := httpSelectNodesBucket.Load()
	if b == nil {
		return nil
	}
	return b.nodes
}

// httpNodeResult is a result from a single HTTP select node query.
type httpNodeResult struct {
	result any
	err    error
}

type partialStringsResult struct {
	data      []string
	isPartial bool
}

type partialUint64Result struct {
	n         uint64
	isPartial bool
}

type partialMetadataResult struct {
	rows      []*metricsmetadata.Row
	isPartial bool
}

type partialTSDBStatusResult struct {
	status    *storage.TSDBStatus
	isPartial bool
}

// runOnHTTPSelectNodes executes f on each HTTP select node in parallel and returns results.
func runOnHTTPSelectNodes(nodes []*httpSelectNode, f func(sn *httpSelectNode) (any, error)) []httpNodeResult {
	results := make([]httpNodeResult, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		wg.Go(func() {
			r, err := f(node)
			results[i] = httpNodeResult{result: r, err: err}
		})
	}
	wg.Wait()
	return results
}

// LabelNamesFromHTTPNodes returns label names from all configured HTTP select nodes.
func LabelNamesFromHTTPNodes(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, maxLabelNames int, deadline searchutil.Deadline) ([]string, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, false, nil
	}

	requestData := marshalSearchQueryData(sq)

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		labels, partial, err := sn.getLabelNames(qt, requestData, maxLabelNames, deadline)
		if err != nil {
			return nil, err
		}
		return &partialStringsResult{data: labels, isPartial: partial}, nil
	})

	var allLabels []string
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get label names from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialStringsResult)
		if typed.isPartial {
			isPartial = true
		}
		allLabels = append(allLabels, typed.data...)
	}
	allLabels = deduplicateStrings(allLabels)
	sort.Strings(allLabels)
	return allLabels, isPartial, nil
}

// LabelValuesFromHTTPNodes returns label values from all configured HTTP select nodes.
func LabelValuesFromHTTPNodes(qt *querytracer.Tracer, denyPartialResponse bool, labelName string, sq *storage.SearchQuery, maxLabelValues int, deadline searchutil.Deadline) ([]string, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, false, nil
	}

	requestData := marshalSearchQueryData(sq)

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		values, partial, err := sn.getLabelValues(qt, labelName, requestData, maxLabelValues, deadline)
		if err != nil {
			return nil, err
		}
		return &partialStringsResult{data: values, isPartial: partial}, nil
	})

	var allValues []string
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get label values from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialStringsResult)
		if typed.isPartial {
			isPartial = true
		}
		allValues = append(allValues, typed.data...)
	}
	allValues = deduplicateStrings(allValues)
	sort.Strings(allValues)
	return allValues, isPartial, nil
}

// TenantsFromHTTPNodes returns tenants from all configured HTTP select nodes.
func TenantsFromHTTPNodes(qt *querytracer.Tracer, tr storage.TimeRange, deadline searchutil.Deadline) ([]string, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, nil
	}

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		return sn.getTenants(qt, tr, deadline)
	})

	var allTenants []string
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("cannot get tenants from HTTP select node %s: %w", nodes[i].addr(), r.err)
		}
		allTenants = append(allTenants, r.result.([]string)...)
	}
	allTenants = deduplicateStrings(allTenants)
	sort.Strings(allTenants)
	return allTenants, nil
}

// SeriesCountFromHTTPNodes returns the total series count from all HTTP select nodes.
func SeriesCountFromHTTPNodes(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool, deadline searchutil.Deadline) (uint64, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return 0, false, nil
	}

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		n, partial, err := sn.getSeriesCount(qt, accountID, projectID, deadline)
		if err != nil {
			return nil, err
		}
		return &partialUint64Result{n: n, isPartial: partial}, nil
	})

	var total uint64
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return 0, false, fmt.Errorf("cannot get series count from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialUint64Result)
		if typed.isPartial {
			isPartial = true
		}
		total += typed.n
	}
	return total, isPartial, nil
}

// SearchMetricNamesFromHTTPNodes returns metric names from all HTTP select nodes.
func SearchMetricNamesFromHTTPNodes(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, deadline searchutil.Deadline) ([]string, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, false, nil
	}

	requestData := marshalSearchQueryData(sq)

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		names, partial, err := sn.getSearchMetricNames(qt, requestData, deadline)
		if err != nil {
			return nil, err
		}
		return &partialStringsResult{data: names, isPartial: partial}, nil
	})

	var allNames []string
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get metric names from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialStringsResult)
		if typed.isPartial {
			isPartial = true
		}
		allNames = append(allNames, typed.data...)
	}
	allNames = deduplicateStrings(allNames)
	sort.Strings(allNames)
	return allNames, isPartial, nil
}

// DeleteSeriesFromHTTPNodes deletes series from all HTTP select nodes.
func DeleteSeriesFromHTTPNodes(qt *querytracer.Tracer, sq *storage.SearchQuery, deadline searchutil.Deadline) (int, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return 0, nil
	}

	requestData := marshalSearchQueryData(sq)

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		return sn.deleteSeries(qt, requestData, deadline)
	})

	total := 0
	for i, r := range results {
		if r.err != nil {
			return total, fmt.Errorf("cannot delete series from HTTP select node %s: %w", nodes[i].addr(), r.err)
		}
		total += r.result.(int)
	}
	return total, nil
}

// RegisterMetricNamesOnHTTPNodes registers metric names on all HTTP select nodes.
func RegisterMetricNamesOnHTTPNodes(qt *querytracer.Tracer, mrs []storage.MetricRow, deadline searchutil.Deadline) error {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil
	}

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		return nil, sn.registerMetricNames(qt, mrs, deadline)
	})

	for i, r := range results {
		if r.err != nil {
			return fmt.Errorf("cannot register metric names on HTTP select node %s: %w", nodes[i].addr(), r.err)
		}
	}
	return nil
}

// TagValueSuffixesFromHTTPNodes returns tag value suffixes from all HTTP select nodes.
func TagValueSuffixesFromHTTPNodes(qt *querytracer.Tracer, accountID, projectID uint32, denyPartialResponse bool,
	tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxSuffixes int, deadline searchutil.Deadline,
) ([]string, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, false, nil
	}

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		suffixes, partial, err := sn.getTagValueSuffixes(qt, accountID, projectID, tr, tagKey, tagValuePrefix, delimiter, maxSuffixes, deadline)
		if err != nil {
			return nil, err
		}
		return &partialStringsResult{data: suffixes, isPartial: partial}, nil
	})

	var allSuffixes []string
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get tag value suffixes from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialStringsResult)
		if typed.isPartial {
			isPartial = true
		}
		allSuffixes = append(allSuffixes, typed.data...)
	}
	allSuffixes = deduplicateStrings(allSuffixes)
	sort.Strings(allSuffixes)
	return allSuffixes, isPartial, nil
}

// GetMetadataFromHTTPNodes returns metrics metadata from HTTP select nodes.
func GetMetadataFromHTTPNodes(qt *querytracer.Tracer, tt *storage.TenantToken, denyPartialResponse bool, limit int, metricName string, deadline searchutil.Deadline) ([]*metricsmetadata.Row, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return nil, false, nil
	}

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		rows, partial, err := sn.getMetricsMetadata(qt, tt, limit, metricName, deadline)
		if err != nil {
			return nil, err
		}
		return &partialMetadataResult{rows: rows, isPartial: partial}, nil
	})

	var allRows []*metricsmetadata.Row
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get metadata from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialMetadataResult)
		if typed.isPartial {
			isPartial = true
		}
		allRows = append(allRows, typed.rows...)
	}
	return allRows, isPartial, nil
}

// TSDBStatusFromHTTPNodes returns merged TSDB status from all HTTP select nodes.
func TSDBStatusFromHTTPNodes(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery, focusLabel string, topN int, deadline searchutil.Deadline) (*storage.TSDBStatus, bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return &storage.TSDBStatus{}, false, nil
	}

	requestData := marshalSearchQueryData(sq)

	results := runOnHTTPSelectNodes(nodes, func(sn *httpSelectNode) (any, error) {
		status, partial, err := sn.getTSDBStatus(qt, requestData, focusLabel, topN, deadline)
		if err != nil {
			return nil, err
		}
		return &partialTSDBStatusResult{status: status, isPartial: partial}, nil
	})

	var statuses []*storage.TSDBStatus
	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return nil, false, fmt.Errorf("cannot get tsdb status from HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		typed := r.result.(*partialTSDBStatusResult)
		if typed.isPartial {
			isPartial = true
		}
		statuses = append(statuses, typed.status)
	}
	status := mergeTSDBStatuses(statuses, topN)
	return status, isPartial, nil
}

// ProcessSearchQueryOnHTTPNodes runs search blocks from all HTTP select nodes through processBlock.
// Returns whether the result is partial and any error.
func ProcessSearchQueryOnHTTPNodes(qt *querytracer.Tracer, denyPartialResponse bool, sq *storage.SearchQuery,
	processBlock func(rawBlock []byte, workerID uint) error, deadline searchutil.Deadline,
) (bool, error) {
	nodes := getHTTPSelectNodes()
	if len(nodes) == 0 {
		return false, nil
	}

	requestData := marshalSearchQueryData(sq)

	type nodeResult struct {
		isPartial bool
		err       error
	}
	results := make([]nodeResult, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		workerID := uint(i)
		wg.Go(func() {
			partial, err := node.processSearchQuery(qt, requestData, processBlock, workerID, deadline)
			results[i] = nodeResult{isPartial: partial, err: err}
		})
	}
	wg.Wait()

	isPartial := false
	for i, r := range results {
		if r.err != nil {
			if denyPartialResponse {
				return false, fmt.Errorf("cannot search on HTTP select node %s: %w", nodes[i].addr(), r.err)
			}
			isPartial = true
			continue
		}
		if r.isPartial {
			isPartial = true
		}
	}
	return isPartial, nil
}

// marshalSearchQueryData marshals a SearchQuery into the binary requestData format
// for HTTP inter-node communication.
//
// Wire format: uint32(numTenants) + [TenantToken(8B)] * numTenants + MarshalWithoutTenant
//
// This preserves all tenant tokens for multi-tenant queries, so the receiving
// vmselect can iterate over them just like the TCP path does via execSearchQuery.
func marshalSearchQueryData(sq *storage.SearchQuery) []byte {
	tokens := sq.TenantTokens
	if len(tokens) == 0 {
		tokens = []storage.TenantToken{{}}
	}

	var dst []byte
	dst = encoding.MarshalUint32(dst, uint32(len(tokens)))
	for _, tt := range tokens {
		dst = tt.Marshal(dst)
	}
	dst = sq.MarshalWithoutTenant(dst)
	return dst
}
