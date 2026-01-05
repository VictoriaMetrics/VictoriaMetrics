package common

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ce"
	"github.com/VictoriaMetrics/metrics"
)

var (
	timeseriesParsedTotal       = metrics.NewCounter("vm_ce_timeseries_parsed_total")
	writeRequestsTotal          = metrics.NewCounter("vm_ce_write_requests_total")
	writeRequestsProcessedTotal = metrics.NewCounter("vm_ce_write_requests_processed_total")

	_ = metrics.NewGauge("vm_ce_hlls_inuse", func() float64 { return float64(CardinalityEstimator.Allocator.Inuse()) })
	_ = metrics.NewGauge("vm_ce_max_hlls_inuse", func() float64 { return float64(*estimatorMaxHllsInuse) })
)

var (
	estimatorEnabled      = flag.Bool("ce.enabled", false, "Whether the cardinality estimator is enabled.")
	estimatorMaxHllsInuse = flag.Uint64("ce.maxHlls", math.MaxUint64, "Maximum number of HLLs to have inuse. Each fixed dimension combination will create a new HLL. This is a safety limit to avoid OOM in case of high cardinality metrics.")
	estimatorSampleRate   = flag.Int("ce.sampleRate", 1, "1/<N> sampling rate for received timeseries. ex. 1/3 means on average every third timeseries is tracked. <N> should be in range [1, inf).")
	estimatorShards       = flag.Int("ce.shards", 64, "Number of shards to use for cardinality estimator. Timeseries will be sharded by __name__ label to different estimators to allow for concurrent inserts.")

	emitEnabled            = flag.Bool("ce.emit.enabled", true, "Whether estimation metrics are emitted at all.")
	emitEnabledByFixed     = flag.Bool("ce.emit.enabled.byFixed", true, "Whether filtered estimation metrics by fixed dimensions are emitted.")
	emitEnabledByMetric    = flag.Bool("ce.emit.enabled.byMetric", true, "Whether top-level estimation metrics by metric name are emitted.")
	emitEnabledMetricCount = flag.Bool("ce.emit.enabled.metricCount", true, "Whether estimation metrics for label cardinalities are emitted.")
	emitNamespace          = flag.String("ce.emit.namespace", "default", "Value to use for 'namespace' label in emitted cardinality metrics.")
	emitMinCardinality     = flag.Uint64("ce.emit.minCardinality", 1, "Minimum cardinality to emit for estimations metrics.")
)

// Check if nil before use!!
var CardinalityEstimator *ce.CardinalityEstimator

var emitter *ce.CardinalityMetricEmitter
var resetOperator *ce.ResetOperator

func InitCardinalityEstimator() {
	if !*estimatorEnabled {
		return
	}

	CardinalityEstimator = ce.NewCardinalityEstimatorWithSettings(*estimatorShards, *estimatorMaxHllsInuse)
	emitter = ce.NewCardinalityMetricEmitter(context.Background(), CardinalityEstimator, "vm_cardinality_count",
		ce.WithEmitEnabled(*emitEnabled),
		ce.WithEmitEnabledByFixed(*emitEnabledByFixed),
		ce.WithEmitEnabledByMetric(*emitEnabledByMetric),
		ce.WithEmitEnabledMetricCount(*emitEnabledMetricCount),
		ce.WithEmitNamespace(*emitNamespace),
		ce.WithEmitMinCardinality(*emitMinCardinality),
	)
	resetOperator = ce.NewResetOperator(context.Background(), CardinalityEstimator)
}

func MustStopCardinalityEstimator() {
	// TODO: looking for review from vm team
}

func HandleCeGetBinary(w http.ResponseWriter, r *http.Request) {
	data, err := CardinalityEstimator.MarshalBinary()
	if err != nil {
		log.Printf("Cardinality estimator failed to marshal: %v", err)
		http.Error(w, "Failed to marshal", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func HandleUpdateCeResetSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rs ce.ResetSchedule
	if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		return
	}

	resetOperator.UpdateSchedule(&rs)
}

func HandleCeGetCardinality(w http.ResponseWriter, r *http.Request) {
	queryType := r.URL.Query().Get("type")

	switch queryType {
	case "fixed":
		estimate := CardinalityEstimator.EstimateFixedMetricCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			log.Printf("Cardinality estimator failed to encode json, %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	default:
		estimate := CardinalityEstimator.EstimateMetricsCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			log.Printf("Cardinality estimator failed to encode json, %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	}
}

func HandleCeReset(w http.ResponseWriter, r *http.Request) {
	CardinalityEstimator.Reset()
	w.WriteHeader(http.StatusOK)
}
