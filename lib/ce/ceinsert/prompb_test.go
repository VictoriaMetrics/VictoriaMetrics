package ceinsert

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/stretchr/testify/assert"
)

func Test_HLLAccuracy(t *testing.T) {
	estimator := ce.NewCardinalityEstimatorWithSettings(64, math.MaxInt64, 1) // 64 shards, unlimited hlls

	// Target unique instances
	numInstances := 1_000_000
	tss := generateUniqueTimeseriesPrompb(numInstances, func() string {
		// low # of metrics => more timeseries per metric => ensure HLLs are in dense representation
		return fmt.Sprintf("test_metric_%d", rand.Int63n(10))
	})

	// Send timeseries concurrently
	numGoroutines := 1000
	chunkSize := numInstances/numGoroutines + 1

	var wg sync.WaitGroup
	for i := range numGoroutines {
		wg.Go(func() {
			start := i * chunkSize
			end := min(start+chunkSize, numInstances)

			if start >= end {
				return
			}

			err := InsertPrompb(estimator, tss[start:end])
			if err != nil {
				t.Errorf("Failed to insert data: %v", err)
			}
		})
	}

	wg.Wait()
	t.Logf("Finished writing %d unique metrics", numInstances)

	// Get the cardinality estimate for instances
	estimate := estimator.EstimateMetricsCardinality().CardinalityTotal

	// Calculate accuracy
	actual := uint64(numInstances)
	accuracy := float64(estimate) / float64(actual)
	errorPercent := math.Abs(1.0-accuracy) * 100

	t.Logf("Actual cardinality: %d", actual)
	t.Logf("HLL estimate: %d", estimate)
	t.Logf("Error: %.2f%%", errorPercent)

	// HyperLogLog with precision 10 should have ~3% standard error
	// We'll be more lenient and expect within 5% accuracy
	if accuracy < 0.95 || accuracy > 1.05 {
		t.Errorf("Accuracy %.2f%% is outside expected range (95%%-105%%)", accuracy*100)
	}

	// Also test that the estimate is reasonable (not way off)
	if estimate < actual/2 || estimate > actual*2 {
		t.Errorf("Estimate %d is too far from actual %d", estimate, actual)
	}
}

func Test_MarshalUnmarshalBinary(t *testing.T) {
	estimator := ce.NewCardinalityEstimatorWithSettings(4, math.MaxUint64, 1)

	testTimeSeries := []prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "metric_a"},
				{Name: "instance", Value: "server1"},
			},
			Samples: []prompb.Sample{{Value: 100.0, Timestamp: 1234567890000}},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "metric_b"},
				{Name: "instance", Value: "server2"},
			},
			Samples: []prompb.Sample{{Value: 200.0, Timestamp: 1234567890000}},
		},
	}

	// insert
	err := InsertPrompb(estimator, testTimeSeries)
	assert.NoError(t, err)

	initialEstimates := estimator.EstimateMetricsCardinality()

	// marshal
	marshaledData, err := estimator.MarshalBinary()
	assert.NoError(t, err)

	// unmarshal
	newCe := ce.NewCardinalityEstimatorWithSettings(4, math.MaxUint64, 1)
	err = newCe.UnmarshalBinary(marshaledData)
	assert.NoError(t, err)

	// basic check
	unmarshaledEstimates := newCe.EstimateMetricsCardinality()
	assert.Equal(t, initialEstimates.CardinalityTotal, unmarshaledEstimates.CardinalityTotal)
	assert.Equal(t, len(initialEstimates.CardinalityDescByMetricName), len(unmarshaledEstimates.CardinalityDescByMetricName))

	// test insert counts are the same
	for i := range estimator.Shards {
		assert.Equal(t, estimator.Shards[i].InsertCounter.Get(), newCe.Shards[i].InsertCounter.Get())
	}

	// test allocator states are the same
	assert.Equal(t, estimator.Allocator.Inuse(), newCe.Allocator.Inuse())
	assert.Equal(t, estimator.Allocator.Created(), newCe.Allocator.Created())
	assert.Equal(t, estimator.Allocator.Max(), newCe.Allocator.Max())
}

