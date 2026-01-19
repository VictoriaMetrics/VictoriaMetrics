package ce

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

type CardinalityMetricEmitter struct {
	ce *CardinalityEstimator

	lock    sync.Mutex
	metrics bytes.Buffer

	// configs
	emitMetricNamePrefix   string // Prefix for emitted metric names
	emitEnabled            bool   // Whether estimation metrics are emitted at all.
	emitEnabledByFixed     bool   // Whether to emit by all fixed dimensions.
	emitEnabledByMetric    bool   // Whether to emit by metric name.
	emitEnabledMetricCount bool   // Whether to emit metric name cardinality.

	emitNamespace      string // User defined namespace for emitted metrics.
	emitMinCardinality uint64 // Minimum cardinality to emit.
}

func NewCardinalityMetricEmitter(ctx context.Context, ce *CardinalityEstimator, emitMetricNamePrefix string, opts ...EmitOption) *CardinalityMetricEmitter {
	cme := &CardinalityMetricEmitter{
		ce:      ce,
		metrics: bytes.Buffer{},
		lock:    sync.Mutex{},

		emitMetricNamePrefix: emitMetricNamePrefix,

		emitEnabled:            true,
		emitEnabledByFixed:     true,
		emitEnabledByMetric:    true,
		emitEnabledMetricCount: true,

		emitNamespace:      "default",
		emitMinCardinality: 0,
	}

	// apply options
	for _, opt := range opts {
		opt(cme)
	}

	// Low frequency goroutine that calculates estimations and emits metrics.
	go func() {
		if !cme.emitEnabled {
			log.Printf("CardinalityMetricEmitter is disabled.")
			return
		}

		b := bytes.Buffer{}

		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Printf("CardinalityMetricEmitter stopped: %v", ctx.Err())
				return
			default:
				b.Reset()

				// Estimate cardinality by metric name
				topLevelEstimates := cme.ce.EstimateMetricsCardinality()
				if cme.emitEnabledByMetric {
					for _, entry := range topLevelEstimates.CardinalityDescByMetricName {
						if entry.Cardinality < cme.emitMinCardinality {
							continue
						}
						b.WriteString(fmt.Sprintf("%s:sum:metric{metric=\"%s\",namespace=\"%s\"} %d\n", cme.emitMetricNamePrefix, entry.MetricName, cme.emitNamespace, entry.Cardinality))
					}
				}
				b.WriteString(fmt.Sprintf("%s:sum{namespace=\"%s\"} %d\n", cme.emitMetricNamePrefix, cme.emitNamespace, topLevelEstimates.CardinalityTotal))

				// Estimate metric name cardinality
				if cme.emitEnabledMetricCount {
					b.WriteString(fmt.Sprintf("%s_metrics{namespace=\"%s\"} %d\n", cme.emitMetricNamePrefix, cme.emitNamespace, len(topLevelEstimates.CardinalityDescByMetricName)))
				}

				// Estimate cardinality by fixed dimensions, ex. metric="mymetric", label1="value1", label2="value2"
				if cme.emitEnabledByFixed {
					fixedDimensionsEstimates := cme.ce.EstimateFixedMetricCardinality()
					for path, cardinality := range fixedDimensionsEstimates {
						if cardinality < cme.emitMinCardinality {
							continue
						}

						metricName, fixedLabel1Val, fixedLabel2Val := DecodeTimeSeriesPath(path)
						b.WriteString(
							fmt.Sprintf("%s{metric=\"%s\",%s=\"%s\",%s=\"%s\",namespace=\"%s\"} %d\n",
								cme.emitMetricNamePrefix,
								metricName,
								*prompb.CardinalityEstimatorFixedLabel1, fixedLabel1Val,
								*prompb.CardinalityEstimatorFixedLabel2, fixedLabel2Val,
								cme.emitNamespace,
								cardinality,
							),
						)
					}
				}

				// Flush the metrics to a read buffer
				cme.lock.Lock()
				cme.metrics.Reset()
				cme.metrics.Write(b.Bytes())
				cme.lock.Unlock()
			}
		}
	}()

	return cme
}

func (cme *CardinalityMetricEmitter) WritePrometheus(w io.Writer) error {
	cme.lock.Lock()
	defer cme.lock.Unlock()

	_, err := w.Write(cme.metrics.Bytes())
	return err
}

type EmitOption func(*CardinalityMetricEmitter)

func WithEmitEnabled(enabled bool) EmitOption {
	return func(cme *CardinalityMetricEmitter) {
		cme.emitEnabled = enabled
	}
}

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
