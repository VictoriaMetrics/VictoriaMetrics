package ce

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"math"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
)

var (
	_ = metrics.NewGauge("vm_ce_hlls_inuse", func() float64 {
		if DefaultCardinalityEstimator != nil {
			return float64(DefaultCardinalityEstimator.Allocator.Inuse())
		}
		return 0
	})
	_ = metrics.NewGauge("vm_ce_max_hlls_inuse", func() float64 { return float64(*EstimatorMaxHllsInuse) })
)

var (
	EstimatorDefaultEnabled = flag.Bool("ce.defaultEnabled", false, "Whether the default cardinality estimator is enabled.")
	EstimatorMaxHllsInuse   = flag.Uint64("ce.maxHlls", math.MaxUint64, "Maximum number of HLLs to have inuse. Each fixed dimension combination will create a new HLL. This is a safety limit to avoid OOM in case of high cardinality metrics.")
	EstimatorSampleRate     = flag.Int("ce.sampleRate", 1, "1/<N> sampling rate for received timeseries. ex. 1/3 means on average every third timeseries is tracked. <N> should be in range [1, inf).")
	EstimatorShards         = flag.Int("ce.shards", 64, "Number of shards to use for cardinality estimator. Timeseries will be sharded by __name__ label to different estimators to allow for concurrent inserts.")

	EstimatorFixedLabel1 = flag.String("ce.fixedLabel1", "job", "First fixed label for cardinality estimator.")
	EstimatorFixedLabel2 = flag.String("ce.fixedLabel2", "region", "Second fixed label for cardinality estimator.")

	EmitEnabled            = flag.Bool("ce.emit.enabled", true, "Whether estimation metrics are emitted at all.")
	EmitEnabledByFixed     = flag.Bool("ce.emit.enabled.byFixed", true, "Whether filtered estimation metrics by fixed dimensions are emitted.")
	EmitEnabledByMetric    = flag.Bool("ce.emit.enabled.byMetric", true, "Whether top-level estimation metrics by metric name are emitted.")
	EmitEnabledMetricCount = flag.Bool("ce.emit.enabled.metricCount", true, "Whether estimation metrics for label cardinalities are emitted.")
	EmitNamespace          = flag.String("ce.emit.namespace", "default", "Value to use for 'namespace' label in emitted cardinality metrics.")
	EmitMinCardinality     = flag.Uint64("ce.emit.minCardinality", 1, "Minimum cardinality to emit for estimations metrics.")
)

var DefaultCardinalityEstimator *CardinalityEstimator
var DefaultCardinalityMetricEmitter *CardinalityMetricEmitter
var DefaultResetOperator *ResetOperator

func InitDefaultCardinalityEstimator() {
	if !*EstimatorDefaultEnabled {
		log.Printf("default cardinality estimator is disabled.")
		return
	}

	DefaultCardinalityEstimator = NewCardinalityEstimatorWithSettings(*EstimatorShards, *EstimatorMaxHllsInuse, *EstimatorSampleRate)
	DefaultCardinalityMetricEmitter = NewCardinalityMetricEmitter(context.Background(), DefaultCardinalityEstimator, "vm_cardinality_count",
		WithEmitEnabled(*EmitEnabled),
		WithEmitEnabledByFixed(*EmitEnabledByFixed),
		WithEmitEnabledByMetric(*EmitEnabledByMetric),
		WithEmitEnabledMetricCount(*EmitEnabledMetricCount),
		WithEmitNamespace(*EmitNamespace),
		WithEmitMinCardinality(*EmitMinCardinality),
	)
	DefaultResetOperator = NewResetOperator(context.Background(), DefaultCardinalityEstimator)

}

func MustStopDefaultCardinalityEstimator() {
	if !*EstimatorDefaultEnabled {
		return
	}
	// TODO: looking for review from vm team to see what to do here
}

func HandleCeGetBinary(w http.ResponseWriter, r *http.Request) {
	if *EstimatorDefaultEnabled == false {
		http.Error(w, "Cardinality estimator is disabled", http.StatusBadRequest)
		return
	}

	data, err := DefaultCardinalityEstimator.MarshalBinary()
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
	if *EstimatorDefaultEnabled == false {
		http.Error(w, "Cardinality estimator is disabled", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rs ResetSchedule
	if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		return
	}

	DefaultResetOperator.UpdateSchedule(&rs)
}

func HandleCeGetCardinality(w http.ResponseWriter, r *http.Request) {
	if *EstimatorDefaultEnabled == false {
		http.Error(w, "Cardinality estimator is disabled", http.StatusBadRequest)
		return
	}

	queryType := r.URL.Query().Get("type")

	switch queryType {
	case "fixed":
		estimate := DefaultCardinalityEstimator.EstimateFixedMetricCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			log.Printf("Cardinality estimator failed to encode json, %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	default:
		estimate := DefaultCardinalityEstimator.EstimateMetricsCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			log.Printf("Cardinality estimator failed to encode json, %v", err)
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	}
}

func HandleCeReset(w http.ResponseWriter, r *http.Request) {
	if *EstimatorDefaultEnabled == false {
		http.Error(w, "Cardinality estimator is disabled", http.StatusBadRequest)
		return
	}

	DefaultCardinalityEstimator.Reset()
	w.WriteHeader(http.StatusOK)
}
