package searchutil

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

var (
	maxExportDuration        = flag.Duration("search.maxExportDuration", time.Hour*24*30, "The maximum duration for /api/v1/export call")
	maxDeleteDuration        = flag.Duration("search.maxDeleteDuration", time.Minute*5, "The maximum duration for /api/v1/admin/tsdb/delete_series call")
	maxQueryDuration         = flag.Duration("search.maxQueryDuration", time.Second*30, "The maximum duration for query execution. It can be overridden to a smaller value on a per-query basis via 'timeout' query arg")
	maxStatusRequestDuration = flag.Duration("search.maxStatusRequestDuration", time.Minute*5, "The maximum duration for /api/v1/status/* requests")
	maxLabelsAPIDuration     = flag.Duration("search.maxLabelsAPIDuration", time.Second*5, "The maximum duration for /api/v1/labels, /api/v1/label/.../values and /api/v1/series requests. "+
		"See also -search.maxLabelsAPISeries and -search.ignoreExtraFiltersAtLabelsAPI")
	maxQueryLen = flagutil.NewBytes("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")
)

// GetMaxQueryDuration returns the maximum duration for query from r.
func GetMaxQueryDuration(r *http.Request) time.Duration {
	dms, err := httputil.GetDuration(r, "timeout", 0)
	if err != nil {
		dms = 0
	}
	d := time.Duration(dms) * time.Millisecond
	if d <= 0 || d > *maxQueryDuration {
		d = *maxQueryDuration
	}
	return d
}

// GetDeadlineForQuery returns context for the given query r.
func GetContextForQuery(r *http.Request, startTime time.Time) (context.Context, func()) {
	dMax := maxQueryDuration.Milliseconds()
	deadline := getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxQueryDuration")
	return NewContext(r.Context(), deadline)
}

// GetContextForStatusRequest returns context for the given request to /api/v1/status/*.
func GetContextForStatusRequest(r *http.Request, startTime time.Time) (context.Context, func()) {
	dMax := maxStatusRequestDuration.Milliseconds()
	deadline := getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxStatusRequestDuration")
	return NewContext(r.Context(), deadline)
}

// GetContextForExport returns context for the given request to /api/v1/export.
func GetContextForExport(r *http.Request, startTime time.Time) (context.Context, func()) {
	dMax := maxExportDuration.Milliseconds()
	deadline := getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxExportDuration")
	return NewContext(r.Context(), deadline)
}

// GetContextForLabelsAPI returns context for the given request to /api/v1/labels, /api/v1/label/.../values or /api/v1/series
func GetContextForLabelsAPI(r *http.Request, startTime time.Time) (context.Context, func()) {
	dMax := maxLabelsAPIDuration.Milliseconds()
	deadline := getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxLabelsAPIDuration")
	return NewContext(r.Context(), deadline)
}

// GetDeadlineForDelete returns context for the given request to /api/v1/admin/tsdb/delete_series.
func GetContextForDelete(r *http.Request, startTime time.Time) (context.Context, func()) {
	dMax := maxDeleteDuration.Milliseconds()
	deadline := getDeadlineWithMaxDuration(r, startTime, dMax, "-search.maxDeleteDuration")
	return NewContext(r.Context(), deadline)
}

func getDeadlineWithMaxDuration(r *http.Request, startTime time.Time, dMax int64, flagHint string) Deadline {
	d, err := httputil.GetDuration(r, "timeout", 0)
	if err != nil {
		d = 0
	}
	if d <= 0 || d > dMax {
		d = dMax
	}
	timeout := time.Duration(d) * time.Millisecond
	return NewDeadline(startTime, timeout, flagHint)
}

// Context defines search context with deadline hint
type searchContext struct {
	context.Context
	deadline Deadline
}

// NewContext return new context for given parent context and deadline
func NewContext(ctx context.Context, deadline Deadline) (context.Context, func()) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithDeadline(ctx, time.Unix(int64(deadline.Deadline()), 0))
	return &searchContext{
		Context:  ctx,
		deadline: deadline,
	}, cancel
}

// IsContextDone check if given context is cancelled
func IsContextDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():

		return true
	default:
	}
	return false
}

// Err return context error if there is any
func (ctx *searchContext) Err() error {
	err := ctx.Context.Err()
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("context deadline timeout: %s: %w", ctx.deadline.String(), context.DeadlineExceeded)
	}
	return err
}

