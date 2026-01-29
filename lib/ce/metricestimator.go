package ce

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strings"

	"github.com/VictoriaMetrics/metrics"
	"github.com/axiomhq/hyperloglog"
)

var (
	metricEstimatorsCreated = metrics.NewCounter("vm_ce_metric_estimators_created_total")
)

// An HLL counts the number of distinct elements in a stream. Given a stream of timeseries, we are interested in the cardinality of the
// entire stream, as well as the cardinality of substreams with fixed label dimensions. One HLL is used to count the cardinality of the
// entire stream, and one HLL is used to count the cardinality of each substream. Each HLL allows us to estimate the cardinality of a single
// (sub)stream. More fixed label dimensions == more (sub)streams == more HLLs.
//
// EXAMPLE:
// We want to know the number of distinct timeseries for a metric "cpu_usage" with labels "task_name", "region", and "ipaddress".
// We are interested in the cardinality of the entire metric "cpu_usage" as well as the cardinality of the metric by the fixed label dimensions
// of "task_name" and "region". Suppose in our stream "task_name" takes on 10 distinct values, "region" takes on 2 distinct values, "ipaddress"
// takes on 1000 values, and each combination of "task_name", "region", and "ipaddress" occurs. So in total, we have 10 * 2 * 1000 = 20000 distinct
// timeseries, and 10 * 2 = 20 distinct combinations of "task_name" and "region" that we need to track cardinalities for.
//
// So in total:
//  1. We will need 1 HLL to count the number of distinct timeseries for the entire metric "cpu_usage". We insert every single timeseries into this HLL.
//  2. We will need 20 HLLs to count the number of distinct timeseries for each combination of "task_name" and "region". Each HLL is responsible for counting
//     the cardinality of a single combination of "task_name" and "region". For example, one HLL will count the cardinality for "task_name=api_server,region=us-east-1",
//     another HLL will count the cardinality for "task_name=api_server,region=us-central", and so on. For each HLL, we only insert timeseries that
//     match the specific combination of "task_name" and "region" that the HLL is responsible for.
//
// After inserting all timeseries according to the above rules, the results of the cardinality estimator will be:
//  1. The HLL for the entire metric "cpu_usage" will estimate a cardinality of ~20000.
//  2. Each of the 20 HLLs for the combinations of "task_name" and "region" will estimate a cardinality of ~1000, since each combination
//     of "task_name" and "region" is associated with 1000 distinct "ipaddress" values.
type MetricCardinalityEstimator struct {
	MetricName string
	Allocator  *Allocator

	MetricHll *hyperloglog.Sketch            // HLL for the entire stream of metrics.
	Hlls      map[string]*hyperloglog.Sketch // HLLs for each substream of metrics by fixed label dimensions.

	B  []byte
	B1 []byte
}

func NewMetricCardinalityEstimator(metricName string) *MetricCardinalityEstimator {
	ret, err := NewMetricCardinalityEstimatorWithAllocator(metricName, NewAllocator(1_000_000))
	if err != nil {
		log.Panicf("BUG: failed to create MetricCardinalityEstimator: %v", err)
	}
	return ret
}

func NewMetricCardinalityEstimatorWithAllocator(metricName string, allocator *Allocator) (*MetricCardinalityEstimator, error) {

	ret := MetricCardinalityEstimator{
		MetricName: metricName,
		Hlls:       make(map[string]*hyperloglog.Sketch),
		MetricHll:  nil,
		B:          make([]byte, 1024),
		B1:         make([]byte, 1024),
		Allocator:  allocator,
	}

	hll, err := allocator.Allocate()
	if err != nil {
		return nil, err
	}
	ret.MetricHll = hll

	metricEstimatorsCreated.Inc()
	return &ret, nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) EstimateFixedMetricCardinality() map[string]uint64 {
	estimates := make(map[string]uint64)

	for key, hll := range mce.Hlls {
		estimates[key] = hll.Estimate()
	}

	return estimates
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) EstimateMetricCardinality() uint64 {
	return mce.MetricHll.Estimate()
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) MarshalBinary() ([]byte, error) {
	anon := struct {
		MetricName          string
		MetricNameFixedHlls map[string]*hyperloglog.Sketch
		MetricNameHll       *hyperloglog.Sketch
		Allocator           *Allocator
	}{
		MetricName:          mce.MetricName,
		MetricNameFixedHlls: mce.Hlls,
		MetricNameHll:       mce.MetricHll,
		Allocator:           mce.Allocator,
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(anon); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) UnmarshalBinary(data []byte) error {
	anon := struct {
		MetricName          string
		MetricNameFixedHlls map[string]*hyperloglog.Sketch
		MetricNameHll       *hyperloglog.Sketch
		Allocator           *Allocator
	}{}

	var buf bytes.Buffer
	buf.Write(data)
	if err := gob.NewDecoder(&buf).Decode(&anon); err != nil {
		return err
	}

	mce.MetricName = anon.MetricName
	mce.Hlls = anon.MetricNameFixedHlls
	mce.MetricHll = anon.MetricNameHll
	mce.Allocator = anon.Allocator

	return nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) Merge(other *MetricCardinalityEstimator) error {
	if mce.MetricName != other.MetricName {
		return fmt.Errorf("BUG: cannot merge estimators with different metric names: %q vs %q", mce.MetricName, other.MetricName)
	}

	if err := mce.MetricHll.Merge(other.MetricHll); err != nil {
		return fmt.Errorf("failed to merge metric name HLL: %w", err)
	}

	for path, otherHll := range other.Hlls {
		hll := mce.Hlls[path]
		if hll == nil {
			mce.Hlls[path] = otherHll.Clone()
			continue
		}

		if err := hll.Merge(otherHll); err != nil {
			return fmt.Errorf("failed to merge fixed metric HLL for path %q: %w", path, err)
		}
	}

	return nil
}

// Return slice only valid until the next call to EncodeTimeseriesPath
func (mce *MetricCardinalityEstimator) EncodeTimeseriesPath(metricName string, fixedLabelValue1 string, fixedLabelValue2 string) []byte {
	mce.B1 = mce.B1[:0]

	mce.B1 = append(mce.B1, metricName...)
	mce.B1 = append(mce.B1, 0x00) // \x00 cannot appear in label names/values, so its okay to use it as a separator
	mce.B1 = append(mce.B1, []byte(fixedLabelValue1)...)
	mce.B1 = append(mce.B1, 0x00)
	mce.B1 = append(mce.B1, []byte(fixedLabelValue2)...)

	return mce.B1
}

func DecodeTimeSeriesPath(path string) (metricName, fixedLabel1Val, fixedLabel2Val string) {
	parts := strings.Split(path, "\x00")
	if len(parts) != 3 {
		log.Panicf("BUG: invalid timeseries path %q with parts %v", path, parts)
	}

	return parts[0], parts[1], parts[2]
}
