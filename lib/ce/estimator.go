package ce

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
)

var (
	timeseriesInsertedTotal = metrics.NewCounter("vm_ce_timeseries_inserted_total")
	ceResetsTotal           = metrics.NewCounter("vm_ce_resets_total")
)

const (
	CE_MAX_SHARDS = 64
)

type CardinalityEstimator struct {
	// Each shard contains a a map of MetricName -> MetricCardinalityEstimator and a lock to protect concurrent access to that map.
	//
	// Invariant: the sets of MetricNames that shards track are disjoint and collectively exhaustive.
	shards []*struct {
		lock          *sync.Mutex
		estimators    map[string]*MetricCardinalityEstimator
		insertCounter *metrics.Counter
	}
	sampleRate int

	insertSequences [][]int

	// READONLY FOR PUBLIC USE
	Allocator *Allocator
}

func NewCardinalityEstimator() *CardinalityEstimator {
	return NewCardinalityEstimatorWithSettings(CE_MAX_SHARDS, math.MaxUint64, 1)
}

// CardinalityEstimator provides a concurrency-safe API for inserting timeseries and estimating cardinalities, across all metrics.
//
// Parameters:
// - shards: the more shards you have, the more parallelism you can achieve, up to the number of CPU cores.
// - maxHllsInuse: this controls the maximum number of HyperLogLog sketches that can be in use at any given time. Primarily used as a mechanism to prevent OOMs.
//
// Why Shards?
// To optimize for throughput, we need some mechanism to partition the workload across parallel threads. We use sharding by MetricName to achieve this.
// For performance reasons, we avoid using channels here as they become expensive compared to other synchronization primitives when called very frequently.
func NewCardinalityEstimatorWithSettings(shards int, maxHllsInuse uint64, sampleRate int) *CardinalityEstimator {
	if shards <= 0 {
		log.Panicf("BUG: invalid estimator shards value %d, must be > 0", shards)
	}
	if sampleRate <= 0 {
		log.Panicf("BUG: invalid estimator sampleRate value %d, must be > 0", sampleRate)
	}
	if shards > CE_MAX_SHARDS {
		log.Panicf("BUG: too many estimator shards: %d; max allowed is %d", shards, CE_MAX_SHARDS)
	}

	ret := &CardinalityEstimator{
		shards: make([]*struct {
			lock          *sync.Mutex
			estimators    map[string]*MetricCardinalityEstimator
			insertCounter *metrics.Counter
		}, shards),
		sampleRate:      sampleRate,
		insertSequences: make([][]int, 10_000),
		Allocator:       NewAllocator(maxHllsInuse),
	}

	// intialize shards
	for i := range ret.shards {
		ret.shards[i] = &struct {
			lock          *sync.Mutex
			estimators    map[string]*MetricCardinalityEstimator
			insertCounter *metrics.Counter
		}{
			lock:          &sync.Mutex{},
			estimators:    make(map[string]*MetricCardinalityEstimator),
			insertCounter: metrics.GetOrCreateCounter(fmt.Sprintf("ce_timeseries_inserted_by_shard_total{shard=\"%d\"}", i)),
		}
	}

	// precompute random insert sequences
	for i := range ret.insertSequences {
		seq := make([]int, shards)
		for j := range seq {
			seq[j] = j
		}
		rand.Shuffle(len(seq), func(a, b int) {
			seq[a], seq[b] = seq[b], seq[a]
		})
		ret.insertSequences[i] = seq
	}

	return ret
}

// Can be called concurrently.
func (ce *CardinalityEstimator) Reset() {
	for _, shard := range ce.shards {
		shard.lock.Lock()
		defer shard.lock.Unlock()

		shard.estimators = make(map[string]*MetricCardinalityEstimator)
	}

	ce.Allocator = NewAllocator(ce.Allocator.Max())

	ceResetsTotal.Inc()
}

// Can be called concurrently.
func (ce *CardinalityEstimator) Insert(tss []prompb.TimeSeries) error {
	if rand.Intn(ce.sampleRate) != 0 {
		return nil
	}
	return ce.InsertRaw(tss)
}