// DeadlineTimeoutHint formats deadline from provided context and fall-back deadline time
func DeadlineTimeoutHint(ctx context.Context, deadline time.Time) string {
	sCtx, ok := ctx.(*searchContext)
	if !ok {
		return deadline.String()
	}
	return sCtx.deadline.String()
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

// Deadline returns deadline in unix timestamp seconds.
func (d *Deadline) Deadline() uint64 {
	return d.deadline
}

// String returns human-readable string representation for d.
func (d *Deadline) String() string {
	startTime := time.Unix(int64(d.deadline), 0).Add(-d.timeout)
	elapsed := time.Since(startTime)
	msg := fmt.Sprintf("%.3f seconds (elapsed %.3f seconds)", d.timeout.Seconds(), elapsed.Seconds())
	if float64(elapsed)/float64(d.timeout) > 0.9 && d.flagHint != "" {
		msg += fmt.Sprintf("; the timeout can be adjusted with `%s` command-line flag", d.flagHint)
	}
	return msg
}

// GetExtraTagFilters returns additional label filters from request.
//
// Label filters can be present in extra_label and extra_filters[] query args.
// They are combined. For example, the following query args:
//
//	extra_label=t1=v1&extra_label=t2=v2&extra_filters[]={env="prod",team="devops"}&extra_filters={env=~"dev|staging",team!="devops"}
//
// should be translated to the following filters joined with "or":
//
//	{env="prod",team="devops",t1="v1",t2="v2"}
//	{env=~"dev|staging",team!="devops",t1="v1",t2="v2"}
//
//	Query args from URL path have precedence over post form args.
func GetExtraTagFilters(r *http.Request) ([][]storage.TagFilter, error) {
	var tagFilters []storage.TagFilter
	urlQueryValues := r.URL.Query()
	getRequestParam := func(key string) []string {
		// query request param must always take precedence over form values
		// in order to simplify security enforcement policy for extra_label and extra_filters
		if uv, ok := urlQueryValues[key]; ok {
			return uv
		}
		return r.Form[key]
	}
	for _, match := range getRequestParam("extra_label") {
		tmp := strings.SplitN(match, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("`extra_label` query arg must have the format `name=value`; got %q", match)
		}
		if tmp[0] == "__name__" {
			// This is required for storage.Search.
			tmp[0] = ""
		}
		tagFilters = append(tagFilters, storage.TagFilter{
			Key:   []byte(tmp[0]),
			Value: []byte(tmp[1]),
		})
	}
	extraFilters := append([]string{}, getRequestParam("extra_filters")...)
	extraFilters = append(extraFilters, getRequestParam("extra_filters[]")...)
	if len(extraFilters) == 0 {
		if len(tagFilters) == 0 {
			return nil, nil
		}
		return [][]storage.TagFilter{tagFilters}, nil
	}
	var etfs [][]storage.TagFilter
	for _, extraFilter := range extraFilters {
		tfss, err := ParseMetricSelector(extraFilter)
		if err != nil {
			return nil, fmt.Errorf("cannot parse extra_filters=%s: %w", extraFilter, err)
		}
		for i := range tfss {
			tfss[i] = append(tfss[i], tagFilters...)
		}
		etfs = append(etfs, tfss...)
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
func ParseMetricSelector(s string) ([][]storage.TagFilter, error) {
	expr, err := metricsql.Parse(s)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*metricsql.MetricExpr)
	if !ok {
		return nil, fmt.Errorf("expecting metricSelector; got %q", expr.AppendString(nil))
	}
	if len(me.LabelFilterss) == 0 {
		return nil, fmt.Errorf("labelFilterss cannot be empty")
	}
	tfss := ToTagFilterss(me.LabelFilterss)
	return tfss, nil
}

// ToTagFilterss converts lfss to or-delimited slices of storage.TagFilter
func ToTagFilterss(lfss [][]metricsql.LabelFilter) [][]storage.TagFilter {
	tfss := make([][]storage.TagFilter, len(lfss))
	for i, lfs := range lfss {
		tfs := make([]storage.TagFilter, len(lfs))
		for j := range lfs {
			toTagFilter(&tfs[j], &lfs[j])
		}
		tfss[i] = tfs
	}
	return tfss
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

// GetMaxQueryLen returns the current value of the search.maxQueryLen flag.
func GetMaxQueryLen() int {
	return maxQueryLen.IntN()
}
