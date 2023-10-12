package vlstorage

import (
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

var (
	retentionPeriod = flagutil.NewDuration("retentionPeriod", "7d", "Log entries with timestamps older than now-retentionPeriod are automatically deleted; "+
		"log entries with timestamps outside the retention are also rejected during data ingestion; the minimum supported retention is 1d (one day); "+
		"see https://docs.victoriametrics.com/VictoriaLogs/#retention")
	futureRetention = flagutil.NewDuration("futureRetention", "2d", "Log entries with timestamps bigger than now+futureRetention are rejected during data ingestion; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/#retention")
	storageDataPath = flag.String("storageDataPath", "victoria-logs-data", "Path to directory with the VictoriaLogs data; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/#storage")
	inmemoryDataFlushInterval = flag.Duration("inmemoryDataFlushInterval", 5*time.Second, "The interval for guaranteed saving of in-memory data to disk. "+
		"The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. "+
		"Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). "+
		"Smaller intervals increase disk IO load. Minimum supported value is 1s")
	logNewStreams = flag.Bool("logNewStreams", false, "Whether to log creation of new streams; this can be useful for debugging of high cardinality issues with log streams; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/keyConcepts.html#stream-fields ; see also -logIngestedRows")
	logIngestedRows = flag.Bool("logIngestedRows", false, "Whether to log all the ingested log entries; this can be useful for debugging of data ingestion; "+
		"see https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/ ; see also -logNewStreams")
	minFreeDiskSpaceBytes = flagutil.NewBytes("storage.minFreeDiskSpaceBytes", 10e6, "The minimum free disk space at -storageDataPath after which "+
		"the storage stops accepting new data")
)

// Init initializes vlstorage.
//
// Stop must be called when vlstorage is no longer needed
func Init() {
	if strg != nil {
		logger.Panicf("BUG: Init() has been already called")
	}

	if retentionPeriod.Duration() < 24*time.Hour {
		logger.Fatalf("-retentionPeriod cannot be smaller than a day; got %s", retentionPeriod)
	}
	cfg := &logstorage.StorageConfig{
		Retention:             retentionPeriod.Duration(),
		FlushInterval:         *inmemoryDataFlushInterval,
		FutureRetention:       futureRetention.Duration(),
		LogNewStreams:         *logNewStreams,
		LogIngestedRows:       *logIngestedRows,
		MinFreeDiskSpaceBytes: minFreeDiskSpaceBytes.N,
	}
	logger.Infof("opening storage at -storageDataPath=%s", *storageDataPath)
	startTime := time.Now()
	strg = logstorage.MustOpenStorage(*storageDataPath, cfg)

	var ss logstorage.StorageStats
	strg.UpdateStats(&ss)
	logger.Infof("successfully opened storage in %.3f seconds; partsCount: %d; blocksCount: %d; rowsCount: %d; sizeBytes: %d",
		time.Since(startTime).Seconds(), ss.FileParts, ss.FileBlocks, ss.FileRowsCount, ss.CompressedFileSize)
	storageMetrics = initStorageMetrics(strg)

	metrics.RegisterSet(storageMetrics)
}

// Stop stops vlstorage.
func Stop() {
	metrics.UnregisterSet(storageMetrics)
	storageMetrics = nil

	strg.MustClose()
	strg = nil
}

var strg *logstorage.Storage
var storageMetrics *metrics.Set

// CanWriteData returns non-nil error if it cannot write data to vlstorage.
func CanWriteData() error {
	if strg.IsReadOnly() {
		return &httpserver.ErrorWithStatusCode{
			Err: fmt.Errorf("cannot add rows into storage in read-only mode; the storage can be in read-only mode "+
				"because of lack of free disk space at -storageDataPath=%s", *storageDataPath),
			StatusCode: http.StatusTooManyRequests,
		}
	}
	return nil
}

// MustAddRows adds lr to vlstorage
//
// It is advised to call CanWriteData() before calling MustAddRows()
func MustAddRows(lr *logstorage.LogRows) {
	strg.MustAddRows(lr)
}

// RunQuery runs the given q and calls processBlock for the returned data blocks
func RunQuery(tenantIDs []logstorage.TenantID, q *logstorage.Query, stopCh <-chan struct{}, processBlock func(columns []logstorage.BlockColumn)) {
	strg.RunQuery(tenantIDs, q, stopCh, processBlock)
}

func initStorageMetrics(strg *logstorage.Storage) *metrics.Set {
	ssCache := &logstorage.StorageStats{}
	var ssCacheLock sync.Mutex
	var lastUpdateTime time.Time

	m := func() *logstorage.StorageStats {
		ssCacheLock.Lock()
		defer ssCacheLock.Unlock()
		if time.Since(lastUpdateTime) < time.Second {
			return ssCache
		}
		var ss logstorage.StorageStats
		strg.UpdateStats(&ss)
		ssCache = &ss
		lastUpdateTime = time.Now()
		return ssCache
	}

	ms := metrics.NewSet()

	ms.NewGauge(fmt.Sprintf(`vl_free_disk_space_bytes{path=%q}`, *storageDataPath), func() float64 {
		return float64(fs.MustGetFreeSpace(*storageDataPath))
	})
	ms.NewGauge(fmt.Sprintf(`vl_storage_is_read_only{path=%q}`, *storageDataPath), func() float64 {
		if m().IsReadOnly {
			return 1
		}
		return 0
	})

	ms.NewGauge(`vl_active_merges{type="inmemory"}`, func() float64 {
		return float64(m().InmemoryActiveMerges)
	})
	ms.NewGauge(`vl_merges_total{type="inmemory"}`, func() float64 {
		return float64(m().InmemoryMergesTotal)
	})
	ms.NewGauge(`vl_active_merges{type="file"}`, func() float64 {
		return float64(m().FileActiveMerges)
	})
	ms.NewGauge(`vl_merges_total{type="file"}`, func() float64 {
		return float64(m().FileMergesTotal)
	})

	ms.NewGauge(`vl_storage_rows{type="inmemory"}`, func() float64 {
		return float64(m().InmemoryRowsCount)
	})
	ms.NewGauge(`vl_storage_rows{type="file"}`, func() float64 {
		return float64(m().FileRowsCount)
	})
	ms.NewGauge(`vl_storage_parts{type="inmemory"}`, func() float64 {
		return float64(m().InmemoryParts)
	})
	ms.NewGauge(`vl_storage_parts{type="file"}`, func() float64 {
		return float64(m().FileParts)
	})
	ms.NewGauge(`vl_storage_blocks{type="inmemory"}`, func() float64 {
		return float64(m().InmemoryBlocks)
	})
	ms.NewGauge(`vl_storage_blocks{type="file"}`, func() float64 {
		return float64(m().FileBlocks)
	})

	ms.NewGauge(`vl_partitions`, func() float64 {
		return float64(m().PartitionsCount)
	})
	ms.NewGauge(`vl_streams_created_total`, func() float64 {
		return float64(m().StreamsCreatedTotal)
	})

	ms.NewGauge(`vl_indexdb_rows`, func() float64 {
		return float64(m().IndexdbItemsCount)
	})
	ms.NewGauge(`vl_indexdb_parts`, func() float64 {
		return float64(m().IndexdbPartsCount)
	})
	ms.NewGauge(`vl_indexdb_blocks`, func() float64 {
		return float64(m().IndexdbBlocksCount)
	})

	ms.NewGauge(`vl_data_size_bytes{type="indexdb"}`, func() float64 {
		return float64(m().IndexdbSizeBytes)
	})
	ms.NewGauge(`vl_data_size_bytes{type="storage"}`, func() float64 {
		dm := m()
		return float64(dm.CompressedInmemorySize + dm.CompressedFileSize)
	})

	ms.NewGauge(`vl_compressed_data_size_bytes{type="inmemory"}`, func() float64 {
		return float64(m().CompressedInmemorySize)
	})
	ms.NewGauge(`vl_compressed_data_size_bytes{type="file"}`, func() float64 {
		return float64(m().CompressedFileSize)
	})
	ms.NewGauge(`vl_uncompressed_data_size_bytes{type="inmemory"}`, func() float64 {
		return float64(m().UncompressedInmemorySize)
	})
	ms.NewGauge(`vl_uncompressed_data_size_bytes{type="file"}`, func() float64 {
		return float64(m().UncompressedFileSize)
	})

	ms.NewGauge(`vl_rows_dropped_total{reason="too_big_timestamp"}`, func() float64 {
		return float64(m().RowsDroppedTooBigTimestamp)
	})
	ms.NewGauge(`vl_rows_dropped_total{reason="too_small_timestamp"}`, func() float64 {
		return float64(m().RowsDroppedTooSmallTimestamp)
	})

	return ms
}