// Can be called concurrently. Does not apply sampling.
func (ce *CardinalityEstimator) InsertRaw(tss []prompb.TimeSeries) error {

	for i := range tss {
		tss[i].ShardIdx = ce.shardIdx(tss[i].MetricName)
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
	workTodo := [CE_MAX_SHARDS]bool{}
	for i := range tss {
		workTodo[tss[i].ShardIdx] = true
	}

	// We iterate throught the shards, and for each shard, we iterate through all the timeseries. We are doing O(shards * timeseries)
	// iterations here which is theoretically suboptimal, but through benchmarking and production profiling this actually yields the best performance.
	// My best guess for why that's the case is because the work required to insert into HLL significantly dominates the work done for the iteration
	// we do here, and so we benefit from a combination of better cache locality, fewer lock operations, and less scheduling overhead.
	for shardIdx := range ce.randomShardIterator() {
		if !workTodo[shardIdx] {
			continue
		}

		err := func() error {
			shard := ce.shards[shardIdx]

			shard.lock.Lock()
			defer shard.lock.Unlock()

			for i := range tss {
				if tss[i].ShardIdx != shardIdx {
					continue
				}

				mce, exists := shard.estimators[tss[i].MetricName]
				if !exists {
					// allocate a new string to prevent memory leak where the entire request body is kept in memory due to string pointing to it
					// in our case, we want to avoid situations like:
					// putting string in longlived hashmap which refs underlying string byte array which is inside original zstd decoded byte array
					// => gc cannot free original zstd decoded byte array until hashmap entry (and any other references) is removed
					metricName := strings.Clone(tss[i].MetricName)

					newMce, err := NewMetricCardinalityEstimatorWithAllocator(metricName, ce.Allocator) // <- this holds a reference to the string
					if err != nil {
						if err == ERROR_MAX_HLLS_INUSE {
							return nil
						}

						return fmt.Errorf("BUG: failed to create MetricCardinalityEstimator for metric %q: %v", metricName, err)
					}

					mce = newMce
					shard.estimators[metricName] = newMce // <- this holds a reference to the string
				}

				if err := mce.Insert(tss[i]); err != nil {
					return err
				}
				timeseriesInsertedTotal.Inc()
				shard.insertCounter.Inc()
			}

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

// Can be called concurrently.
func (ce *CardinalityEstimator) EstimateFixedMetricCardinality() map[string]uint64 {
	estimate := make([]map[string]uint64, len(ce.shards))

	for i, shard := range ce.shards {
		func() {
			shard.lock.Lock()
			defer shard.lock.Unlock()

			estimate[i] = make(map[string]uint64)

			for _, estimator := range shard.estimators {
				for label, cardinality := range estimator.EstimateFixedMetricCardinality() {
					estimate[i][label] = cardinality
				}
			}
		}()
	}

	estimateMap := make(map[string]uint64)
	for _, shardEstimate := range estimate {
		for label, cardinality := range shardEstimate {
			estimateMap[label] = cardinality
		}
	}

	return estimateMap
}

// Can be called concurrently.
func (ce *CardinalityEstimator) EstimateMetricsCardinality() (
	estimate struct {
		CardinalityTotal            uint64 `json:"cardinality_total"`
		CardinalityDescByMetricName []struct {
			MetricName  string `json:"metric_name"`
			Cardinality uint64 `json:"cardinality"`
		} `json:"cardinality_desc_by_metric_name"`
	},
) {

	for _, shard := range ce.shards {
		shard.lock.Lock()

		for _, estimator := range shard.estimators {
			estimate.CardinalityDescByMetricName = append(estimate.CardinalityDescByMetricName, struct {
				MetricName  string `json:"metric_name"`
				Cardinality uint64 `json:"cardinality"`
			}{
				MetricName:  estimator.metricName,
				Cardinality: estimator.EstimateMetricCardinality(),
			})

			estimate.CardinalityTotal += estimator.EstimateMetricCardinality()
		}

		shard.lock.Unlock()
	}

	sort.Slice(estimate.CardinalityDescByMetricName, func(i, j int) bool {
		return estimate.CardinalityDescByMetricName[i].Cardinality > estimate.CardinalityDescByMetricName[j].Cardinality
	})

	return
}

// Can be called concurrently.
func (ce *CardinalityEstimator) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)

	// First encode the number of shards
	if err := encoder.Encode(len(ce.shards)); err != nil {
		return nil, err
	}

	// Encode the allocator's state
	if err := encoder.Encode(ce.Allocator); err != nil {
		return nil, err
	}

	// Encode each shard one at a time
	for _, shard := range ce.shards {
		shard.lock.Lock()

		if err := encoder.Encode(shard.estimators); err != nil {
			shard.lock.Unlock()
			return nil, err
		}

		shard.lock.Unlock()
	}

	return buf.Bytes(), nil
}

// Can be called concurrently.
func (ce *CardinalityEstimator) UnmarshalBinary(data []byte) error {
	for _, shard := range ce.shards {
		// lock all shards
		shard.lock.Lock()
		defer shard.lock.Unlock()
	}

	decoder := gob.NewDecoder(bytes.NewReader(data))

	// First decode the number of shards
	var numShards int
	if err := decoder.Decode(&numShards); err != nil {
		return fmt.Errorf("Failed to decode shard count: %v", err)
	}

	if numShards != len(ce.shards) {
		return fmt.Errorf("BUG: mismatched shard counts, received %d, expected %d", numShards, len(ce.shards))
	}

	// Decode the allocator's state
	var allocator Allocator
	if err := decoder.Decode(&allocator); err != nil {
		return fmt.Errorf("Failed to decode allocator: %v", err)
	}
	ce.Allocator = &allocator

	// Decode each shard one at a time
	for i, shard := range ce.shards {
		var shardEstimators map[string]*MetricCardinalityEstimator
		if err := decoder.Decode(&shardEstimators); err != nil {
			return fmt.Errorf("Failed to decode shard %d: %v", i, err)
		}
		shard.estimators = shardEstimators
	}

	return nil
}

// Do not call concurrently. The other estimator should not be used after the merge.
func (ce *CardinalityEstimator) Merge(other *CardinalityEstimator) error {
	if len(ce.shards) != len(other.shards) {
		return fmt.Errorf("mismatched shard counts, self has %d, other has %d", len(ce.shards), len(other.shards))
	}

	// merge allocator state
	ce.Allocator.Merge(other.Allocator)

	for i := range ce.shards {
		err := func() error {
			selfShard := ce.shards[i]
			otherShard := other.shards[i]

			selfShard.lock.Lock()
			otherShard.lock.Lock()
			defer selfShard.lock.Unlock()
			defer otherShard.lock.Unlock()

			// merge estimators
			for metricName, otherEstimator := range otherShard.estimators {
				selfEstimator, exists := selfShard.estimators[metricName]
				if !exists {
					selfShard.estimators[metricName] = otherEstimator
					otherEstimator.allocator = ce.Allocator // policy: use the self estimator's allocator
					continue
				}

				if err := selfEstimator.Merge(otherEstimator); err != nil {
					return fmt.Errorf("failed to merge estimator for metric %s: %v", metricName, err)
				}
			}

			// merge insert counters
			selfShard.insertCounter.Set(selfShard.insertCounter.Get() + otherShard.insertCounter.Get())

			return nil
		}()

		if err != nil {
			return err
		}
	}

	return nil
}

// Can be called concurrently.
func (ce *CardinalityEstimator) ShardsCount() int {
	return len(ce.shards)
}

func (ce *CardinalityEstimator) shardIdx(metricName string) int {
	fnv := fnv.New64a()
	fnv.Write(unsafe.Slice(unsafe.StringData(metricName), len(metricName)))
	return int(fnv.Sum64() % uint64(len(ce.shards)))
}

func (ce *CardinalityEstimator) randomShardIterator() func(yield func(int) bool) {
	return func(yield func(int) bool) {
		seq := ce.insertSequences[rand.Intn(len(ce.insertSequences))]
		if len(seq) != len(ce.shards) {
			log.Panicf("BUG: len(seq)=%d, len(ce.shards)=%d", len(seq), len(ce.shards))
		}
		for i := range ce.shards {
			if !yield(seq[i]) {
				return
			}
		}
	}
}
