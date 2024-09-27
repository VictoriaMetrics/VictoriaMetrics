package logstorage

import (
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/workingsetcache"
)

// StorageStats represents stats for the storage. It may be obtained by calling Storage.UpdateStats().
type StorageStats struct {
	// RowsDroppedTooBigTimestamp is the number of rows dropped during data ingestion because their timestamp is smaller than the minimum allowed
	RowsDroppedTooBigTimestamp uint64

	// RowsDroppedTooSmallTimestamp is the number of rows dropped during data ingestion because their timestamp is bigger than the maximum allowed
	RowsDroppedTooSmallTimestamp uint64

	// PartitionsCount is the number of partitions in the storage
	PartitionsCount uint64

	// IsReadOnly indicates whether the storage is read-only.
	IsReadOnly bool

	// PartitionStats contains partition stats.
	PartitionStats
}

// Reset resets s.
func (s *StorageStats) Reset() {
	*s = StorageStats{}
}

// StorageConfig is the config for the Storage.
type StorageConfig struct {
	// Retention is the retention for the ingested data.
	//
	// Older data is automatically deleted.
	Retention time.Duration

	// MaxDiskSpaceUsageBytes is an optional maximum disk space logs can use.
	//
	// The oldest per-day partitions are automatically dropped if the total disk space usage exceeds this limit.
	MaxDiskSpaceUsageBytes int64

	// FlushInterval is the interval for flushing the in-memory data to disk at the Storage.
	FlushInterval time.Duration

	// FutureRetention is the allowed retention from the current time to future for the ingested data.
	//
	// Log entries with timestamps bigger than now+FutureRetention are ignored.
	FutureRetention time.Duration

	// MinFreeDiskSpaceBytes is the minimum free disk space at storage path after which the storage stops accepting new data
	// and enters read-only mode.
	MinFreeDiskSpaceBytes int64

	// LogNewStreams indicates whether to log newly created log streams.
	//
	// This can be useful for debugging of high cardinality issues.
	// https://docs.victoriametrics.com/victorialogs/keyconcepts/#high-cardinality
	LogNewStreams bool

	// LogIngestedRows indicates whether to log the ingested log entries.
	//
	// This can be useful for debugging of data ingestion.
	LogIngestedRows bool
}

// Storage is the storage for log entries.
type Storage struct {
	rowsDroppedTooBigTimestamp   atomic.Uint64
	rowsDroppedTooSmallTimestamp atomic.Uint64

	// path is the path to the Storage directory
	path string

	// retention is the retention for the stored data
	//
	// older data is automatically deleted
	retention time.Duration

	// maxDiskSpaceUsageBytes is an optional maximum disk space logs can use.
	//
	// The oldest per-day partitions are automatically dropped if the total disk space usage exceeds this limit.
	maxDiskSpaceUsageBytes int64

	// flushInterval is the interval for flushing in-memory data to disk
	flushInterval time.Duration

	// futureRetention is the maximum allowed interval to write data into the future
	futureRetention time.Duration

	// minFreeDiskSpaceBytes is the minimum free disk space at path after which the storage stops accepting new data
	minFreeDiskSpaceBytes uint64

	// logNewStreams instructs to log new streams if it is set to true
	logNewStreams bool

	// logIngestedRows instructs to log all the ingested log entries if it is set to true
	logIngestedRows bool

	// flockF is a file, which makes sure that the Storage is opened by a single process
	flockF *os.File

	// partitions is a list of partitions for the Storage.
	//
	// It must be accessed under partitionsLock.
	partitions []*partitionWrapper

	// ptwHot is the "hot" partition, were the last rows were ingested.
	//
	// It must be accessed under partitionsLock.
	ptwHot *partitionWrapper

	// partitionsLock protects partitions and ptwHot.
	partitionsLock sync.Mutex

	// stopCh is closed when the Storage must be stopped.
	stopCh chan struct{}

	// wg is used for waiting for background workers at MustClose().
	wg sync.WaitGroup

	// streamIDCache caches (partition, streamIDs) seen during data ingestion.
	//
	// It reduces the load on persistent storage during data ingestion by skipping
	// the check whether the given stream is already registered in the persistent storage.
	streamIDCache *workingsetcache.Cache

	// filterStreamCache caches streamIDs keyed by (partition, []TenanID, StreamFilter).
	//
	// It reduces the load on persistent storage during querying by _stream:{...} filter.
	filterStreamCache *workingsetcache.Cache
}

