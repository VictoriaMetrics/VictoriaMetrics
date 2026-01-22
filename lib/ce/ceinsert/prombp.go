package ceinsert

import (
	"fmt"
	"math/rand"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// Can be called concurrently.
func InsertPrompb(estimator *ce.CardinalityEstimator, tss []prompb.TimeSeries) error {
	if rand.Intn(estimator.SampleRate) != 0 {
		return nil
	}
	return InsertRaw(estimator, tss)
}

// Can be called concurrently. Does not apply sampling.
func InsertRaw(estimator *ce.CardinalityEstimator, tss []prompb.TimeSeries) error {

	for i := range tss {
		tss[i].ShardIdx = estimator.ShardIdx(tss[i].MetricName)
	}

	// We need some kind of scheduling to optimize contention on shards. The simplest scheduling is to make each request insert into shards in the same order.
	// However, this has a major flaw:
	//
	// Suppose we always insert into shards in some order, lets say 0, 1, 2, ..., N-1 for simplicity.
	// Given a sequence of requests, r1, r2, ..., rk, it is possible that r1 may take a long time to insert into shard one, blocking all
	// subsequent requests r2, ..., rk from making any progress as they are all waiting for r1 to finish with shard 1. Subsequently, once
	// r1 finishes with shard 1, it may take a long time to finish with shard 2, blocking all subsequent requests again.
	// In this implementation, one slow insert can block all other inserts from making any progress, which is very bad.
	// Also, it's not enough to simply randomize the starting shard for each request, since the ordering of access is what causes the problem.
	// The described behavior was actually observed in a production deployment, which was the initial motivation here to implement a better
	// scheduling mechanism.
	//
	// To avoid the above problem, we need a better scheduling that eliminates the ordering problem, allowing requests to make progress even if some
	// requests are slow. To optimize for a balance of scheduling expense, simplicity, and low contention, we try to access shards uniformly randomly.
	// This breaks the ordering problem described above, allowing requests to make progress with high probability even if some requests are slow.
	//
	// To reduce scheduling costs, we precompute a large number of random sequences, and randomly select one of them to use for each request. Since
	// the CE itself is long-lived, the amortized cost per request of computing these sequences is basically zero.

	// mark which shards need to be inserted into using a bitmask (zero allocation for <= 64 shards)
	workTodo := [ce.CE_MAX_SHARDS]bool{}
	for i := range tss {
		workTodo[tss[i].ShardIdx] = true
	}

	// We iterate throught the shards, and for each shard, we iterate through all the timeseries. We are doing O(shards * timeseries)
	// iterations here which is theoretically suboptimal, but through benchmarking and production profiling this actually yields the best performance.
	// My best guess for why that's the case is because the work required to insert into HLL significantly dominates the work done for the iteration
	// we do here, and so we benefit from a combination of better cache locality, fewer lock operations, and less scheduling overhead.
	for shardIdx := range estimator.RandomShardIterator() {
		if !workTodo[shardIdx] {
			continue
		}

		err := func() error {
			shard := estimator.Shards[shardIdx]

			shard.Lock.Lock()
			defer shard.Lock.Unlock()

			for i := range tss {
				if tss[i].ShardIdx != shardIdx {
					continue
				}

				mce, exists := shard.Estimators[tss[i].MetricName]
				if !exists {
					// allocate a new string to prevent memory leak where the entire request body is kept in memory due to string pointing to it
					// in our case, we want to avoid situations like:
					// putting string in longlived hashmap which refs underlying string byte array which is inside original zstd decoded byte array
					// => gc cannot free original zstd decoded byte array until hashmap entry (and any other references) is removed
					metricName := strings.Clone(tss[i].MetricName)

					newMce, err := ce.NewMetricCardinalityEstimatorWithAllocator(metricName, estimator.Allocator) // <- this holds a reference to the string
					if err != nil {
						if err == ce.ERROR_MAX_HLLS_INUSE {
							return nil
						}

						return fmt.Errorf("BUG: failed to create MetricCardinalityEstimator for metric %q: %v", metricName, err)
					}

					mce = newMce
					shard.Estimators[metricName] = newMce // <- this holds a reference to the string
				}

				if err := mceInsertPrompb(mce, tss[i]); err != nil {
					return err
				}
				timeseriesInsertedTotal.Inc()
				shard.InsertCounter.Inc()
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

// Do not call this function concurrently.
func mceInsertPrompb(mce *ce.MetricCardinalityEstimator, ts prompb.TimeSeries) error {
	// Make sure the timeseries has a metric name label and it matches the estimator's metric name
	if ts.MetricName != mce.MetricName {
		return fmt.Errorf("BUG: timeseries metric name (%s) does not match estimator metric name (%s)", ts.MetricName, mce.MetricName)
	}

	tsEncoding := byteifyLabelSet(mce, ts.Labels)

	// Count cardinality for the whole metric
	mce.MetricHll.Insert(tsEncoding)

	// Count cardinality for the whole metric by fixed dimension
	pathBytes := encodeTimeseriesPath(mce, ts)
	path := unsafe.String(unsafe.SliceData(pathBytes), len(pathBytes))

	hll := mce.Hlls[path]
	if hll == nil {
		path := strings.Clone(path) // ensure we own the string

		newHll, err := mce.Allocator.Allocate()
		if err != nil {
			return err
		}

		hll = newHll
		mce.Hlls[path] = newHll
	}

	hll.Insert(tsEncoding)

	return nil
}

// Return slice only valid until the next call to EncodeTimeseriesPath
func encodeTimeseriesPath(mce *ce.MetricCardinalityEstimator, ts prompb.TimeSeries) []byte {
	mce.B1 = mce.B1[:0]

	mce.B1 = append(mce.B1, ts.MetricName...)
	mce.B1 = append(mce.B1, 0x00) // \x00 cannot appear in label names/values, so its okay to use it as a separator
	mce.B1 = append(mce.B1, []byte(ts.FixedLabelValue1)...)
	mce.B1 = append(mce.B1, 0x00)
	mce.B1 = append(mce.B1, []byte(ts.FixedLabelValue2)...)

	return mce.B1
}

// Return slice only valid until the next call to ByteifyLabelSet
func byteifyLabelSet(mce *ce.MetricCardinalityEstimator, labels []prompb.Label) []byte {
	mce.B = mce.B[:0]

	for _, l := range labels {
		if l.Name == "__name__" { // We require this label to be static, so skip it and save cpu
			continue
		}

		mce.B = append(mce.B, l.Name...)
		mce.B = append(mce.B, 0x00) // \x00 cannot appear in label names/values, so its okay to use it as a separator
		mce.B = append(mce.B, l.Value...)
		mce.B = append(mce.B, 0x00)
	}

	return mce.B
}
