package logstorage

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// asyncTaskType identifies the type of background (asynchronous) task attached to a partition.
// More types can be added in the future (e.g. compaction, ttl, schema-changes).
type asyncTaskType int

const (
	asyncTaskNone   asyncTaskType = iota // no task
	asyncTaskDelete                      // delete rows matching a query
)

// asyncTask describes a durable task that must be eventually applied to every part in the partition.
// For now only delete tasks are supported.

// Status field tracks the outcome of the task.
//
//	0 - pending (not attempted or still running)
//	1 - success (completed without errors)
//	2 - error   (attempted but failed; worker will skip the task but it can be inspected)
type asyncTaskStatus int

const (
	taskPending asyncTaskStatus = iota
	taskSuccess
	taskError
)

type asyncTask struct {
	Type      asyncTaskType `json:"type"`
	TenantIDs []TenantID    `json:"tenantIDs,omitempty"` // affected tenants (empty slice = all)
	Query     string        `json:"query,omitempty"`     // serialized LogSQL query
	Seq       uint64        `json:"seq,omitempty"`       // monotonically increasing *global* sequence

	// Status tracks the last execution state; omitted from JSON when zero (pending) to
	// preserve compatibility with tasks created before this field existed.
	Status asyncTaskStatus `json:"status,omitempty"`
}

// globalTaskSeq provides unique, monotonically increasing sequence numbers for async tasks.
var globalTaskSeq atomic.Uint64

func init() {
	// Initialise with current unix-nano in order to minimise collision with seqs that may be present on disk.
	globalTaskSeq.Store(uint64(time.Now().UnixNano()))
}

// marshalAsyncTasks converts async tasks to JSON for persistence
func marshalAsyncTasks(tasks []asyncTask) []byte {
	data, err := json.Marshal(tasks)
	if err != nil {
		logger.Panicf("FATAL: cannot marshal async tasks: %s", err)
	}
	return data
}

// unmarshalAsyncTasks converts JSON data back to async tasks
func unmarshalAsyncTasks(data []byte) []asyncTask {
	if len(data) == 0 {
		return nil
	}

	var tasks []asyncTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		logger.Panicf("FATAL: cannot unmarshal async tasks: %s", err)
	}
	return tasks
}
