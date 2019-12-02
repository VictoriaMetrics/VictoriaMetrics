package main

import (
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage/transport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr  = flag.String("httpListenAddr", ":8482", "Address to listen for http connections")
	retentionPeriod = flag.Int("retentionPeriod", 1, "Retention period in months")
	storageDataPath = flag.String("storageDataPath", "vmstorage-data", "Path to storage data")
	vminsertAddr    = flag.String("vminsertAddr", ":8400", "TCP address to accept connections from vminsert services")
	vmselectAddr    = flag.String("vmselectAddr", ":8401", "TCP address to accept connections from vmselect services")
	snapshotAuthKey = flag.String("snapshotAuthKey", "", "authKey, which must be passed in query string to /snapshot* pages")

	bigMergeConcurrency   = flag.Int("bigMergeConcurrency", 0, "The maximum number of CPU cores to use for big merges. Default value is used if set to 0")
	smallMergeConcurrency = flag.Int("smallMergeConcurrency", 0, "The maximum number of CPU cores to use for small merges. Default value is used if set to 0")
)

func main() {
	flag.Parse()
	buildinfo.Init()
	logger.Init()

	storage.SetBigMergeWorkersCount(*bigMergeConcurrency)
	storage.SetSmallMergeWorkersCount(*smallMergeConcurrency)

	logger.Infof("opening storage at %q with retention period %d months", *storageDataPath, *retentionPeriod)
	startTime := time.Now()
	strg, err := storage.OpenStorage(*storageDataPath, *retentionPeriod)
	if err != nil {
		logger.Fatalf("cannot open a storage at %s with retention period %d months: %s", *storageDataPath, *retentionPeriod, err)
	}

	var m storage.Metrics
	strg.UpdateMetrics(&m)
	tm := &m.TableMetrics
	partsCount := tm.SmallPartsCount + tm.BigPartsCount
	blocksCount := tm.SmallBlocksCount + tm.BigBlocksCount
	rowsCount := tm.SmallRowsCount + tm.BigRowsCount
	sizeBytes := tm.SmallSizeBytes + tm.BigSizeBytes
	logger.Infof("successfully opened storage %q in %s; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		*storageDataPath, time.Since(startTime), partsCount, blocksCount, rowsCount, sizeBytes)

	registerStorageMetrics(strg)

	srv, err := transport.NewServer(*vminsertAddr, *vmselectAddr, strg)
	if err != nil {
		logger.Fatalf("cannot create a server with vminsertAddr=%s, vmselectAddr=%s: %s", *vminsertAddr, *vmselectAddr, err)
	}

	go srv.RunVMInsert()
	go srv.RunVMSelect()

	requestHandler := newRequestHandler(strg)
	go func() {
		httpserver.Serve(*httpListenAddr, requestHandler)
	}()

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)

	logger.Infof("gracefully shutting down the service")
	startTime = time.Now()
	srv.MustClose()
	logger.Infof("successfully shut down the service in %s", time.Since(startTime))

	logger.Infof("gracefully closing the storage at %s", *storageDataPath)
	startTime = time.Now()
	strg.MustClose()
	logger.Infof("successfully closed the storage in %s", time.Since(startTime))

	fs.MustStopDirRemover()

	logger.Infof("the vmstorage has been stopped")
}

func newRequestHandler(strg *storage.Storage) httpserver.RequestHandler {
	return func(w http.ResponseWriter, r *http.Request) bool {
		return requestHandler(w, r, strg)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request, strg *storage.Storage) bool {
	path := r.URL.Path
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
		snapshotPath, err := strg.CreateSnapshot()
		if err != nil {
			msg := fmt.Sprintf("cannot create snapshot: %s", err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok","snapshot":%q}`, snapshotPath)
		return true
	case "/list":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := strg.ListSnapshots()
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
		if err := strg.DeleteSnapshot(snapshotName); err != nil {
			msg := fmt.Sprintf("cannot delete snapshot %q: %s", snapshotName, err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/delete_all":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := strg.ListSnapshots()
		if err != nil {
			msg := fmt.Sprintf("cannot list snapshots: %s", err)
			logger.Errorf("%s", msg)
			fmt.Fprintf(w, `{"status":"error","msg":%q}`, msg)
			return true
		}
		for _, snapshotName := range snapshots {
			if err := strg.DeleteSnapshot(snapshotName); err != nil {
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

func registerStorageMetrics(strg *storage.Storage) {
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
		strg.UpdateMetrics(&mc)
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

	metrics.NewGauge(`vm_date_range_search_calls_total`, func() float64 {
		return float64(idbm().DateRangeSearchCalls)
	})
	metrics.NewGauge(`vm_date_range_hits_total`, func() float64 {
		return float64(idbm().DateRangeSearchHits)
	})

	metrics.NewGauge(`vm_missing_metric_names_for_metric_id_total`, func() float64 {
		return float64(idbm().MissingMetricNamesForMetricID)
	})

	metrics.NewGauge(`vm_date_metric_id_cache_syncs_total`, func() float64 {
		return float64(m().DateMetricIDCacheSyncsCount)
	})
	metrics.NewGauge(`vm_date_metric_id_cache_resets_total`, func() float64 {
		return float64(m().DateMetricIDCacheResetsCount)
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
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/hour_metric_ids"}`, func() float64 {
		return float64(m().HourMetricIDCacheSizeBytes)
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
}
