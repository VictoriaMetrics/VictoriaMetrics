package logstorage

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// startAsyncTaskWorker launches a background goroutine, which periodically
// scans partitions for parts lagging behind async tasks and applies these
// tasks by re-executing their underlying queries via MarkRows (with
// createTask=false).
func (s *Storage) startAsyncTaskWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx := context.Background()

		logger.Infof("DEBUG: start async task worker")

		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		const maxFailedTime = 3
		var failedTime int
		for {
			select {
			case <-s.stopCh:
				// Drain the timer channel if needed before returning
				if !timer.Stop() {
					<-timer.C
				}
				return
			case <-timer.C:
				var seq uint64
				err := s.runAsyncTasksOnce(ctx, &seq)
				if err != nil {
					logger.Errorf("async task worker: %s", err)
					if failedTime++; failedTime > maxFailedTime {
						failedTime = 0
					}
				} else {
					failedTime = 0
				}

				if failedTime == 0 {
					s.advanceAsyncTask(seq, err)
				}

				if seq != 0 {
					logger.Infof("DEBUG: done task (seq=%d, err=%v)", seq, err)
				}

				// Start the 5-second wait *after* the task completes
				timer.Reset(5 * time.Second)
			}
		}
	}()
}

// runAsyncTasksOnce performs a single pass over all partitions (latest → oldest)
// and applies pending async tasks to every part that hasn't caught up yet.
func (s *Storage) runAsyncTasksOnce(ctx context.Context, seq *uint64) error {
	*seq = 0

	// Snapshot partitions (most recent first).
	s.partitionsLock.Lock()
	ptws := append([]*partitionWrapper{}, s.partitions...)
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	task := s.findNextAsyncTask(ptws)
	if task.Type == asyncTaskNone {
		return nil
	}
	logger.Infof("DEBUG: found task seq=%d", task.Seq)
	*seq = task.Seq

	// Gather all lagging parts in the target partition for this sequence.
	var lagging []*partWrapper
	for _, ptw := range ptws {
		pt := ptw.pt
		pt.ddb.partsLock.Lock()
		allPws := [][]*partWrapper{pt.ddb.inmemoryParts, pt.ddb.smallParts, pt.ddb.bigParts}
		for _, arr := range allPws {
			for _, pw := range arr {
				if pw.isInMerge || pw.mustDrop.Load() {
					continue
				}
				if pw.p.appliedTSeq.Load() < task.Seq {
					pw.incRef()
					lagging = append(lagging, pw)
				}
			}
		}
		pt.ddb.partsLock.Unlock()
	}

	if len(lagging) == 0 {
		for _, ptw := range ptws {
			ptw.decRef()
		}
		return nil
	}

	if task.Type == asyncTaskDelete {
		logger.Infof("DEBUG: delete-task (seq=%d) on %d lagging parts", task.Seq, len(lagging))
		err := s.runDeleteTask(ctx, task, lagging)
		if err != nil {
			return err
		}
	}

	// Update parts as caught up.
	for _, pw := range lagging {
		pw.p.setAppliedTSeq(task.Seq)
		pw.decRef()
	}

	logger.Infof("DEBUG: task (seq=%d, query=%q) applied to %d parts", task.Seq, task.Query, len(lagging))

	// Release partition refs
	for _, ptw := range ptws {
		ptw.decRef()
	}

	return nil
}

func (s *Storage) advanceAsyncTask(sequence uint64, err error) error {
	if sequence == 0 {
		return nil // nothing to advance
	}

	// Determine resulting status for the task.
	newStatus := taskSuccess
	if err != nil {
		newStatus = taskError
	}

	// Take a snapshot of partitions to iterate safely without holding the lock for long.
	s.partitionsLock.Lock()
	ptws := append([]*partitionWrapper{}, s.partitions...)
	for _, ptw := range ptws {
		ptw.incRef()
	}
	s.partitionsLock.Unlock()

	for _, ptw := range ptws {
		pt := ptw.pt

		// 1) Ensure every part in this partition has appliedTSeq at least `sequence`.
		pt.ddb.partsLock.Lock()
		all := [][]*partWrapper{pt.ddb.inmemoryParts, pt.ddb.smallParts, pt.ddb.bigParts}
		for _, arr := range all {
			for _, pw := range arr {
				if pw.p.appliedTSeq.Load() < sequence {
					pw.p.setAppliedTSeq(sequence)
				}
			}
		}
		pt.ddb.partsLock.Unlock()

		// 2) Update task status in this partition, if present and still pending.
		updated := false
		if pt.asyncTasksLen.Load() >= sequence {
			pt.asyncTasksLock.Lock()
			for i := range pt.asyncTasks {
				if pt.asyncTasks[i].Seq == sequence {
					if pt.asyncTasks[i].Status == taskPending {
						pt.asyncTasks[i].Status = newStatus
						updated = true
					}
					break
				}
			}
			pt.asyncTasksLock.Unlock()

			if updated {
				pt.mustSaveAsyncTasks()
			}
		}
	}

	for _, ptw := range ptws {
		ptw.decRef()
	}

	return nil
}

func (s *Storage) runDeleteTask(ctx context.Context, task asyncTask, lagging []*partWrapper) error {
	// Build allowed set
	allowed := make(map[*partition][]*partWrapper, len(lagging))
	for _, pw := range lagging {
		allowed[pw.p.pt] = append(allowed[pw.p.pt], pw)
	}

	err := s.markDeleteRowsOnParts(ctx, task.TenantIDs, task.Query, task.Seq, allowed)
	if err != nil {
		return fmt.Errorf("failed to mark delete rows on parts: %w", err)
	}

	return err
}

func (s *Storage) findNextAsyncTask(ptws []*partitionWrapper) asyncTask {
	var minSeq uint64 = math.MaxUint64
	var result asyncTask

	for _, ptw := range ptws {
		pt := ptw.pt

		task := pt.getOldestPendingAsyncTask()
		// Skip partitions without pending tasks.
		if task.Type == asyncTaskNone {
			continue
		}
		// Select the task with the smallest global sequence.
		if task.Seq < minSeq {
			result = task
			minSeq = task.Seq
		}
	}

	return result
}