type partitionWrapper struct {
	// refCount is the number of active references to p.
	// When it reaches zero, then the p is closed.
	refCount atomic.Int32

	// The flag, which is set when the partition must be deleted after refCount reaches zero.
	mustDrop atomic.Bool

	// day is the day for the partition in the unix timestamp divided by the number of seconds in the day.
	day int64

	// pt is the wrapped partition.
	pt *partition
}

func newPartitionWrapper(pt *partition, day int64) *partitionWrapper {
	pw := &partitionWrapper{
		day: day,
		pt:  pt,
	}
	pw.incRef()
	return pw
}

func (ptw *partitionWrapper) incRef() {
	ptw.refCount.Add(1)
}

func (ptw *partitionWrapper) decRef() {
	n := ptw.refCount.Add(-1)
	if n > 0 {
		return
	}

	deletePath := ""
	if ptw.mustDrop.Load() {
		deletePath = ptw.pt.path
	}

	// Close pw.pt, since nobody refers to it.
	mustClosePartition(ptw.pt)
	ptw.pt = nil

	// Delete partition if needed.
	if deletePath != "" {
		mustDeletePartition(deletePath)
	}
}

func (ptw *partitionWrapper) canAddAllRows(lr *LogRows) bool {
	minTimestamp := ptw.day * nsecsPerDay
	maxTimestamp := minTimestamp + nsecsPerDay - 1
	for _, ts := range lr.timestamps {
		if ts < minTimestamp || ts > maxTimestamp {
			return false
		}
	}
	return true
}

// mustCreateStorage creates Storage at the given path.
func mustCreateStorage(path string) {
	fs.MustMkdirFailIfExist(path)

	partitionsPath := filepath.Join(path, partitionsDirname)
	fs.MustMkdirFailIfExist(partitionsPath)
}

