package promdb

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/oklog/ulid"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/go-kit/kit/log"
	"github.com/prometheus/prometheus/pkg/labels"
	promstorage "github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
)

var (
	prometheusDataPath = flag.String("prometheusDataPath", "", "Optinal path to readonly historical Prometheus data")
)

// filters prometheus db response.
var maxRetentionTimeStampMsecs int64

// Init must be called after flag.Parse and before using the package.
//
// See also MustClose.
func Init(retentionMsecs int64) {
	maxRetentionTimeStampMsecs = time.Now().Unix()*1000 - retentionMsecs
	if promDB != nil {
		logger.Fatalf("BUG: it looks like MustOpenPromDB is called multiple times without MustClosePromDB call")
	}
	if *prometheusDataPath == "" {
		return
	}
	l := log.LoggerFunc(func(a ...interface{}) error {
		logger.Infof("%v", a)
		return nil
	})
	opts := tsdb.DefaultOptions()
	// set max block duration to the 10% of retention period or 31 day.
	// its common setting for promethues.
	// https://prometheus.io/docs/prometheus/latest/storage/#compaction
	maxBlockDuration := int64((31 * 24 * time.Hour) / time.Millisecond)

	opts.MaxBlockDuration = maxBlockDuration
	opts.RetentionDuration = retentionMsecs
	if retentionMsecs/10 < maxBlockDuration {
		opts.MaxBlockDuration = retentionMsecs / 10
	}
	// its needed to make correct compaction ranges.
	// if minBlockDuration*3 > maxBlockDuration, no compaction will be made.
	// its case for retention less then 60 hours.
	if opts.MaxBlockDuration < opts.MinBlockDuration {
		opts.MinBlockDuration = opts.MaxBlockDuration / 3
	}

	// custom delete function is needed, because prometheus uses BeyondTimeRetention func,
	// that calculates the difference between the first block and this block is larger than
	// the retention period so any blocks after that are added as deletable.
	// https://github.com/prometheus/prometheus/blob/997bb7134fcfd7279f250e183e78681e48a56aff/tsdb/db.go#L1116
	opts.BlocksToDelete = func(blocks []*tsdb.Block) map[ulid.ULID]struct{} {
		deletable := make(map[ulid.ULID]struct{})
		for _, block := range blocks {
			// add block marked for deletion by compaction.
			if block.Meta().Compaction.Deletable {
				deletable[block.Meta().ULID] = struct{}{}
				continue
			}
			if block.MaxTime() < maxRetentionTimeStampMsecs {
				deletable[block.Meta().ULID] = struct{}{}
			}
		}
		return deletable
	}
	pdb, err := tsdb.Open(*prometheusDataPath, l, nil, opts)
	if err != nil {
		logger.Panicf("FATAL: cannot open Prometheus data at -prometheusDataPath=%q: %s", *prometheusDataPath, err)
	}
	promDB = pdb
	logger.Infof("successfully opened historical Prometheus data at -prometheusDataPath=%q with retentionMsecs=%d", *prometheusDataPath, retentionMsecs)
}

// MustClose must be called on graceful shutdown.
//
// Package functionality cannot be used after this call.
func MustClose() {
	if *prometheusDataPath == "" {
		return
	}
	if promDB == nil {
		logger.Panicf("BUG: it looks like MustClosePromDB is called without MustOpenPromDB call")
	}
	if err := promDB.Close(); err != nil {
		logger.Panicf("FATAL: cannot close promDB: %s", err)
	}
	promDB = nil
	logger.Infof("successfully closed historical Prometheus data at -prometheusDataPath=%q", *prometheusDataPath)
}

var promDB *tsdb.DB

// GetLabelNamesOnTimeRange returns label names.
func GetLabelNamesOnTimeRange(tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	d := time.Unix(int64(deadline.Deadline()), 0)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	q, err := promDB.Querier(ctx, tr.MinTimestamp, tr.MaxTimestamp)
	if err != nil {
		return nil, err
	}
	defer mustCloseQuerier(q)

	names, _, err := q.LabelNames()
	// Make full copy of names, since they cannot be used after q is closed.
	names = copyStringsWithMemory(names)
	return names, err
}

// GetLabelNames returns label names.
func GetLabelNames(deadline searchutils.Deadline) ([]string, error) {
	tr := storage.TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: time.Now().UnixNano() / 1e6,
	}
	return GetLabelNamesOnTimeRange(tr, deadline)
}

