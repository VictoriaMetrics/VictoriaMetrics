package vlstorage

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
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
		"see https://docs.victoriametrics.com/victorialogs/#retention ; see also -retention.maxDiskSpaceUsageBytes")
	maxDiskSpaceUsageBytes = flagutil.NewBytes("retention.maxDiskSpaceUsageBytes", 0, "The maximum disk space usage at -storageDataPath before older per-day "+
		"partitions are automatically dropped; see https://docs.victoriametrics.com/victorialogs/#retention-by-disk-space-usage ; see also -retentionPeriod")
	futureRetention = flagutil.NewDuration("futureRetention", "2d", "Log entries with timestamps bigger than now+futureRetention are rejected during data ingestion; "+
		"see https://docs.victoriametrics.com/victorialogs/#retention")
	storageDataPath = flag.String("storageDataPath", "victoria-logs-data", "Path to directory where to store VictoriaLogs data; "+
		"see https://docs.victoriametrics.com/victorialogs/#storage")
	inmemoryDataFlushInterval = flag.Duration("inmemoryDataFlushInterval", 5*time.Second, "The interval for guaranteed saving of in-memory data to disk. "+
		"The saved data survives unclean shutdowns such as OOM crash, hardware reset, SIGKILL, etc. "+
		"Bigger intervals may help increase the lifetime of flash storage with limited write cycles (e.g. Raspberry PI). "+
		"Smaller intervals increase disk IO load. Minimum supported value is 1s")
	logNewStreams = flag.Bool("logNewStreams", false, "Whether to log creation of new streams; this can be useful for debugging of high cardinality issues with log streams; "+
		"see https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields ; see also -logIngestedRows")
	logIngestedRows = flag.Bool("logIngestedRows", false, "Whether to log all the ingested log entries; this can be useful for debugging of data ingestion; "+
		"see https://docs.victoriametrics.com/victorialogs/data-ingestion/ ; see also -logNewStreams")
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
		Retention:              retentionPeriod.Duration(),
		MaxDiskSpaceUsageBytes: maxDiskSpaceUsageBytes.N,
		FlushInterval:          *inmemoryDataFlushInterval,
		FutureRetention:        futureRetention.Duration(),
		LogNewStreams:          *logNewStreams,
		LogIngestedRows:        *logIngestedRows,
		MinFreeDiskSpaceBytes:  minFreeDiskSpaceBytes.N,
	}
	logger.Infof("opening storage at -storageDataPath=%s", *storageDataPath)
	startTime := time.Now()
	strg = logstorage.MustOpenStorage(*storageDataPath, cfg)

	var ss logstorage.StorageStats
	strg.UpdateStats(&ss)
	logger.Infof("successfully opened storage in %.3f seconds; smallParts: %d; bigParts: %d; smallPartBlocks: %d; bigPartBlocks: %d; smallPartRows: %d; bigPartRows: %d; "+
		"smallPartSize: %d bytes; bigPartSize: %d bytes",
		time.Since(startTime).Seconds(), ss.SmallParts, ss.BigParts, ss.SmallPartBlocks, ss.BigPartBlocks, ss.SmallPartRowsCount, ss.BigPartRowsCount,
		ss.CompressedSmallPartSize, ss.CompressedBigPartSize)

	// register storage metrics
	storageMetrics = metrics.NewSet()
	storageMetrics.RegisterMetricsWriter(func(w io.Writer) {
		writeStorageMetrics(w, strg)
	})
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

// RunQuery runs the given q and calls writeBlock for the returned data blocks
func RunQuery(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, writeBlock logstorage.WriteBlockFunc) error {
	return strg.RunQuery(ctx, tenantIDs, q, writeBlock)
}

// GetFieldNames executes q and returns field names seen in results.
func GetFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	return strg.GetFieldNames(ctx, tenantIDs, q)
}

// GetFieldValues executes q and returns unique values for the fieldName seen in results.
//
// If limit > 0, then up to limit unique values are returned.
func GetFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	return strg.GetFieldValues(ctx, tenantIDs, q, fieldName, limit)
}

// GetStreamFieldNames executes q and returns stream field names seen in results.
func GetStreamFieldNames(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query) ([]logstorage.ValueWithHits, error) {
	return strg.GetStreamFieldNames(ctx, tenantIDs, q)
}

// GetStreamFieldValues executes q and returns stream field values for the given fieldName seen in results.
//
// If limit > 0, then up to limit unique stream field values are returned.
func GetStreamFieldValues(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, fieldName string, limit uint64) ([]logstorage.ValueWithHits, error) {
	return strg.GetStreamFieldValues(ctx, tenantIDs, q, fieldName, limit)
}

// GetStreams executes q and returns streams seen in query results.
//
// If limit > 0, then up to limit unique streams are returned.
func GetStreams(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	return strg.GetStreams(ctx, tenantIDs, q, limit)
}