// MustOpenStorage opens Storage at the given path.
//
// MustClose must be called on the returned Storage when it is no longer needed.
func MustOpenStorage(path string, cfg *StorageConfig) *Storage {
	flushInterval := cfg.FlushInterval
	if flushInterval < time.Second {
		flushInterval = time.Second
	}

	retention := cfg.Retention
	if retention < 24*time.Hour {
		retention = 24 * time.Hour
	}

	futureRetention := cfg.FutureRetention
	if futureRetention < 24*time.Hour {
		futureRetention = 24 * time.Hour
	}

	var minFreeDiskSpaceBytes uint64
	if cfg.MinFreeDiskSpaceBytes >= 0 {
		minFreeDiskSpaceBytes = uint64(cfg.MinFreeDiskSpaceBytes)
	}

	if !fs.IsPathExist(path) {
		mustCreateStorage(path)
	}

	flockF := fs.MustCreateFlockFile(path)

	// Load caches
	mem := memory.Allowed()
	streamIDCachePath := filepath.Join(path, cacheDirname, streamIDCacheFilename)
	streamIDCache := workingsetcache.Load(streamIDCachePath, mem/16)

	filterStreamCache := workingsetcache.New(mem / 10)

	s := &Storage{
		path:                   path,
		retention:              retention,
		maxDiskSpaceUsageBytes: cfg.MaxDiskSpaceUsageBytes,
		flushInterval:          flushInterval,
		futureRetention:        futureRetention,
		minFreeDiskSpaceBytes:  minFreeDiskSpaceBytes,
		logNewStreams:          cfg.LogNewStreams,
		logIngestedRows:        cfg.LogIngestedRows,
		flockF:                 flockF,
		stopCh:                 make(chan struct{}),

		streamIDCache:     streamIDCache,
		filterStreamCache: filterStreamCache,
	}

	partitionsPath := filepath.Join(path, partitionsDirname)
	fs.MustMkdirIfNotExist(partitionsPath)
	des := fs.MustReadDir(partitionsPath)
	ptws := make([]*partitionWrapper, len(des))
	for i, de := range des {
		fname := de.Name()

		// Parse the day for the partition
		t, err := time.Parse(partitionNameFormat, fname)
		if err != nil {
			logger.Panicf("FATAL: cannot parse partition filename %q at %q; it must be in the form YYYYMMDD: %s", fname, partitionsPath, err)
		}
		day := t.UTC().UnixNano() / nsecsPerDay

		partitionPath := filepath.Join(partitionsPath, fname)
		pt := mustOpenPartition(s, partitionPath)
		ptws[i] = newPartitionWrapper(pt, day)
	}
	sort.Slice(ptws, func(i, j int) bool {
		return ptws[i].day < ptws[j].day
	})

	// Delete partitions from the future if needed
	maxAllowedDay := s.getMaxAllowedDay()
	j := len(ptws) - 1
	for j >= 0 {
		ptw := ptws[j]
		if ptw.day <= maxAllowedDay {
			break
		}
		logger.Infof("the partition %s is scheduled to be deleted because it is outside the -futureRetention=%dd", ptw.pt.path, durationToDays(s.futureRetention))
		ptw.mustDrop.Store(true)
		ptw.decRef()
		j--
	}
	j++
	for i := j; i < len(ptws); i++ {
		ptws[i] = nil
	}
	ptws = ptws[:j]

	s.partitions = ptws
	s.runRetentionWatcher()
	s.runMaxDiskSpaceUsageWatcher()
	return s
}

const partitionNameFormat = "20060102"

func (s *Storage) runRetentionWatcher() {
	s.wg.Add(1)
	go func() {
		s.watchRetention()
		s.wg.Done()
	}()
}

func (s *Storage) runMaxDiskSpaceUsageWatcher() {
	if s.maxDiskSpaceUsageBytes <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		s.watchMaxDiskSpaceUsage()
		s.wg.Done()
	}()
}

func (s *Storage) watchRetention() {
	d := timeutil.AddJitterToDuration(time.Hour)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		var ptwsToDelete []*partitionWrapper
		minAllowedDay := s.getMinAllowedDay()

		s.partitionsLock.Lock()

		// Delete outdated partitions.
		// s.partitions are sorted by day, so the partitions, which can become outdated, are located at the beginning of the list
		for _, ptw := range s.partitions {
			if ptw.day >= minAllowedDay {
				break
			}
			ptwsToDelete = append(ptwsToDelete, ptw)
			if ptw == s.ptwHot {
				s.ptwHot = nil
			}
		}
		for i := range ptwsToDelete {
			s.partitions[i] = nil
		}
		s.partitions = s.partitions[len(ptwsToDelete):]

		s.partitionsLock.Unlock()

		for _, ptw := range ptwsToDelete {
			logger.Infof("the partition %s is scheduled to be deleted because it is outside the -retentionPeriod=%dd", ptw.pt.path, durationToDays(s.retention))
			ptw.mustDrop.Store(true)
			ptw.decRef()
		}

		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (s *Storage) watchMaxDiskSpaceUsage() {
	d := timeutil.AddJitterToDuration(10 * time.Second)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		s.partitionsLock.Lock()
		var n uint64
		ptws := s.partitions
		var ptwsToDelete []*partitionWrapper
		for i := len(ptws) - 1; i >= 0; i-- {
			ptw := ptws[i]
			var ps PartitionStats
			ptw.pt.updateStats(&ps)
			n += ps.IndexdbSizeBytes + ps.CompressedSmallPartSize + ps.CompressedBigPartSize
			if n <= uint64(s.maxDiskSpaceUsageBytes) {
				continue
			}
			if i >= len(ptws)-2 {
				// Keep the last two per-day partitions, so logs could be queried for one day time range.
				continue
			}

			// ptws are sorted by time, so just drop all the partitions until i, including i.
			i++
			ptwsToDelete = ptws[:i]
			s.partitions = ptws[i:]

			// Remove reference to deleted partitions from s.ptwHot
			for _, ptw := range ptwsToDelete {
				if ptw == s.ptwHot {
					s.ptwHot = nil
					break
				}
			}

			break
		}
		s.partitionsLock.Unlock()

		for i, ptw := range ptwsToDelete {
			logger.Infof("the partition %s is scheduled to be deleted because the total size of partitions exceeds -retention.maxDiskSpaceUsageBytes=%d",
				ptw.pt.path, s.maxDiskSpaceUsageBytes)
			ptw.mustDrop.Store(true)
			ptw.decRef()

			ptwsToDelete[i] = nil
		}

		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
	}
}

