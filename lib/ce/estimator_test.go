package ce

import (
	"fmt"
	"math"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/stretchr/testify/assert"
)

func Test_ShardIterator(t *testing.T) {

	n := 10
	ce := NewCardinalityEstimatorWithSettings(n, math.MaxUint64)

	correct := []int{}
	for i := range n {
		correct = append(correct, i)
	}

	for range 10000 {

		seq := []int{}
		for i := range ce.randomShardIterator() {
			seq = append(seq, i)
		}

		if !assert.ElementsMatch(t, seq, correct, fmt.Sprintf("sequences do not match: %v vs %v", seq, correct)) {
			return
		}
	}

}

func Test_MarshalUnmarshalBinary(t *testing.T) {
	ce := NewCardinalityEstimatorWithSettings(4, math.MaxUint64)

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
	err := ce.Insert(testTimeSeries)
	assert.NoError(t, err)

	initialEstimates := ce.EstimateMetricsCardinality()

	// marshal
	marshaledData, err := ce.MarshalBinary()
	assert.NoError(t, err)

	// unmarshal
	newCe := NewCardinalityEstimatorWithSettings(4, math.MaxUint64)
	err = newCe.UnmarshalBinary(marshaledData)
	assert.NoError(t, err)

	// basic check
	unmarshaledEstimates := newCe.EstimateMetricsCardinality()
	assert.Equal(t, initialEstimates.CardinalityTotal, unmarshaledEstimates.CardinalityTotal)
	assert.Equal(t, len(initialEstimates.CardinalityDescByMetricName), len(unmarshaledEstimates.CardinalityDescByMetricName))

	// test insert counts are the same
	for i := range ce.shards {
		assert.Equal(t, ce.shards[i].insertCounter.Get(), newCe.shards[i].insertCounter.Get())
	}

	// test allocator states are the same
	assert.Equal(t, ce.Allocator.Inuse(), newCe.Allocator.Inuse())
	assert.Equal(t, ce.Allocator.Created(), newCe.Allocator.Created())
	assert.Equal(t, ce.Allocator.Max(), newCe.Allocator.Max())
}