// GetStreamIDs executes q and returns streamIDs seen in query results.
//
// If limit > 0, then up to limit unique streamIDs are returned.
func GetStreamIDs(ctx context.Context, tenantIDs []logstorage.TenantID, q *logstorage.Query, limit uint64) ([]logstorage.ValueWithHits, error) {
	return strg.GetStreamIDs(ctx, tenantIDs, q, limit)
}

func writeStorageMetrics(w io.Writer, strg *logstorage.Storage) {
	var ss logstorage.StorageStats
	strg.UpdateStats(&ss)

	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vl_free_disk_space_bytes{path=%q}`, *storageDataPath), fs.MustGetFreeSpace(*storageDataPath))

	isReadOnly := uint64(0)
	if ss.IsReadOnly {
		isReadOnly = 1
	}
	metrics.WriteGaugeUint64(w, fmt.Sprintf(`vl_storage_is_read_only{path=%q}`, *storageDataPath), isReadOnly)

	metrics.WriteGaugeUint64(w, `vl_active_merges{type="storage/inmemory"}`, ss.InmemoryActiveMerges)
	metrics.WriteGaugeUint64(w, `vl_active_merges{type="storage/small"}`, ss.SmallPartActiveMerges)
	metrics.WriteGaugeUint64(w, `vl_active_merges{type="storage/big"}`, ss.BigPartActiveMerges)

	metrics.WriteCounterUint64(w, `vl_merges_total{type="storage/inmemory"}`, ss.InmemoryMergesTotal)
	metrics.WriteCounterUint64(w, `vl_merges_total{type="storage/small"}`, ss.SmallPartMergesTotal)
	metrics.WriteCounterUint64(w, `vl_merges_total{type="storage/big"}`, ss.BigPartMergesTotal)

	metrics.WriteGaugeUint64(w, `vl_storage_rows{type="storage/inmemory"}`, ss.InmemoryRowsCount)
	metrics.WriteGaugeUint64(w, `vl_storage_rows{type="storage/small"}`, ss.SmallPartRowsCount)
	metrics.WriteGaugeUint64(w, `vl_storage_rows{type="storage/big"}`, ss.BigPartRowsCount)

	metrics.WriteGaugeUint64(w, `vl_storage_parts{type="storage/inmemory"}`, ss.InmemoryParts)
	metrics.WriteGaugeUint64(w, `vl_storage_parts{type="storage/small"}`, ss.SmallParts)
	metrics.WriteGaugeUint64(w, `vl_storage_parts{type="storage/big"}`, ss.BigParts)

	metrics.WriteGaugeUint64(w, `vl_storage_blocks{type="storage/inmemory"}`, ss.InmemoryBlocks)
	metrics.WriteGaugeUint64(w, `vl_storage_blocks{type="storage/small"}`, ss.SmallPartBlocks)
	metrics.WriteGaugeUint64(w, `vl_storage_blocks{type="storage/big"}`, ss.BigPartBlocks)

	metrics.WriteGaugeUint64(w, `vl_partitions`, ss.PartitionsCount)
	metrics.WriteCounterUint64(w, `vl_streams_created_total`, ss.StreamsCreatedTotal)

	metrics.WriteGaugeUint64(w, `vl_indexdb_rows`, ss.IndexdbItemsCount)
	metrics.WriteGaugeUint64(w, `vl_indexdb_parts`, ss.IndexdbPartsCount)
	metrics.WriteGaugeUint64(w, `vl_indexdb_blocks`, ss.IndexdbBlocksCount)

	metrics.WriteGaugeUint64(w, `vl_data_size_bytes{type="indexdb"}`, ss.IndexdbSizeBytes)
	metrics.WriteGaugeUint64(w, `vl_data_size_bytes{type="storage"}`, ss.CompressedInmemorySize+ss.CompressedSmallPartSize+ss.CompressedBigPartSize)

	metrics.WriteGaugeUint64(w, `vl_compressed_data_size_bytes{type="storage/inmemory"}`, ss.CompressedInmemorySize)
	metrics.WriteGaugeUint64(w, `vl_compressed_data_size_bytes{type="storage/small"}`, ss.CompressedSmallPartSize)
	metrics.WriteGaugeUint64(w, `vl_compressed_data_size_bytes{type="storage/big"}`, ss.CompressedBigPartSize)

	metrics.WriteGaugeUint64(w, `vl_uncompressed_data_size_bytes{type="storage/inmemory"}`, ss.UncompressedInmemorySize)
	metrics.WriteGaugeUint64(w, `vl_uncompressed_data_size_bytes{type="storage/small"}`, ss.UncompressedSmallPartSize)
	metrics.WriteGaugeUint64(w, `vl_uncompressed_data_size_bytes{type="storage/big"}`, ss.UncompressedBigPartSize)

	metrics.WriteCounterUint64(w, `vl_rows_dropped_total{reason="too_big_timestamp"}`, ss.RowsDroppedTooBigTimestamp)
	metrics.WriteCounterUint64(w, `vl_rows_dropped_total{reason="too_small_timestamp"}`, ss.RowsDroppedTooSmallTimestamp)
}
