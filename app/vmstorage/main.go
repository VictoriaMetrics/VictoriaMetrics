package vmstorage

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/metrics"
)

var (
	retentionPeriod = flag.Int("retentionPeriod", 1, "Retention period in months")
	snapshotAuthKey = flag.String("snapshotAuthKey", "", "authKey, which must be passed in query string to /snapshot* pages")

	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss")

	// DataPath is a path to storage data.
	DataPath = flag.String("storageDataPath", "victoria-metrics-data", "Path to storage data")

	bigMergeConcurrency   = flag.Int("bigMergeConcurrency", 0, "The maximum number of CPU cores to use for big merges. Default value is used if set to 0")
	smallMergeConcurrency = flag.Int("smallMergeConcurrency", 0, "The maximum number of CPU cores to use for small merges. Default value is used if set to 0")
)

// Init initializes vmstorage.
func Init() {
	InitWithoutMetrics()
	registerStorageMetrics()
}

// InitWithoutMetrics must be called instead of Init inside tests.
//
// This allows multiple Init / Stop cycles.
func InitWithoutMetrics() {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid `-precisionBits`: %s", err)
	}

	storage.SetBigMergeWorkersCount(*bigMergeConcurrency)
	storage.SetSmallMergeWorkersCount(*smallMergeConcurrency)

	logger.Infof("opening storage at %q with retention period %d months", *DataPath, *retentionPeriod)
	startTime := time.Now()
	WG = syncwg.WaitGroup{}
	strg, err := storage.OpenStorage(*DataPath, *retentionPeriod)
	if err != nil {
		logger.Fatalf("cannot open a storage at %s with retention period %d months: %s", *DataPath, *retentionPeriod, err)
	}
	Storage = strg

	var m storage.Metrics
	Storage.UpdateMetrics(&m)
	tm := &m.TableMetrics
	partsCount := tm.SmallPartsCount + tm.BigPartsCount
	blocksCount := tm.SmallBlocksCount + tm.BigBlocksCount
	rowsCount := tm.SmallRowsCount + tm.BigRowsCount
	sizeBytes := tm.SmallSizeBytes + tm.BigSizeBytes
	logger.Infof("successfully opened storage %q in %s; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		*DataPath, time.Since(startTime), partsCount, blocksCount, rowsCount, sizeBytes)
}

// Storage is a storage.
//
// Every storage call must be wrapped into WG.Add(1) ... WG.Done()
// for proper graceful shutdown when Stop is called.
var Storage *storage.Storage

// WG must be incremented before Storage call.
//
// Use syncwg instead of sync, since Add is called from concurrent goroutines.
var WG syncwg.WaitGroup

// AddRows adds mrs to the storage.
func AddRows(mrs []storage.MetricRow) error {
	WG.Add(1)
	err := Storage.AddRows(mrs, uint8(*precisionBits))
	WG.Done()
	return err
}

// DeleteMetrics deletes metrics matching tfss.
//
// Returns the number of deleted metrics.
func DeleteMetrics(tfss []*storage.TagFilters) (int, error) {
	WG.Add(1)
	n, err := Storage.DeleteMetrics(tfss)
	WG.Done()
	return n, err
}

// SearchTagKeys searches for tag keys
func SearchTagKeys(maxTagKeys int) ([]string, error) {
	WG.Add(1)
	keys, err := Storage.SearchTagKeys(maxTagKeys)
	WG.Done()
	return keys, err
}

// SearchTagValues searches for tag values for the given tagKey
func SearchTagValues(tagKey []byte, maxTagValues int) ([]string, error) {
	WG.Add(1)
	values, err := Storage.SearchTagValues(tagKey, maxTagValues)
	WG.Done()
	return values, err
}

// SearchTagEntries searches for tag entries.
func SearchTagEntries(maxTagKeys, maxTagValues int) ([]storage.TagEntry, error) {
	WG.Add(1)
	tagEntries, err := Storage.SearchTagEntries(maxTagKeys, maxTagValues)
	WG.Done()
	return tagEntries, err
}

