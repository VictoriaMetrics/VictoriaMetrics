package searchutils

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metricsql"
)

var (
	maxExportDuration        = flag.Duration("search.maxExportDuration", time.Hour*24*30, "The maximum duration for /api/v1/export call")
	maxQueryDuration         = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum duration for query execution")
	maxStatusRequestDuration = flag.Duration("search.maxStatusRequestDuration", time.Minute*5, "The maximum duration for /api/v1/status/* requests")
	denyPartialResponse      = flag.Bool("search.denyPartialResponse", false, "Whether to deny partial responses if a part of -storageNode instances fail to perform queries; "+
		"this trades availability over consistency; see also -search.maxQueryDuration")
)

func roundToSeconds(ms int64) int64 {
	return ms - ms%1000
}

// GetTime returns time from the given argKey query arg.
//
// If argKey is missing in r, then defaultMs rounded to seconds is returned.
// The rounding is needed in order to align query results in Grafana
// executed at different times. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/720
func GetTime(r *http.Request, argKey string, defaultMs int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return roundToSeconds(defaultMs), nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		t, err := time.Parse(time.RFC3339, argValue)
		if err != nil {
			// Handle Prometheus'-provided minTime and maxTime.
			// See https://github.com/prometheus/client_golang/issues/614
			switch argValue {
			case prometheusMinTimeFormatted:
				return minTimeMsecs, nil
			case prometheusMaxTimeFormatted:
				return maxTimeMsecs, nil
			}
			// Try parsing duration relative to the current time
			d, err1 := metricsql.DurationValue(argValue, 0)
			if err1 != nil {
				return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
			}
			if d > 0 {
				d = -d
			}
			t = time.Now().Add(time.Duration(d) * time.Millisecond)
		}
		secs = float64(t.UnixNano()) / 1e9
	}
	msecs := int64(secs * 1e3)
	if msecs < minTimeMsecs {
		msecs = 0
	}
	if msecs > maxTimeMsecs {
		msecs = maxTimeMsecs
	}
	return msecs, nil
}

var (
	// These constants were obtained from https://github.com/prometheus/prometheus/blob/91d7175eaac18b00e370965f3a8186cc40bf9f55/web/api/v1/api.go#L442
	// See https://github.com/prometheus/client_golang/issues/614 for details.
	prometheusMinTimeFormatted = time.Unix(math.MinInt64/1000+62135596801, 0).UTC().Format(time.RFC3339Nano)
	prometheusMaxTimeFormatted = time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC().Format(time.RFC3339Nano)
)

const (
	// These values prevent from overflow when storing msec-precision time in int64.
	minTimeMsecs = 0 // use 0 instead of `int64(-1<<63) / 1e6` because the storage engine doesn't actually support negative time
	maxTimeMsecs = int64(1<<63-1) / 1e6
)

// GetDuration returns duration from the given argKey query arg.
func GetDuration(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		d, err := metricsql.DurationValue(argValue, 0)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
		}
		secs = float64(d) / 1000
	}
	msecs := int64(secs * 1e3)
	if msecs <= 0 || msecs > maxDurationMsecs {
		return 0, fmt.Errorf("%q=%dms is out of allowed range [%d ... %d]", argKey, msecs, 0, int64(maxDurationMsecs))
	}
	return msecs, nil
}

const maxDurationMsecs = 100 * 365 * 24 * 3600 * 1000

// GetMaxQueryDuration returns the maximum duration for query from r.
func GetMaxQueryDuration(r *http.Request) time.Duration {
	dms, err := GetDuration(r, "timeout", 0)
	if err != nil {
		dms = 0
	}
	d := time.Duration(dms) * time.Millisecond
	if d <= 0 || d > *maxQueryDuration {
		d = *maxQueryDuration
	}
	return d
}

// GetDeadlineForQuery returns deadline for the given query r.
func GetDeadlineForQuery(r *http.Request, startTime time.Time) Deadline {
	dMax := maxQueryDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxQueryDuration")
}

// GetDeadlineForStatusRequest returns deadline for the given request to /api/v1/status/*.
func GetDeadlineForStatusRequest(r *http.Request, startTime time.Time) Deadline {
	dMax := maxStatusRequestDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxStatusRequestDuration")
}

// GetDeadlineForExport returns deadline for the given request to /api/v1/export.
func GetDeadlineForExport(r *http.Request, startTime time.Time) Deadline {
	dMax := maxExportDuration.Milliseconds()
	return getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxExportDuration")
}

func getDeadlineWithMaxDuration(r *http.Request, startTime time.Time, dMax int64, flagHint string) Deadline {
	d, err := GetDuration(r, "timeout", 0)
	if err != nil {
		d = 0
	}
	if d <= 0 || d > dMax {
		d = dMax
	}
	timeout := time.Duration(d) * time.Millisecond
	return NewDeadline(startTime, timeout, flagHint)
}

// GetBool returns boolean value from the given argKey query arg.
func GetBool(r *http.Request, argKey string) bool {
	argValue := r.FormValue(argKey)
	switch strings.ToLower(argValue) {
	case "", "0", "f", "false", "no":
		return false
	default:
		return true
	}
}

