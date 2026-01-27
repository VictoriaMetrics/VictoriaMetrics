package logstorage

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// DeleteTask describes a task for logs' deletion.
type DeleteTask struct {
	// TaskID is the id of the task
	TaskID string `json:"task_id"`

	// TenantIDs are tenant ids for the task
	TenantIDs []TenantID `json:"tenant_ids"`

	// Filter is the filter used for logs' deletion; Logs matching the given filter are deleted
	Filter string `json:"filter"`

	// StartTime is the time when the task has been created
	StartTime time.Time `json:"start_time"`

	// ctx is set to non-nil during task execution. Pending tasks have nil ctx.
	ctx context.Context

	// cancel is set to non-nil during task execution. It is used for canceling the delete task.
	cancel func()

	// doneCh is used for waiting until the delete task is complete.
	doneCh chan struct{}
}

// String returns string representation for the dt
func (dt *DeleteTask) String() string {
	data, err := json.Marshal(dt)
	if err != nil {
		logger.Panicf("BUG: cannot marshal DeleteTask: %s", err)
	}
	return string(data)
}

func newDeleteTask(taskID string, tenantIDs []TenantID, filter string, startTime int64) *DeleteTask {
	return &DeleteTask{
		TaskID:    taskID,
		TenantIDs: tenantIDs,
		Filter:    filter,
		StartTime: time.Unix(0, startTime).UTC(),
	}
}

// MarshalDeleteTasksToJSON marshals tasks into a JSON array and returns the result
func MarshalDeleteTasksToJSON(tasks []*DeleteTask) []byte {
	data, err := json.Marshal(tasks)
	if err != nil {
		logger.Panicf("BUG: cannot marshal tasks: %s", err)
	}
	return data
}

// UnmarshalDeleteTasksFromJSON unmarshals DeleteTask slice from JSON array at data
func UnmarshalDeleteTasksFromJSON(data []byte) ([]*DeleteTask, error) {
	var tasks []*DeleteTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func mustReadDeleteTasksFromFile(path string) []*DeleteTask {
	if !fs.IsPathExist(path) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		logger.Panicf("FATAL: cannot read %s: %s", path, err)
	}
	dts, err := UnmarshalDeleteTasksFromJSON(data)
	if err != nil {
		logger.Panicf("FATAL: cannot parse delete tasks from %s: %s", path, err)
	}
	return dts
}

func mustWriteDeleteTasksToFile(path string, dts []*DeleteTask) {
	data := MarshalDeleteTasksToJSON(dts)
	fs.MustWriteAtomic(path, data, true)
}