// GetLabelValuesOnTimeRange returns values for the given labelName on the given tr.
func GetLabelValuesOnTimeRange(labelName string, tr storage.TimeRange, deadline searchutils.Deadline) ([]string, error) {
	d := time.Unix(int64(deadline.Deadline()), 0)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	q, err := promDB.Querier(ctx, tr.MinTimestamp, tr.MaxTimestamp)
	if err != nil {
		return nil, err
	}
	defer mustCloseQuerier(q)

	values, _, err := q.LabelValues(labelName)
	// Make full copy of values, since they cannot be used after q is closed.
	values = copyStringsWithMemory(values)
	return values, err
}

// GetLabelValues returns values for the given labelName.
func GetLabelValues(labelName string, deadline searchutils.Deadline) ([]string, error) {
	tr := storage.TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: time.Now().UnixNano() / 1e6,
	}
	return GetLabelValuesOnTimeRange(labelName, tr, deadline)
}

func copyStringsWithMemory(a []string) []string {
	result := make([]string, len(a))
	for i, s := range a {
		result[i] = string(append([]byte{}, s...))
	}
	return result
}

// SeriesVisitor is called by VisitSeries for each matching time series.
//
// The caller shouldn't hold references to metricName, values and timestamps after returning.
type SeriesVisitor func(metricName []byte, values []float64, timestamps []int64)

// VisitSeries calls f for each series found in the pdb.
//
// If fetchData is false, then empty values and timestamps are passed to f.
func VisitSeries(sq *storage.SearchQuery, fetchData bool, deadline searchutils.Deadline, f SeriesVisitor) error {
	if *prometheusDataPath == "" {
		return nil
	}
	d := time.Unix(int64(deadline.Deadline()), 0)
	ctx, cancel := context.WithDeadline(context.Background(), d)
	defer cancel()
	// adjust search query time range.
	// Prometheus keeps recent data at wal and it will not be deleted.
	min, max := adjustSearchQueryMinMaxTimestamps(sq, maxRetentionTimeStampMsecs)
	q, err := promDB.Querier(ctx, min, max)
	if err != nil {
		return err
	}
	defer mustCloseQuerier(q)
	if len(sq.TagFilterss) != 1 {
		return fmt.Errorf("unexpected len(sq.TagFilters); got %d; want 1", len(sq.TagFilterss))
	}
	ms, err := convertTagFiltersToMatchers(sq.TagFilterss[0])
	if err != nil {
		return fmt.Errorf("cannot convert tag filters to matchers: %w", err)
	}
	ss := q.Select(false, nil, ms...)
	var (
		mn         storage.MetricName
		metricName []byte
		values     []float64
		timestamps []int64
	)
	for ss.Next() {
		s := ss.At()
		convertPromLabelsToMetricName(&mn, s.Labels())
		metricName = mn.SortAndMarshal(metricName[:0])
		values = values[:0]
		timestamps = timestamps[:0]
		if fetchData {
			it := s.Iterator()
			for it.Next() {
				ts, v := it.At()
				values = append(values, v)
				timestamps = append(timestamps, ts)
			}
			if err := it.Err(); err != nil {
				return fmt.Errorf("error when iterating Prometheus series: %w", err)
			}
		}
		f(metricName, values, timestamps)
	}
	return ss.Err()
}

// ensures, that search query is inside retention period.
func adjustSearchQueryMinMaxTimestamps(sq *storage.SearchQuery, maxTimestamp int64) (int64, int64) {
	max := sq.MaxTimestamp
	min := sq.MinTimestamp
	if sq.MaxTimestamp < maxTimestamp {
		max = maxRetentionTimeStampMsecs
	}
	if sq.MinTimestamp < maxTimestamp {
		min = maxRetentionTimeStampMsecs
	}
	return min, max
}

func convertPromLabelsToMetricName(dst *storage.MetricName, labels []labels.Label) {
	dst.Reset()
	for _, label := range labels {
		if label.Name == "__name__" {
			dst.MetricGroup = append(dst.MetricGroup[:0], label.Value...)
		} else {
			dst.AddTag(label.Name, label.Value)
		}
	}
}

func convertTagFiltersToMatchers(tfs []storage.TagFilter) ([]*labels.Matcher, error) {
	ms := make([]*labels.Matcher, 0, len(tfs))
	for _, tf := range tfs {
		var mt labels.MatchType
		if tf.IsNegative {
			if tf.IsRegexp {
				mt = labels.MatchNotRegexp
			} else {
				mt = labels.MatchNotEqual
			}
		} else {
			if tf.IsRegexp {
				mt = labels.MatchRegexp
			} else {
				mt = labels.MatchEqual
			}
		}
		key := string(tf.Key)
		if key == "" {
			key = "__name__"
		}
		value := string(tf.Value)
		m, err := labels.NewMatcher(mt, key, value)
		if err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

func mustCloseQuerier(q promstorage.Querier) {
	if err := q.Close(); err != nil {
		logger.Panicf("FATAL: cannot close querier: %s", err)
	}
}
