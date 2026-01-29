package ce

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ce"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"golang.org/x/sync/errgroup"
)

var (
	EstimatorAggrDefaultGaugeEnabled   = flag.Bool("ceaggr.defaultGaugeEnabled", false, "Whether to emit gauge aggregated cardinality metrics.")
	EstimatorAggrDefaultCounterEnabled = flag.Bool("ceaggr.defaultCounterEnabled", false, "Whether to emit counter aggregated cardinality metrics.")
	EstimatorAggrLookbackWindow        = flag.Duration("ceaggr.lookbackWindow", 5*time.Minute, "Lookback window for the cardinality estimator.")
	EstimatorAggrCeNodes               = flagutil.NewArrayString("ceaggr.ceNode", "Comma-separated addresses of cardinality estimator nodes; usage: -ceaggr.ceNode=vmce-host1,...,vmce-hostN. Alternatively, a file path containing the list of addresses (newline seperated) can be used.")
)

var DefaultGaugeAggrCardinalityEstimator *ce.CardinalityEstimator
var DefaultGaugeAggrCardinalityMetricEmitter *ce.CardinalityMetricEmitter

var DefaultCounterAggrCardinalityEstimator *ce.CardinalityEstimator
var DefaultCounterAggrCardinalityMetricEmitter *ce.CardinalityMetricEmitter
var DefaultCounterAggrResetOperator *ce.ResetOperator

func InitDefaultCardinalityEstimatorAggr() {
	if *EstimatorDefaultEnabled && IsDefaultCardinalityEstimatorAggrEnabled() {
		log.Panic("Cannot enable both default and aggregator cardinality estimators")
	}

	if IsDefaultCardinalityEstimatorAggrEnabled() == false {
		log.Printf("Cardinality estimator aggregation is disabled")
		return
	}

	if *EstimatorAggrCeNodes == nil || len(*EstimatorAggrCeNodes) == 0 {
		logger.Fatalf("At least one CE node address must be provided via -ceaggr.ceNode")
	}

	_ = metrics.NewGauge("vm_ce_aggr_hlls_inuse", func() float64 {
		return float64(DefaultGaugeAggrCardinalityEstimator.Allocator.Inuse() + DefaultCounterAggrCardinalityEstimator.Allocator.Inuse())
	})

	// init gauge
	DefaultGaugeAggrCardinalityEstimator = ce.NewCardinalityEstimator(
		ce.WithEstimatorMaxHllsInuse(*EstimatorMaxHllsInuse),
		ce.WithEstimatorSampleRate(*EstimatorSampleRate),
		ce.WithEstimatorFixedLabel1(*EstimatorFixedLabel1),
		ce.WithEstimatorFixedLabel2(*EstimatorFixedLabel2),
	)
	DefaultGaugeAggrCardinalityMetricEmitter = ce.NewCardinalityMetricEmitter(context.Background(), DefaultGaugeAggrCardinalityEstimator, "vm_cardinality")

	// init counter
	DefaultCounterAggrCardinalityEstimator = ce.NewCardinalityEstimator(
		ce.WithEstimatorMaxHllsInuse(*EstimatorMaxHllsInuse),
		ce.WithEstimatorSampleRate(*EstimatorSampleRate),
		ce.WithEstimatorFixedLabel1(*EstimatorFixedLabel1),
		ce.WithEstimatorFixedLabel2(*EstimatorFixedLabel2),
	)
	DefaultCounterAggrCardinalityMetricEmitter = ce.NewCardinalityMetricEmitter(context.Background(), DefaultCounterAggrCardinalityEstimator, "vm_cardinality_count")
	DefaultCounterAggrResetOperator = ce.NewResetOperator(context.Background(), DefaultCounterAggrCardinalityEstimator)

	c := make(chan []string, 1)
	c1 := make(chan []string, 1)

	go resetWorker(c, time.Tick(time.Second))
	go mergeWorker(c1, time.Tick(30*time.Second))

	go ceDiscoveryWorker([]chan<- []string{c, c1}, time.Tick(15*time.Second))
}

func MustStopDefaultCardinalityEstimatorAggr() {
	if !IsDefaultCardinalityEstimatorAggrEnabled() {
		return
	}
	// TODO
}

func IsDefaultCardinalityEstimatorAggrEnabled() bool {
	return *EstimatorAggrDefaultGaugeEnabled || *EstimatorAggrDefaultCounterEnabled
}

