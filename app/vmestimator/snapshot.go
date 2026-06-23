package main

import (
	"encoding/gob"
	"io"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/axiomhq/hyperloglog"
)

type snapshots struct {
	mu sync.Mutex
	m  map[string]*snapshot
}

func newSnapshots() *snapshots {
	return &snapshots{m: make(map[string]*snapshot)}
}

func (ss *snapshots) add(newS *snapshot) {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	key := newS.GroupByKeysLabel
	if s, found := ss.m[key]; found {
		s.merge(newS)
		return
	}

	s := newSnapshot()
	s.merge(newS)
	ss.m[key] = s
}

func (ss *snapshots) writeMetrics(w io.Writer) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	for _, s := range ss.m {
		if err := s.writeMetrics(w); err != nil {
			return err
		}
	}

	return nil
}

type snapshot struct {
	MetricPrefix        string
	GroupByKeysLabel    string
	GroupRejectedSketch *hyperloglog.Sketch
	GroupBy             []string
	// prom string metric => hll
	Sketches map[string]*hyperloglog.Sketch
}

func newSnapshot() *snapshot {
	return &snapshot{
		Sketches: make(map[string]*hyperloglog.Sketch),
	}
}

// decodeSnapshot reads a stream of gob-encoded EstimatorMerge objects from the response and merges them into the provided estimatorMerge object.
func decodeSnapshots(r io.Reader, cb func(s *snapshot)) error {
	d := gob.NewDecoder(r)
	s := newSnapshot()
	for {
		s.reset()
		if err := d.Decode(s); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		cb(s)
	}
}

func (s *snapshot) merge(other *snapshot) {
	if s.GroupByKeysLabel != "" && s.GroupByKeysLabel != other.GroupByKeysLabel {
		logger.Panicf("BUG: merge snapshots must have the same groupByKeysLabel; s: %s; other: %s", s.GroupByKeysLabel, other.GroupByKeysLabel)
	}

	for name, otherSK := range other.Sketches {
		if existing, ok := s.Sketches[name]; ok {
			existing.Merge(otherSK)
		} else {
			s.Sketches[name] = otherSK.Clone()
		}
	}

	s.MetricPrefix = other.MetricPrefix
	s.GroupByKeysLabel = other.GroupByKeysLabel
	s.GroupBy = append(s.GroupBy, other.GroupBy...)
	if other.GroupRejectedSketch != nil {
		if s.GroupRejectedSketch == nil {
			s.GroupRejectedSketch = other.GroupRejectedSketch.Clone()
		} else {
			s.GroupRejectedSketch.Merge(other.GroupRejectedSketch)
		}
	}
}

// writeMetrics writes metrics to w.
// w must be a buffered writer.
func (s *snapshot) writeMetrics(w io.Writer) error {
	for name, sketch := range s.Sketches {
		if _, err := w.Write(bytesutil.ToUnsafeBytes(name)); err != nil {
			return err
		}
		if _, err := w.Write(strconv.AppendUint(nil, sketch.Estimate(), 10)); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}

	if len(s.GroupBy) > 0 {
		groupSize := int64(len(s.Sketches))
		if s.GroupRejectedSketch != nil {
			groupSize += int64(s.GroupRejectedSketch.Estimate())
		}

		formatBuf := make([]byte, 0, 1024)
		formatBuf = appendGroupMetric(formatBuf, s.MetricPrefix, s.GroupByKeysLabel)
		formatBuf = strconv.AppendInt(formatBuf, groupSize, 10)
		formatBuf = append(formatBuf, "\n"...)
		if _, err := w.Write(formatBuf); err != nil {
			logger.Errorf("writing metrics failed: %s; written cardinality metrics might be incomplete or invalid", err)
		}
	}

	return nil
}

func (s *snapshot) reset() {
	s.GroupByKeysLabel = ""
	s.GroupRejectedSketch = nil
	s.MetricPrefix = ""
	s.GroupBy = s.GroupBy[:0]
	clear(s.Sketches)
}

func convertNoGroupToSnapshot(e *estimator, s *snapshot) *snapshot {
	if len(e.groupBy) != 0 {
		panic("BUG: do not use this function for estimator with non-empty groupBy")
	}
	if s == nil {
		s = newSnapshot()
	}
	s.reset()

	eb0 := e.buckets[0]

	resSK := eb0.newSketch()
	for _, eb := range e.buckets {
		eb.writeNoGroupMetric(resSK)
	}

	formatBuf := make([]byte, 0, 1024)
	formatBuf = appendGlobalMetric(formatBuf, eb0.metricPrefix)
	s.Sketches[string(formatBuf)] = resSK
	s.GroupByKeysLabel = eb0.groupByKeysLabel
	s.MetricPrefix = eb0.metricPrefix
	s.GroupBy = append(s.GroupBy[:0], eb0.groupBy...)

	return s
}

func convertGroupToSnapshot(e *estimator, s *snapshot) *snapshot {
	if len(e.groupBy) == 0 {
		panic("BUG: do not use this function for estimator with empty groupBy")
	}

	eb0 := e.buckets[0]

	formatBuf := make([]byte, 0, 16384)
	formatBuf = appendGroupByKeysAndValuesPrefix(formatBuf, eb0.metricPrefix, eb0.groupByKeysLabel)

	if s == nil {
		s = newSnapshot()
	}
	s.reset()

	for _, eb := range e.buckets {
		eb.groupRejectedMu.Lock()
		if eb.groupRejectedSketch != nil {
			s.GroupRejectedSketch = eb.groupRejectedSketch.Clone()
		}
		eb.groupRejectedMu.Unlock()
		s = convertGroupBucketToSnapshot(eb, s, formatBuf)
	}
	return s
}

func convertGroupBucketToSnapshot(eb *estimatorBucket, s *snapshot, formatBuf []byte) *snapshot {
	if len(eb.groupBy) == 0 {
		panic("BUG: do not use this function for estimator with empty groupBy")
	}

	prefixLen := len(formatBuf)
	resSK := eb.newSketch()

	eb.mu.Lock()
	defer eb.mu.Unlock()
	for valuesKey, gsk := range eb.groups {
		resSK.Reset()
		formatBuf = append(formatBuf[:prefixLen], gsk.groupValueLabels...)
		eb.mergeSketches(gsk.Sketch, eb.prevGroups[valuesKey].Sketch, resSK)
		s.Sketches[string(formatBuf)] = resSK.Clone()
	}
	for valuesKey := range eb.prevGroups {
		if _, ok := eb.groups[valuesKey]; ok {
			continue
		}

		resSK.Reset()
		formatBuf = formatBuf[:prefixLen]

		gsk := eb.prevGroups[valuesKey]
		formatBuf = append(formatBuf, gsk.groupValueLabels...)

		eb.mergeSketches(nil, eb.prevGroups[valuesKey].Sketch, resSK)
		s.Sketches[string(formatBuf)] = resSK.Clone()
	}

	s.GroupByKeysLabel = eb.groupByKeysLabel
	s.MetricPrefix = eb.metricPrefix
	s.GroupBy = append(s.GroupBy[:0], eb.groupBy...)

	return s
}
