package timeserieslimits

import (
	"math"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
)

var (
	enabled bool
	// The maximum length of label name.
	//
	// Samples with longer names are ignored.
	maxLabelNameLen = 256

	// The maximum length of label value.
	//
	// Samples with longer label values are ignored.
	maxLabelValueLen = 4 * 1024

	// The maximum number of labels per each timeseries.
	//
	// Samples with exceeding amount of labels are ignored.
	maxLabelsPerTimeseries = 40
)

// MustInit checks if limits are with-in supported range and prepares package for usage
func MustInit(inputMaxLabelsPerTimeseries, inputMaxLabelNameLen, inputMaxLabelValueLen int) {
	mustBeInRange := func(name string, limit int) {
		if limit <= 0 || limit > math.MaxUint16 {
			logger.Fatalf("incorrect limit: %q value: %d, must be in range 1..%d", name, limit, math.MaxUint16)
		}
	}
	mustBeInRange("maxLabelNameLen", inputMaxLabelNameLen)
	mustBeInRange("maxLabelValueLen", inputMaxLabelValueLen)
	Init(inputMaxLabelsPerTimeseries, inputMaxLabelNameLen, inputMaxLabelValueLen)
}

// Init prepares package for usage
func Init(inputMaxLabelsPerTimeseries, inputMaxLabelNameLen, inputMaxLabelValueLen int) {
	maxLabelsPerTimeseries = inputMaxLabelsPerTimeseries
	maxLabelNameLen = inputMaxLabelNameLen
	maxLabelValueLen = inputMaxLabelValueLen
	enabled = maxLabelsPerTimeseries > 0 || maxLabelNameLen > 0 || maxLabelValueLen > 0

	_ = metrics.GetOrCreateGauge(`vm_rows_ignored_total{reason="too_many_labels"}`, func() float64 {
		return float64(ignoredSeriesWithTooManyLabels.Load())
	})
	_ = metrics.GetOrCreateGauge(`vm_rows_ignored_total{reason="too_long_label_name"}`, func() float64 {
		return float64(ignoredSeriesWithTooLongLabelName.Load())
	})
	_ = metrics.GetOrCreateGauge(`vm_rows_ignored_total{reason="too_long_label_value"}`, func() float64 {
		return float64(ignoredSeriesWithTooLongLabelValue.Load())
	})
	_ = metrics.GetOrCreateGauge(`vm_rows_ignored_total{reason="too_long_metric_metadata_value"}`, func() float64 {
		return float64(ignoredMetricsMetadataWithTooLongValue.Load())
	})
}

var (
	ignoredSeriesWithTooManyLabelsLogTicker     = time.NewTicker(5 * time.Second)
	ignoredSeriesWithTooLongLabelNameLogTicker  = time.NewTicker(5 * time.Second)
	ignoredSeriesWithTooLongLabelValueLogTicker = time.NewTicker(5 * time.Second)
)

var (
	// ignoredSeriesWithTooManyLabels is the number of ignored series with too many labels
	ignoredSeriesWithTooManyLabels atomicutil.Uint64

	// ignoredSeriesWithTooLongLabelName is the number of ignored series which contain labels with too long names
	ignoredSeriesWithTooLongLabelName atomicutil.Uint64

	// ignoredSeriesWithTooLongLabelValue is the number of ignored series which contain labels with too long values
	ignoredSeriesWithTooLongLabelValue atomicutil.Uint64

	ignoredMetricsMetadataWithTooLongValue atomicutil.Uint64
)

func trackIgnoredSeriesWithTooManyLabels(labels []prompb.Label) {
	ignoredSeriesWithTooManyLabels.Add(1)
	select {
	case <-ignoredSeriesWithTooManyLabelsLogTicker.C:
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because prompb.LabelsToString() will be called with each trackIgnoredSeriesWithTooManyLabels call.
		logger.Warnf("ignoring series with %d labels for %s; either reduce the number of labels for this metric "+
			"or increase -maxLabelsPerTimeseries=%d cmd-line flag value",
			len(labels), prompb.LabelsToString(labels), maxLabelsPerTimeseries)
	default:
	}
}

func trackIgnoredSeriesWithTooLongLabelValue(l *prompb.Label, labels []prompb.Label) {
	ignoredSeriesWithTooLongLabelValue.Add(1)
	select {
	case <-ignoredSeriesWithTooLongLabelValueLogTicker.C:
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because prompb.LabelsToString() will be called with each trackIgnoredSeriesWithTooLongLabelValue call.
		logger.Warnf("ignoring series with %s=%q label for %s; label value length=%d exceeds -maxLabelValueLen=%d; "+
			"either reduce the label value length or increase -maxLabelValueLen command-line flag value",
			l.Name, l.Value, prompb.LabelsToString(labels), len(l.Value), maxLabelValueLen)
	default:
	}
}

