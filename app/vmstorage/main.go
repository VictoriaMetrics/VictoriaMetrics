package vmstorage

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/metrics"
)

var (
	retentionPeriod   = flagutil.NewDuration("retentionPeriod", "1", "Data with timestamps outside the retentionPeriod is automatically deleted")
	snapshotAuthKey   = flag.String("snapshotAuthKey", "", "authKey, which must be passed in query string to /snapshot* pages")
	forceMergeAuthKey = flag.String("forceMergeAuthKey", "", "authKey, which must be passed in query string to /internal/force_merge pages")
	forceFlushAuthKey = flag.String("forceFlushAuthKey", "", "authKey, which must be passed in query string to /internal/force_flush pages")
	snapshotsMaxAge   = flagutil.NewDuration("snapshotsMaxAge", "0", "Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted")

	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss")

	// DataPath is a path to storage data.
	DataPath = flag.String("storageDataPath", "victoria-metrics-data", "Path to storage data")

	finalMergeDelay = flag.Duration("finalMergeDelay", 0, "The delay before starting final merge for per-month partition after no new data is ingested into it. "+
		"Final merge may require additional disk IO and CPU resources. Final merge may increase query speed and reduce disk space usage in some cases. "+
		"Zero value disables final merge")
	bigMergeConcurrency     = flag.Int("bigMergeConcurrency", 0, "The maximum number of CPU cores to use for big merges. Default value is used if set to 0")
	smallMergeConcurrency   = flag.Int("smallMergeConcurrency", 0, "The maximum number of CPU cores to use for small merges. Default value is used if set to 0")
	retentionTimezoneOffset = flag.Duration("retentionTimezoneOffset", 0, "The offset for performing indexdb rotation. "+
		"If set to 0, then the indexdb rotation is performed at 4am UTC time per each -retentionPeriod. "+
		"If set to 2h, then the indexdb rotation is performed at 4am EET time (the timezone with +2h offset)")

	logNewSeries = flag.Bool("logNewSeries", false, "Whether to log new series. This option is for debug purposes only. It can lead to performance issues "+
		"when big number of new series are ingested into VictoriaMetrics")
	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside of the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
	maxHourlySeries = flag.Int("storage.maxHourlySeries", 0, "The maximum number of unique series can be added to the storage during the last hour. "+
		"Excess series are logged and dropped. This can be useful for limiting series cardinality. See also -storage.maxDailySeries")
	maxDailySeries = flag.Int("storage.maxDailySeries", 0, "The maximum number of unique series can be added to the storage during the last 24 hours. "+
		"Excess series are logged and dropped. This can be useful for limiting series churn rate. See also -storage.maxHourlySeries")

	minFreeDiskSpaceBytes = flagutil.NewBytes("storage.minFreeDiskSpaceBytes", 10e6, "The minimum free disk space at -storageDataPath after which the storage stops accepting new data")

	cacheSizeStorageTSID        = flagutil.NewBytes("storage.cacheSizeStorageTSID", 0, "Overrides max size for storage/tsid cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBIndexBlocks = flagutil.NewBytes("storage.cacheSizeIndexDBIndexBlocks", 0, "Overrides max size for indexdb/indexBlocks cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBDataBlocks  = flagutil.NewBytes("storage.cacheSizeIndexDBDataBlocks", 0, "Overrides max size for indexdb/dataBlocks cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBTagFilters  = flagutil.NewBytes("storage.cacheSizeIndexDBTagFilters", 0, "Overrides max size for indexdb/tagFilters cache. See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
)

// CheckTimeRange returns true if the given tr is denied for querying.
func CheckTimeRange(tr storage.TimeRange) error {
	if !*denyQueriesOutsideRetention {
		return nil
	}
	minAllowedTimestamp := int64(fasttime.UnixTimestamp()*1000) - retentionPeriod.Msecs
	if tr.MinTimestamp > minAllowedTimestamp {
		return nil
	}
	return &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("the given time range %s is outside the allowed -retentionPeriod=%s according to -denyQueriesOutsideRetention", &tr, retentionPeriod),
		StatusCode: http.StatusServiceUnavailable,
	}
}

// Init initializes vmstorage.
func Init(resetCacheIfNeeded func(mrs []storage.MetricRow)) {
	InitWithoutMetrics(resetCacheIfNeeded)
	registerStorageMetrics(Storage)
}

// InitWithoutMetrics must be called instead of Init inside tests.
//
// This allows multiple Init / Stop cycles.
func InitWithoutMetrics(resetCacheIfNeeded func(mrs []storage.MetricRow)) {
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid `-precisionBits`: %s", err)
	}

	resetResponseCacheIfNeeded = resetCacheIfNeeded
	storage.SetLogNewSeries(*logNewSeries)
	storage.SetFinalMergeDelay(*finalMergeDelay)
	storage.SetBigMergeWorkersCount(*bigMergeConcurrency)
	storage.SetSmallMergeWorkersCount(*smallMergeConcurrency)
	storage.SetRetentionTimezoneOffset(*retentionTimezoneOffset)
	storage.SetFreeDiskSpaceLimit(minFreeDiskSpaceBytes.N)
	storage.SetTSIDCacheSize(cacheSizeStorageTSID.N)
	storage.SetTagFilterCacheSize(cacheSizeIndexDBTagFilters.N)
	mergeset.SetIndexBlocksCacheSize(cacheSizeIndexDBIndexBlocks.N)
	mergeset.SetDataBlocksCacheSize(cacheSizeIndexDBDataBlocks.N)

	if retentionPeriod.Msecs < 24*3600*1000 {
		logger.Fatalf("-retentionPeriod cannot be smaller than a day; got %s", retentionPeriod)
	}
	logger.Infof("opening storage at %q with -retentionPeriod=%s", *DataPath, retentionPeriod)
	startTime := time.Now()
	WG = syncwg.WaitGroup{}
	strg, err := storage.OpenStorage(*DataPath, retentionPeriod.Msecs, *maxHourlySeries, *maxDailySeries)
	if err != nil {
		logger.Fatalf("cannot open a storage at %s with -retentionPeriod=%s: %s", *DataPath, retentionPeriod, err)
	}
	Storage = strg
	initStaleSnapshotsRemover(strg)

	var m storage.Metrics
	strg.UpdateMetrics(&m)
	tm := &m.TableMetrics
	partsCount := tm.SmallPartsCount + tm.BigPartsCount
	blocksCount := tm.SmallBlocksCount + tm.BigBlocksCount
	rowsCount := tm.SmallRowsCount + tm.BigRowsCount
	sizeBytes := tm.SmallSizeBytes + tm.BigSizeBytes
	logger.Infof("successfully opened storage %q in %.3f seconds; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		*DataPath, time.Since(startTime).Seconds(), partsCount, blocksCount, rowsCount, sizeBytes)
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

// resetResponseCacheIfNeeded is a callback for automatic resetting of response cache if needed.
var resetResponseCacheIfNeeded func(mrs []storage.MetricRow)

// AddRows adds mrs to the storage.
func AddRows(mrs []storage.MetricRow) error {
	if Storage.IsReadOnly() {
		return errReadOnly
	}
	resetResponseCacheIfNeeded(mrs)
	WG.Add(1)
	err := Storage.AddRows(mrs, uint8(*precisionBits))
	WG.Done()
	return err
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

// RegisterMetricNames registers all the metrics from mrs in the storage.
func RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow) error {
	WG.Add(1)
	err := Storage.RegisterMetricNames(qt, mrs)
	WG.Done()
	return err
}

// DeleteSeries deletes series matching tfss.
//
// Returns the number of deleted series.
func DeleteSeries(qt *querytracer.Tracer, tfss []*storage.TagFilters) (int, error) {
	WG.Add(1)
	n, err := Storage.DeleteSeries(qt, tfss)
	WG.Done()
	return n, err
}

// SearchMetricNames returns metric names for the given tfss on the given tr.
func SearchMetricNames(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxMetrics int, deadline uint64) ([]string, error) {
	WG.Add(1)
	metricNames, err := Storage.SearchMetricNames(qt, tfss, tr, maxMetrics, deadline)
	WG.Done()
	return metricNames, err
}

// SearchLabelNamesWithFiltersOnTimeRange searches for tag keys matching the given tfss on tr.
func SearchLabelNamesWithFiltersOnTimeRange(qt *querytracer.Tracer, tfss []*storage.TagFilters, tr storage.TimeRange, maxTagKeys, maxMetrics int, deadline uint64) ([]string, error) {
	WG.Add(1)
	labelNames, err := Storage.SearchLabelNamesWithFiltersOnTimeRange(qt, tfss, tr, maxTagKeys, maxMetrics, deadline)
	WG.Done()
	return labelNames, err
}

// SearchLabelValuesWithFiltersOnTimeRange searches for label values for the given labelName, tfss and tr.
func SearchLabelValuesWithFiltersOnTimeRange(qt *querytracer.Tracer, labelName string, tfss []*storage.TagFilters,
	tr storage.TimeRange, maxLabelValues, maxMetrics int, deadline uint64) ([]string, error) {
	WG.Add(1)
	labelValues, err := Storage.SearchLabelValuesWithFiltersOnTimeRange(qt, labelName, tfss, tr, maxLabelValues, maxMetrics, deadline)
	WG.Done()
	return labelValues, err
}

// SearchTagValueSuffixes returns all the tag value suffixes for the given tagKey and tagValuePrefix on the given tr.
//
// This allows implementing https://graphite-api.readthedocs.io/en/latest/api.html#metrics-find or similar APIs.
func SearchTagValueSuffixes(qt *querytracer.Tracer, tr storage.TimeRange, tagKey, tagValuePrefix string, delimiter byte, maxTagValueSuffixes int, deadline uint64) ([]string, error) {
	WG.Add(1)
	suffixes, err := Storage.SearchTagValueSuffixes(qt, tr, tagKey, tagValuePrefix, delimiter, maxTagValueSuffixes, deadline)
	WG.Done()
	return suffixes, err
}

// SearchGraphitePaths returns all the metric names matching the given Graphite query.
func SearchGraphitePaths(qt *querytracer.Tracer, tr storage.TimeRange, query []byte, maxPaths int, deadline uint64) ([]string, error) {
	WG.Add(1)
	paths, err := Storage.SearchGraphitePaths(qt, tr, query, maxPaths, deadline)
	WG.Done()
	return paths, err
}

// GetTSDBStatus returns TSDB status for given filters on the given date.
func GetTSDBStatus(qt *querytracer.Tracer, tfss []*storage.TagFilters, date uint64, focusLabel string, topN, maxMetrics int, deadline uint64) (*storage.TSDBStatus, error) {
	WG.Add(1)
	status, err := Storage.GetTSDBStatus(qt, tfss, date, focusLabel, topN, maxMetrics, deadline)
	WG.Done()
	return status, err
}

// GetSeriesCount returns the number of time series in the storage.
func GetSeriesCount(deadline uint64) (uint64, error) {
	WG.Add(1)
	n, err := Storage.GetSeriesCount(deadline)
	WG.Done()
	return n, err
}

// Stop stops the vmstorage
func Stop() {
	logger.Infof("gracefully closing the storage at %s", *DataPath)
	startTime := time.Now()
	WG.WaitAndBlock()
	stopStaleSnapshotsRemover()
	Storage.MustClose()
	logger.Infof("successfully closed the storage in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("the storage has been stopped")
}

// RequestHandler is a storage request handler.
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if path == "/internal/force_merge" {
		authKey := r.FormValue("authKey")
		if authKey != *forceMergeAuthKey {
			httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -forceMergeAuthKey command line flag", authKey)
			return true
		}
		// Run force merge in background
		partitionNamePrefix := r.FormValue("partition_prefix")
		go func() {
			activeForceMerges.Inc()
			defer activeForceMerges.Dec()
			logger.Infof("forced merge for partition_prefix=%q has been started", partitionNamePrefix)
			startTime := time.Now()
			if err := Storage.ForceMergePartitions(partitionNamePrefix); err != nil {
				logger.Errorf("error in forced merge for partition_prefix=%q: %s", partitionNamePrefix, err)
			}
			logger.Infof("forced merge for partition_prefix=%q has been successfully finished in %.3f seconds", partitionNamePrefix, time.Since(startTime).Seconds())
		}()
		return true
	}
	if path == "/internal/force_flush" {
		authKey := r.FormValue("authKey")
		if authKey != *forceFlushAuthKey {
			httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -forceFlushAuthKey command line flag", authKey)
			return true
		}
		logger.Infof("flushing storage to make pending data available for reading")
		Storage.DebugFlush()
		return true
	}
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
		httpserver.Errorf(w, r, "invalid authKey %q. It must match the value from -snapshotAuthKey command line flag", authKey)
		return true
	}
	path = path[len("/snapshot"):]

	switch path {
	case "/create":
		w.Header().Set("Content-Type", "application/json")
		snapshotPath, err := Storage.CreateSnapshot()
		if err != nil {
			err = fmt.Errorf("cannot create snapshot: %w", err)
			jsonResponseError(w, err)
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
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
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
			err = fmt.Errorf("cannot delete snapshot %q: %w", snapshotName, err)
			jsonResponseError(w, err)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/delete_all":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			return true
		}
		for _, snapshotName := range snapshots {
			if err := Storage.DeleteSnapshot(snapshotName); err != nil {
				err = fmt.Errorf("cannot delete snapshot %q: %w", snapshotName, err)
				jsonResponseError(w, err)
				return true
			}
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	default:
		return false
	}
}

func initStaleSnapshotsRemover(strg *storage.Storage) {
	staleSnapshotsRemoverCh = make(chan struct{})
	if snapshotsMaxAge.Msecs <= 0 {
		return
	}
	snapshotsMaxAgeDur := time.Duration(snapshotsMaxAge.Msecs) * time.Millisecond
	staleSnapshotsRemoverWG.Add(1)
	go func() {
		defer staleSnapshotsRemoverWG.Done()
		t := time.NewTicker(11 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-staleSnapshotsRemoverCh:
				return
			case <-t.C:
			}
			if err := strg.DeleteStaleSnapshots(snapshotsMaxAgeDur); err != nil {
				// Use logger.Errorf instead of logger.Fatalf in the hope the error is temporary.
				logger.Errorf("cannot delete stale snapshots: %s", err)
			}
		}
	}()
}

func stopStaleSnapshotsRemover() {
	close(staleSnapshotsRemoverCh)
	staleSnapshotsRemoverWG.Wait()
}

var (
	staleSnapshotsRemoverCh chan struct{}
	staleSnapshotsRemoverWG sync.WaitGroup
)

var activeForceMerges = metrics.NewCounter("vm_active_force_merges")

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

	metrics.NewGauge(fmt.Sprintf(`vm_free_disk_space_bytes{path=%q}`, *DataPath), func() float64 {
		return float64(fs.MustGetFreeSpace(*DataPath))
	})
	metrics.NewGauge(fmt.Sprintf(`vm_free_disk_space_limit_bytes{path=%q}`, *DataPath), func() float64 {
		return float64(minFreeDiskSpaceBytes.N)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_storage_is_read_only{path=%q}`, *DataPath), func() float64 {
		if strg.IsReadOnly() {
			return 1
		}
		return 0
	})

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
	metrics.NewGauge(`vm_timeseries_repopulated_total`, func() float64 {
		return float64(idbm().TimeseriesRepopulated)
	})
	metrics.NewGauge(`vm_missing_tsids_for_metric_id_total`, func() float64 {
		return float64(idbm().MissingTSIDsForMetricID)
	})
	metrics.NewGauge(`vm_index_blocks_with_metric_ids_processed_total`, func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsProcessed)
	})
	metrics.NewGauge(`vm_index_blocks_with_metric_ids_incorrect_order_total`, func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsIncorrectOrder)
	})
	metrics.NewGauge(`vm_composite_index_min_timestamp`, func() float64 {
		return float64(idbm().MinTimestampForCompositeIndex) / 1e3
	})
	metrics.NewGauge(`vm_composite_filter_success_conversions_total`, func() float64 {
		return float64(idbm().CompositeFilterSuccessConversions)
	})
	metrics.NewGauge(`vm_composite_filter_missing_conversions_total`, func() float64 {
		return float64(idbm().CompositeFilterMissingConversions)
	})

	metrics.NewGauge(`vm_assisted_merges_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallAssistedMerges)
	})
	metrics.NewGauge(`vm_assisted_merges_total{type="indexdb"}`, func() float64 {
		return float64(idbm().AssistedMerges)
	})

	metrics.NewGauge(`vm_indexdb_items_added_total`, func() float64 {
		return float64(idbm().ItemsAdded)
	})
	metrics.NewGauge(`vm_indexdb_items_added_size_bytes_total`, func() float64 {
		return float64(idbm().ItemsAddedSizeBytes)
	})

	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/686
	metrics.NewGauge(`vm_merge_need_free_disk_space{type="storage/small"}`, func() float64 {
		return float64(tm().SmallMergeNeedFreeDiskSpace)
	})
	metrics.NewGauge(`vm_merge_need_free_disk_space{type="storage/big"}`, func() float64 {
		return float64(tm().BigMergeNeedFreeDiskSpace)
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

	metrics.NewGauge(`vm_rows_added_to_storage_total`, func() float64 {
		return float64(m().RowsAddedTotal)
	})
	metrics.NewGauge(`vm_deduplicated_samples_total{type="merge"}`, func() float64 {
		return float64(m().DedupsDuringMerge)
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

	metrics.NewGauge(`vm_concurrent_search_tsids_limit_reached_total`, func() float64 {
		return float64(m().SearchTSIDsConcurrencyLimitReached)
	})
	metrics.NewGauge(`vm_concurrent_search_tsids_limit_timeout_total`, func() float64 {
		return float64(m().SearchTSIDsConcurrencyLimitTimeout)
	})
	metrics.NewGauge(`vm_concurrent_search_tsids_capacity`, func() float64 {
		return float64(m().SearchTSIDsConcurrencyCapacity)
	})
	metrics.NewGauge(`vm_concurrent_search_tsids_current`, func() float64 {
		return float64(m().SearchTSIDsConcurrencyCurrent)
	})

	metrics.NewGauge(`vm_search_delays_total`, func() float64 {
		return float64(m().SearchDelays)
	})

	metrics.NewGauge(`vm_slow_row_inserts_total`, func() float64 {
		return float64(m().SlowRowInserts)
	})
	metrics.NewGauge(`vm_slow_per_day_index_inserts_total`, func() float64 {
		return float64(m().SlowPerDayIndexInserts)
	})
	metrics.NewGauge(`vm_slow_metric_name_loads_total`, func() float64 {
		return float64(m().SlowMetricNameLoads)
	})

	metrics.NewGauge(`vm_hourly_series_limit_rows_dropped_total`, func() float64 {
		return float64(m().HourlySeriesLimitRowsDropped)
	})
	metrics.NewGauge(`vm_daily_series_limit_rows_dropped_total`, func() float64 {
		return float64(m().DailySeriesLimitRowsDropped)
	})

	metrics.NewGauge(`vm_timestamps_blocks_merged_total`, func() float64 {
		return float64(m().TimestampsBlocksMerged)
	})
	metrics.NewGauge(`vm_timestamps_bytes_saved_total`, func() float64 {
		return float64(m().TimestampsBytesSaved)
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
	metrics.NewGauge(`vm_global_search_calls_total`, func() float64 {
		return float64(idbm().GlobalSearchCalls)
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
	metrics.NewGauge(`vm_cache_entries{type="storage/next_day_metric_ids"}`, func() float64 {
		return float64(m().NextDayMetricIDCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/indexBlocks"}`, func() float64 {
		return float64(tm().IndexBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagFiltersCacheSize)
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheSize())
	})
	metrics.NewGauge(`vm_cache_entries{type="storage/regexpPrefixes"}`, func() float64 {
		return float64(storage.RegexpPrefixesCacheSize())
	})

	metrics.NewGauge(`vm_cache_entries{type="storage/prefetchedMetricIDs"}`, func() float64 {
		return float64(m().PrefetchedMetricIDsSize)
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
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/indexBlocks"}`, func() float64 {
		return float64(tm().IndexBlocksCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/date_metricID"}`, func() float64 {
		return float64(m().DateMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/hour_metric_ids"}`, func() float64 {
		return float64(m().HourMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/next_day_metric_ids"}`, func() float64 {
		return float64(m().NextDayMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagFiltersCacheSizeBytes)
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheSizeBytes())
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/regexpPrefixes"}`, func() float64 {
		return float64(storage.RegexpPrefixesCacheSizeBytes())
	})
	metrics.NewGauge(`vm_cache_size_bytes{type="storage/prefetchedMetricIDs"}`, func() float64 {
		return float64(m().PrefetchedMetricIDsSizeBytes)
	})

	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/tsid"}`, func() float64 {
		return float64(m().TSIDCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/metricIDs"}`, func() float64 {
		return float64(m().MetricIDCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/metricName"}`, func() float64 {
		return float64(m().MetricNameCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/indexBlocks"}`, func() float64 {
		return float64(tm().IndexBlocksCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagFiltersCacheSizeMaxBytes)
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheMaxSizeBytes())
	})
	metrics.NewGauge(`vm_cache_size_max_bytes{type="storage/regexpPrefixes"}`, func() float64 {
		return float64(storage.RegexpPrefixesCacheMaxSizeBytes())
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
	metrics.NewGauge(`vm_cache_requests_total{type="storage/indexBlocks"}`, func() float64 {
		return float64(tm().IndexBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagFiltersCacheRequests)
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheRequests())
	})
	metrics.NewGauge(`vm_cache_requests_total{type="storage/regexpPrefixes"}`, func() float64 {
		return float64(storage.RegexpPrefixesCacheRequests())
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
	metrics.NewGauge(`vm_cache_misses_total{type="storage/indexBlocks"}`, func() float64 {
		return float64(tm().IndexBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/dataBlocks"}`, func() float64 {
		return float64(idbm().DataBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/indexBlocks"}`, func() float64 {
		return float64(idbm().IndexBlocksCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/tagFilters"}`, func() float64 {
		return float64(idbm().TagFiltersCacheMisses)
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/regexps"}`, func() float64 {
		return float64(storage.RegexpCacheMisses())
	})
	metrics.NewGauge(`vm_cache_misses_total{type="storage/regexpPrefixes"}`, func() float64 {
		return float64(storage.RegexpPrefixesCacheMisses())
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

	metrics.NewGauge(`vm_next_retention_seconds`, func() float64 {
		return float64(m().NextRetentionSeconds)
	})
}

func jsonResponseError(w http.ResponseWriter, err error) {
	logger.Errorf("%s", err)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"status":"error","msg":%q}`, err)
}
