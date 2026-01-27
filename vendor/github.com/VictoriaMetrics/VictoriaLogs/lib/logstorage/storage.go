package logstorage

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/contextutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/snapshot/snapshotutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// StorageStats represents stats for the storage. It may be obtained by calling Storage.UpdateStats().
type StorageStats struct {
	// RowsDroppedTooBigTimestamp is the number of rows dropped during data ingestion because their timestamp is bigger than the maximum allowed.
	RowsDroppedTooBigTimestamp uint64

	// RowsDroppedTooSmallTimestamp is the number of rows dropped during data ingestion because their timestamp is smaller than the minimum allowed.
	RowsDroppedTooSmallTimestamp uint64

	// PartitionsCount is the number of partitions in the storage.
	PartitionsCount uint64

	// MaxDiskSpaceUsageBytes is the maximum disk space logs can use.
	MaxDiskSpaceUsageBytes int64

	// IsReadOnly indicates whether the storage is read-only.
	IsReadOnly bool

	// PartitionStats contains partition stats.
	PartitionStats

	// MinTimestamp is the minimum event timestamp across the entire storage (in nanoseconds).
	// It is set to math.MinInt64 if there is no data.
	MinTimestamp int64

	// MaxTimestamp is the maximum event timestamp across the entire storage (in nanoseconds).
	// It is set to math.MaxInt64 if there is no data.
	MaxTimestamp int64
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

	// DefaultParallelReaders is the default number of parallel readers to use per each query execution.
	//
	// Higher value can help improving query performance on storage with high disk read latency such as S3.
	DefaultParallelReaders int

	// MaxDiskSpaceUsageBytes is an optional maximum disk space logs can use.
	//
	// The oldest per-day partitions are automatically dropped if the total disk space usage exceeds this limit.
	MaxDiskSpaceUsageBytes int64

	// MaxDiskUsagePercent is an optional threshold in percentage (1-100) for disk usage of the filesystem holding the storage path.
	// When the current disk usage exceeds this percentage, the oldest per-day partitions are automatically dropped.
	MaxDiskUsagePercent int

	// FlushInterval is the interval for flushing the in-memory data to disk at the Storage.
	FlushInterval time.Duration

	// FutureRetention is the allowed retention from the current time to future for the ingested data.
	//
	// Log entries with timestamps bigger than now+FutureRetention are ignored.
	FutureRetention time.Duration

	// MaxBackfillAge is the maximum allowed age for the backfilled logs.
	//
	// Log entries with timestamps older than now-MaxBackfillAge are ignored.
	MaxBackfillAge time.Duration

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

	// defaultParallelReaders is the default number of parallel IO-bound readers to use for query execution.
	//
	// Higher number of readers may help increasing query performance on storage with high read latency such as S3.
	defaultParallelReaders int

	// maxDiskSpaceUsageBytes is an optional maximum disk space logs can use.
	//
	// The oldest per-day partitions are automatically dropped if the total disk space usage exceeds this limit.
	maxDiskSpaceUsageBytes int64

	// maxDiskUsagePercent is an optional threshold for disk usage percentage at which the oldest partitions are automatically dropped.
	maxDiskUsagePercent int

	// flushInterval is the interval for flushing in-memory data to disk
	flushInterval time.Duration

	// futureRetention is the maximum allowed interval to write data into the future
	futureRetention time.Duration

	// maxBackfillAge is the maximum age of logs with historical timestamps to accept
	maxBackfillAge time.Duration

	// minFreeDiskSpaceBytes is the minimum free disk space at path after which the storage stops accepting new data
	minFreeDiskSpaceBytes uint64

	// logNewStreams instructs to log new streams if it is set to true
	logNewStreams atomic.Bool

	// logIngestedRows instructs to log all the ingested log entries if it is set to true
	logIngestedRows bool

	// flockF is a file, which makes sure that the Storage is opened by a single process
	flockF *os.File

	// partitions is a list of partitions for the Storage.
	//
	// It must be accessed under partitionsLock.
	//
	// partitions are sorted by time, e.g. partitions[0] has the smallest time.
	partitions []*partitionWrapper

	// ptwHot is the "hot" partition, were the last rows were ingested.
	//
	// It must be accessed under partitionsLock.
	ptwHot *partitionWrapper

	// deletedPartitions contains days for the deleted partitions.
	//
	// It prevents from re-creating already deleted partitions.
	//
	// It must be accessed under partitionsLock.
	deletedPartitions []int64

	// partitionsLock protects partitions, ptwHot, deletedPartitions.
	partitionsLock sync.Mutex

	// stopCh is closed when the Storage must be stopped.
	stopCh chan struct{}

	// wg is used for waiting for background workers at MustClose().
	wg sync.WaitGroup

	// streamIDCache caches (partition, streamIDs) seen during data ingestion.
	//
	// It reduces the load on persistent storage during data ingestion by skipping
	// the check whether the given stream is already registered in the persistent storage.
	streamIDCache *cache

	// filterStreamCache caches streamIDs keyed by (partition, []TenanID, StreamFilter).
	//
	// It reduces the load on persistent storage during querying by _stream:{...} filter.
	filterStreamCache *cache

	// deleteTasksLock protects deleteTasks
	deleteTasksLock sync.Mutex

	// deleteTasks contains a list of active and pending delete tasks
	deleteTasks []*DeleteTask
}

