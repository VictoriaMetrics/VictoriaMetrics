package storage

import (
	"container/heap"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// TSID is unique id for a time series.
//
// Time series blocks are sorted by TSID.
//
// All the fields except MetricID are optional. They exist solely for better
// grouping of related metrics.
// It is OK if their meaning differ from their naming.
type TSID struct {
	// MetricGroupID is the id of metric group.
	//
	// MetricGroupID must be unique.
	//
	// Metric group contains metrics with the identical name like
	// 'memory_usage', 'http_requests', but with different
	// labels. For instance, the following metrics belong
	// to a metric group 'memory_usage':
	//
	//   memory_usage{datacenter="foo1", job="bar1", instance="baz1:1234"}
	//   memory_usage{datacenter="foo1", job="bar1", instance="baz2:1234"}
	//   memory_usage{datacenter="foo1", job="bar2", instance="baz1:1234"}
	//   memory_usage{datacenter="foo2", job="bar1", instance="baz2:1234"}
	MetricGroupID uint64

	// JobID is the id of an individual job (aka service).
	//
	// JobID must be unique.
	//
	// Service may consist of multiple instances.
	// See https://prometheus.io/docs/concepts/jobs_instances/ for details.
	JobID uint32

	// InstanceID is the id of an instance (aka process).
	//
	// InstanceID must be unique.
	//
	// See https://prometheus.io/docs/concepts/jobs_instances/ for details.
	InstanceID uint32

	// MetricID is the unique id of the metric (time series).
	//
	// All the other TSID fields may be obtained by MetricID.
	MetricID uint64
}

// marshaledTSIDSize is the size of marshaled TSID.
var marshaledTSIDSize = func() int {
	var t TSID
	dst := t.Marshal(nil)
	return len(dst)
}()

// Marshal appends marshaled t to dst and returns the result.
func (t *TSID) Marshal(dst []byte) []byte {
	dst = encoding.MarshalUint64(dst, t.MetricGroupID)
	dst = encoding.MarshalUint32(dst, t.JobID)
	dst = encoding.MarshalUint32(dst, t.InstanceID)
	dst = encoding.MarshalUint64(dst, t.MetricID)
	return dst
}

// Unmarshal unmarshals t from src and returns the rest of src.
func (t *TSID) Unmarshal(src []byte) ([]byte, error) {
	if len(src) < marshaledTSIDSize {
		return nil, fmt.Errorf("too short src; got %d bytes; want %d bytes", len(src), marshaledTSIDSize)
	}

	t.MetricGroupID = encoding.UnmarshalUint64(src)
	src = src[8:]
	t.JobID = encoding.UnmarshalUint32(src)
	src = src[4:]
	t.InstanceID = encoding.UnmarshalUint32(src)
	src = src[4:]
	t.MetricID = encoding.UnmarshalUint64(src)
	src = src[8:]

	return src, nil
}

// Less return true if t < b.
func (t *TSID) Less(b *TSID) bool {
	// Do not compare MetricIDs here as fast path for determining identical TSIDs,
	// since identical TSIDs aren't passed here in hot paths.
	if t.MetricGroupID != b.MetricGroupID {
		return t.MetricGroupID < b.MetricGroupID
	}
	if t.JobID != b.JobID {
		return t.JobID < b.JobID
	}
	if t.InstanceID != b.InstanceID {
		return t.InstanceID < b.InstanceID
	}
	return t.MetricID < b.MetricID
}

// mergeTSIDsNWay merges sorted TSID slices into one. Duplicates are removed.
func mergeSortedTSIDs(tsidss [][]TSID) []TSID {
	items := make([]tsidItem, len(tsidss))
	h := make(tsidHeap, 0)
	heap.Init(&h)
	var n int
	for i, d := range tsidss {
		if len(d) > 0 {
			item := &items[i]
			item.tsid = &d[0]
			item.sliceIdx = i
			item.elementIdx = 0
			heap.Push(&h, item)
			n += len(d)
		}
	}
	all := make([]TSID, 0, n)
	var lastAdded *TSID
	for h.Len() > 0 {
		item := heap.Pop(&h).(*tsidItem)

		if len(all) == 0 || *item.tsid != *lastAdded {
			all = append(all, *item.tsid)
			lastAdded = item.tsid
		}

		if item.elementIdx+1 < len(tsidss[item.sliceIdx]) {
			item.tsid = &tsidss[item.sliceIdx][item.elementIdx+1]
			item.elementIdx++
			heap.Push(&h, item)
		}
	}
	return all
}

// tsidItem represents a single element in tsidHeap used for implementing N-way
// merge of N sorted TSID slices. See mergeTSIDsNWay().
//
// Given the slice of TSID slices [][]TSID, tsidItem holds the pointer to a TSID
// that is stored in sliceIdx slice at elementIdx index.
type tsidItem struct {
	tsid       *TSID
	sliceIdx   int
	elementIdx int
}

// tsidHeap is a slice of tsidItems that implements methods that allow to use it
// as a heap. It is used for implementing N-way merge of N sorted TSID slices.
// See mergeTSIDsNWay().
type tsidHeap []*tsidItem

// Len returns the length of the heap.
func (h tsidHeap) Len() int {
	return len(h)
}

// Less compares two TSIDs.
func (h tsidHeap) Less(i, j int) bool {
	return h[i].tsid.Less(h[j].tsid)
}

// Swap swaps to heap items.
func (h tsidHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds a new item to the heap.
func (h *tsidHeap) Push(v any) {
	item := v.(*tsidItem)
	*h = append(*h, item)
}

// Pop removes the smallest item from the heap and returns it to the caller.
func (h *tsidHeap) Pop() any {
	old := *h
	item := old[len(old)-1]
	*h = old[0 : len(old)-1]
	return item
}
