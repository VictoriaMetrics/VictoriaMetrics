package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage/servers"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

var (
	retentionPeriod  = flagutil.NewDuration("retentionPeriod", "1", "Data with timestamps outside the retentionPeriod is automatically deleted. The minimum retentionPeriod is 24h or 1d. See also -retentionFilter")
	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "Address to listen for incoming http requests. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the given -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	storageDataPath   = flag.String("storageDataPath", "vmstorage-data", "Path to storage data")
	vminsertAddr      = flag.String("vminsertAddr", ":8400", "TCP address to accept connections from vminsert services")
	vmselectAddr      = flag.String("vmselectAddr", ":8401", "TCP address to accept connections from vmselect services")
	snapshotAuthKey   = flagutil.NewPassword("snapshotAuthKey", "authKey, which must be passed in query string to /snapshot* pages")
	forceMergeAuthKey = flagutil.NewPassword("forceMergeAuthKey", "authKey, which must be passed in query string to /internal/force_merge pages")
	forceFlushAuthKey = flagutil.NewPassword("forceFlushAuthKey", "authKey, which must be passed in query string to /internal/force_flush pages")
	snapshotsMaxAge   = flagutil.NewDuration("snapshotsMaxAge", "0", "Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted")
	_                 = flag.Duration("snapshotCreateTimeout", 0, "Deprecated: this flag does nothing")

	_ = flag.Duration("finalMergeDelay", 0, "Deprecated: this flag does nothing")
	_ = flag.Int("bigMergeConcurrency", 0, "Deprecated: this flag does nothing")
	_ = flag.Int("smallMergeConcurrency", 0, "Deprecated: this flag does nothing")

	retentionTimezoneOffset = flag.Duration("retentionTimezoneOffset", 0, "The offset for performing indexdb rotation. "+
		"If set to 0, then the indexdb rotation is performed at 4am UTC time per each -retentionPeriod. "+
		"If set to 2h, then the indexdb rotation is performed at 4am EET time (the timezone with +2h offset)")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Leave only the last sample in every time series per each discrete interval "+
		"equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details")
	inmemoryDataFlushInterval = flag.Duration("inmemoryDataFlushInterval", 5*time.Second, "The interval for guaranteed saving of in-memory data to disk. "+
		"The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. "+
		"Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). "+
		"Smaller intervals increase disk IO load. Minimum supported value is 1s")

	logNewSeries = flag.Bool("logNewSeries", false, "Whether to log new series. This option is for debug purposes only. It can lead to performance issues "+
		"when big number of new series are ingested into VictoriaMetrics")
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

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	storage.SetDedupInterval(*minScrapeInterval)
	storage.SetDataFlushInterval(*inmemoryDataFlushInterval)
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
	logger.Infof("opening storage at %q with -retentionPeriod=%s", *storageDataPath, retentionPeriod)
	startTime := time.Now()
	strg := storage.MustOpenStorage(*storageDataPath, retentionPeriod.Duration(), *maxHourlySeries, *maxDailySeries)
	initStaleSnapshotsRemover(strg)

	var m storage.Metrics
	strg.UpdateMetrics(&m)
	tm := &m.TableMetrics
	partsCount := tm.SmallPartsCount + tm.BigPartsCount
	blocksCount := tm.SmallBlocksCount + tm.BigBlocksCount
	rowsCount := tm.SmallRowsCount + tm.BigRowsCount
	sizeBytes := tm.SmallSizeBytes + tm.BigSizeBytes
	logger.Infof("successfully opened storage %q in %.3f seconds; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		*storageDataPath, time.Since(startTime).Seconds(), partsCount, blocksCount, rowsCount, sizeBytes)

	// register storage metrics
	storageMetrics := metrics.NewSet()
	storageMetrics.RegisterMetricsWriter(func(w io.Writer) {
		writeStorageMetrics(w, strg)
	})
	metrics.RegisterSet(storageMetrics)

	common.StartUnmarshalWorkers()

	servers.GetMaxUniqueTimeSeries() // for init and logging only.
	vminsertSrv, err := servers.NewVMInsertServer(*vminsertAddr, strg)
	if err != nil {
		logger.Fatalf("cannot create a server with -vminsertAddr=%s: %s", *vminsertAddr, err)
	}
	vmselectSrv, err := servers.NewVMSelectServer(*vmselectAddr, strg)
	if err != nil {
		logger.Fatalf("cannot create a server with -vmselectAddr=%s: %s", *vmselectAddr, err)
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8482"}
	}
	requestHandler := newRequestHandler(strg)
	go httpserver.Serve(listenAddrs, useProxyProtocol, requestHandler)

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)
	pushmetrics.Stop()

	logger.Infof("gracefully shutting down http service at %q", listenAddrs)
	startTime = time.Now()
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("gracefully shutting down the service")
	startTime = time.Now()

	// deregister storage metrics
	metrics.UnregisterSet(storageMetrics, true)
	storageMetrics = nil

	stopStaleSnapshotsRemover()
	vmselectSrv.MustStop()
	vminsertSrv.MustStop()
	common.StopUnmarshalWorkers()
	logger.Infof("successfully shut down the service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("gracefully closing the storage at %s", *storageDataPath)
	startTime = time.Now()
	strg.MustClose()
	logger.Infof("successfully closed the storage in %.3f seconds", time.Since(startTime).Seconds())

	fs.MustStopDirRemover()

	logger.Infof("the vmstorage has been stopped")
}

