package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage/transport"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
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
	groupSwitch     = flag.Bool("groupSwitch", false, "Whether to open group storage.")
	storageGroups   = flagutil.NewArray("storageGroups", "Information of storageGroups. Usage: -storageGroups=\"step path queryRangeMin switch\"")

	bigMergeConcurrency   = flag.Int("bigMergeConcurrency", 0, "The maximum number of CPU cores to use for big merges. Default value is used if set to 0")
	smallMergeConcurrency = flag.Int("smallMergeConcurrency", 0, "The maximum number of CPU cores to use for small merges. Default value is used if set to 0")
	minScrapeInterval     = flag.Duration("dedup.minScrapeInterval", 0, "Remove superflouos samples from time series if they are located closer to each other than this duration. "+
		"This may be useful for reducing overhead when multiple identically configured Prometheus instances write data to the same VictoriaMetrics. "+
		"Deduplication is disabled if the -dedup.minScrapeInterval is 0")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	storage.SetMinScrapeIntervalForDeduplication(*minScrapeInterval)
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
	logger.Infof("successfully opened storage %q in %.3f seconds; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		*storageDataPath, time.Since(startTime).Seconds(), partsCount, blocksCount, rowsCount, sizeBytes)

	registerStorageMetrics(strg)

	var strgGroups storage.StorageGroups
	if *groupSwitch {
		registerStorageGroups(&strgGroups)
	}
	srv, err := transport.NewServer(*vminsertAddr, *vmselectAddr, strg, strgGroups)

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

	logger.Infof("gracefully shutting down http service at %q", *httpListenAddr)
	startTime = time.Now()
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop http service: %s", err)
	}
	logger.Infof("successfully shut down http service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("gracefully shutting down the service")
	startTime = time.Now()
	srv.MustClose()
	logger.Infof("successfully shut down the service in %.3f seconds", time.Since(startTime).Seconds())

	logger.Infof("gracefully closing the storage at %s", *storageDataPath)
	startTime = time.Now()
	strg.MustClose()
	logger.Infof("successfully closed the storage in %.3f seconds", time.Since(startTime).Seconds())

	fs.MustStopDirRemover()

	logger.Infof("the vmstorage has been stopped")
}