// PartitionAttach attaches the partition with the given name to s.
//
// The name must have the YYYYMMDD format.
//
// The attached partition can be detached via PartitionDetach() call.
func (s *Storage) PartitionAttach(name string) error {
	day, err := getPartitionDayFromName(name)
	if err != nil {
		return err
	}

	s.partitionsLock.Lock()
	defer s.partitionsLock.Unlock()

	if slices.Contains(s.deletedPartitions, day) {
		return fmt.Errorf("cannot attach the partition %q, since it is automatically deleted because of retention; see https://docs.victoriametrics.com/victorialogs/#retention", name)
	}

	// Verify whether the given partition already exists in the attached partitions list.
	for _, ptw := range s.partitions {
		if ptw.pt.name == name {
			return fmt.Errorf("cannot attach the partition %q, because it is arleady attached", name)
		}
	}

	// Open the partition and add it to the s.partitions.
	partitionsPath := filepath.Join(s.path, partitionsDirname)
	partitionPath := filepath.Join(partitionsPath, name)
	if !fs.IsPathExist(partitionPath) {
		return fmt.Errorf("cannot attach the partition %q, because there is no the corresponding directory %q", name, partitionPath)
	}

	pt := mustOpenPartition(s, partitionPath)
	ptw := newPartitionWrapper(pt, day)

	s.partitions = append(s.partitions, ptw)
	sortPartitions(s.partitions)

	logger.Infof("successfully attached partition %q from %q", name, partitionPath)

	return nil
}

// PartitionDetach detaches the partition with the given name from s.
//
// The name must have the YYYYMMDD format.
//
// The detached partition can be attached again via PartitionAttach() call.
func (s *Storage) PartitionDetach(name string) error {
	ptw := func() *partitionWrapper {
		s.partitionsLock.Lock()
		defer s.partitionsLock.Unlock()

		for i, ptw := range s.partitions {
			if ptw.pt.name != name {
				continue
			}

			// Found the partition to detach. Detach it.
			s.partitions = append(s.partitions[:i], s.partitions[i+1:]...)
			if ptw == s.ptwHot {
				s.ptwHot = nil
			}
			return ptw
		}
		return nil
	}()

	if ptw == nil {
		return fmt.Errorf("cannot detach the partition %q, because it isn't attached", name)
	}

	partitionPath := ptw.pt.path
	ptw.decRef()

	logger.Infof("waiting until the partition %q isn't accessed", name)
	<-ptw.doneCh

	logger.Infof("successfully detached partition %q from %q", name, partitionPath)

	return nil
}

// PartitionList returns the list of the names for the currently attached partitions.
//
// Every partition name has YYYYMMDD format.
func (s *Storage) PartitionList() []string {
	s.partitionsLock.Lock()
	ptNames := make([]string, len(s.partitions))
	for i, ptw := range s.partitions {
		ptNames[i] = ptw.pt.name
	}
	s.partitionsLock.Unlock()

	return ptNames
}

// PartitionSnapshotCreate creates a snapshot for the partition with the given name
//
// The snaphsot name must have YYYYMMDD format.
//
// The function returns path to the created snapshot on success.
func (s *Storage) PartitionSnapshotCreate(name string) (string, error) {
	ptw := func() *partitionWrapper {
		s.partitionsLock.Lock()
		defer s.partitionsLock.Unlock()

		for _, ptw := range s.partitions {
			if ptw.pt.name == name {
				ptw.incRef()
				return ptw
			}
		}
		return nil
	}()

	if ptw == nil {
		return "", fmt.Errorf("cannot create snapshot from partition %q, because it is missing", name)
	}

	snapshotPath := ptw.pt.mustCreateSnapshot()
	ptw.decRef()

	return snapshotPath, nil
}