func HandleCeAggrGetBinary(w http.ResponseWriter, r *http.Request) {
	if !IsDefaultCardinalityEstimatorAggrEnabled() {
		http.Error(w, "Cardinality estimator aggregation is disabled", http.StatusBadRequest)
		return
	}

	queryType := r.URL.Query().Get("type")

	switch queryType {
	case "counter":
		data, err := DefaultCounterAggrCardinalityEstimator.MarshalBinary()
		if err != nil {
			log.Printf("Failed to marshal: %v", err)
			http.Error(w, "Failed to marshal", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	case "gauge":
		data, err := DefaultGaugeAggrCardinalityEstimator.MarshalBinary()
		if err != nil {
			log.Printf("Failed to marshal: %v", err)
			http.Error(w, "Failed to marshal", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		w.Write(data)
	default:
		http.Error(w, "Url parameter 'type' must be either 'counter' or 'gauge'", http.StatusBadRequest)
	}
}

func HandleCeAggrGetCardinality(w http.ResponseWriter, r *http.Request) {
	if !IsDefaultCardinalityEstimatorAggrEnabled() {
		http.Error(w, "Cardinality estimator aggregation is disabled", http.StatusBadRequest)
		return
	}

	queryType := r.URL.Query().Get("type")

	switch queryType {
	case "counter":
		estimate := DefaultCounterAggrCardinalityEstimator.EstimateMetricsCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	case "gauge":
		estimate := DefaultGaugeAggrCardinalityEstimator.EstimateMetricsCardinality()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(estimate); err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "Url parameter 'type' must be either 'counter' or 'gauge'", http.StatusBadRequest)
	}
}

// gets all CE node addresses from various sources and sends them to the provided consumers
func ceDiscoveryWorker(ceAddrsConsumers []chan<- []string, tickerC <-chan time.Time) {
	for range tickerC {
		var allAddrs []string

		for _, node := range *EstimatorAggrCeNodes {
			if strings.HasPrefix(node, "file:") {
				filePath := strings.TrimPrefix(node, "file:")
				addrs, err := readAddressesFromFile(filePath)
				if err != nil {
					log.Printf("Failed to read addresses from file %q: %v", filePath, err)
					continue
				}
				allAddrs = append(allAddrs, addrs...)
			} else {
				allAddrs = append(allAddrs, node)
			}
		}

		for _, ceAddrsC := range ceAddrsConsumers {
			ceAddrsC <- append([]string{}, allAddrs...)
		}
	}

}

func resetWorker(ceAddrsC <-chan []string, tickerC <-chan time.Time) {
	var ceAddrs []string

	for {
		select {
		case addrs := <-ceAddrsC:
			ceAddrs = addrs
		case <-tickerC:
			wg := sync.WaitGroup{}

			for idx, addr := range ceAddrs {
				wg.Go(func() {
					resetSchedule := ce.NewResetSchedule(*EstimatorAggrLookbackWindow, len(ceAddrs), idx)

					data, err := json.Marshal(resetSchedule)
					if err != nil {
						log.Printf("Failed to marshal reset schedule: %v", err)
						return
					}

					// set the schedule
					req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/insert/0/ce/updateResetSchedule", bytes.NewReader(data))
					if err != nil {
						log.Printf("Failed to create request: %v", err)
						return
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						log.Printf("Failed to send request: %v", err)
						return
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						log.Printf("Non-OK response: %s", resp.Status)
						return
					}

					log.Printf("Set reset schedule on %s to %+v", addr, resetSchedule)
				})
			}

			wg.Wait()
		}
	}
}

func mergeWorker(ceAddrsC <-chan []string, tickerC <-chan time.Time) {
	var ceAddrs []string

	for {
		select {
		case addrs := <-ceAddrsC:
			ceAddrs = addrs
		case <-tickerC:
			eg := errgroup.Group{}

			tmpCe := ce.NewCardinalityEstimator(
				ce.WithEstimatorMaxHllsInuse(*EstimatorMaxHllsInuse),
				ce.WithEstimatorSampleRate(*EstimatorSampleRate),
				ce.WithEstimatorFixedLabel1(*EstimatorFixedLabel1),
				ce.WithEstimatorFixedLabel2(*EstimatorFixedLabel2),
			)

			sem := make(chan struct{}, 3) // Limit concurrency to 3

			// fetch and merge the estimators
			for _, addr := range ceAddrs {
				eg.Go(func() error {

					sem <- struct{}{}
					defer func() { <-sem }()

					// fetch the binary
					req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/insert/0/ce/binary", nil)
					if err != nil {
						return fmt.Errorf("Failed to create request: %v", err)
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						return fmt.Errorf("Failed to send request: %v", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("Non-OK response: %s", resp.Status)
					}

					// read
					bin, err := io.ReadAll(resp.Body)
					if err != nil {
						return fmt.Errorf("Failed to read response body: %v", err)
					}

					// merge
					newCe := ce.NewCardinalityEstimator()
					if err := newCe.UnmarshalBinary(bin); err != nil {
						log.Panicf("FATAL: failed to unmarshal binary data: %v", err)
					}

					if err := tmpCe.Merge(newCe); err != nil {
						log.Panicf("FATAL: failed to merge estimators: %v", err)
					}

					log.Printf("Done merging from %s", addr)
					return nil
				})
			}

			err := eg.Wait()
			if err != nil {
				log.Printf("Failed to merge estimators: %v", err)
				continue
			}

			// update the gauge estimator
			if *EstimatorAggrDefaultGaugeEnabled {
				DefaultGaugeAggrCardinalityEstimator.Reset()
				if err := DefaultGaugeAggrCardinalityEstimator.Merge(tmpCe); err != nil {
					log.Panicf("BUG: failed to merge gauge estimators: %v", err)
				}
			}

			if *EstimatorAggrDefaultCounterEnabled {
				// merge counter estimator
				if err := DefaultCounterAggrCardinalityEstimator.Merge(tmpCe); err != nil {
					log.Panicf("BUG: failed to merge counter estimators: %v", err)
				}
			}
		}
	}

}

// readAddressesFromFile reads a file where each line contains a host:port address.
// Empty lines and lines starting with '#' are ignored.
// Returns the list of addresses.
func readAddressesFromFile(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer f.Close()

	var addresses []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		addresses = append(addresses, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", filePath, err)
	}

	return addresses, nil
}