func registerStorageGroups(strgGroups *storage.StorageGroups) {
	var storageAll []*storage.Storage
	//frist convert string to groupInfo
	var groupInfoAll []*storage.GroupInfo
	if len(*storageGroups) > 0 {
		logger.Infof("storageGroup length is:%d", len(*storageGroups))
		for _, info := range *storageGroups {
			group := storage.StorageGroupEncode(info)
			groupInfoAll = append(groupInfoAll, group)
		}
	}
	//sort group by QueryRange,Sort in reverse order
	sort.Slice(groupInfoAll, func(i, j int) bool {
		return groupInfoAll[i].QueryRangeMin > groupInfoAll[j].QueryRangeMin
	})

	//create storageGroups
	for _, groupInfo := range groupInfoAll {
		storageGroup, err := createStorageGroup(groupInfo)
		if err != nil {
			logger.Fatalf("create group for data store error,err is:%s", err)
		}
		storageAll = append(storageAll, storageGroup)
	}
	strgGroups.GroupSwitch = *groupSwitch
	strgGroups.StorageGroupsAll = storageAll
}
func createStorageGroup(groupInfo *storage.GroupInfo) (*storage.Storage, error) {
	startTime := time.Now()
	strg, err := storage.OpenStorage(groupInfo.Path, *retentionPeriod)
	if err != nil {
		logger.Fatalf("cannot open a storage at %s with retention period %d months: %s", groupInfo.Path, *retentionPeriod, err)
		return nil, err
	}
	var m storage.Metrics
	strg.UpdateMetrics(&m)
	tm := &m.TableMetrics
	partsCount := tm.SmallPartsCount + tm.BigPartsCount
	blocksCount := tm.SmallBlocksCount + tm.BigBlocksCount
	rowsCount := tm.SmallRowsCount + tm.BigRowsCount
	sizeBytes := tm.SmallSizeBytes + tm.BigSizeBytes
	logger.Infof("successfully opened storage %q in %.3f seconds; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		groupInfo.Path, time.Since(startTime).Seconds(), partsCount, blocksCount, rowsCount, sizeBytes)

	registerStorageMetrics(strg, groupInfo.Path)
	strg.SetGroupInfo(groupInfo)
	return strg, nil

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
			err = fmt.Errorf("cannot create snapshot: %s", err)
			jsonResponseError(w, err)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok","snapshot":%q}`, snapshotPath)
		return true
	case "/list":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := strg.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %s", err)
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
		if err := strg.DeleteSnapshot(snapshotName); err != nil {
			err = fmt.Errorf("cannot delete snapshot %q: %s", snapshotName, err)
			jsonResponseError(w, err)
			return true
		}
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	case "/delete_all":
		w.Header().Set("Content-Type", "application/json")
		snapshots, err := strg.ListSnapshots()
		if err != nil {
			err = fmt.Errorf("cannot list snapshots: %s", err)
			jsonResponseError(w, err)
			return true
		}
		for _, snapshotName := range snapshots {
			if err := strg.DeleteSnapshot(snapshotName); err != nil {
				err = fmt.Errorf("cannot delete snapshot %q: %s", snapshotName, err)
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

func registerStorageMetrics(strg *storage.Storage, groupPath ...string) {
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

	var path string
	if len(groupPath) == 0 {
		path = *storageDataPath
	} else {
		path = groupPath[0]
	}
	metrics.NewGauge(fmt.Sprintf(`vm_free_disk_space_bytes{path=%q}`, path), func() float64 {
		return float64(fs.MustGetFreeSpace(*storageDataPath))
	})

	metrics.NewGauge(fmt.Sprintf(`vm_active_merges{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().ActiveBigMerges)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_active_merges{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().ActiveSmallMerges)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_active_merges{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().ActiveMerges)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_merges_total{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigMergesCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_merges_total{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallMergesCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_merges_total{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().MergesCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_rows_merged_total{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigRowsMerged)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows_merged_total{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallRowsMerged)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows_merged_total{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().ItemsMerged)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_rows_deleted_total{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigRowsDeleted)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows_deleted_total{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallRowsDeleted)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_references{type="storage/big", name="parts",path=%q}`, path), func() float64 {
		return float64(tm().BigPartsRefCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_references{type="storage/small", name="parts",path=%q}`, path), func() float64 {
		return float64(tm().SmallPartsRefCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_references{type="storage", name="partitions",path=%q}`, path), func() float64 {
		return float64(tm().PartitionsRefCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_references{type="indexdb", name="objects",path=%q}`, path), func() float64 {
		return float64(idbm().IndexDBRefCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_references{type="indexdb", name="parts",path=%q}`, path), func() float64 {
		return float64(idbm().PartsRefCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_new_timeseries_created_total{path=%q}`, path), func() float64 {
		return float64(idbm().NewTimeseriesCreated)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_missing_tsids_for_metric_id_total{path=%q}`, path), func() float64 {
		return float64(idbm().MissingTSIDsForMetricID)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_recent_hour_metric_ids_search_calls_total{path=%q}`, path), func() float64 {
		return float64(idbm().RecentHourMetricIDsSearchCalls)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_recent_hour_metric_ids_search_hits_total{path=%q}`, path), func() float64 {
		return float64(idbm().RecentHourMetricIDsSearchHits)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_date_metric_ids_search_calls_total{path=%q}`, path), func() float64 {
		return float64(idbm().DateMetricIDsSearchCalls)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_date_metric_ids_search_hits_total{path=%q}`, path), func() float64 {
		return float64(idbm().DateMetricIDsSearchHits)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_index_blocks_with_metric_ids_processed_total{path=%q}`, path), func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsProcessed)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_index_blocks_with_metric_ids_incorrect_order_total{path=%q}`, path), func() float64 {
		return float64(idbm().IndexBlocksWithMetricIDsIncorrectOrder)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_assisted_merges_total{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallAssistedMerges)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_assisted_merges_total{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().AssistedMerges)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_pending_rows{type="storage",path=%q}`, path), func() float64 {
		return float64(tm().PendingRows)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_pending_rows{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().PendingItems)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_parts{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigPartsCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_parts{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallPartsCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_parts{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().PartsCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_blocks{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigBlocksCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_blocks{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallBlocksCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_blocks{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().BlocksCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_data_size_bytes{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_data_size_bytes{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_data_size_bytes{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().SizeBytes)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_deduplicated_samples_total{type="merge",path=%q}`, path), func() float64 {
		return float64(m().DedupsDuringMerge)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_rows_ignored_total{reason="big_timestamp",path=%q}`, path), func() float64 {
		return float64(m().TooBigTimestampRows)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows_ignored_total{reason="small_timestamp",path=%q}`, path), func() float64 {
		return float64(m().TooSmallTimestampRows)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_concurrent_addrows_limit_reached_total{path=%q}`, path), func() float64 {
		return float64(m().AddRowsConcurrencyLimitReached)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_concurrent_addrows_limit_timeout_total{path=%q}`, path), func() float64 {
		return float64(m().AddRowsConcurrencyLimitTimeout)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_concurrent_addrows_dropped_rows_total{path=%q}`, path), func() float64 {
		return float64(m().AddRowsConcurrencyDroppedRows)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_concurrent_addrows_capacity{path=%q}`, path), func() float64 {
		return float64(m().AddRowsConcurrencyCapacity)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_concurrent_addrows_current{path=%q}`, path), func() float64 {
		return float64(m().AddRowsConcurrencyCurrent)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_slow_row_inserts_total{path=%q}`, path), func() float64 {
		return float64(m().SlowRowInserts)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_slow_per_day_index_inserts_total{path=%q}`, path), func() float64 {
		return float64(m().SlowPerDayIndexInserts)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_slow_metric_name_loads_total{path=%q}`, path), func() float64 {
		return float64(m().SlowMetricNameLoads)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_rows{type="storage/big",path=%q}`, path), func() float64 {
		return float64(tm().BigRowsCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows{type="storage/small",path=%q}`, path), func() float64 {
		return float64(tm().SmallRowsCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_rows{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().ItemsCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_date_range_search_calls_total{path=%q}`, path), func() float64 {
		return float64(idbm().DateRangeSearchCalls)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_date_range_hits_total{path=%q}`, path), func() float64 {
		return float64(idbm().DateRangeSearchHits)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_missing_metric_names_for_metric_id_total{path=%q}`, path), func() float64 {
		return float64(idbm().MissingMetricNamesForMetricID)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_date_metric_id_cache_syncs_total{path=%q}`, path), func() float64 {
		return float64(m().DateMetricIDCacheSyncsCount)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_date_metric_id_cache_resets_total{path=%q}`, path), func() float64 {
		return float64(m().DateMetricIDCacheResetsCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/tsid",path=%q}`, path), func() float64 {
		return float64(m().TSIDCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/metricIDs",path=%q}`, path), func() float64 {
		return float64(m().MetricIDCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/metricName",path=%q}`, path), func() float64 {
		return float64(m().MetricNameCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/date_metricID",path=%q}`, path), func() float64 {
		return float64(m().DateMetricIDCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/hour_metric_ids",path=%q}`, path), func() float64 {
		return float64(m().HourMetricIDCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/next_day_metric_ids",path=%q}`, path), func() float64 {
		return float64(m().NextDayMetricIDCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/bigIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().BigIndexBlocksCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/smallIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().SmallIndexBlocksCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="indexdb/dataBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().DataBlocksCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="indexdb/indexBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().IndexBlocksCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="indexdb/tagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().TagCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="indexdb/uselessTagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().UselessTagFiltersCacheSize)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_entries{type="storage/regexps",path=%q}`, path), func() float64 {
		return float64(storage.RegexpCacheSize())
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_entries{type="storage/prefetchedMetricIDs",path=%q}`, path), func() float64 {
		return float64(m().PrefetchedMetricIDsSize)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/tsid",path=%q}`, path), func() float64 {
		return float64(m().TSIDCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/metricIDs",path=%q}`, path), func() float64 {
		return float64(m().MetricIDCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/metricName",path=%q}`, path), func() float64 {
		return float64(m().MetricNameCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/date_metricID",path=%q}`, path), func() float64 {
		return float64(m().DateMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/hour_metric_ids",path=%q}`, path), func() float64 {
		return float64(m().HourMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/next_day_metric_ids",path=%q}`, path), func() float64 {
		return float64(m().NextDayMetricIDCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="indexdb/tagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().TagCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="indexdb/uselessTagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().UselessTagFiltersCacheSizeBytes)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_size_bytes{type="storage/prefetchedMetricIDs",path=%q}`, path), func() float64 {
		return float64(m().PrefetchedMetricIDsSizeBytes)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/tsid",path=%q}`, path), func() float64 {
		return float64(m().TSIDCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/metricIDs",path=%q}`, path), func() float64 {
		return float64(m().MetricIDCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/metricName",path=%q}`, path), func() float64 {
		return float64(m().MetricNameCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/bigIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().BigIndexBlocksCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/smallIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().SmallIndexBlocksCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="indexdb/dataBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().DataBlocksCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="indexdb/indexBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().IndexBlocksCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="indexdb/tagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().TagCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="indexdb/uselessTagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().UselessTagFiltersCacheRequests)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_requests_total{type="storage/regexps",path=%q}`, path), func() float64 {
		return float64(storage.RegexpCacheRequests())
	})

	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/tsid",path=%q}`, path), func() float64 {
		return float64(m().TSIDCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/metricIDs",path=%q}`, path), func() float64 {
		return float64(m().MetricIDCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/metricName",path=%q}`, path), func() float64 {
		return float64(m().MetricNameCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/bigIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().BigIndexBlocksCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/smallIndexBlocks",path=%q}`, path), func() float64 {
		return float64(tm().SmallIndexBlocksCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="indexdb/dataBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().DataBlocksCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="indexdb/indexBlocks",path=%q}`, path), func() float64 {
		return float64(idbm().IndexBlocksCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="indexdb/tagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().TagCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="indexdb/uselessTagFilters",path=%q}`, path), func() float64 {
		return float64(idbm().UselessTagFiltersCacheMisses)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_misses_total{type="storage/regexps",path=%q}`, path), func() float64 {
		return float64(storage.RegexpCacheMisses())
	})

	metrics.NewGauge(fmt.Sprintf(`vm_deleted_metrics_total{type="indexdb",path=%q}`, path), func() float64 {
		return float64(idbm().DeletedMetricsCount)
	})

	metrics.NewGauge(fmt.Sprintf(`vm_cache_collisions_total{type="storage/tsid",path=%q}`, path), func() float64 {
		return float64(m().TSIDCacheCollisions)
	})
	metrics.NewGauge(fmt.Sprintf(`vm_cache_collisions_total{type="storage/metricName",path=%q}`, path), func() float64 {
		return float64(m().MetricNameCacheCollisions)
	})
}

func jsonResponseError(w http.ResponseWriter, err error) {
	logger.Errorf("%s", err)
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"status":"error","msg":%q}`, err)
}