// PartitionSnapshotList returns a list of paths to all the snapshots across active partitions.
func (s *Storage) PartitionSnapshotList() []string {
	s.partitionsLock.Lock()
	ptws := append([]*partitionWrapper{}, s.partitions...)
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	var snapshotPaths []string
	for _, ptw := range ptws {
		ptPath := ptw.pt.path
		snapshotsPath := filepath.Join(ptPath, snapshotsDirname)
		if !fs.IsPathExist(snapshotsPath) {
			continue
		}

		des := fs.MustReadDir(snapshotsPath)
		for _, de := range des {
			name := de.Name()
			if err := snapshotutil.Validate(name); err != nil {
				logger.Warnf("unsupported snapshot name %q at %q: %s", name, snapshotsPath, err)
				continue
			}

			path := filepath.Join(snapshotsPath, name)
			snapshotPaths = append(snapshotPaths, path)
		}
	}

	for _, ptw := range ptws {
		ptw.decRef()
	}

	sort.Strings(snapshotPaths)

	return snapshotPaths
}

// PartitionSnapshotDelete removes the snapshot located at the given snapshotPath if it belongs to an active partition.
func (s *Storage) PartitionSnapshotDelete(snapshotPath string) error {
	snapshotName := filepath.Base(snapshotPath)
	if err := snapshotutil.Validate(snapshotName); err != nil {
		return fmt.Errorf("unsupported snapshot name %q at %q: %s", snapshotName, snapshotPath, err)
	}

	snapshotDir := filepath.Dir(snapshotPath)
	if filepath.Base(snapshotDir) != snapshotsDirname {
		return fmt.Errorf("snapshot path %q must point to a directory inside %q", snapshotPath, snapshotsDirname)
	}
	partitionPath := filepath.Dir(snapshotDir)

	ptw := func() *partitionWrapper {
		s.partitionsLock.Lock()
		defer s.partitionsLock.Unlock()

		for _, ptw := range s.partitions {
			if partitionPath == ptw.pt.path {
				ptw.incRef()
				return ptw
			}
		}
		return nil
	}()

	if ptw == nil {
		return fmt.Errorf("partition path %q cannot be found across active partitions", partitionPath)
	}
	defer ptw.decRef()

	return ptw.pt.deleteSnapshot(snapshotName)
}

// DeleteRunTask starts deletion of logs according to the given filter f for the given tenantIDs.
//
// The taskID must contain an unique id of the task. It is used for tracking the task at the list returned by DeleteActiveTasks().
// The timestamp must contain the timestamp in seconds when the task is started.
func (s *Storage) DeleteRunTask(_ context.Context, taskID string, timestamp int64, tenantIDs []TenantID, f *Filter) error {
	// Register the task in the list of active delete tasks, so it survives application restarts and crashes.
	dt := newDeleteTask(taskID, tenantIDs, f.String(), timestamp)

	s.deleteTasksLock.Lock()
	defer s.deleteTasksLock.Unlock()

	// Verify that the task with the given taskID doesn't exist yet
	for _, dt := range s.deleteTasks {
		if dt.TaskID == taskID {
			return fmt.Errorf("the delete task with task_id=%q is already registered", taskID)
		}
	}

	// Register the task and persist it to the file.
	s.deleteTasks = append(s.deleteTasks, dt)
	s.mustSaveDeleteTasksLocked()

	return nil
}

// mustSaveDeleteTasksLocked saves s.deleteTasks to file
//
// The s.deleteTaskLock must be locked while calling this function.
func (s *Storage) mustSaveDeleteTasksLocked() {
	deleteTasksPath := filepath.Join(s.path, deleteTasksFilename)
	mustWriteDeleteTasksToFile(deleteTasksPath, s.deleteTasks)
}

