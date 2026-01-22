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
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/metrics"
)

var (
	ceResetsTotal = metrics.NewCounter("vm_ce_resets_total")
)

const (
	CE_MAX_SHARDS = 64
)

type CardinalityEstimator struct {
	// Each shard contains a a map of MetricName -> MetricCardinalityEstimator and a lock to protect concurrent access to that map.
	//
	// Invariant: the sets of MetricNames that Shards track are disjoint and collectively exhaustive.
	Shards []*struct {
		Lock          *sync.Mutex
		Estimators    map[string]*MetricCardinalityEstimator
		InsertCounter *metrics.Counter
	}
	SampleRate int

	InsertSequences [][]int

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
		Shards: make([]*struct {
			Lock          *sync.Mutex
			Estimators    map[string]*MetricCardinalityEstimator
			InsertCounter *metrics.Counter
		}, shards),
		SampleRate:      sampleRate,
		InsertSequences: make([][]int, 10_000),
		Allocator:       NewAllocator(maxHllsInuse),
	}

	// intialize shards
	for i := range ret.Shards {
		ret.Shards[i] = &struct {
			Lock          *sync.Mutex
			Estimators    map[string]*MetricCardinalityEstimator
			InsertCounter *metrics.Counter
		}{
			Lock:          &sync.Mutex{},
			Estimators:    make(map[string]*MetricCardinalityEstimator),
			InsertCounter: metrics.GetOrCreateCounter(fmt.Sprintf("ce_timeseries_inserted_by_shard_total{shard=\"%d\"}", i)),
		}
	}

	// precompute random insert sequences
	for i := range ret.InsertSequences {
		seq := make([]int, shards)
		for j := range seq {
			seq[j] = j
		}
		rand.Shuffle(len(seq), func(a, b int) {
			seq[a], seq[b] = seq[b], seq[a]
		})
		ret.InsertSequences[i] = seq
	}

	return ret
}

// Can be called concurrently.
func (ce *CardinalityEstimator) Reset() {
	for _, shard := range ce.Shards {
		shard.Lock.Lock()
		defer shard.Lock.Unlock()

		shard.Estimators = make(map[string]*MetricCardinalityEstimator)
	}

	ce.Allocator = NewAllocator(ce.Allocator.Max())

	ceResetsTotal.Inc()
}

// Can be called concurrently.
func (ce *CardinalityEstimator) EstimateFixedMetricCardinality() map[string]uint64 {
	estimate := make([]map[string]uint64, len(ce.Shards))

	for i, shard := range ce.Shards {
		func() {
			shard.Lock.Lock()
			defer shard.Lock.Unlock()

			estimate[i] = make(map[string]uint64)

			for _, estimator := range shard.Estimators {
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

	for _, shard := range ce.Shards {
		shard.Lock.Lock()

		for _, estimator := range shard.Estimators {
			estimate.CardinalityDescByMetricName = append(estimate.CardinalityDescByMetricName, struct {
				MetricName  string `json:"metric_name"`
				Cardinality uint64 `json:"cardinality"`
			}{
				MetricName:  estimator.MetricName,
				Cardinality: estimator.EstimateMetricCardinality(),
			})

			estimate.CardinalityTotal += estimator.EstimateMetricCardinality()
		}

		shard.Lock.Unlock()
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
	if err := encoder.Encode(len(ce.Shards)); err != nil {
		return nil, err
	}

	// Encode the allocator's state
	if err := encoder.Encode(ce.Allocator); err != nil {
		return nil, err
	}

	// Encode each shard one at a time
	for _, shard := range ce.Shards {
		shard.Lock.Lock()

		if err := encoder.Encode(shard.Estimators); err != nil {
			shard.Lock.Unlock()
			return nil, err
		}

		shard.Lock.Unlock()
	}

	return buf.Bytes(), nil
}

// Can be called concurrently.
func (ce *CardinalityEstimator) UnmarshalBinary(data []byte) error {
	for _, shard := range ce.Shards {
		// lock all shards
		shard.Lock.Lock()
		defer shard.Lock.Unlock()
	}

	decoder := gob.NewDecoder(bytes.NewReader(data))

	// First decode the number of shards
	var numShards int
	if err := decoder.Decode(&numShards); err != nil {
		return fmt.Errorf("Failed to decode shard count: %v", err)
	}

	if numShards != len(ce.Shards) {
		return fmt.Errorf("BUG: mismatched shard counts, received %d, expected %d", numShards, len(ce.Shards))
	}

	// Decode the allocator's state
	var allocator Allocator
	if err := decoder.Decode(&allocator); err != nil {
		return fmt.Errorf("Failed to decode allocator: %v", err)
	}
	ce.Allocator = &allocator

	// Decode each shard one at a time
	for i, shard := range ce.Shards {
		var shardEstimators map[string]*MetricCardinalityEstimator
		if err := decoder.Decode(&shardEstimators); err != nil {
			return fmt.Errorf("Failed to decode shard %d: %v", i, err)
		}
		shard.Estimators = shardEstimators
	}

	return nil
}

// Can be called concurrently. The other estimator should not be used after the merge.
func (ce *CardinalityEstimator) Merge(other *CardinalityEstimator) error {
	if len(ce.Shards) != len(other.Shards) {
		return fmt.Errorf("mismatched shard counts, self has %d, other has %d", len(ce.Shards), len(other.Shards))
	}

	// merge allocator state
	ce.Allocator.Merge(other.Allocator)

	for i := range ce.Shards {
		err := func() error {
			selfShard := ce.Shards[i]
			otherShard := other.Shards[i]

			selfShard.Lock.Lock()
			otherShard.Lock.Lock()
			defer selfShard.Lock.Unlock()
			defer otherShard.Lock.Unlock()

			// merge estimators
			for metricName, otherEstimator := range otherShard.Estimators {
				selfEstimator, exists := selfShard.Estimators[metricName]
				if !exists {
					selfShard.Estimators[metricName] = otherEstimator
					otherEstimator.Allocator = ce.Allocator // policy: use the self estimator's allocator
					continue
				}

				if err := selfEstimator.Merge(otherEstimator); err != nil {
					return fmt.Errorf("failed to merge estimator for metric %s: %v", metricName, err)
				}
			}

			// merge insert counters
			selfShard.InsertCounter.Set(selfShard.InsertCounter.Get() + otherShard.InsertCounter.Get())

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
	return len(ce.Shards)
}

func (ce *CardinalityEstimator) ShardIdx(metricName string) int {
	fnv := fnv.New64a()
	fnv.Write(unsafe.Slice(unsafe.StringData(metricName), len(metricName)))
	return int(fnv.Sum64() % uint64(len(ce.Shards)))
}

func (ce *CardinalityEstimator) RandomShardIterator() func(yield func(int) bool) {
	return func(yield func(int) bool) {
		seq := ce.InsertSequences[rand.Intn(len(ce.InsertSequences))]
		if len(seq) != len(ce.Shards) {
			log.Panicf("BUG: len(seq)=%d, len(ce.shards)=%d", len(seq), len(ce.Shards))
		}
		for i := range ce.Shards {
			if !yield(seq[i]) {
				return
			}
		}
	}
}