func trackIgnoredSeriesWithTooLongLabelName(l *prompb.Label, labels []prompb.Label) {
	ignoredSeriesWithTooLongLabelName.Add(1)
	select {
	case <-ignoredSeriesWithTooLongLabelNameLogTicker.C:
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		// because prompb.LabelsToString() will be called with each trackIgnoredSeriesWithTooLongLabelName call.
		logger.Warnf("ignoring series with label %q for %s; label name length=%d exceeds max allowed %d - consider reducing label name length.",
			l.Name, prompb.LabelsToString(labels), len(l.Name), maxLabelNameLen)
	default:
	}
}

// Enabled returns true if any of limits is configured
func Enabled() bool {
	return enabled
}

// IsExceeding checks if passed labels exceed one of the limits:
// * Maximum allowed labels limit
// * Maximum allowed label name length limit
// * Maximum allowed label value length limit
//
// increments metrics and shows warning in logs
func IsExceeding(labels []prompb.Label) bool {
	if maxLabelsPerTimeseries > 0 && len(labels) > maxLabelsPerTimeseries {
		trackIgnoredSeriesWithTooManyLabels(labels)
		return true
	}
	if maxLabelNameLen == 0 && maxLabelValueLen == 0 {
		return false
	}
	for _, l := range labels {
		if maxLabelNameLen > 0 && len(l.Name) > maxLabelNameLen {
			trackIgnoredSeriesWithTooLongLabelName(&l, labels)
			return true
		}
		if maxLabelValueLen > 0 && len(l.Value) > maxLabelValueLen {
			trackIgnoredSeriesWithTooLongLabelValue(&l, labels)
			return true
		}
	}
	return false
}
func trackIgnoredMetricMetadataWithTooLongValue(fieldName, metricName string, fieldSize int) {
	ignoredMetricsMetadataWithTooLongValue.Add(1)
	select {
	case <-ignoredSeriesWithTooLongLabelValueLogTicker.C:
		// Do not call logger.WithThrottler() here, since this will result in increased CPU usage
		logger.Warnf("ignoring metric metadata with metric name %q; field %q value length=%d exceeds %d limit; "+
			"reduce the size of field at metric metadata source.",
			metricName, fieldName, fieldSize, metricMetadataMaxFieldValueSize)
	default:
	}
}

// metricMetadataMaxFieldValueSize defines max size of string fields at MetricMetadata
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11128 for details
const metricMetadataMaxFieldValueSize = math.MaxUint16

// IsMetricMetadataExceeding returns true if prompb.MetricMetadata Help, MetricFamilyName, or Unit field value size exceed the 64KiB limit.
//
// Additionally, it increments the corresponding metrics and prints warning messages to the log.
func IsMetricMetadataExceeding(md *prompb.MetricMetadata) bool {
	if len(md.Help) > metricMetadataMaxFieldValueSize {
		trackIgnoredMetricMetadataWithTooLongValue("help", md.MetricFamilyName, len(md.Help))
		return true
	}
	if len(md.MetricFamilyName) > metricMetadataMaxFieldValueSize {
		trackIgnoredMetricMetadataWithTooLongValue("metricFamilyName", md.MetricFamilyName, len(md.MetricFamilyName))

		return true
	}
	if len(md.Unit) > metricMetadataMaxFieldValueSize {
		trackIgnoredMetricMetadataWithTooLongValue("unit", md.MetricFamilyName, len(md.Unit))
		return true
	}

	return false
}

// IsPrometheusMetadataExceeding returns true if prometheus.Metadata Help or Metric field value size exceed the 64KiB limit.
//
// Additionally, it increments the corresponding metrics and prints warning messages to the log.
func IsPrometheusMetadataExceeding(md *prometheus.Metadata) bool {
	if len(md.Help) > metricMetadataMaxFieldValueSize {
		trackIgnoredMetricMetadataWithTooLongValue("help", md.Metric, len(md.Help))
		return true
	}
	if len(md.Metric) > metricMetadataMaxFieldValueSize {
		trackIgnoredMetricMetadataWithTooLongValue("metric", md.Metric, len(md.Metric))

		return true
	}

	return false
}
