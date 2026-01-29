package ce

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/VictoriaMetrics/metrics"
)

type CardinalityMetricEmitter struct {
	ce         *CardinalityEstimator
	metricsSet *metrics.Set

	// configs
	emitMetricNamePrefix   string // Prefix for emitted metric names
	emitEnabledByFixed     bool   // Whether to emit by all fixed dimensions.
	emitEnabledByMetric    bool   // Whether to emit by metric name.
	emitEnabledMetricCount bool   // Whether to emit metric name cardinality.

	emitNamespace      string // User defined namespace for emitted metrics.
	emitMinCardinality uint64 // Minimum cardinality to emit.
	emitGauge          bool   // Whether this is to emit gauge metrics.
}

func NewCardinalityMetricEmitter(ctx context.Context, ce *CardinalityEstimator, emitMetricNamePrefix string, opts ...EmitOption) *CardinalityMetricEmitter {
	cme := &CardinalityMetricEmitter{
		ce:         ce,
		metricsSet: metrics.NewSet(),

		emitMetricNamePrefix:   emitMetricNamePrefix,
		emitEnabledByFixed:     true,
		emitEnabledByMetric:    true,
		emitEnabledMetricCount: true,

		emitNamespace:      "default",
		emitMinCardinality: 0,
		emitGauge:          false,
	}
	metrics.RegisterSet(cme.metricsSet)

	// apply options
	for _, opt := range opts {
		opt(cme)
	}

	// Low frequency goroutine that calculates estimations and emits metrics.
	go func() {

		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Printf("CardinalityMetricEmitter stopped: %v", ctx.Err())
				return
			default:
				cme.emitMetrics()
			}
		}
	}()

	return cme
}

func (cme *CardinalityMetricEmitter) WritePrometheus(w io.Writer) {
	cme.metricsSet.WritePrometheus(w)
}

func (cme *CardinalityMetricEmitter) emitMetric(name string, val uint64) {
	if cme.emitGauge {
		cme.metricsSet.GetOrCreateGauge(name, nil).Set(float64(val))
	} else {
		cme.metricsSet.GetOrCreateCounter(name).Set(val)
	}
}

func (cme *CardinalityMetricEmitter) emitMetrics() {
	// Estimate cardinality by metric name
	topLevelEstimates := cme.ce.EstimateMetricsCardinality()
	if cme.emitEnabledByMetric {
		for _, entry := range topLevelEstimates.CardinalityDescByMetricName {
			if entry.Cardinality < cme.emitMinCardinality {
				continue
			}

			cme.emitMetric(fmt.Sprintf("%s:sum:metric{metric=\"%s\",namespace=\"%s\"}", cme.emitMetricNamePrefix, entry.MetricName, cme.emitNamespace), entry.Cardinality)
		}
	}
	cme.emitMetric(fmt.Sprintf("%s:sum{namespace=\"%s\"}", cme.emitMetricNamePrefix, cme.emitNamespace), topLevelEstimates.CardinalityTotal)

	// Estimate metric name cardinality
	if cme.emitEnabledMetricCount {
		cme.emitMetric(fmt.Sprintf("%s_metrics{namespace=\"%s\"}", cme.emitMetricNamePrefix, cme.emitNamespace), uint64(len(topLevelEstimates.CardinalityDescByMetricName)))
	}

	// Estimate cardinality by fixed dimensions, ex. metric="mymetric", label1="value1", label2="value2"
	if cme.emitEnabledByFixed {
		fixedDimensionsEstimates := cme.ce.EstimateFixedMetricCardinality()
		for path, cardinality := range fixedDimensionsEstimates {
			if cardinality < cme.emitMinCardinality {
				continue
			}

			metricName, fixedLabel1Val, fixedLabel2Val := DecodeTimeSeriesPath(path)
			cme.emitMetric(
				fmt.Sprintf("%s{metric=\"%s\",%s=\"%s\",%s=\"%s\",namespace=\"%s\"}",
					cme.emitMetricNamePrefix,
					metricName,
					cme.ce.FixedLabel1, fixedLabel1Val,
					cme.ce.FixedLabel2, fixedLabel2Val,
					cme.emitNamespace,
				),
				cardinality,
			)
		}
	}
}

type EmitOption func(*CardinalityMetricEmitter)

func WithEmitEnabledByFixed(enabled bool) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitEnabledByFixed = enabled
	}
}

func WithEmitEnabledByMetric(enabled bool) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitEnabledByMetric = enabled
	}
}

func WithEmitEnabledMetricCount(enabled bool) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitEnabledMetricCount = enabled
	}
}

func WithEmitNamespace(namespace string) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitNamespace = namespace
	}
}

func WithEmitMinCardinality(minCardinality uint64) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitMinCardinality = minCardinality
	}
}

func WithEmitGauge(isGauge bool) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitGauge = isGauge
	}
}
