package storage

import (
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
	// implement marshaling of TSID
	// hint:
	// format: MetricGroupID(8 bytes) + JobID(4 bytes) + InstanceID(4 bytes) + MetricID(8 bytes)
	// size: 8 + 4 + 4 + 8 = 24 bytes
	// you can use encoding.MarshalUint64(dst, t.MetricGroupID) to marshal MetricGroupID and MetricID

	encoding.MarshalUint64(dst, t.MetricGroupID)
	encoding.MarshalUint32(dst, t.JobID)
	encoding.MarshalUint32(dst, t.InstanceID)
	encoding.MarshalUint64(dst, t.MetricID)
	return dst
}

// Unmarshal unmarshals t from src and returns the rest of src.
func (t *TSID) Unmarshal(src []byte) ([]byte, error) {
	if len(src) < marshaledTSIDSize {
		return nil, fmt.Errorf("too short src; got %d bytes; want %d bytes", len(src), marshaledTSIDSize)
	}

	// hint:
	// format: MetricGroupID(8 bytes) + JobID(4 bytes) + InstanceID(4 bytes) + MetricID(8 bytes)
	// size: 8 + 4 + 4 + 8 = 24 bytes
	// you can use encoding.UnmarshalUint64(src) to unmarshal MetricGroupID and MetricID
	// you can use encoding.UnmarshalUint32(src) to unmarshal JobID and InstanceID
	// implement unmarshaling of TSID
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

	// implement Less for TSID
	// hint:
	// compare MetricGroupID, JobID, InstanceID, MetricID as separate conditions
	// return true if t < b
	if t.MetricGroupID != b.MetricGroupID {
		return t.MetricGroupID < b.MetricGroupID
	}
	if t.JobID != b.JobID {
		return t.JobID < b.JobID
	}
	if t.InstanceID != b.InstanceID {
		return t.InstanceID < b.InstanceID
	}
	if t.MetricID != b.MetricID {
		return t.MetricID < b.MetricID
	}
	return false
}