func (s *Storage) getMinAllowedDay() int64 {
	return time.Now().UTC().Add(-s.retention).UnixNano() / nsecsPerDay
}

func (s *Storage) getMaxAllowedDay() int64 {
	return time.Now().UTC().Add(s.futureRetention).UnixNano() / nsecsPerDay
}

// MustClose closes s.
//
// It is expected that nobody uses the storage at the close time.
func (s *Storage) MustClose() {
	// Stop background workers
	close(s.stopCh)
	s.wg.Wait()

	// Close partitions
	for _, pw := range s.partitions {
		pw.decRef()
		if n := pw.refCount.Load(); n != 0 {
			logger.Panicf("BUG: there are %d users of partition", n)
		}
	}
	s.partitions = nil
	s.ptwHot = nil

	// Save caches
	streamIDCachePath := filepath.Join(s.path, cacheDirname, streamIDCacheFilename)
	if err := s.streamIDCache.Save(streamIDCachePath); err != nil {
		logger.Panicf("FATAL: cannot save streamID cache to %q: %s", streamIDCachePath, err)
	}
	s.streamIDCache.Stop()
	s.streamIDCache = nil

	s.filterStreamCache.Stop()
	s.filterStreamCache = nil

	// release lock file
	fs.MustClose(s.flockF)
	s.flockF = nil

	s.path = ""
}

// MustAddRows adds lr to s.
//
// It is recommended checking whether the s is in read-only mode by calling IsReadOnly()
// before calling MustAddRows.
func (s *Storage) MustAddRows(lr *LogRows) {
	// Fast path - try adding all the rows to the hot partition
	s.partitionsLock.Lock()
	ptwHot := s.ptwHot
	if ptwHot != nil {
		ptwHot.incRef()
	}
	s.partitionsLock.Unlock()

	if ptwHot != nil {
		if ptwHot.canAddAllRows(lr) {
			ptwHot.pt.mustAddRows(lr)
			ptwHot.decRef()
			return
		}
		ptwHot.decRef()
	}

	// Slow path - rows cannot be added to the hot partition, so split rows among available partitions
	minAllowedDay := s.getMinAllowedDay()
	maxAllowedDay := s.getMaxAllowedDay()
	m := make(map[int64]*LogRows)
	for i, ts := range lr.timestamps {
		day := ts / nsecsPerDay
		if day < minAllowedDay {
			rf := RowFormatter(lr.rows[i])
			tsf := TimeFormatter(ts)
			minAllowedTsf := TimeFormatter(minAllowedDay * nsecsPerDay)
			tooSmallTimestampLogger.Warnf("skipping log entry with too small timestamp=%s; it must be bigger than %s according "+
				"to the configured -retentionPeriod=%dd. See https://docs.victoriametrics.com/victorialogs/#retention ; "+
				"log entry: %s", &tsf, &minAllowedTsf, durationToDays(s.retention), &rf)
			s.rowsDroppedTooSmallTimestamp.Add(1)
			continue
		}
		if day > maxAllowedDay {
			rf := RowFormatter(lr.rows[i])
			tsf := TimeFormatter(ts)
			maxAllowedTsf := TimeFormatter(maxAllowedDay * nsecsPerDay)
			tooBigTimestampLogger.Warnf("skipping log entry with too big timestamp=%s; it must be smaller than %s according "+
				"to the configured -futureRetention=%dd; see https://docs.victoriametrics.com/victorialogs/#retention ; "+
				"log entry: %s", &tsf, &maxAllowedTsf, durationToDays(s.futureRetention), &rf)
			s.rowsDroppedTooBigTimestamp.Add(1)
			continue
		}
		lrPart := m[day]
		if lrPart == nil {
			lrPart = GetLogRows(nil, nil)
			m[day] = lrPart
		}
		lrPart.mustAddInternal(lr.streamIDs[i], ts, lr.rows[i], lr.streamTagsCanonicals[i])
	}
	for day, lrPart := range m {
		ptw := s.getPartitionForDay(day)
		ptw.pt.mustAddRows(lrPart)
		ptw.decRef()
		PutLogRows(lrPart)
	}
}