func generateUniqueTimeseriesPrompb(count int, metricNameGen func() string) []prompb.TimeSeries {
	timeseries := []prompb.TimeSeries{}

	randGen := rand.New(rand.NewSource(123))

	if metricNameGen == nil {
		metricNameGen = func() string {
			return fmt.Sprintf("test_metric_%d", randGen.Int63n(30000))
		}
	}

	for i := range count {
		name := metricNameGen()

		ts := prompb.TimeSeries{
			Labels: []prompb.Label{
				{Name: "__name__", Value: name},
				{Name: "instance", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "region", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "dc", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "label1", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "label2", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "label3", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "label4", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
				{Name: "label5", Value: fmt.Sprintf("onlytwentycharacters%d", randGen.Int63n(math.MaxInt))},
			},
			Samples: []prompb.Sample{
				{Value: float64(i), Timestamp: time.Now().UnixMilli()},
			},
			MetricName:       name,
			FixedLabelValue1: "",
			FixedLabelValue2: "",
			ShardIdx:         0,
		}
		timeseries = append(timeseries, ts)
	}

	return timeseries
}

func Benchmark_MetricCardinalityEstimator(b *testing.B) {

	timeseries := generateUniqueTimeseriesPrompb(1000, func() string { return "test_metric" })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mce := ce.NewMetricCardinalityEstimator("test_metric")

		for _, data := range timeseries {
			err := mceInsertPrompb(mce, data)
			if err != nil {
				b.Errorf("Failed to insert data: %v", err)
			}
		}
	}
	b.StopTimer()
}

// Benchmark_CardinalityEstimator_EndToEnd benchmarks the full end-to-end flow
// using the CardinalityEstimator with realistic batch sizes and concurrent access patterns.
func Benchmark_CardinalityEstimator_EndToEnd(b *testing.B) {
	// Setup: create a fresh estimator with default settings
	estimator := ce.NewCardinalityEstimatorWithSettings(64, math.MaxUint64, 1)

	// Generate test data: 10k timeseries across 100 metrics
	timeseries := generateUniqueTimeseriesPrompb(10000, func() string {
		return fmt.Sprintf("test_metric_%d", rand.Int63n(100))
	})

	// Simulate realistic batch sizes (typical remote write batch is 500-2000 samples)
	batchSize := 500
	batches := make([][]prompb.TimeSeries, 0)
	for i := 0; i < len(timeseries); i += batchSize {
		end := min(i+batchSize, len(timeseries))
		batches = append(batches, timeseries[i:end])
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Insert all batches
		for _, batch := range batches {
			if err := InsertRawPrompb(estimator, batch); err != nil {
				b.Fatalf("Failed to insert batch: %v", err)
			}
		}
	}

	b.StopTimer()

	// Report throughput
	b.ReportMetric(float64(len(timeseries)), "timeseries/op")
}

// Benchmark_CardinalityEstimator_EndToEnd_Concurrent benchmarks concurrent inserts
// simulating multiple concurrent remote write requests.
func Benchmark_CardinalityEstimator_EndToEnd_Concurrent(b *testing.B) {
	// Setup: create a fresh estimator with default settings
	estimator := ce.NewCardinalityEstimatorWithSettings(64, math.MaxUint64, 1)

	// Generate test data: 10k timeseries across 100 metrics
	timeseries := generateUniqueTimeseriesPrompb(10000, func() string {
		return fmt.Sprintf("test_metric_%d", rand.Int63n(100))
	})

	// Simulate realistic batch sizes
	batchSize := 500
	batches := make([][]prompb.TimeSeries, 0)
	for i := 0; i < len(timeseries); i += batchSize {
		end := min(i+batchSize, len(timeseries))
		batches = append(batches, timeseries[i:end])
	}

	numGoroutines := 8 // Simulate 8 concurrent writers

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		batchChan := make(chan []prompb.TimeSeries, len(batches))

		// Queue all batches
		for _, batch := range batches {
			batchChan <- batch
		}
		close(batchChan)

		// Start concurrent workers
		for range numGoroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for batch := range batchChan {
					if err := InsertRawPrompb(estimator, batch); err != nil {
						b.Errorf("Failed to insert batch: %v", err)
					}
				}
			}()
		}

		wg.Wait()
	}

	b.StopTimer()

	// Report throughput
	b.ReportMetric(float64(len(timeseries)), "timeseries/op")
	b.ReportMetric(float64(numGoroutines), "goroutines")
}

// MAKE SURE THIS BENCHMARK RETURNS 0 ALLOCS/OP
func Benchmark_CardinalityEstimator_EndToEnd_SingleBatch_1Alloc(b *testing.B) {
	// Setup: create a fresh estimator with default settings
	estimator := ce.NewCardinalityEstimatorWithSettings(64, math.MaxUint64, 1)

	// Generate a single batch of 500 timeseries (typical remote write batch size)
	batch := generateUniqueTimeseriesPrompb(500, func() string {
		return fmt.Sprintf("test_metric")
	})

	// Warm up: insert once to ensure all maps and structures are initialized
	if err := InsertRawPrompb(estimator, batch); err != nil {
		b.Fatalf("Failed to insert batch: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := InsertRawPrompb(estimator, batch); err != nil {
			b.Fatalf("Failed to insert batch: %v", err)
		}
	}

	b.StopTimer()
}
