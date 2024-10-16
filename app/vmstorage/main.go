package vmstorage

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/syncwg"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	retentionPeriod   = flagutil.NewDuration("retentionPeriod", "1", "Data with timestamps outside the retentionPeriod is automatically deleted. The minimum retentionPeriod is 24h or 1d. See also -retentionFilter")
	snapshotAuthKey   = flagutil.NewPassword("snapshotAuthKey", "authKey, which must be passed in query string to /snapshot* pages. It overrides -httpAuth.*")
	forceMergeAuthKey = flagutil.NewPassword("forceMergeAuthKey", "authKey, which must be passed in query string to /internal/force_merge pages. It overrides -httpAuth.*")
	forceFlushAuthKey = flagutil.NewPassword("forceFlushAuthKey", "authKey, which must be passed in query string to /internal/force_flush pages. It overrides -httpAuth.*")
	snapshotsMaxAge   = flagutil.NewDuration("snapshotsMaxAge", "0", "Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted")
	_                 = flag.Duration("snapshotCreateTimeout", 0, "Deprecated: this flag does nothing")

	precisionBits = flag.Int("precisionBits", 64, "The number of precision bits to store per each value. Lower precision bits improves data compression at the cost of precision loss")

	// DataPath is a path to storage data.
	DataPath = flag.String("storageDataPath", "victoria-metrics-data", "Path to storage data")

	_ = flag.Duration("finalMergeDelay", 0, "Deprecated: this flag does nothing")
	_ = flag.Int("bigMergeConcurrency", 0, "Deprecated: this flag does nothing")
	_ = flag.Int("smallMergeConcurrency", 0, "Deprecated: this flag does nothing")

	retentionTimezoneOffset = flag.Duration("retentionTimezoneOffset", 0, "The offset for performing indexdb rotation. "+
		"If set to 0, then the indexdb rotation is performed at 4am UTC time per each -retentionPeriod. "+
		"If set to 2h, then the indexdb rotation is performed at 4am EET time (the timezone with +2h offset)")

	logNewSeries = flag.Bool("logNewSeries", false, "Whether to log new series. This option is for debug purposes only. It can lead to performance issues "+
		"when big number of new series are ingested into VictoriaMetrics")
	denyQueriesOutsideRetention = flag.Bool("denyQueriesOutsideRetention", false, "Whether to deny queries outside the configured -retentionPeriod. "+
		"When set, then /api/v1/query_range would return '503 Service Unavailable' error for queries with 'from' value outside -retentionPeriod. "+
		"This may be useful when multiple data sources with distinct retentions are hidden behind query-tee")
	maxHourlySeries = flag.Int("storage.maxHourlySeries", 0, "The maximum number of unique series can be added to the storage during the last hour. "+
		"Excess series are logged and dropped. This can be useful for limiting series cardinality. See https://docs.victoriametrics.com/#cardinality-limiter . "+
		"See also -storage.maxDailySeries")
	maxDailySeries = flag.Int("storage.maxDailySeries", 0, "The maximum number of unique series can be added to the storage during the last 24 hours. "+
		"Excess series are logged and dropped. This can be useful for limiting series churn rate. See https://docs.victoriametrics.com/#cardinality-limiter . "+
		"See also -storage.maxHourlySeries")

	minFreeDiskSpaceBytes = flagutil.NewBytes("storage.minFreeDiskSpaceBytes", 10e6, "The minimum free disk space at -storageDataPath after which the storage stops accepting new data")

	cacheSizeStorageTSID = flagutil.NewBytes("storage.cacheSizeStorageTSID", 0, "Overrides max size for storage/tsid cache. "+
		"See https://docs.victoriametrics.com/single-server-victoriametrics/#cache-tuning")
	cacheSizeIndexDBIndexBlocks = flagutil.NewBytes("storage.cacheSizeIndexDBIndexBlocks", 0, "Overrides max size for indexdb/indexBlocks cache. "+
		"See https://docs.victoriametrics.com/single-server-victoriametrics/#cache-tuning")
	cacheSizeIndexDBDataBlocks = flagutil.NewBytes("storage.cacheSizeIndexDBDataBlocks", 0, "Overrides max size for indexdb/dataBlocks cache. "+
		"See https://docs.victoriametrics.com/single-server-victoriametrics/#cache-tuning")
	cacheSizeIndexDBTagFilters = flagutil.NewBytes("storage.cacheSizeIndexDBTagFilters", 0, "Overrides max size for indexdb/tagFiltersToMetricIDs cache. "+
		"See https://docs.victoriametrics.com/single-server-victoriametrics/#cache-tuning")
)