var tooSmallTimestampLogger = logger.WithThrottler("too_small_timestamp", 5*time.Second)
var tooBigTimestampLogger = logger.WithThrottler("too_big_timestamp", 5*time.Second)

// TimeFormatter implements fmt.Stringer for timestamp in nanoseconds
type TimeFormatter int64

// String returns human-readable representation for tf.
func (tf *TimeFormatter) String() string {
	ts := int64(*tf)
	t := time.Unix(0, ts).UTC()
	return t.Format(time.RFC3339Nano)
}

func (s *Storage) getPartitionForDay(day int64) *partitionWrapper {
	s.partitionsLock.Lock()

	// Search for the partition using binary search
	ptws := s.partitions
	n := sort.Search(len(ptws), func(i int) bool {
		return ptws[i].day >= day
	})
	var ptw *partitionWrapper
	if n < len(ptws) {
		ptw = ptws[n]
		if ptw.day != day {
			ptw = nil
		}
	}
	if ptw == nil {
		// Missing partition for the given day. Create it.
		fname := time.Unix(0, day*nsecsPerDay).UTC().Format(partitionNameFormat)
		partitionPath := filepath.Join(s.path, partitionsDirname, fname)
		mustCreatePartition(partitionPath)

		pt := mustOpenPartition(s, partitionPath)
		ptw = newPartitionWrapper(pt, day)
		if n == len(ptws) {
			ptws = append(ptws, ptw)
		} else {
			ptws = append(ptws[:n+1], ptws[n:]...)
			ptws[n] = ptw
		}
		s.partitions = ptws
	}

	s.ptwHot = ptw
	ptw.incRef()

	s.partitionsLock.Unlock()

	return ptw
}

// UpdateStats updates ss for the given s.
func (s *Storage) UpdateStats(ss *StorageStats) {
	ss.RowsDroppedTooBigTimestamp += s.rowsDroppedTooBigTimestamp.Load()
	ss.RowsDroppedTooSmallTimestamp += s.rowsDroppedTooSmallTimestamp.Load()

	s.partitionsLock.Lock()
	ss.PartitionsCount += uint64(len(s.partitions))
	for _, ptw := range s.partitions {
		ptw.pt.updateStats(&ss.PartitionStats)
	}
	s.partitionsLock.Unlock()

	ss.IsReadOnly = s.IsReadOnly()
}

// IsReadOnly returns true if s is in read-only mode.
func (s *Storage) IsReadOnly() bool {
	available := fs.MustGetFreeSpace(s.path)
	return available < s.minFreeDiskSpaceBytes
}

func (s *Storage) debugFlush() {
	s.partitionsLock.Lock()
	ptws := append([]*partitionWrapper{}, s.partitions...)
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	for _, ptw := range ptws {
		ptw.pt.debugFlush()
		ptw.decRef()
	}
}

func durationToDays(d time.Duration) int64 {
	return int64(d / (time.Hour * 24))
}
