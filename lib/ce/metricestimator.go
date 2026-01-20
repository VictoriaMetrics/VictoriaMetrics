package ce

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/metrics"
	"github.com/axiomhq/hyperloglog"
)

var (
	metricEstimatorsCreated = metrics.NewCounter("ce_metric_estimators_created_total")
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
	metricName string
	allocator  *Allocator

	metricHll *hyperloglog.Sketch            // HLL for the entire stream of metrics.
	hlls      map[string]*hyperloglog.Sketch // HLLs for each substream of metrics by fixed label dimensions.

	b  []byte
	b1 []byte
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
		metricName: metricName,
		hlls:       make(map[string]*hyperloglog.Sketch),
		metricHll:  nil,
		b:          make([]byte, 1024),
		b1:         make([]byte, 1024),
		allocator:  allocator,
	}

	hll, err := allocator.Allocate()
	if err != nil {
		return nil, err
	}
	ret.metricHll = hll

	metricEstimatorsCreated.Inc()
	return &ret, nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) Insert(ts prompb.TimeSeries) error {
	// Make sure the timeseries has a metric name label and it matches the estimator's metric name
	if ts.MetricName != mce.metricName {
		return fmt.Errorf("BUG: timeseries metric name (%s) does not match estimator metric name (%s)", ts.MetricName, mce.metricName)
	}

	tsEncoding := mce.byteifyLabelSet(ts.Labels)

	// Count cardinality for the whole metric
	mce.metricHll.Insert(tsEncoding)

	// Count cardinality for the whole metric by fixed dimension
	pathBytes := mce.encodeTimeseriesPath(ts)
	path := unsafe.String(unsafe.SliceData(pathBytes), len(pathBytes))

	hll := mce.hlls[path]
	if hll == nil {
		path := strings.Clone(path) // ensure we own the string

		newHll, err := mce.allocator.Allocate()
		if err != nil {
			return err
		}

		hll = newHll
		mce.hlls[path] = newHll
	}

	hll.Insert(tsEncoding)

	return nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) EstimateFixedMetricCardinality() map[string]uint64 {
	estimates := make(map[string]uint64)

	for key, hll := range mce.hlls {
		estimates[key] = hll.Estimate()
	}

	return estimates
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) EstimateMetricCardinality() uint64 {
	return mce.metricHll.Estimate()
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) MarshalBinary() ([]byte, error) {
	anon := struct {
		MetricName          string
		MetricNameFixedHlls map[string]*hyperloglog.Sketch
		MetricNameHll       *hyperloglog.Sketch
		Allocator           *Allocator
	}{
		MetricName:          mce.metricName,
		MetricNameFixedHlls: mce.hlls,
		MetricNameHll:       mce.metricHll,
		Allocator:           mce.allocator,
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

	mce.metricName = anon.MetricName
	mce.hlls = anon.MetricNameFixedHlls
	mce.metricHll = anon.MetricNameHll
	mce.allocator = anon.Allocator

	return nil
}

// Do not call this function concurrently.
func (mce *MetricCardinalityEstimator) Merge(other *MetricCardinalityEstimator) error {
	if mce.metricName != other.metricName {
		return fmt.Errorf("BUG: cannot merge estimators with different metric names: %q vs %q", mce.metricName, other.metricName)
	}

	if err := mce.metricHll.Merge(other.metricHll); err != nil {
		return fmt.Errorf("failed to merge metric name HLL: %w", err)
	}

	for path, otherHll := range other.hlls {
		hll := mce.hlls[path]
		if hll == nil {
			mce.hlls[path] = otherHll.Clone()
			continue
		}

		if err := hll.Merge(otherHll); err != nil {
			return fmt.Errorf("failed to merge fixed metric HLL for path %q: %w", path, err)
		}
	}

	return nil
}

// Return slice only valid until the next call to encodeTimeseriesPath
func (mce *MetricCardinalityEstimator) encodeTimeseriesPath(ts prompb.TimeSeries) []byte {
	mce.b1 = mce.b1[:0]

	mce.b1 = append(mce.b1, ts.MetricName...)
	mce.b1 = append(mce.b1, 0x00) // \x00 cannot appear in label names/values, so its okay to use it as a separator
	mce.b1 = append(mce.b1, []byte(ts.FixedLabelValue1)...)
	mce.b1 = append(mce.b1, 0x00)
	mce.b1 = append(mce.b1, []byte(ts.FixedLabelValue2)...)

	return mce.b1
}

// Return slice only valid until the next call to byteifyLabelSet
func (mce *MetricCardinalityEstimator) byteifyLabelSet(labels []prompb.Label) []byte {
	mce.b = mce.b[:0]

	for _, l := range labels {
		if l.Name == "__name__" { // We require this label to be static, so skip it and save cpu
			continue
		}

		mce.b = append(mce.b, l.Name...)
		mce.b = append(mce.b, 0x00) // \x00 cannot appear in label names/values, so its okay to use it as a separator
		mce.b = append(mce.b, l.Value...)
		mce.b = append(mce.b, 0x00)
	}

	return mce.b
}

func DecodeTimeSeriesPath(path string) (metricName, fixedLabel1Val, fixedLabel2Val string) {
	parts := strings.Split(path, "\x00")
	if len(parts) != 3 {
		log.Panicf("BUG: invalid timeseries path %q with parts %v", path, parts)
	}

	return parts[0], parts[1], parts[2]
}
