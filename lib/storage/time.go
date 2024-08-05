package storage

import (
	"fmt"
	"time"
)

// dateToString returns human readable representation of the date.
func dateToString(date uint64) string {
	if date == 0 {
		return "[entire retention period]"
	}
	t := time.Unix(int64(date*24*3600), 0).UTC()
	return t.Format("2006-01-02")
}

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

// Zero time range and zero date are used to force global index search.
var (
	globalIndexTimeRange = TimeRange{}
	globalIndexDate      = uint64(0)
)

// DateRange returns the date range for the given time range.
func (tr *TimeRange) DateRange() (uint64, uint64) {
	minDate := uint64(tr.MinTimestamp) / msecPerDay
	// Max timestamp may point to the first millisecond of the next day. As the
	// result, the returned date range will cover one more day than needed.
	// Decrementing by 1 removes this extra day.
	maxDate := uint64(tr.MaxTimestamp-1) / msecPerDay

	// However, if both timestamps are the same and point to the beginning of
	// the day, then maxDate will be smaller that the minDate. In this case
	// maxDate is set to minDate.
	if maxDate < minDate {
		maxDate = minDate
	}

	return minDate, maxDate
}

func (tr *TimeRange) String() string {
	if *tr == globalIndexTimeRange {
		return "[entire retention period]"
	}
	start := TimestampToHumanReadableFormat(tr.MinTimestamp)
	end := TimestampToHumanReadableFormat(tr.MaxTimestamp)
	return fmt.Sprintf("[%s..%s]", start, end)
}

// TimestampToHumanReadableFormat converts the given timestamp to human-readable format.
func TimestampToHumanReadableFormat(timestamp int64) string {
	t := timestampToTime(timestamp).UTC()
	return t.Format("2006-01-02T15:04:05.999Z")
}

// timestampToPartitionName returns partition name for the given timestamp.
func timestampToPartitionName(timestamp int64) string {
	t := timestampToTime(timestamp)
	return t.Format("2006_01")
}

// fromPartitionName initializes tr from the given partition name.
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