// CheckTimeRange returns true if the given tr is denied for querying.
func CheckTimeRange(tr storage.TimeRange) error {
	if !*denyQueriesOutsideRetention {
		return nil
	}
	minAllowedTimestamp := int64(fasttime.UnixTimestamp()*1000) - retentionPeriod.Milliseconds()
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
	if err := encoding.CheckPrecisionBits(uint8(*precisionBits)); err != nil {
		logger.Fatalf("invalid `-precisionBits`: %s", err)
	}

	resetResponseCacheIfNeeded = resetCacheIfNeeded
	storage.SetLogNewSeries(*logNewSeries)
	storage.SetRetentionTimezoneOffset(*retentionTimezoneOffset)
	storage.SetFreeDiskSpaceLimit(minFreeDiskSpaceBytes.N)
	storage.SetTSIDCacheSize(cacheSizeStorageTSID.IntN())
	storage.SetTagFiltersCacheSize(cacheSizeIndexDBTagFilters.IntN())
	mergeset.SetIndexBlocksCacheSize(cacheSizeIndexDBIndexBlocks.IntN())
	mergeset.SetDataBlocksCacheSize(cacheSizeIndexDBDataBlocks.IntN())

	if retentionPeriod.Duration() < 24*time.Hour {
		logger.Fatalf("-retentionPeriod cannot be smaller than a day; got %s", retentionPeriod)
	}
	logger.Infof("opening storage at %q with -retentionPeriod=%s", *DataPath, retentionPeriod)
	startTime := time.Now()
	WG = syncwg.WaitGroup{}
	strg := storage.MustOpenStorage(*DataPath, retentionPeriod.Duration(), *maxHourlySeries, *maxDailySeries)
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

	// register storage metrics
	storageMetrics = metrics.NewSet()
	storageMetrics.RegisterMetricsWriter(func(w io.Writer) {
		writeStorageMetrics(w, strg)
	})
	metrics.RegisterSet(storageMetrics)
}

var storageMetrics *metrics.Set

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
//
// The caller should limit the number of concurrent calls to AddRows() in order to limit memory usage.
func AddRows(mrs []storage.MetricRow) error {
	if Storage.IsReadOnly() {
		return errReadOnly
	}
	resetResponseCacheIfNeeded(mrs)
	WG.Add(1)
	Storage.AddRows(mrs, uint8(*precisionBits))
	WG.Done()
	return nil
}

var errReadOnly = errors.New("the storage is in read-only mode; check -storage.minFreeDiskSpaceBytes command-line flag value")

// RegisterMetricNames registers all the metrics from mrs in the storage.
func RegisterMetricNames(qt *querytracer.Tracer, mrs []storage.MetricRow) {
	WG.Add(1)
	Storage.RegisterMetricNames(qt, mrs)
	WG.Done()
}

