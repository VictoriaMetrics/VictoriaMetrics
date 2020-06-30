package storage

import (
	"fmt"
	"time"
)

// timestampToTime returns time representation of the given timestamp.
//
// The returned time is in UTC timezone.
func timestampToTime(timestamp int64) time.Time {
	return time.Unix(timestamp/1e3, (timestamp%1e3)*1e6).UTC()
}

// timestampFromTime returns timestamp value for the given time.
func timestampFromTime(t time.Time) int64 {
	// There is no need in converting t to UTC, since UnixNano must
	// return the same value for any timezone.
	return t.UnixNano() / 1e6
}

// TimeRange is time range.
type TimeRange struct {
	MinTimestamp int64
	MaxTimestamp int64
}

func (tr *TimeRange) String() string {
	minTime := timestampToTime(tr.MinTimestamp)
	maxTime := timestampToTime(tr.MaxTimestamp)
	return fmt.Sprintf("[%s - %s]", minTime, maxTime)
}

// timestampToPartitionName returns partition name for the given timestamp.
func timestampToPartitionName(timestamp int64) string {
	t := timestampToTime(timestamp)
	return t.Format("2006_01")
}

// fromPartitionName initializes tr from the given parition name.
func (tr *TimeRange) fromPartitionName(name string) error {
	t, err := time.Parse("2006_01", name)
	if err != nil {
		return fmt.Errorf("cannot parse partition name %q: %w", name, err)
	}
	tr.fromPartitionTime(t)
	return nil
}

// fromPartitionTimestamp initializes tr from the given partition timestamp.
func (tr *TimeRange) fromPartitionTimestamp(timestamp int64) {
	t := timestampToTime(timestamp)
	tr.fromPartitionTime(t)
}

// fromPartitionTime initializes tr from the given partition time t.
func (tr *TimeRange) fromPartitionTime(t time.Time) {
	y, m, _ := t.UTC().Date()
	minTime := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC)
	tr.MinTimestamp = minTime.Unix() * 1e3
	tr.MaxTimestamp = maxTime.Unix()*1e3 - 1
}

const msecPerDay = 24 * 3600 * 1000

const msecPerHour = 3600 * 1000