// DeleteStopTask stops the delete task with the given taskID.
//
// It waits until the task is stopped before returning.
// If there is no a task with the given taskID, then the function returns immediately.
func (s *Storage) DeleteStopTask(ctx context.Context, taskID string) error {
	var doneCh <-chan struct{}

	s.deleteTasksLock.Lock()

	for i, dt := range s.deleteTasks {
		if dt.TaskID != taskID {
			continue
		}

		if dt.cancel != nil {
			// Cancel the currently executed task. The task executor will remove this task from s.deleteTasks
			dt.cancel()
			doneCh = dt.doneCh
		} else {
			// The task is waiting to be executed. Drop it.
			s.deleteTasks = append(s.deleteTasks[:i], s.deleteTasks[i+1:]...)
			s.mustSaveDeleteTasksLocked()
		}
		break
	}

	s.deleteTasksLock.Unlock()

	if doneCh == nil {
		return nil
	}

	// Wait until the task is canceled.
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DeleteActiveTasks returns currently running active delete tasks, which were started via DeleteRunTask().
func (s *Storage) DeleteActiveTasks(_ context.Context) ([]*DeleteTask, error) {
	s.deleteTasksLock.Lock()
	dts := append([]*DeleteTask{}, s.deleteTasks...)
	s.deleteTasksLock.Unlock()

	return dts, nil
}

// EnableLogNewStreams enables logging newly ingested streams during the given number of seconds
func (s *Storage) EnableLogNewStreams(seconds int) {
	if seconds <= 0 {
		// Do nothing.
		return
	}

	vPrev := s.logNewStreams.Swap(true)
	if vPrev {
		logger.Infof("logging of new streams is already enabled")
		return
	}

	logger.Infof("enabled logging of new streams for %d seconds", seconds)

	d := time.Second * time.Duration(seconds)
	time.AfterFunc(d, func() {
		s.logNewStreams.Swap(false)
		logger.Infof("disabled logging of new streams")
	})
}

type partitionWrapper struct {
	// refCount is the number of active references to partition.
	// When it reaches zero, then the partition is closed.
	refCount atomic.Int32

	// mustDrop is set when the partition must be deleted after refCount reaches zero.
	mustDrop atomic.Bool

	// day is the day for the partition in the unix timestamp divided by the number of seconds in the day.
	day int64

	// pt is the wrapped partition.
	pt *partition

	// doneCh is closed when refCount reaches zero, e.g. when the partitionWrapper is no longer accessed.
	doneCh chan struct{}
}

func newPartitionWrapper(pt *partition, day int64) *partitionWrapper {
	pw := &partitionWrapper{
		day:    day,
		pt:     pt,
		doneCh: make(chan struct{}),
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

	// signal that the ptw is no longer accessed.
	close(ptw.doneCh)
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

	fs.MustSyncPathAndParentDir(path)
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

	maxBackfillAge := cfg.MaxBackfillAge
	if maxBackfillAge <= 0 || maxBackfillAge > retention {
		maxBackfillAge = retention
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
	streamIDCache := newCache()
	filterStreamCache := newCache()

	// Load delete tasks which may be left since the previous restart
	deleteTasksPath := filepath.Join(path, deleteTasksFilename)
	deleteTasks := mustReadDeleteTasksFromFile(deleteTasksPath)

	s := &Storage{
		path:                   path,
		retention:              retention,
		defaultParallelReaders: cfg.DefaultParallelReaders,
		maxDiskSpaceUsageBytes: cfg.MaxDiskSpaceUsageBytes,
		maxDiskUsagePercent:    cfg.MaxDiskUsagePercent,
		flushInterval:          flushInterval,
		futureRetention:        futureRetention,
		maxBackfillAge:         maxBackfillAge,
		minFreeDiskSpaceBytes:  minFreeDiskSpaceBytes,
		logIngestedRows:        cfg.LogIngestedRows,
		flockF:                 flockF,
		stopCh:                 make(chan struct{}),

		streamIDCache:     streamIDCache,
		filterStreamCache: filterStreamCache,

		deleteTasks: deleteTasks,
	}
	s.logNewStreams.Store(cfg.LogNewStreams)

	partitionsPath := filepath.Join(path, partitionsDirname)
	fs.MustMkdirIfNotExist(partitionsPath)
	fs.MustSyncPath(path)

	des := fs.MustReadDir(partitionsPath)
	ptws := make([]*partitionWrapper, len(des))

	// Open partitions in parallel. This should improve VictoriaLogs initialization duration
	// when it opens many partitions.
	var wg sync.WaitGroup
	concurrencyLimiterCh := make(chan struct{}, cgroup.AvailableCPUs())
	for i, de := range des {
		fname := de.Name()

		partitionDir := filepath.Join(partitionsPath, fname)
		if fs.IsPartiallyRemovedDir(partitionDir) {
			// Drop partially removed partition directory. This may happen when unclean shutdown happens during partition deletion.
			fs.MustRemoveDir(partitionDir)
			continue
		}

		wg.Add(1)
		concurrencyLimiterCh <- struct{}{}
		go func(idx int) {
			defer func() {
				<-concurrencyLimiterCh
				wg.Done()
			}()

			day, err := getPartitionDayFromName(fname)
			if err != nil {
				logger.Panicf("FATAL: cannot parse partition filename %q at %q: %s", fname, partitionsPath, err)
			}

			partitionPath := filepath.Join(partitionsPath, fname)
			pt := mustOpenPartition(s, partitionPath)
			ptws[idx] = newPartitionWrapper(pt, day)
		}(i)
	}
	wg.Wait()

	sortPartitions(ptws)

	// Delete partitions from the future if needed
	now := time.Now().UnixNano()
	maxAllowedDay := s.getMaxAllowedDay(now)
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
	s.runDeleteTasksWatcher()
	return s
}

func sortPartitions(ptws []*partitionWrapper) {
	sort.Slice(ptws, func(i, j int) bool {
		return ptws[i].day < ptws[j].day
	})
}

func (s *Storage) runRetentionWatcher() {
	s.wg.Add(1)
	go func() {
		s.watchRetention()
		s.wg.Done()
	}()
}

func (s *Storage) runMaxDiskSpaceUsageWatcher() {
	if s.maxDiskSpaceUsageBytes <= 0 && s.maxDiskUsagePercent <= 0 {
		return // nothing to watch
	}
	s.wg.Add(1)
	go func() {
		s.watchMaxDiskSpaceUsage()
		s.wg.Done()
	}()
}

func (s *Storage) runDeleteTasksWatcher() {
	s.wg.Add(1)
	go func() {
		s.watchDeleteTasks()
		s.wg.Done()
	}()
}

func (s *Storage) watchRetention() {
	d := timeutil.AddJitterToDuration(time.Hour)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		var ptwsToDelete []*partitionWrapper
		now := time.Now().UnixNano()
		minAllowedDay := s.getMinAllowedDay(now)

		s.partitionsLock.Lock()

		// Delete outdated partitions.
		// s.partitions are sorted by day, so the partitions, which can become outdated, are located at the beginning of the list
		ptws := s.partitions
		for i, ptw := range ptws {
			if ptw.day < minAllowedDay {
				continue
			}

			// ptws are sorted by time, so just drop all the partitions until i.
			ptwsToDelete = ptws[:i]
			s.partitions = ptws[i:]
			s.updateDeletedPartitionsLocked(ptwsToDelete)

			// Remove reference to deleted partitions from s.ptwHot
			if slices.Contains(ptwsToDelete, s.ptwHot) {
				s.ptwHot = nil
			}

			break
		}

		s.partitionsLock.Unlock()

		for i, ptw := range ptwsToDelete {
			logger.Infof("the partition %s is scheduled to be deleted because it is outside the -retentionPeriod=%dd", ptw.pt.path, durationToDays(s.retention))
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

func (s *Storage) watchMaxDiskSpaceUsage() {
	d := timeutil.AddJitterToDuration(10 * time.Second)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		// Determine dynamic limit in bytes
		var limitBytes uint64
		if s.maxDiskSpaceUsageBytes > 0 {
			limitBytes = uint64(s.maxDiskSpaceUsageBytes)
		} else if s.maxDiskUsagePercent > 0 {
			total := fs.MustGetTotalSpace(s.path)
			if total > 0 {
				limitBytes = (total * uint64(s.maxDiskUsagePercent)) / 100
			}
		}
		if limitBytes == 0 {
			// Nothing to enforce
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				continue
			}
		}

		s.partitionsLock.Lock()
		var n uint64
		ptws := s.partitions
		var ptwsToDelete []*partitionWrapper
		for i := len(ptws) - 1; i >= 0; i-- {
			ptw := ptws[i]
			var ps PartitionStats
			ptw.pt.updateStats(&ps)
			n += ps.IndexdbSizeBytes + ps.CompressedSmallPartSize + ps.CompressedBigPartSize
			if n <= limitBytes {
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
			s.updateDeletedPartitionsLocked(ptwsToDelete)

			// Remove reference to deleted partitions from s.ptwHot
			if slices.Contains(ptwsToDelete, s.ptwHot) {
				s.ptwHot = nil
			}

			break
		}

		s.partitionsLock.Unlock()

		for i, ptw := range ptwsToDelete {
			var reason string
			if s.maxDiskSpaceUsageBytes > 0 {
				reason = fmt.Sprintf("-retention.maxDiskSpaceUsageBytes=%d", s.maxDiskSpaceUsageBytes)
			} else {
				reason = fmt.Sprintf("-retention.maxDiskUsagePercent=%d%%", s.maxDiskUsagePercent)
			}
			logger.Infof("the partition %s is scheduled to be deleted because the total size of partitions exceeds %s", ptw.pt.path, reason)
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

func (s *Storage) watchDeleteTasks() {
	d := timeutil.AddJitterToDuration(time.Second)
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}

		var dt *DeleteTask

		s.deleteTasksLock.Lock()
		if len(s.deleteTasks) > 0 {
			dt = s.deleteTasks[0]

			// initialize dt.ctx and dt.cancel under the lock in order to avoid races
			// with canceling the task at Storage.DeleteStopTask()
			dt.ctx, dt.cancel = contextutil.NewStopChanContext(s.stopCh)
			dt.doneCh = make(chan struct{})
		}
		s.deleteTasksLock.Unlock()

		if dt == nil {
			// There are no delete tasks.
			continue
		}

		// Process delete tasks sequentially in order to limit resource usage needed for the logs' deletion.

		ok := s.processDeleteTask(dt.ctx, dt)
		close(dt.doneCh)
		dt.cancel()

		s.deleteTasksLock.Lock()

		// Set dt.ctx and dt.cancel to nil under the lock in order to avoid races
		// with canceling the task at Storage.DeleteStopTask().
		dt.ctx = nil
		dt.cancel = nil
		dt.doneCh = nil

		s.deleteTasks = s.deleteTasks[1:]
		if !ok {
			// The delete task coudn't be completed now. Try it later.
			s.deleteTasks = append(s.deleteTasks, dt)
		}
		s.mustSaveDeleteTasksLocked()

		s.deleteTasksLock.Unlock()
	}
}

// processDeleteTask processes dt.
//
// true is returned on successfully processed dt or on explicitly canceled dt.
// false is returned if dt couldn't be processed at the moment, so it must be processed later.
func (s *Storage) processDeleteTask(ctx context.Context, dt *DeleteTask) bool {
	logger.Infof("started processing delete task %s", dt)
	startTime := time.Now()

	f, err := ParseFilter(dt.Filter)
	if err != nil {
		logger.Panicf("BUG: cannot parse filter from delete task: [%s]", dt.Filter)
	}

	q := &Query{
		f:         f.f,
		timestamp: dt.StartTime.UnixNano(),
	}

	// Add time filter ending at the delete task start time.
	// This avoids deleting logs from the future.
	start := int64(math.MinInt64)
	end := dt.StartTime.UnixNano()
	q.AddTimeFilter(start, end)

	var qs QueryStats
	qctx := NewQueryContext(ctx, &qs, dt.TenantIDs, q, false, nil)

	// Initialize subqueries
	qNew, err := initSubqueries(qctx, s.runQuery, true)
	if err != nil {
		logger.Errorf("cannot process delete task with task_id=%q while initializing subqueries: %s; retrying later", dt.TaskID, err)
		return false
	}
	q = qNew

	sso := s.getSearchOptions(dt.TenantIDs, q, qctx.HiddenFieldsFilters)

	// reset fieldsFilter in order to avoid loading all the log fields
	// during search for parts which contain rows to delete, since these fields aren't needed.
	sso.fieldsFilter.Reset()

	// delete rows matching q.f
	stopCh := ctx.Done()
	if !s.deleteRows(sso, stopCh) {
		if needStop(s.stopCh) {
			logger.Infof("the storage is stopped while executing the delete task with task_id=%q; postponing the task for later execution", dt.TaskID)
			return false
		}

		if needStop(stopCh) {
			// The task has been canceled explicitly. Return true, so it isn't re-scheduled for later execution.
			logger.Infof("the delete task with task_id=%q is explicitly canceled after %.3f seconds", dt.TaskID, time.Since(startTime).Seconds())
			return true
		}

		// The task couldn't be processed at the moment
		logger.Warnf("cannot proceeed with the delete task with task_id=%q in %.3f seconds; retrying it later", dt.TaskID, time.Since(startTime).Seconds())
		return false
	}

	logger.Infof("finished processing delete task %s in %.3f seconds", dt, time.Since(startTime).Seconds())
	return true
}

func (s *Storage) deleteRows(sso *storageSearchOptions, stopCh <-chan struct{}) bool {
	ptws, ptwsDecRef := s.getPartitionsForTimeRange(sso.minTimestamp, sso.maxTimestamp)
	defer ptwsDecRef()

	// Delete rows sequentially in every partition in order to limit resource usage needed for the logs' deletion.
	ok := true
	for _, ptw := range ptws {
		if !ptw.pt.deleteRows(sso, stopCh) {
			// Return false if at least a single deletion was unsuccessful.
			// Continue deletion of rows at other partitions, since they may be successful.
			ok = false
		}
	}

	return ok
}

func (s *Storage) updateDeletedPartitionsLocked(ptwsToDelete []*partitionWrapper) {
	for _, ptw := range ptwsToDelete {
		if !slices.Contains(s.deletedPartitions, ptw.day) {
			s.deletedPartitions = append(s.deletedPartitions, ptw.day)
		}
	}
}

func (s *Storage) getMinAllowedDay(now int64) int64 {
	return (now - s.retention.Nanoseconds()) / nsecsPerDay
}

func (s *Storage) getMaxAllowedDay(now int64) int64 {
	return (now + s.futureRetention.Nanoseconds()) / nsecsPerDay
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

	// Stop caches

	// Do not persist caches, since they may become out of sync with partitions
	// if partitions are deleted, restored from backups or copied from other sources
	// between VictoriaLogs restarts. This may result in various issues
	// during data ingestion and querying.

	s.streamIDCache.MustStop()
	s.streamIDCache = nil

	s.filterStreamCache.MustStop()
	s.filterStreamCache = nil

	// release lock file
	fs.MustClose(s.flockF)
	s.flockF = nil

	s.path = ""
}

// MustForceMerge force-merges parts in s partitions with names starting from the given partitionNamePrefix.
//
// Partitions are merged sequentially in order to reduce load on the system.
func (s *Storage) MustForceMerge(partitionNamePrefix string) {
	var ptws []*partitionWrapper

	s.partitionsLock.Lock()
	for _, ptw := range s.partitions {
		if strings.HasPrefix(ptw.pt.name, partitionNamePrefix) {
			ptw.incRef()
			ptws = append(ptws, ptw)
		}
	}
	s.partitionsLock.Unlock()

	s.wg.Add(1)
	defer s.wg.Done()

	for _, ptw := range ptws {
		logger.Infof("started force merge for partition %s", ptw.pt.name)
		startTime := time.Now()
		ptw.pt.mustForceMerge()
		ptw.decRef()
		logger.Infof("finished force merge for partition %s in %.3fs", ptw.pt.name, time.Since(startTime).Seconds())
	}
}

// MustAddRows adds lr to s.
//
// It is recommended checking whether the s is in read-only mode by calling IsReadOnly()
// before calling MustAddRows.
//
// The added rows become visible for search after small duration of time.
// Call DebugFlush if the added rows must be queried immediately (for example, in tests).
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
	now := time.Now().UnixNano()
	minAllowedDay := s.getMinAllowedDay(now)
	maxAllowedDay := s.getMaxAllowedDay(now)
	minAllowedTimestamp := now - s.maxBackfillAge.Nanoseconds()

	m := make(map[int64]*LogRows)
	for i, ts := range lr.timestamps {
		day := ts / nsecsPerDay
		if day < minAllowedDay {
			line := MarshalFieldsToJSON(nil, lr.rows[i])
			tsf := TimeFormatter(ts)
			minAllowedTsf := TimeFormatter(minAllowedDay * nsecsPerDay)
			tooSmallTimestampLogger.Warnf("skipping log entry with too small timestamp=%s; it must be bigger than %s according "+
				"to the configured -retentionPeriod=%dd. See https://docs.victoriametrics.com/victorialogs/#retention ; "+
				"log entry: %s", &tsf, &minAllowedTsf, durationToDays(s.retention), line)
			s.rowsDroppedTooSmallTimestamp.Add(1)
			continue
		}
		if day > maxAllowedDay {
			line := MarshalFieldsToJSON(nil, lr.rows[i])
			tsf := TimeFormatter(ts)
			maxAllowedTsf := TimeFormatter(maxAllowedDay * nsecsPerDay)
			tooBigTimestampLogger.Warnf("skipping log entry with too big timestamp=%s; it must be smaller than %s according "+
				"to the configured -futureRetention=%dd; see https://docs.victoriametrics.com/victorialogs/#retention ; "+
				"log entry: %s", &tsf, &maxAllowedTsf, durationToDays(s.futureRetention), line)
			s.rowsDroppedTooBigTimestamp.Add(1)
			continue
		}
		if ts < minAllowedTimestamp {
			line := MarshalFieldsToJSON(nil, lr.rows[i])
			tsf := TimeFormatter(ts)
			minAllowedTsf := TimeFormatter(minAllowedTimestamp)
			tooSmallTimestampLogger.Warnf("skipping log entry with too small timestamp=%s; it must be bigger than %s according "+
				"to the configured -maxBackfillAge=%s. See https://docs.victoriametrics.com/victorialogs/#backfilling ; "+
				"log entry: %s", &tsf, &minAllowedTsf, s.maxBackfillAge, line)
			s.rowsDroppedTooSmallTimestamp.Add(1)
			continue
		}

		lrPart := m[day]
		if lrPart == nil {
			lrPart = GetLogRows(nil, nil, nil, nil, "")
			m[day] = lrPart
		}
		lrPart.mustAddInternal(lr.streamIDs[i], ts, lr.rows[i], lr.streamTagsCanonicals[i])
	}
	for day, lrPart := range m {
		ptw := s.getPartitionForWriting(day)
		if ptw != nil {
			ptw.pt.mustAddRows(lrPart)
			ptw.decRef()
		} else {
			// the lrPart must contain at least a single row, so log it.
			line := MarshalFieldsToJSON(nil, lrPart.rows[0])
			inactivePartitionLogger.Warnf("skipping log entry because it cannot be saved into inactive per-day partition; "+
				"see https://docs.victoriametrics.com/victorialogs/#partitions-lifecycle; log entry %s", line)
		}
		PutLogRows(lrPart)
	}
}

var tooSmallTimestampLogger = logger.WithThrottler("too_small_timestamp", 5*time.Second)
var tooBigTimestampLogger = logger.WithThrottler("too_big_timestamp", 5*time.Second)
var inactivePartitionLogger = logger.WithThrottler("inactive_partition", 5*time.Second)

// TimeFormatter implements fmt.Stringer for timestamp in nanoseconds
type TimeFormatter int64

// String returns human-readable representation for tf.
func (tf *TimeFormatter) String() string {
	ts := int64(*tf)
	t := time.Unix(0, ts).UTC()
	return t.Format(time.RFC3339Nano)
}

// getPartitionForWriting returns the partition for the given day for writing.
//
// The partition is automatically created if it didn't exist.
//
// nil is returned in the following cases:
//
//   - When the partition is outside the configured retention.
//   - When the partition has been detached via Storage.PartitionDetach().
//   - When the partition directory has been manually added, but wasn't attached yet via Storage.PartitionAttach().
//
// The caller must log this case and drop pending logs for this partition.
func (s *Storage) getPartitionForWriting(day int64) *partitionWrapper {
	s.partitionsLock.Lock()
	defer s.partitionsLock.Unlock()

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
		// Missing partition for the given day.
		if slices.Contains(s.deletedPartitions, day) {
			// The partition has been already deleted.
			return nil
		}

		fname := getPartitionNameFromDay(day)
		partitionPath := filepath.Join(s.path, partitionsDirname, fname)
		if fs.IsPathExist(partitionPath) {
			// The partition directory exists. This can happen in the following cases:
			// - When the partition directory has been manually added, but it wasn't attached yet via Storage.PartitionAttach().
			// - When the partition has been detached via Storage.PartitionDetach().
			return nil
		}

		// Create missing partition.
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

	return ptw
}

// UpdateStats updates ss for the given s.
func (s *Storage) UpdateStats(ss *StorageStats) {
	ss.RowsDroppedTooBigTimestamp += s.rowsDroppedTooBigTimestamp.Load()
	ss.RowsDroppedTooSmallTimestamp += s.rowsDroppedTooSmallTimestamp.Load()
	if s.maxDiskSpaceUsageBytes > 0 {
		ss.MaxDiskSpaceUsageBytes = s.maxDiskSpaceUsageBytes
	} else {
		ss.MaxDiskSpaceUsageBytes = int64(fs.MustGetTotalSpace(s.path) * uint64(s.maxDiskUsagePercent) / 100)
	}
	// Use sentinel values to indicate unbounded / no data for consistency
	ss.MinTimestamp, ss.MaxTimestamp = math.MinInt64, math.MaxInt64

	s.partitionsLock.Lock()
	ss.PartitionsCount += uint64(len(s.partitions))
	for _, ptw := range s.partitions {
		ptw.pt.updateStats(&ss.PartitionStats)
	}

	if len(s.partitions) > 0 {
		p0 := s.partitions[0]
		pLast := s.partitions[len(s.partitions)-1]

		ss.MinTimestamp, _ = p0.pt.ddb.getMinMaxTimestamps()
		_, ss.MaxTimestamp = pLast.pt.ddb.getMinMaxTimestamps()
	}
	s.partitionsLock.Unlock()

	ss.IsReadOnly = s.IsReadOnly()
}

// IsReadOnly returns true if s is in read-only mode.
func (s *Storage) IsReadOnly() bool {
	available := fs.MustGetFreeSpace(s.path)
	return available < s.minFreeDiskSpaceBytes
}

// DebugFlush flushes all the buffered rows, so they become visible for search.
//
// This function is for debugging and testing purposes only, since it is slow.
func (s *Storage) DebugFlush() {
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