func newRequestHandler(strg *storage.Storage) httpserver.RequestHandler {
	return func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/" {
			if r.Method != http.MethodGet {
				return false
			}
			w.Header().Add("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `vmstorage - a component of VictoriaMetrics cluster<br/>
			<a href="https://docs.victoriametrics.com/cluster-victoriametrics/">docs</a><br>
`)
			return true
		}
		return requestHandler(w, r, strg)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request, strg *storage.Storage) bool {
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
			if err := strg.ForceMergePartitions(partitionNamePrefix); err != nil {
				logger.Errorf("error in forced merge for partition_prefix=%q: %s", partitionNamePrefix, err)
				return
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
		strg.DebugFlush()
		return true
	}
	if !strings.HasPrefix(path, "/snapshot") {
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
		snapshotPath, err := strg.CreateSnapshot()
		if err != nil {
			err = fmt.Errorf("cannot create snapshot: %w", err)
			jsonResponseError(w, err)
			snapshotsCreateErrorsTotal.Inc()
			return true
		}
		fmt.Fprintf(w, `{"status":"ok","snapshot":%s}`, stringsutil.JSONString(snapshotPath))
		return true
	case "/list":
		snapshotsListTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := strg.ListSnapshots()
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

		snapshots, err := strg.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			snapshotsDeleteErrorsTotal.Inc()
			return true
		}
		for _, snName := range snapshots {
			if snName == snapshotName {
				if err := strg.DeleteSnapshot(snName); err != nil {
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
		snapshots, err := strg.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %w", err)
			jsonResponseError(w, err)
			snapshotsDeleteAllErrorsTotal.Inc()
			return true
		}
		for _, snapshotName := range snapshots {
			if err := strg.DeleteSnapshot(snapshotName); err != nil {
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

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_free_disk_space_bytes{path=%q}`, *storageDataPath), fs.MustGetFreeSpace(*storageDataPath))
	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_free_disk_space_limit_bytes{path=%q}`, *storageDataPath), uint64(minFreeDiskSpaceBytes.N))

	isReadOnly := 0
	if strg.IsReadOnly() {
		isReadOnly = 1
	}
	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vm_storage_is_read_only{path=%q}`, *storageDataPath), uint64(isReadOnly))

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

	metrics.WriteGaugeUint64(w, `vm_search_max_unique_timeseries`, uint64(servers.GetMaxUniqueTimeSeries()))
}

func jsonResponseError(w http.ResponseWriter, err error) {
	logger.Errorf("%s", err)
	w.WriteHeader(http.StatusInternalServerError)
	errStr := err.Error()
	fmt.Fprintf(w, `{"status":"error","msg":%s}`, stringsutil.JSONString(errStr))
}

func usage() {
	const s = `
vmstorage stores time series data obtained from vminsert and returns the requested data to vmselect.

See the docs at https://docs.victoriametrics.com/cluster-victoriametrics/ .
`
	flagutil.Usage(s)
}
