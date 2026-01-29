package ce

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

	EstimatorFixedLabel1 = flag.String("ce.fixedLabel1", ce.CE_DEFAULT_FIXED_LABEL_1, "First fixed label for cardinality estimator.")
	EstimatorFixedLabel2 = flag.String("ce.fixedLabel2", ce.CE_DEFAULT_FIXED_LABEL_2, "Second fixed label for cardinality estimator.")

	EmitEnabledByFixed     = flag.Bool("ce.emit.enabled.byFixed", true, "Whether filtered estimation metrics by fixed dimensions are emitted.")
	EmitEnabledByMetric    = flag.Bool("ce.emit.enabled.byMetric", true, "Whether top-level estimation metrics by metric name are emitted.")
	EmitEnabledMetricCount = flag.Bool("ce.emit.enabled.metricCount", true, "Whether estimation metrics for label cardinalities are emitted.")
	EmitNamespace          = flag.String("ce.emit.namespace", "default", "Value to use for 'namespace' label in emitted cardinality metrics.")
	EmitMinCardinality     = flag.Uint64("ce.emit.minCardinality", 1, "Minimum cardinality to emit for estimations metrics.")
)

var DefaultCardinalityEstimator *ce.CardinalityEstimator
var DefaultCardinalityMetricEmitter *ce.CardinalityMetricEmitter
var DefaultResetOperator *ce.ResetOperator

func InitDefaultCardinalityEstimator() {
	DefaultCardinalityEstimator = ce.NewCardinalityEstimator(
		ce.WithEstimatorMaxHllsInuse(*EstimatorMaxHllsInuse),
		ce.WithEstimatorSampleRate(*EstimatorSampleRate),
		ce.WithEstimatorFixedLabel1(*EstimatorFixedLabel1),
		ce.WithEstimatorFixedLabel2(*EstimatorFixedLabel2),
	)
	DefaultCardinalityMetricEmitter = ce.NewCardinalityMetricEmitter(context.Background(), DefaultCardinalityEstimator, "vm_cardinality_count",
		ce.WithEmitEnabledByFixed(*EmitEnabledByFixed),
		ce.WithEmitEnabledByMetric(*EmitEnabledByMetric),
		ce.WithEmitEnabledMetricCount(*EmitEnabledMetricCount),
		ce.WithEmitNamespace(*EmitNamespace),
		ce.WithEmitMinCardinality(*EmitMinCardinality),
	)
	DefaultResetOperator = ce.NewResetOperator(context.Background(), DefaultCardinalityEstimator)
}

func MustStopDefaultCardinalityEstimator() {
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

	var rs ce.ResetSchedule
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