// DeleteSeries deletes series matching tfss.
//
// Returns the number of deleted series.
func DeleteSeries(qt *querytracer.Tracer, tfss []*storage.TagFilters, maxMetrics int) (int, error) {
	WG.Add(1)
	n, err := Storage.DeleteSeries(qt, tfss, maxMetrics)
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
	tr storage.TimeRange, maxLabelValues, maxMetrics int, deadline uint64,
) ([]string, error) {
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
	// deregister storage metrics
	metrics.UnregisterSet(storageMetrics, true)
	storageMetrics = nil

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
		if !httpserver.CheckAuthFlag(w, r, forceMergeAuthKey) {
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
		if !httpserver.CheckAuthFlag(w, r, forceFlushAuthKey) {
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
	if !strings.HasPrefix(path, "/snapshot/") {
		return false
	}
	if !httpserver.CheckAuthFlag(w, r, snapshotAuthKey) {
		return true
	}
	path = path[len("/snapshot"):]

	switch path {
	case "/create":
		snapshotsCreateTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		snapshotPath, err := Storage.CreateSnapshot()
		if err != nil {
			err = fmt.Errorf("cannot create snapshot: %w", err)
			jsonResponseError(w, err)
			snapshotsCreateErrorsTotal.Inc()
			return true
		}
		if prometheusCompatibleResponse {
			fmt.Fprintf(w, `{"status":"success","data":{"name":%s}}`, stringsutil.JSONString(snapshotPath))
		} else {
			fmt.Fprintf(w, `{"status":"ok","snapshot":%s}`, stringsutil.JSONString(snapshotPath))
		}
		return true
	case "/list":
		snapshotsListTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			snapshotsListErrorsTotal.Inc()
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
		snapshotsDeleteTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		snapshotName := r.FormValue("snapshot")

		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			snapshotsDeleteErrorsTotal.Inc()
			return true
		}
		for _, snName := range snapshots {
			if snName == snapshotName {
				if err := Storage.DeleteSnapshot(snName); err != nil {
					err = fmt.Errorf("cannot delete snapshot %q: %w", snName, err)
					jsonResponseError(w, err)
					snapshotsDeleteErrorsTotal.Inc()
					return true
				}
				fmt.Fprintf(w, `{"status":"ok"}`)
				return true
			}
		}

		err = fmt.Errorf("cannot find snapshot %q", snapshotName)
		jsonResponseError(w, err)
		return true
	case "/delete_all":
		snapshotsDeleteAllTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := Storage.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			snapshotsDeleteAllErrorsTotal.Inc()
			return true
		}
		for _, snapshotName := range snapshots {
			if err := Storage.DeleteSnapshot(snapshotName); err != nil {
				err = fmt.Errorf("cannot delete snapshot %q: %w", snapshotName, err)
				jsonResponseError(w, err)
				snapshotsDeleteAllErrorsTotal.Inc()
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
	if snapshotsMaxAge.Duration() <= 0 {
		return
	}
	snapshotsMaxAgeDur := snapshotsMaxAge.Duration()
	staleSnapshotsRemoverWG.Add(1)
	go func() {
		defer staleSnapshotsRemoverWG.Done()
		d := timeutil.AddJitterToDuration(time.Second * 11)
		t := time.NewTicker(d)
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

var (
	activeForceMerges = metrics.NewCounter("vm_active_force_merges")

	snapshotsCreateTotal       = metrics.NewCounter(`vm_http_requests_total{path="/snapshot/create"}`)
	snapshotsCreateErrorsTotal = metrics.NewCounter(`vm_http_request_errors_total{path="/snapshot/create"}`)

	snapshotsListTotal       = metrics.NewCounter(`vm_http_requests_total{path="/snapshot/list"}`)
	snapshotsListErrorsTotal = metrics.NewCounter(`vm_http_request_errors_total{path="/snapshot/list"}`)

	snapshotsDeleteTotal       = metrics.NewCounter(`vm_http_requests_total{path="/snapshot/delete"}`)
	snapshotsDeleteErrorsTotal = metrics.NewCounter(`vm_http_request_errors_total{path="/snapshot/delete"}`)

	snapshotsDeleteAllTotal       = metrics.NewCounter(`vm_http_requests_total{path="/snapshot/delete_all"}`)
	snapshotsDeleteAllErrorsTotal = metrics.NewCounter(`vm_http_request_errors_total{path="/snapshot/delete_all"}`)
)

func writeStorageMetrics(w io.Writer, strg *storage.Storage) {
	var m storage.Metrics
	strg.UpdateMetrics(&m)
	tm := &m.TableMetrics
	idbm := &m.IndexDBMetrics

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_free_disk_space_bytes{path=%q}`, *DataPath), fs.MustGetFreeSpace(*DataPath))
	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_free_disk_space_limit_bytes{path=%q}`, *DataPath), uint64(minFreeDiskSpaceBytes.N))

	isReadOnly := 0
	if strg.IsReadOnly() {
		isReadOnly = 1
	}
	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_storage_is_read_only{path=%q}`, *DataPath), uint64(isReadOnly))

	metrics.WriteGaugeUint64(w, `vm_active_merges{type="storage/inmemory"}`, tm.ActiveInmemoryMerges)
	metrics.WriteGaugeUint64(w, `vm_active_merges{type="storage/small"}`, tm.ActiveSmallMerges)
	metrics.WriteGaugeUint64(w, `vm_active_merges{type="storage/big"}`, tm.ActiveBigMerges)
	metrics.WriteGaugeUint64(w, `vm_active_merges{type="indexdb/inmemory"}`, idbm.ActiveInmemoryMerges)
	metrics.WriteGaugeUint64(w, `vm_active_merges{type="indexdb/file"}`, idbm.ActiveFileMerges)

	metrics.WriteCounterUint64(w, `vm_merges_total{type="storage/inmemory"}`, tm.InmemoryMergesCount)
	metrics.WriteCounterUint64(w, `vm_merges_total{type="storage/small"}`, tm.SmallMergesCount)
	metrics.WriteCounterUint64(w, `vm_merges_total{type="storage/big"}`, tm.BigMergesCount)
	metrics.WriteCounterUint64(w, `vm_merges_total{type="indexdb/inmemory"}`, idbm.InmemoryMergesCount)
	metrics.WriteCounterUint64(w, `vm_merges_total{type="indexdb/file"}`, idbm.FileMergesCount)

	metrics.WriteCounterUint64(w, `vm_rows_merged_total{type="storage/inmemory"}`, tm.InmemoryRowsMerged)
	metrics.WriteCounterUint64(w, `vm_rows_merged_total{type="storage/small"}`, tm.SmallRowsMerged)
	metrics.WriteCounterUint64(w, `vm_rows_merged_total{type="storage/big"}`, tm.BigRowsMerged)
	metrics.WriteCounterUint64(w, `vm_rows_merged_total{type="indexdb/inmemory"}`, idbm.InmemoryItemsMerged)
	metrics.WriteCounterUint64(w, `vm_rows_merged_total{type="indexdb/file"}`, idbm.FileItemsMerged)

	metrics.WriteCounterUint64(w, `vm_rows_deleted_total{type="storage/inmemory"}`, tm.InmemoryRowsDeleted)
	metrics.WriteCounterUint64(w, `vm_rows_deleted_total{type="storage/small"}`, tm.SmallRowsDeleted)
	metrics.WriteCounterUint64(w, `vm_rows_deleted_total{type="storage/big"}`, tm.BigRowsDeleted)

	metrics.WriteGaugeUint64(w, `vm_part_references{type="storage/inmemory"}`, tm.InmemoryPartsRefCount)
	metrics.WriteGaugeUint64(w, `vm_part_references{type="storage/small"}`, tm.SmallPartsRefCount)
	metrics.WriteGaugeUint64(w, `vm_part_references{type="storage/big"}`, tm.BigPartsRefCount)
	metrics.WriteGaugeUint64(w, `vm_partition_references{type="storage"}`, tm.PartitionsRefCount)
	metrics.WriteGaugeUint64(w, `vm_object_references{type="indexdb"}`, idbm.IndexDBRefCount)
	metrics.WriteGaugeUint64(w, `vm_part_references{type="indexdb"}`, idbm.PartsRefCount)

	metrics.WriteCounterUint64(w, `vm_missing_tsids_for_metric_id_total`, idbm.MissingTSIDsForMetricID)
	metrics.WriteCounterUint64(w, `vm_index_blocks_with_metric_ids_processed_total`, idbm.IndexBlocksWithMetricIDsProcessed)
	metrics.WriteCounterUint64(w, `vm_index_blocks_with_metric_ids_incorrect_order_total`, idbm.IndexBlocksWithMetricIDsIncorrectOrder)
	metrics.WriteGaugeUint64(w, `vm_composite_index_min_timestamp`, idbm.MinTimestampForCompositeIndex/1e3)
	metrics.WriteCounterUint64(w, `vm_composite_filter_success_conversions_total`, idbm.CompositeFilterSuccessConversions)
	metrics.WriteCounterUint64(w, `vm_composite_filter_missing_conversions_total`, idbm.CompositeFilterMissingConversions)

	// vm_assisted_merges_total name is used for backwards compatibility.
	metrics.WriteCounterUint64(w, `vm_assisted_merges_total{type="indexdb/inmemory"}`, idbm.InmemoryPartsLimitReachedCount)

	metrics.WriteCounterUint64(w, `vm_indexdb_items_added_total`, idbm.ItemsAdded)
	metrics.WriteCounterUint64(w, `vm_indexdb_items_added_size_bytes_total`, idbm.ItemsAddedSizeBytes)
	metrics.WriteCounterUint64(w, `vm_indexdb_items_dropped_total{reason="too_long_item"}`, idbm.TooLongItemsDroppedTotal)

	metrics.WriteGaugeUint64(w, `vm_pending_rows{type="storage"}`, tm.PendingRows)
	metrics.WriteGaugeUint64(w, `vm_pending_rows{type="indexdb"}`, idbm.PendingItems)

	metrics.WriteGaugeUint64(w, `vm_parts{type="storage/inmemory"}`, tm.InmemoryPartsCount)
	metrics.WriteGaugeUint64(w, `vm_parts{type="storage/small"}`, tm.SmallPartsCount)
	metrics.WriteGaugeUint64(w, `vm_parts{type="storage/big"}`, tm.BigPartsCount)
	metrics.WriteGaugeUint64(w, `vm_parts{type="indexdb/inmemory"}`, idbm.InmemoryPartsCount)
	metrics.WriteGaugeUint64(w, `vm_parts{type="indexdb/file"}`, idbm.FilePartsCount)

	metrics.WriteGaugeUint64(w, `vm_last_partition_parts{type="storage/inmemory"}`, tm.LastPartition.InmemoryPartsCount)
	metrics.WriteGaugeUint64(w, `vm_last_partition_parts{type="storage/small"}`, tm.LastPartition.SmallPartsCount)
	metrics.WriteGaugeUint64(w, `vm_last_partition_parts{type="storage/big"}`, tm.LastPartition.BigPartsCount)

	metrics.WriteGaugeUint64(w, `vm_blocks{type="storage/inmemory"}`, tm.InmemoryBlocksCount)
	metrics.WriteGaugeUint64(w, `vm_blocks{type="storage/small"}`, tm.SmallBlocksCount)
	metrics.WriteGaugeUint64(w, `vm_blocks{type="storage/big"}`, tm.BigBlocksCount)
	metrics.WriteGaugeUint64(w, `vm_blocks{type="indexdb/inmemory"}`, idbm.InmemoryBlocksCount)
	metrics.WriteGaugeUint64(w, `vm_blocks{type="indexdb/file"}`, idbm.FileBlocksCount)

	metrics.WriteGaugeUint64(w, `vm_data_size_bytes{type="storage/inmemory"}`, tm.InmemorySizeBytes)
	metrics.WriteGaugeUint64(w, `vm_data_size_bytes{type="storage/small"}`, tm.SmallSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_data_size_bytes{type="storage/big"}`, tm.BigSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_data_size_bytes{type="indexdb/inmemory"}`, idbm.InmemorySizeBytes)
	metrics.WriteGaugeUint64(w, `vm_data_size_bytes{type="indexdb/file"}`, idbm.FileSizeBytes)

	metrics.WriteCounterUint64(w, `vm_rows_received_by_storage_total`, m.RowsReceivedTotal)
	metrics.WriteCounterUint64(w, `vm_rows_added_to_storage_total`, m.RowsAddedTotal)
	metrics.WriteCounterUint64(w, `vm_deduplicated_samples_total{type="merge"}`, m.DedupsDuringMerge)
	metrics.WriteGaugeUint64(w, `vm_snapshots`, m.SnapshotsCount)

	metrics.WriteCounterUint64(w, `vm_rows_ignored_total{reason="big_timestamp"}`, m.TooBigTimestampRows)
	metrics.WriteCounterUint64(w, `vm_rows_ignored_total{reason="small_timestamp"}`, m.TooSmallTimestampRows)
	metrics.WriteCounterUint64(w, `vm_rows_ignored_total{reason="invalid_raw_metric_name"}`, m.InvalidRawMetricNames)
	if *maxHourlySeries > 0 {
		metrics.WriteCounterUint64(w, `vm_rows_ignored_total{reason="hourly_limit_exceeded"}`, m.HourlySeriesLimitRowsDropped)
	}
	if *maxDailySeries > 0 {
		metrics.WriteCounterUint64(w, `vm_rows_ignored_total{reason="daily_limit_exceeded"}`, m.DailySeriesLimitRowsDropped)
	}

	metrics.WriteCounterUint64(w, `vm_timeseries_repopulated_total`, m.TimeseriesRepopulated)
	metrics.WriteCounterUint64(w, `vm_timeseries_precreated_total`, m.TimeseriesPreCreated)
	metrics.WriteCounterUint64(w, `vm_new_timeseries_created_total`, m.NewTimeseriesCreated)
	metrics.WriteCounterUint64(w, `vm_slow_row_inserts_total`, m.SlowRowInserts)
	metrics.WriteCounterUint64(w, `vm_slow_per_day_index_inserts_total`, m.SlowPerDayIndexInserts)
	metrics.WriteCounterUint64(w, `vm_slow_metric_name_loads_total`, m.SlowMetricNameLoads)

	if *maxHourlySeries > 0 {
		metrics.WriteGaugeUint64(w, `vm_hourly_series_limit_current_series`, m.HourlySeriesLimitCurrentSeries)
		metrics.WriteGaugeUint64(w, `vm_hourly_series_limit_max_series`, m.HourlySeriesLimitMaxSeries)
		metrics.WriteCounterUint64(w, `vm_hourly_series_limit_rows_dropped_total`, m.HourlySeriesLimitRowsDropped)
	}

	if *maxDailySeries > 0 {
		metrics.WriteGaugeUint64(w, `vm_daily_series_limit_current_series`, m.DailySeriesLimitCurrentSeries)
		metrics.WriteGaugeUint64(w, `vm_daily_series_limit_max_series`, m.DailySeriesLimitMaxSeries)
		metrics.WriteCounterUint64(w, `vm_daily_series_limit_rows_dropped_total`, m.DailySeriesLimitRowsDropped)
	}

	metrics.WriteCounterUint64(w, `vm_timestamps_blocks_merged_total`, m.TimestampsBlocksMerged)
	metrics.WriteCounterUint64(w, `vm_timestamps_bytes_saved_total`, m.TimestampsBytesSaved)

	metrics.WriteGaugeUint64(w, `vm_rows{type="storage/inmemory"}`, tm.InmemoryRowsCount)
	metrics.WriteGaugeUint64(w, `vm_rows{type="storage/small"}`, tm.SmallRowsCount)
	metrics.WriteGaugeUint64(w, `vm_rows{type="storage/big"}`, tm.BigRowsCount)
	metrics.WriteGaugeUint64(w, `vm_rows{type="indexdb/inmemory"}`, idbm.InmemoryItemsCount)
	metrics.WriteGaugeUint64(w, `vm_rows{type="indexdb/file"}`, idbm.FileItemsCount)

	metrics.WriteCounterUint64(w, `vm_date_range_search_calls_total`, idbm.DateRangeSearchCalls)
	metrics.WriteCounterUint64(w, `vm_date_range_hits_total`, idbm.DateRangeSearchHits)
	metrics.WriteCounterUint64(w, `vm_global_search_calls_total`, idbm.GlobalSearchCalls)

	metrics.WriteCounterUint64(w, `vm_missing_metric_names_for_metric_id_total`, idbm.MissingMetricNamesForMetricID)

	metrics.WriteCounterUint64(w, `vm_date_metric_id_cache_syncs_total`, m.DateMetricIDCacheSyncsCount)
	metrics.WriteCounterUint64(w, `vm_date_metric_id_cache_resets_total`, m.DateMetricIDCacheResetsCount)

	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/tsid"}`, m.TSIDCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/metricIDs"}`, m.MetricIDCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/metricName"}`, m.MetricNameCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/date_metricID"}`, m.DateMetricIDCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/hour_metric_ids"}`, m.HourMetricIDCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/next_day_metric_ids"}`, m.NextDayMetricIDCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/indexBlocks"}`, tm.IndexBlocksCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="indexdb/dataBlocks"}`, idbm.DataBlocksCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="indexdb/indexBlocks"}`, idbm.IndexBlocksCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="indexdb/tagFiltersToMetricIDs"}`, idbm.TagFiltersToMetricIDsCacheSize)
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/regexps"}`, uint64(storage.RegexpCacheSize()))
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/regexpPrefixes"}`, uint64(storage.RegexpPrefixesCacheSize()))
	metrics.WriteGaugeUint64(w, `vm_cache_entries{type="storage/prefetchedMetricIDs"}`, m.PrefetchedMetricIDsSize)

	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/tsid"}`, m.TSIDCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/metricIDs"}`, m.MetricIDCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/metricName"}`, m.MetricNameCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/indexBlocks"}`, tm.IndexBlocksCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="indexdb/dataBlocks"}`, idbm.DataBlocksCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="indexdb/indexBlocks"}`, idbm.IndexBlocksCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/date_metricID"}`, m.DateMetricIDCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/hour_metric_ids"}`, m.HourMetricIDCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/next_day_metric_ids"}`, m.NextDayMetricIDCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="indexdb/tagFiltersToMetricIDs"}`, idbm.TagFiltersToMetricIDsCacheSizeBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/regexps"}`, uint64(storage.RegexpCacheSizeBytes()))
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/regexpPrefixes"}`, uint64(storage.RegexpPrefixesCacheSizeBytes()))
	metrics.WriteGaugeUint64(w, `vm_cache_size_bytes{type="storage/prefetchedMetricIDs"}`, m.PrefetchedMetricIDsSizeBytes)

	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/tsid"}`, m.TSIDCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/metricIDs"}`, m.MetricIDCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/metricName"}`, m.MetricNameCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/indexBlocks"}`, tm.IndexBlocksCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="indexdb/dataBlocks"}`, idbm.DataBlocksCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="indexdb/indexBlocks"}`, idbm.IndexBlocksCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="indexdb/tagFiltersToMetricIDs"}`, idbm.TagFiltersToMetricIDsCacheSizeMaxBytes)
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/regexps"}`, uint64(storage.RegexpCacheMaxSizeBytes()))
	metrics.WriteGaugeUint64(w, `vm_cache_size_max_bytes{type="storage/regexpPrefixes"}`, uint64(storage.RegexpPrefixesCacheMaxSizeBytes()))

	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/tsid"}`, m.TSIDCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/metricIDs"}`, m.MetricIDCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/metricName"}`, m.MetricNameCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/indexBlocks"}`, tm.IndexBlocksCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="indexdb/dataBlocks"}`, idbm.DataBlocksCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="indexdb/indexBlocks"}`, idbm.IndexBlocksCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="indexdb/tagFiltersToMetricIDs"}`, idbm.TagFiltersToMetricIDsCacheRequests)
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/regexps"}`, storage.RegexpCacheRequests())
	metrics.WriteCounterUint64(w, `vm_cache_requests_total{type="storage/regexpPrefixes"}`, storage.RegexpPrefixesCacheRequests())

	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/tsid"}`, m.TSIDCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/metricIDs"}`, m.MetricIDCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/metricName"}`, m.MetricNameCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/indexBlocks"}`, tm.IndexBlocksCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="indexdb/dataBlocks"}`, idbm.DataBlocksCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="indexdb/indexBlocks"}`, idbm.IndexBlocksCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="indexdb/tagFiltersToMetricIDs"}`, idbm.TagFiltersToMetricIDsCacheMisses)
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/regexps"}`, storage.RegexpCacheMisses())
	metrics.WriteCounterUint64(w, `vm_cache_misses_total{type="storage/regexpPrefixes"}`, storage.RegexpPrefixesCacheMisses())

	metrics.WriteCounterUint64(w, `vm_deleted_metrics_total{type="indexdb"}`, idbm.DeletedMetricsCount)

	metrics.WriteCounterUint64(w, `vm_cache_collisions_total{type="storage/tsid"}`, m.TSIDCacheCollisions)
	metrics.WriteCounterUint64(w, `vm_cache_collisions_total{type="storage/metricName"}`, m.MetricNameCacheCollisions)

	metrics.WriteGaugeUint64(w, `vm_next_retention_seconds`, m.NextRetentionSeconds)

	metrics.WriteGaugeUint64(w, `vm_downsampling_partitions_scheduled`, tm.ScheduledDownsamplingPartitions)
	metrics.WriteGaugeUint64(w, `vm_downsampling_partitions_scheduled_size_bytes`, tm.ScheduledDownsamplingPartitionsSize)
}

func jsonResponseError(w http.ResponseWriter, err error) {
	logger.Errorf("%s", err)
	w.WriteHeader(http.StatusInternalServerError)
	errStr := err.Error()
	fmt.Fprintf(w, `{"status":"error","msg":%s}`, stringsutil.JSONString(errStr))
}
