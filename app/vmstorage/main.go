package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage/servers"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	retentionPeriod  = flagutil.NewDuration("retentionPeriod", "1", "Data with timestamps outside the retentionPeriod is automatically deleted. The minimum retentionPeriod is 24h or 1d. See also -retentionFilter")
	httpListenAddr   = flag.String("httpListenAddr", ":8482", "Address to listen for http connections. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flag.Bool("httpListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	storageDataPath       = flag.String("storageDataPath", "vmstorage-data", "Path to storage data")
	vminsertAddr          = flag.String("vminsertAddr", ":8400", "TCP address to accept connections from vminsert services")
	vmselectAddr          = flag.String("vmselectAddr", ":8401", "TCP address to accept connections from vmselect services")
	snapshotAuthKey       = flag.String("snapshotAuthKey", "", "authKey, which must be passed in query string to /snapshot* pages")
	forceMergeAuthKey     = flag.String("forceMergeAuthKey", "", "authKey, which must be passed in query string to /internal/force_merge pages")
	forceFlushAuthKey     = flag.String("forceFlushAuthKey", "", "authKey, which must be passed in query string to /internal/force_flush pages")
	snapshotsMaxAge       = flagutil.NewDuration("snapshotsMaxAge", "0", "Automatically delete snapshots older than -snapshotsMaxAge if it is set to non-zero duration. Make sure that backup process has enough time to finish the backup before the corresponding snapshot is automatically deleted")
	snapshotCreateTimeout = flag.Duration("snapshotCreateTimeout", 0, "The timeout for creating new snapshot. If set, make sure that timeout is lower than backup period")

	finalMergeDelay = flag.Duration("finalMergeDelay", 0, "The delay before starting final merge for per-month partition after no new data is ingested into it. "+
		"Final merge may require additional disk IO and CPU resources. Final merge may increase query speed and reduce disk space usage in some cases. "+
		"Zero value disables final merge")
	_ = flag.Int("bigMergeConcurrency", 0, "Deprecated: this flag does nothing. Please use -smallMergeConcurrency "+
		"for controlling the concurrency of background merges. See https://docs.victoriametrics.com/#storage")
	smallMergeConcurrency = flag.Int("smallMergeConcurrency", 0, "The maximum number of workers for background merges. See https://docs.victoriametrics.com/#storage . "+
		"It isn't recommended tuning this flag in general case, since this may lead to uncontrolled increase in the number of parts and increased CPU usage during queries")
	retentionTimezoneOffset = flag.Duration("retentionTimezoneOffset", 0, "The offset for performing indexdb rotation. "+
		"If set to 0, then the indexdb rotation is performed at 4am UTC time per each -retentionPeriod. "+
		"If set to 2h, then the indexdb rotation is performed at 4am EET time (the timezone with +2h offset)")
	minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Leave only the last sample in every time series per each discrete interval "+
		"equal to -dedup.minScrapeInterval > 0. See https://docs.victoriametrics.com/#deduplication for details")

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
		"See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBIndexBlocks = flagutil.NewBytes("storage.cacheSizeIndexDBIndexBlocks", 0, "Overrides max size for indexdb/indexBlocks cache. "+
		"See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBDataBlocks = flagutil.NewBytes("storage.cacheSizeIndexDBDataBlocks", 0, "Overrides max size for indexdb/dataBlocks cache. "+
		"See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
	cacheSizeIndexDBTagFilters = flagutil.NewBytes("storage.cacheSizeIndexDBTagFilters", 0, "Overrides max size for indexdb/tagFiltersToMetricIDs cache. "+
		"See https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#cache-tuning")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	pushmetrics.Init()

	storage.SetDedupInterval(*minScrapeInterval)
	storage.SetLogNewSeries(*logNewSeries)
	storage.SetFinalMergeDelay(*finalMergeDelay)
	storage.SetMergeWorkersCount(*smallMergeConcurrency)
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

	registerStorageMetrics(strg)

	common.StartUnmarshalWorkers()
	vminsertSrv, err := servers.NewVMInsertServer(*vminsertAddr, strg)
	if err != nil {
		logger.Fatalf("cannot create a server with -vminsertAddr=%s: %s", *vminsertAddr, err)
	}
	vmselectSrv, err := servers.NewVMSelectServer(*vmselectAddr, strg)
	if err != nil {
		logger.Fatalf("cannot create a server with -vmselectAddr=%s: %s", *vmselectAddr, err)
	}

	requestHandler := newRequestHandler(strg)
	go func() {
		httpserver.Serve(*httpListenAddr, *useProxyProtocol, requestHandler)
	}()

	sig := procutil.WaitForSigterm()
	logger.Infof("service received signal %s", sig)

	logger.Infof("gracefully shutting down http service at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("gracefully shutting down the service")
	startTime = time.Now()
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
			<a href="https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html">docs</a><br>
`)
			return true
		}
		return requestHandler(w, r, strg)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request, strg *storage.Storage) bool {
	path := r.URL.Path
	if path == "/internal/force_merge" {
		if !httpserver.CheckAuthFlag(w, r, *forceMergeAuthKey, "forceMergeAuthKey") {
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
		if !httpserver.CheckAuthFlag(w, r, *forceFlushAuthKey, "forceFlushAuthKey") {
			return true
		}
		logger.Infof("flushing storage to make pending data available for reading")
		strg.DebugFlush()
		return true
	}
	if !strings.HasPrefix(path, "/snapshot") {
		return false
	}
	if !httpserver.CheckAuthFlag(w, r, *snapshotAuthKey, "snapshotAuthKey") {
		return true
	}
	path = path[len("/snapshot"):]

	switch path {
	case "/create":
		snapshotsCreateTotal.Inc()
		w.Header().Set("Content-Type", "application/json")
		deadline := uint64(0)
		if *snapshotCreateTimeout > 0 {
			deadline = fasttime.UnixTimestamp() + uint64(snapshotCreateTimeout.Seconds())
		}
		snapshotPath, err := strg.CreateSnapshot(deadline)
		if err != nil {
			err = fmt.Errorf("cannot create snapshot: %w", err)
			jsonResponseError(w, err)
			snapshotsCreateErrorsTotal.Inc()
			return true
		}
		fmt.Fprintf(w, `{"status":"ok","snapshot":%q}`, snapshotPath)
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

	metrics.NewGauge(fmt.Sprintf(`vm_free_disk_space_bytes{path=%q}`, *storageDataPath), func() float64 {
		return float64(fs.MustGetFreeSpace(*storageDataPath))
	})

	metrics.NewGauge(fmt.Sprintf(`vm_free_disk_space_limit_bytes{path=%q}`, *storageDataPath), func() float64 {
		return float64(minFreeDiskSpaceBytes.N)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_storage_is_read_only{path=%q}`, *storageDataPath), func() float64 {
		if strg.IsReadOnly() {
			return 1
		}
		return 0
	})

	metrics.NewGauge(`vm_active_merges{type="storage/inmemory"}`, func() float64 {
		return float64(tm().ActiveInmemoryMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="storage/small"}`, func() float64 {
		return float64(tm().ActiveSmallMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="storage/big"}`, func() float64 {
		return float64(tm().ActiveBigMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().ActiveInmemoryMerges)
	})
	metrics.NewGauge(`vm_active_merges{type="indexdb/file"}`, func() float64 {
		return float64(idbm().ActiveFileMerges)
	})

	metrics.NewGauge(`vm_merges_total{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryMergesCount)
	})
	metrics.NewGauge(`vm_merges_total{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileMergesCount)
	})

	metrics.NewGauge(`vm_rows_merged_total{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryRowsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryItemsMerged)
	})
	metrics.NewGauge(`vm_rows_merged_total{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileItemsMerged)
	})

	metrics.NewGauge(`vm_rows_deleted_total{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryRowsDeleted)
	})
	metrics.NewGauge(`vm_rows_deleted_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsDeleted)
	})
	metrics.NewGauge(`vm_rows_deleted_total{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsDeleted)
	})

	metrics.NewGauge(`vm_part_references{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryPartsRefCount)
	})
	metrics.NewGauge(`vm_part_references{type="storage/small"}`, func() float64 {
		return float64(tm().SmallPartsRefCount)
	})
	metrics.NewGauge(`vm_part_references{type="storage/big"}`, func() float64 {
		return float64(tm().BigPartsRefCount)
	})
	metrics.NewGauge(`vm_partition_references{type="storage"}`, func() float64 {
		return float64(tm().PartitionsRefCount)
	})
	metrics.NewGauge(`vm_object_references{type="indexdb"}`, func() float64 {
		return float64(idbm().IndexDBRefCount)
	})
	metrics.NewGauge(`vm_part_references{type="indexdb"}`, func() float64 {
		return float64(idbm().PartsRefCount)
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

	metrics.NewGauge(`vm_assisted_merges_total{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryAssistedMerges)
	})
	metrics.NewGauge(`vm_assisted_merges_total{type="storage/small"}`, func() float64 {
		return float64(tm().SmallAssistedMerges)
	})

	metrics.NewGauge(`vm_assisted_merges_total{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryAssistedMerges)
	})
	metrics.NewGauge(`vm_assisted_merges_total{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileAssistedMerges)
	})

	metrics.NewGauge(`vm_indexdb_items_added_total`, func() float64 {
		return float64(idbm().ItemsAdded)
	})
	metrics.NewGauge(`vm_indexdb_items_added_size_bytes_total`, func() float64 {
		return float64(idbm().ItemsAddedSizeBytes)
	})

	metrics.NewGauge(`vm_pending_rows{type="storage"}`, func() float64 {
		return float64(tm().PendingRows)
	})
	metrics.NewGauge(`vm_pending_rows{type="indexdb"}`, func() float64 {
		return float64(idbm().PendingItems)
	})

	metrics.NewGauge(`vm_parts{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="storage/small"}`, func() float64 {
		return float64(tm().SmallPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="storage/big"}`, func() float64 {
		return float64(tm().BigPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryPartsCount)
	})
	metrics.NewGauge(`vm_parts{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FilePartsCount)
	})

	metrics.NewGauge(`vm_blocks{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="storage/small"}`, func() float64 {
		return float64(tm().SmallBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="storage/big"}`, func() float64 {
		return float64(tm().BigBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryBlocksCount)
	})
	metrics.NewGauge(`vm_blocks{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileBlocksCount)
	})

	metrics.NewGauge(`vm_data_size_bytes{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemorySizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="storage/small"}`, func() float64 {
		return float64(tm().SmallSizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="storage/big"}`, func() float64 {
		return float64(tm().BigSizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemorySizeBytes)
	})
	metrics.NewGauge(`vm_data_size_bytes{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileSizeBytes)
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

	metrics.NewGauge(`vm_timeseries_repopulated_total`, func() float64 {
		return float64(m().TimeseriesRepopulated)
	})
	metrics.NewGauge(`vm_timeseries_precreated_total`, func() float64 {
		return float64(m().TimeseriesPreCreated)
	})
	metrics.NewGauge(`vm_new_timeseries_created_total`, func() float64 {
		return float64(m().NewTimeseriesCreated)
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

	if *maxHourlySeries > 0 {
		metrics.NewGauge(`vm_hourly_series_limit_current_series`, func() float64 {
			return float64(m().HourlySeriesLimitCurrentSeries)
		})
		metrics.NewGauge(`vm_hourly_series_limit_max_series`, func() float64 {
			return float64(m().HourlySeriesLimitMaxSeries)
		})
		metrics.NewGauge(`vm_hourly_series_limit_rows_dropped_total`, func() float64 {
			return float64(m().HourlySeriesLimitRowsDropped)
		})
	}

	if *maxDailySeries > 0 {
		metrics.NewGauge(`vm_daily_series_limit_current_series`, func() float64 {
			return float64(m().DailySeriesLimitCurrentSeries)
		})
		metrics.NewGauge(`vm_daily_series_limit_max_series`, func() float64 {
			return float64(m().DailySeriesLimitMaxSeries)
		})
		metrics.NewGauge(`vm_daily_series_limit_rows_dropped_total`, func() float64 {
			return float64(m().DailySeriesLimitRowsDropped)
		})
	}

	metrics.NewGauge(`vm_timestamps_blocks_merged_total`, func() float64 {
		return float64(m().TimestampsBlocksMerged)
	})
	metrics.NewGauge(`vm_timestamps_bytes_saved_total`, func() float64 {
		return float64(m().TimestampsBytesSaved)
	})

	metrics.NewGauge(`vm_rows{type="storage/inmemory"}`, func() float64 {
		return float64(tm().InmemoryRowsCount)
	})
	metrics.NewGauge(`vm_rows{type="storage/small"}`, func() float64 {
		return float64(tm().SmallRowsCount)
	})
	metrics.NewGauge(`vm_rows{type="storage/big"}`, func() float64 {
		return float64(tm().BigRowsCount)
	})
	metrics.NewGauge(`vm_rows{type="indexdb/inmemory"}`, func() float64 {
		return float64(idbm().InmemoryItemsCount)
	})
	metrics.NewGauge(`vm_rows{type="indexdb/file"}`, func() float64 {
		return float64(idbm().FileItemsCount)
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
	metrics.NewGauge(`vm_cache_entries{type="indexdb/tagFiltersToMetricIDs"}`, func() float64 {
		return float64(idbm().TagFiltersToMetricIDsCacheSize)
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
	metrics.NewGauge(`vm_cache_size_bytes{type="indexdb/tagFiltersToMetricIDs"}`, func() float64 {
		return float64(idbm().TagFiltersToMetricIDsCacheSizeBytes)
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
	metrics.NewGauge(`vm_cache_size_max_bytes{type="indexdb/tagFiltersToMetricIDs"}`, func() float64 {
		return float64(idbm().TagFiltersToMetricIDsCacheSizeMaxBytes)
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
	metrics.NewGauge(`vm_cache_requests_total{type="indexdb/tagFiltersToMetricIDs"}`, func() float64 {
		return float64(idbm().TagFiltersToMetricIDsCacheRequests)
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
	metrics.NewGauge(`vm_cache_misses_total{type="indexdb/tagFiltersToMetricIDs"}`, func() float64 {
		return float64(idbm().TagFiltersToMetricIDsCacheMisses)
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

func usage() {
	const s = `
vmstorage stores time series data obtained from vminsert and returns the requested data to vmselect.

See the docs at https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html .
`
	flagutil.Usage(s)
}