// GetDenyPartialResponse returns whether partial responses are denied.
func GetDenyPartialResponse(r *http.Request) bool {
	if *denyPartialResponse {
		return true
	}
	return GetBool(r, "deny_partial_response")
}

// Deadline contains deadline with the corresponding timeout for pretty error messages.
type Deadline struct {
	deadline uint64

	timeout  time.Duration
	flagHint string
}

// NewDeadline returns deadline for the given timeout.
//
// flagHint must contain a hit for command-line flag, which could be used
// in order to increase timeout.
func NewDeadline(startTime time.Time, timeout time.Duration, flagHint string) Deadline {
	return Deadline{
		deadline: uint64(startTime.Add(timeout).Unix()),
		timeout:  timeout,
		flagHint: flagHint,
	}
}

// Exceeded returns true if deadline is exceeded.
func (d *Deadline) Exceeded() bool {
	return fasttime.UnixTimestamp() > d.deadline
}

// Deadline returns deadline in unix timestamp seconds.
func (d *Deadline) Deadline() uint64 {
	return d.deadline
}

// String returns human-readable string representation for d.
func (d *Deadline) String() string {
	startTime := time.Unix(int64(d.deadline), 0).Add(-d.timeout)
	elapsed := time.Since(startTime)
	return fmt.Sprintf("%.3f seconds (elapsed %.3f seconds); the timeout can be adjusted with `%s` command-line flag", d.timeout.Seconds(), elapsed.Seconds(), d.flagHint)
}

// GetExtraTagFilters returns additional label filters from request.
//
// Label filters can be present in extra_label and extra_filters[] query args.
// They are combined. For example, the following query args:
//   extra_label=t1=v1&extra_label=t2=v2&extra_filters[]={env="prod",team="devops"}&extra_filters={env=~"dev|staging",team!="devops"}
// should be translated to the following filters joined with "or":
//   {env="prod",team="devops",t1="v1",t2="v2"}
//   {env=~"dev|staging",team!="devops",t1="v1",t2="v2"}
func GetExtraTagFilters(r *http.Request) ([][]storage.TagFilter, error) {
	var tagFilters []storage.TagFilter
	for _, match := range r.Form["extra_label"] {
		tmp := strings.SplitN(match, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("`extra_label` query arg must have the format `name=value`; got %q", match)
		}
		tagFilters = append(tagFilters, storage.TagFilter{
			Key:   []byte(tmp[0]),
			Value: []byte(tmp[1]),
		})
	}
	extraFilters := r.Form["extra_filters"]
	extraFilters = append(extraFilters, r.Form["extra_filters[]"]...)
	if len(extraFilters) == 0 {
		if len(tagFilters) == 0 {
			return nil, nil
		}
		return [][]storage.TagFilter{tagFilters}, nil
	}
	var etfs [][]storage.TagFilter
	for _, extraFilter := range extraFilters {
		tfs, err := ParseMetricSelector(extraFilter)
		if err != nil {
			return nil, fmt.Errorf("cannot parse extra_filters=%s: %w", extraFilter, err)
		}
		tfs = append(tfs, tagFilters...)
		etfs = append(etfs, tfs)
	}
	return etfs, nil
}

// JoinTagFilterss adds etfs to every src filter and returns the result.
func JoinTagFilterss(src, etfs [][]storage.TagFilter) [][]storage.TagFilter {
	if len(src) == 0 {
		return etfs
	}
	if len(etfs) == 0 {
		return src
	}
	var dst [][]storage.TagFilter
	for _, tf := range src {
		for _, etf := range etfs {
			tfs := append([]storage.TagFilter{}, tf...)
			tfs = append(tfs, etf...)
			dst = append(dst, tfs)
		}
	}
	return dst
}

// ParseMetricSelector parses s containing PromQL metric selector and returns the corresponding LabelFilters.
func ParseMetricSelector(s string) ([]storage.TagFilter, error) {
	expr, err := metricsql.Parse(s)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("expecting metricSelector; got %q", expr.AppendString(nil))
	}
	if len(me.LabelFilters) == 0 {
		return nil, fmt.Errorf("labelFilters cannot be empty")
	}
	tfs := ToTagFilters(me.LabelFilters)
	return tfs, nil
}

// ToTagFilters converts lfs to a slice of storage.TagFilter
func ToTagFilters(lfs []metricsql.LabelFilter) []storage.TagFilter {
	tfs := make([]storage.TagFilter, len(lfs))
	for i := range lfs {
		toTagFilter(&tfs[i], &lfs[i])
	}
	return tfs
}

func toTagFilter(dst *storage.TagFilter, src *metricsql.LabelFilter) {
	if src.Label != "__name__" {
		dst.Key = []byte(src.Label)
	} else {
		// This is required for storage.Search.
		dst.Key = nil
	}
	dst.Value = []byte(src.Value)
	dst.IsRegexp = src.IsRegexp
	dst.IsNegative = src.IsNegative
}
