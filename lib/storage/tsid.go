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

// mergeSortedTSIDs merges sorted TSID slices into one. Duplicates are removed.
func mergeSortedTSIDs(tsidss [][]TSID) []TSID {
	var h tsidHeap
	var n int
	for _, tsids := range tsidss {
		if len(tsids) > 0 {
			h = append(h, tsids)
			n += len(tsids)
		}
	}
	all := make([]TSID, 0, n)

	heap.Init(&h)
	for h.Len() > 0 {
		top := h[0]
		tsid := top[0]
		if len(all) == 0 || tsid != all[len(all)-1] {
			all = append(all, tsid)
		}
		if len(top) == 1 {
			heap.Pop(&h)
		} else {
			h[0] = top[1:]
			heap.Fix(&h, 0)
		}
	}

	return all
}

// tsidHeap is a slice of tsidItems that implements methods that allow to use it
// as a heap. It is used for implementing N-way merge of N sorted TSID slices.
// See mergeSortedTSIDs().
//
// Slice elements initially must not be empty.
type tsidHeap [][]TSID

func (h tsidHeap) Len() int {
	return len(h)
}

func (h tsidHeap) Less(i, j int) bool {
	return h[i][0].Less(&h[j][0])
}

func (h tsidHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *tsidHeap) Push(_ any) {
	panic(fmt.Errorf("BUG: Push shouldn't be called"))
}

func (h *tsidHeap) Pop() any {
	a := *h
	x := a[len(a)-1]
	*h = a[:len(a)-1]
	return x
}