// GetSeriesCount returns the number of time series in the storage.
func GetSeriesCount() (uint64, error) {
	WG.Add(1)
	n, err := Storage.GetSeriesCount()
	WG.Done()
	return n, err
}

// Stop stops the vmstorage
func Stop() {
	logger.Infof("gracefully closing the storage at %s", *DataPath)
	startTime := time.Now()
	WG.WaitAndBlock()
	Storage.MustClose()
	logger.Infof("successfully closed the storage in %s", time.Since(startTime))

	logger.Infof("the storage has been stopped")
}

// RequestHandler is a storage request handler.
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	prometheusCompatibleResponse := false
	if path == "/api/v1/admin/tsdb/snapshot" {
		// Handle Prometheus API - https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot .
		prometheusCompatibleResponse = true
		path = "/snapshot/create"
	}
	if !strings.HasPrefix(path, "/snapshot") {
		return false
	}
	authKey := r.FormValue("authKey")
	if authKey != *snapshotAuthKey {
		httpserver.Errorf(w, "invalid authKey %q. It must match the value from -snapshotAuthKey command line flag", authKey)
		return true
	}
	path = path[len("/snapshot"):]

	switch path {
	case "/create":
		w.Header().Set("Content-Type", "application/json")
		snapshotPath, err := Storage.CreateSnapshot()
		if err != nil {
			msg := fmt.Sprintf("cannot create snapshot: %s", err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		if prometheusCompatibleResponse {
			fmt.Fprintf(w, `{"status":"success","data":{"name":%q}}`, snapshotPath)
		} else {
			fmt.Fprintf(w, `{"status":"ok","snapshot":%q}`, snapshotPath)
		}
		return true
	case "/list":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			msg := fmt.Sprintf("cannot list snapshots: %s", err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok","snapshots":[`)
		if len(snapshots) > 0 {
			for _, snapshot := range snapshots[:len(snapshots)-1] {
				fmt.Fprintf(w, "\n%q,", snapshot)
			}
			fmt.Fprintf(w, "\n%q\n", snapshots[len(snapshots)-1])
		}
		fmt.Fprintf(w, `]}`)
		return true
	case "/delete":
		w.Header().Set("Content-Type", "application/json")
		snapshotName := r.FormValue("snapshot")
		if err := Storage.DeleteSnapshot(snapshotName); err != nil {
			msg := fmt.Sprintf("cannot delete snapshot %q: %s", snapshotName, err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/delete_all":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			msg := fmt.Sprintf("cannot list snapshots: %s", err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		for _, snapshotName := range snapshots {
			if err := Storage.DeleteSnapshot(snapshotName); err != nil {
				msg := fmt.Sprintf("cannot delete snapshot %q: %s", snapshotName, err)
				logger.Errorf("%s", msg)
				fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
				return true
			}
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	default:
		return false
	}
}

func registerStorageMetrics() {
	mCache := &storage.Metrics{}
	var mCacheLock sync.Mutex
	var lastUpdateTime time.Time

	m := func() *storage.Metrics {
		mCacheLock.Lock()
		defer mCacheLock.Unlock()
		if time.Since(lastUpdateTime) < time.Second {
			return mCache
		}
		var mc storage.Metrics
		Storage.UpdateMetrics(&mc)
		mCache = &mc
		lastUpdateTime = time.Now()
		return mCache
	}
	tm := func() *storage.TableMetrics {
		sm := m()
		return &sm.TableMetrics
	}
	idbm := func() *storage.IndexDBMetrics {
		sm := m()
		return &sm.IndexDBMetrics
	}

	metrics.NewGauge(`vm_active_merges{type="storage/big"}`, func() float64 {
		return float64(tm().ActiveBigMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="storage/small"}`, func() float64 {
		return float64(tm().ActiveSmallMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="indexdb"}`, func() float64 {
		return float64(idbm().ActiveMerges)
	})

	metrics.NewGauge(`vm_merges_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="indexdb"}`, func() float64 {
		return float64(idbm().MergesCount)
	})

	metrics.NewGauge(`vm_rows_merged_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="indexdb"}`, func() float64 {
		return float64(idbm().ItemsMerged)
	})

	metrics.NewGauge(`vm_rows_deleted_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsDeleted)
	})
	metrics.NewGauge(`vm_rows_deleted_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsDeleted)
	})

	metrics.NewGauge(`vm_references{type="storage/big", name="parts"}`, func() float64 {
		return float64(tm().BigPartsRefCount)
	})
	metrics.NewGauge(`vm_references{type="storage/small", name="parts"}`, func() float64 {
		return float64(tm().SmallPartsRefCount)
	})
	metrics.NewGauge(`vm_references{type="storage", name="partitions"}`, func() float64 {
		return float64(tm().PartitionsRefCount)
	})
	metrics.NewGauge(`vm_references{type="indexdb", name="objects"}`, func() float64 {
		return float64(idbm().IndexDBRefCount)
	})
	metrics.NewGauge(`vm_references{type="indexdb", name="parts"}`, func() float64 {
		return float64(idbm().PartsRefCount)
	})

	metrics.NewGauge(`vm_new_timeseries_created_total`, func() float64 {
		return float64(idbm().NewTimeseriesCreated)
	})
	metrics.NewGauge(`vm_missing_tsids_for_metric_id_total`, func() float64 {
		return float64(idbm().MissingTSIDsForMetricID)
	})
	metrics.NewGauge(`vm_recent_hour_metric_ids_search_calls_total`, func() float64 {
		return float64(idbm().RecentHourMetricIDsSearchCalls)
	})
	metrics.NewGauge(`vm_recent_hour_metric_ids_search_hits_total`, func() float64 {
		return float64(idbm().RecentHourMetricIDsSearchHits)
	})
	metrics.NewGauge(`vm_date_metric_ids_search_calls_total`, func() float64 {
		return float64(idbm().DateMetricIDsSearchCalls)
	})
	metrics.NewGauge(`vm_date_metric_ids_search_hits_total`, func() float64 {
		return float64(idbm().DateMetricIDsSearchHits)
	})
	metrics.NewGauge(`vm_index_blocks_with_metric_ids_processed_total`, func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsProcessed)
	})
	metrics.NewGauge(`vm_index_blocks_with_metric_ids_incorrect_order_total`, func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsIncorrectOrder)
	})

	metrics.NewGauge(`vm_assisted_merges_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallAssistedMerges)
	})
	metrics.NewGauge(`vm_assisted_merges_total{type="indexdb"}`, func() float64 {
		return float64(idbm().AssistedMerges)
	})

	metrics.NewGauge(`vm_pending_rows{type="storage"}`, func() float64 {
		return float64(tm().PendingRows)
	})
	metrics.NewGauge(`vm_pending_rows{type="indexdb"}`, func() float64 {
		return float64(idbm().PendingItems)
	})

	metrics.NewGauge(`vm_parts{type="storage/big"}`, func() float64 {
		return float64(tm().BigPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="storage/small"}`, func() float64 {
		return float64(tm().SmallPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="indexdb"}`, func() float64 {
		return float64(idbm().PartsCount)
	})

	metrics.NewGauge(`vm_blocks{type="storage/big"}`, func() float64 {
		return float64(tm().BigBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="storage/small"}`, func() float64 {
		return float64(tm().SmallBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="indexdb"}`, func() float64 {
		return float64(idbm().BlocksCount)
	})

	metrics.NewGauge(`vm_data_size_bytes{type="storage/big"}`, func() float64 {
		return float64(tm().BigSizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="storage/small"}`, func() float64 {
		return float64(tm().SmallSizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="indexdb"}`, func() float64 {
		return float64(idbm().SizeBytes)
	})

	metrics.NewGauge(`vm_rows_ignored_total{reason="big_timestamp"}`, func() float64 {
		return float64(m().TooBigTimestampRows)
	})
	metrics.NewGauge(`vm_rows_ignored_total{reason="small_timestamp"}`, func() float64 {
		return float64(m().TooSmallTimestampRows)
	})

	metrics.NewGauge(`vm_concurrent_addrows_limit_reached_total`, func() float64 {
		return float64(m().AddRowsConcurrencyLimitReached)
	})
	metrics.NewGauge(`vm_concurrent_addrows_limit_timeout_total`, func() float64 {
		return float64(m().AddRowsConcurrencyLimitTimeout)
	})
	metrics.NewGauge(`vm_concurrent_addrows_dropped_rows_total`, func() float64 {
		return float64(m().AddRowsConcurrencyDroppedRows)
	})
	metrics.NewGauge(`vm_concurrent_addrows_capacity`, func() float64 {
		return float64(m().AddRowsConcurrencyCapacity)
	})
	metrics.NewGauge(`vm_concurrent_addrows_current`, func() float64 {
		return float64(m().AddRowsConcurrencyCurrent)
	})

	metrics.NewGauge(`vm_rows{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsCount)
	})
	metrics.NewGauge(`vm_rows{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsCount)
	})
	metrics.NewGauge(`vm_rows{type="indexdb"}`, func() float64 {
		return float64(idbm().ItemsCount)
	})

	metrics.NewGauge(`vm_recent_hour_inverted_index_entries`, func() float64 {
		return float64(m().RecentHourInvertedIndexSize)
	})
	metrics.NewGauge(`vm_recent_hour_inverted_index_unique_tag_pairs`, func() float64 {
		return float64(m().RecentHourInvertedIndexUniqueTagPairsSize)
	})
	metrics.NewGauge(`vm_recent_hour_inverted_index_pending_metric_ids`, func() float64 {
		return float64(m().RecentHourInvertedIndexPendingMetricIDsSize)
	})
	metrics.NewGauge(`vm_recent_hour_inverted_index_search_calls_total`, func() float64 {
		return float64(idbm().RecentHourInvertedIndexSearchCalls)
	})
	metrics.NewGauge(`vm_recent_hour_inverted_index_search_hits_total`, func() float64 {
		return float64(idbm().RecentHourInvertedIndexSearchHits)
	})

	metrics.NewGauge(`vm_cache_entries{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/metricIDs"}`, func() float64 {
		return float64(m().MetricIDCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/hour_metric_ids"}`, func() float64 {
		return float64(m().HourMetricIDCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/bigIndexBlocks"}`, func() float64 {
		return float64(tm().BigIndexBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/smallIndexBlocks"}`, func() float64 {
		return float64(tm().SmallIndexBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/uselessTagFilters"}`, func() float64 {
		return float64(idbm().UselessTagFiltersCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheSize())
	})

	metrics.NewGauge(`vm_cache_size_bytes{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/metricIDs"}`, func() float64 {
		return float64(m().MetricIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/uselessTagFilters"}`, func() float64 {
		return float64(idbm().UselessTagFiltersCacheSizeBytes)
	})

	metrics.NewGauge(`vm_cache_requests_total{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/metricIDs"}`, func() float64 {
		return float64(m().MetricIDCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/bigIndexBlocks"}`, func() float64 {
		return float64(tm().BigIndexBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/smallIndexBlocks"}`, func() float64 {
		return float64(tm().SmallIndexBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/uselessTagFilters"}`, func() float64 {
		return float64(idbm().UselessTagFiltersCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheRequests())
	})

	metrics.NewGauge(`vm_cache_misses_total{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/metricIDs"}`, func() float64 {
		return float64(m().MetricIDCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/bigIndexBlocks"}`, func() float64 {
		return float64(tm().BigIndexBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/smallIndexBlocks"}`, func() float64 {
		return float64(tm().SmallIndexBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/uselessTagFilters"}`, func() float64 {
		return float64(idbm().UselessTagFiltersCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheMisses())
	})

	metrics.NewGauge(`vm_deleted_metrics_total{type="indexdb"}`, func() float64 {
		return float64(idbm().DeletedMetricsCount)
	})

	metrics.NewGauge(`vm_cache_collisions_total{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheCollisions)
	})
	metrics.NewGauge(`vm_cache_collisions_total{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheCollisions)
	})
	metrics.NewGauge(`vm_cache_collisions_total{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheCollisions)
	})
}
