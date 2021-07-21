package prometheus

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/querystats"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson/fastfloat"
	"github.com/valyala/quicktemplate"
)

var (
	latencyOffset = flag.Duration("search.latencyOffset", time.Second*30, "The time when data points become visible in query results after the collection. "+
		"Too small value can result in incomplete last points for query results")
	maxQueryLen = flagutil.NewBytes("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")
	maxLookback = flag.Duration("search.maxLookback", 0, "Synonym to -search.lookback-delta from Prometheus. "+
		"The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. "+
		"See also '-search.maxStalenessInterval' flag, which has the same meaining due to historical reasons")
	maxStalenessInterval = flag.Duration("search.maxStalenessInterval", 0, "The maximum interval for staleness calculations. "+
		"By default it is automatically calculated from the median interval between samples. This flag could be useful for tuning "+
		"Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. "+
		"See also '-search.maxLookback' flag, which has the same meaning due to historical reasons")
	maxStepForPointsAdjustment = flag.Duration("search.maxStepForPointsAdjustment", time.Minute, "The maximum step when /api/v1/query_range handler adjusts "+
		"points with timestamps closer than -search.latencyOffset to the current time. The adjustment is needed because such points may contain incomplete data")
)

// Default step used if not set.
const defaultStep = 5 * 60 * 1000

// FederateHandler implements /federate . See https://prometheus.io/docs/prometheus/latest/federation/
func FederateHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	if lookbackDelta <= 0 {
		lookbackDelta = defaultStep
	}
	start, err := searchutils.GetTime(r, "start", ct-lookbackDelta)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if start >= end {
		start = end - defaultStep
	}
	tagFilterss, err := getTagFilterssFromRequest(r)
	if err != nil {
		return err
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	rss, err := netstorage.ProcessSearchQuery(sq, true, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	err = rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
		if err := bw.Error(); err != nil {
			return err
		}
		bb := quicktemplate.AcquireByteBuffer()
		WriteFederate(bb, rs)
		_, err := bw.Write(bb.B)
		quicktemplate.ReleaseByteBuffer(bb)
		return err
	})
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	federateDuration.UpdateDuration(startTime)
	return nil
}

var federateDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/federate"}`)

// ExportCSVHandler exports data in CSV format from /api/v1/export/csv
func ExportCSVHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	format := r.FormValue("format")
	if len(format) == 0 {
		return fmt.Errorf("missing `format` arg; see https://docs.victoriametrics.com/#how-to-export-csv-data")
	}
	fieldNames := strings.Split(format, ",")
	start, err := searchutils.GetTime(r, "start", 0)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForExport(r, startTime)
	tagFilterss, err := getTagFilterssFromRequest(r)
	if err != nil {
		return err
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)

	resultsCh := make(chan *quicktemplate.ByteBuffer, cgroup.AvailableCPUs())
	doneCh := make(chan error)
	go func() {
		err := netstorage.ExportBlocks(sq, deadline, func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error {
			if err := bw.Error(); err != nil {
				return err
			}
			if err := b.UnmarshalData(); err != nil {
				return fmt.Errorf("cannot unmarshal block during export: %s", err)
			}
			xb := exportBlockPool.Get().(*exportBlock)
			xb.mn = mn
			xb.timestamps, xb.values = b.AppendRowsWithTimeRangeFilter(xb.timestamps[:0], xb.values[:0], tr)
			if len(xb.timestamps) > 0 {
				bb := quicktemplate.AcquireByteBuffer()
				WriteExportCSVLine(bb, xb, fieldNames)
				resultsCh <- bb
			}
			xb.reset()
			exportBlockPool.Put(xb)
			return nil
		})
		close(resultsCh)
		doneCh <- err
	}()
	// Consume all the data from resultsCh.
	for bb := range resultsCh {
		// Do not check for error in bw.Write, since this error is checked inside netstorage.ExportBlocks above.
		_, _ = bw.Write(bb.B)
		quicktemplate.ReleaseByteBuffer(bb)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during exporting data to csv: %w", err)
	}
	exportCSVDuration.UpdateDuration(startTime)
	return nil
}

var exportCSVDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export/csv"}`)

// ExportNativeHandler exports data in native format from /api/v1/export/native.
func ExportNativeHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	start, err := searchutils.GetTime(r, "start", 0)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForExport(r, startTime)
	tagFilterss, err := getTagFilterssFromRequest(r)
	if err != nil {
		return err
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	w.Header().Set("Content-Type", "VictoriaMetrics/native")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)

	// Marshal tr
	trBuf := make([]byte, 0, 16)
	trBuf = encoding.MarshalInt64(trBuf, start)
	trBuf = encoding.MarshalInt64(trBuf, end)
	_, _ = bw.Write(trBuf)

	// Marshal native blocks.
	err = netstorage.ExportBlocks(sq, deadline, func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error {
		if err := bw.Error(); err != nil {
			return err
		}
		dstBuf := bbPool.Get()
		tmpBuf := bbPool.Get()
		dst := dstBuf.B
		tmp := tmpBuf.B

		// Marshal mn
		tmp = mn.Marshal(tmp[:0])
		dst = encoding.MarshalUint32(dst, uint32(len(tmp)))
		dst = append(dst, tmp...)

		// Marshal b
		tmp = b.MarshalPortable(tmp[:0])
		dst = encoding.MarshalUint32(dst, uint32(len(tmp)))
		dst = append(dst, tmp...)

		tmpBuf.B = tmp
		bbPool.Put(tmpBuf)

		_, err := bw.Write(dst)

		dstBuf.B = dst
		bbPool.Put(dstBuf)
		return err
	})
	if err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	exportNativeDuration.UpdateDuration(startTime)
	return nil
}

var exportNativeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export/native"}`)

var bbPool bytesutil.ByteBufferPool

// ExportHandler exports data in raw format from /api/v1/export.
func ExportHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	matches := getMatchesFromRequest(r)
	if len(matches) == 0 {
		return fmt.Errorf("missing `match[]` query arg")
	}
	start, err := searchutils.GetTime(r, "start", 0)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	format := r.FormValue("format")
	maxRowsPerLine := int(fastfloat.ParseInt64BestEffort(r.FormValue("max_rows_per_line")))
	reduceMemUsage := searchutils.GetBool(r, "reduce_mem_usage")
	deadline := searchutils.GetDeadlineForExport(r, startTime)
	if start >= end {
		end = start + defaultStep
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	if err := exportHandler(w, matches, etf, start, end, format, maxRowsPerLine, reduceMemUsage, deadline); err != nil {
		return fmt.Errorf("error when exporting data for queries=%q on the time range (start=%d, end=%d): %w", matches, start, end, err)
	}
	exportDuration.UpdateDuration(startTime)
	return nil
}

var exportDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export"}`)

func exportHandler(w http.ResponseWriter, matches []string, etf []storage.TagFilter, start, end int64, format string, maxRowsPerLine int, reduceMemUsage bool, deadline searchutils.Deadline) error {
	writeResponseFunc := WriteExportStdResponse
	writeLineFunc := func(xb *exportBlock, resultsCh chan<- *quicktemplate.ByteBuffer) {
		bb := quicktemplate.AcquireByteBuffer()
		WriteExportJSONLine(bb, xb)
		resultsCh <- bb
	}
	contentType := "application/stream+json; charset=utf-8"
	if format == "prometheus" {
		contentType = "text/plain; charset=utf-8"
		writeLineFunc = func(xb *exportBlock, resultsCh chan<- *quicktemplate.ByteBuffer) {
			bb := quicktemplate.AcquireByteBuffer()
			WriteExportPrometheusLine(bb, xb)
			resultsCh <- bb
		}
	} else if format == "promapi" {
		writeResponseFunc = WriteExportPromAPIResponse
		writeLineFunc = func(xb *exportBlock, resultsCh chan<- *quicktemplate.ByteBuffer) {
			bb := quicktemplate.AcquireByteBuffer()
			WriteExportPromAPILine(bb, xb)
			resultsCh <- bb
		}
	}
	if maxRowsPerLine > 0 {
		writeLineFuncOrig := writeLineFunc
		writeLineFunc = func(xb *exportBlock, resultsCh chan<- *quicktemplate.ByteBuffer) {
			valuesOrig := xb.values
			timestampsOrig := xb.timestamps
			values := valuesOrig
			timestamps := timestampsOrig
			for len(values) > 0 {
				var valuesChunk []float64
				var timestampsChunk []int64
				if len(values) > maxRowsPerLine {
					valuesChunk = values[:maxRowsPerLine]
					timestampsChunk = timestamps[:maxRowsPerLine]
					values = values[maxRowsPerLine:]
					timestamps = timestamps[maxRowsPerLine:]
				} else {
					valuesChunk = values
					timestampsChunk = timestamps
					values = nil
					timestamps = nil
				}
				xb.values = valuesChunk
				xb.timestamps = timestampsChunk
				writeLineFuncOrig(xb, resultsCh)
			}
			xb.values = valuesOrig
			xb.timestamps = timestampsOrig
		}
	}

	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return err
	}
	tagFilterss = addEnforcedFiltersToTagFilterss(tagFilterss, etf)

	sq := storage.NewSearchQuery(start, end, tagFilterss)
	w.Header().Set("Content-Type", contentType)
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)

	resultsCh := make(chan *quicktemplate.ByteBuffer, cgroup.AvailableCPUs())
	doneCh := make(chan error)
	if !reduceMemUsage {
		rss, err := netstorage.ProcessSearchQuery(sq, true, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
		}
		go func() {
			err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
				if err := bw.Error(); err != nil {
					return err
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = &rs.MetricName
				xb.timestamps = rs.Timestamps
				xb.values = rs.Values
				writeLineFunc(xb, resultsCh)
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			close(resultsCh)
			doneCh <- err
		}()
	} else {
		go func() {
			err := netstorage.ExportBlocks(sq, deadline, func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange) error {
				if err := bw.Error(); err != nil {
					return err
				}
				if err := b.UnmarshalData(); err != nil {
					return fmt.Errorf("cannot unmarshal block during export: %s", err)
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = mn
				xb.timestamps, xb.values = b.AppendRowsWithTimeRangeFilter(xb.timestamps[:0], xb.values[:0], tr)
				if len(xb.timestamps) > 0 {
					writeLineFunc(xb, resultsCh)
				}
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			close(resultsCh)
			doneCh <- err
		}()
	}

	// writeResponseFunc must consume all the data from resultsCh.
	writeResponseFunc(bw, resultsCh)
	if err := bw.Flush(); err != nil {
		return err
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	return nil
}

type exportBlock struct {
	mn         *storage.MetricName
	timestamps []int64
	values     []float64
}

func (xb *exportBlock) reset() {
	xb.mn = nil
	xb.timestamps = xb.timestamps[:0]
	xb.values = xb.values[:0]
}

var exportBlockPool = &sync.Pool{
	New: func() interface{} {
		return &exportBlock{}
	},
}

// DeleteHandler processes /api/v1/admin/tsdb/delete_series prometheus API request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#delete-series
func DeleteHandler(startTime time.Time, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse request form values: %w", err)
	}
	if r.FormValue("start") != "" || r.FormValue("end") != "" {
		return fmt.Errorf("start and end aren't supported. Remove these args from the query in order to delete all the matching metrics")
	}
	tagFilterss, err := getTagFilterssFromRequest(r)
	if err != nil {
		return err
	}
	ct := startTime.UnixNano() / 1e6
	sq := storage.NewSearchQuery(0, ct, tagFilterss)
	deletedCount, err := netstorage.DeleteSeries(sq, deadline)
	if err != nil {
		return fmt.Errorf("cannot delete time series: %w", err)
	}
	if deletedCount > 0 {
		promql.ResetRollupResultCache()
	}
	deleteDuration.UpdateDuration(startTime)
	return nil
}

var deleteDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/admin/tsdb/delete_series"}`)

// LabelValuesHandler processes /api/v1/label/<labelName>/values request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values
func LabelValuesHandler(startTime time.Time, labelName string, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	matches := getMatchesFromRequest(r)
	var labelValues []string
	if len(matches) == 0 && len(etf) == 0 {
		if len(r.Form["start"]) == 0 && len(r.Form["end"]) == 0 {
			var err error
			labelValues, err = netstorage.GetLabelValues(labelName, deadline)
			if err != nil {
				return fmt.Errorf(`cannot obtain label values for %q: %w`, labelName, err)
			}
		} else {
			ct := startTime.UnixNano() / 1e6
			end, err := searchutils.GetTime(r, "end", ct)
			if err != nil {
				return err
			}
			start, err := searchutils.GetTime(r, "start", end-defaultStep)
			if err != nil {
				return err
			}
			tr := storage.TimeRange{
				MinTimestamp: start,
				MaxTimestamp: end,
			}
			labelValues, err = netstorage.GetLabelValuesOnTimeRange(labelName, tr, deadline)
			if err != nil {
				return fmt.Errorf(`cannot obtain label values on time range for %q: %w`, labelName, err)
			}
		}
	} else {
		// Extended functionality that allows filtering by label filters and time range
		// i.e. /api/v1/label/foo/values?match[]=foobar{baz="abc"}&start=...&end=...
		// is equivalent to `label_values(foobar{baz="abc"}, foo)` call on the selected
		// time range in Grafana templating.
		if len(matches) == 0 {
			matches = []string{fmt.Sprintf("{%s!=''}", labelName)}
		}
		ct := startTime.UnixNano() / 1e6
		end, err := searchutils.GetTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := searchutils.GetTime(r, "start", end-defaultStep)
		if err != nil {
			return err
		}
		labelValues, err = labelValuesWithMatches(labelName, matches, etf, start, end, deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain label values for %q, match[]=%q, start=%d, end=%d: %w", labelName, matches, start, end, err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteLabelValuesResponse(bw, labelValues)
	if err := bw.Flush(); err != nil {
		return err
	}
	labelValuesDuration.UpdateDuration(startTime)
	return nil
}

func labelValuesWithMatches(labelName string, matches []string, etf []storage.TagFilter, start, end int64, deadline searchutils.Deadline) ([]string, error) {
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}

	// Add `labelName!=''` tag filter in order to filter out series without the labelName.
	// There is no need in adding `__name__!=''` filter, since all the time series should
	// already have non-empty name.
	if labelName != "__name__" {
		key := []byte(labelName)
		for i, tfs := range tagFilterss {
			tagFilterss[i] = append(tfs, storage.TagFilter{
				Key:        key,
				IsNegative: true,
			})
		}
	}
	if start >= end {
		end = start + defaultStep
	}
	tagFilterss = addEnforcedFiltersToTagFilterss(tagFilterss, etf)
	if len(tagFilterss) == 0 {
		logger.Panicf("BUG: tagFilterss must be non-empty")
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	m := make(map[string]struct{})
	if end-start > 24*3600*1000 {
		// It is cheaper to call SearchMetricNames on time ranges exceeding a day.
		mns, err := netstorage.SearchMetricNames(sq, deadline)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch time series for %q: %w", sq, err)
		}
		for _, mn := range mns {
			labelValue := mn.GetTagValue(labelName)
			if len(labelValue) == 0 {
				continue
			}
			m[string(labelValue)] = struct{}{}
		}
	} else {
		rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch data for %q: %w", sq, err)
		}
		var mLock sync.Mutex
		err = rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
			labelValue := rs.MetricName.GetTagValue(labelName)
			if len(labelValue) == 0 {
				return nil
			}
			mLock.Lock()
			m[string(labelValue)] = struct{}{}
			mLock.Unlock()
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error when data fetching: %w", err)
		}
	}
	labelValues := make([]string, 0, len(m))
	for labelValue := range m {
		labelValues = append(labelValues, labelValue)
	}
	sort.Strings(labelValues)
	return labelValues, nil
}

var labelValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/label/{}/values"}`)

// LabelsCountHandler processes /api/v1/labels/count request.
func LabelsCountHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForStatusRequest(r, startTime)
	labelEntries, err := netstorage.GetLabelEntries(deadline)
	if err != nil {
		return fmt.Errorf(`cannot obtain label entries: %w`, err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteLabelsCountResponse(bw, labelEntries)
	if err := bw.Flush(); err != nil {
		return err
	}
	labelsCountDuration.UpdateDuration(startTime)
	return nil
}

var labelsCountDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels/count"}`)

const secsPerDay = 3600 * 24

// TSDBStatusHandler processes /api/v1/status/tsdb request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It can accept `match[]` filters in order to narrow down the search.
func TSDBStatusHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForStatusRequest(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	matches := getMatchesFromRequest(r)

	date := fasttime.UnixDate()
	dateStr := r.FormValue("date")
	if len(dateStr) > 0 {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return fmt.Errorf("cannot parse `date` arg %q: %w", dateStr, err)
		}
		date = uint64(t.Unix()) / secsPerDay
	}
	topN := 10
	topNStr := r.FormValue("topN")
	if len(topNStr) > 0 {
		n, err := strconv.Atoi(topNStr)
		if err != nil {
			return fmt.Errorf("cannot parse `topN` arg %q: %w", topNStr, err)
		}
		if n <= 0 {
			n = 1
		}
		if n > 1000 {
			n = 1000
		}
		topN = n
	}
	var status *storage.TSDBStatus
	if len(matches) == 0 && len(etf) == 0 {
		status, err = netstorage.GetTSDBStatusForDate(deadline, date, topN)
		if err != nil {
			return fmt.Errorf(`cannot obtain tsdb status for date=%d, topN=%d: %w`, date, topN, err)
		}
	} else {
		status, err = tsdbStatusWithMatches(matches, etf, date, topN, deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain tsdb status with matches for date=%d, topN=%d: %w", date, topN, err)
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTSDBStatusResponse(bw, status)
	if err := bw.Flush(); err != nil {
		return err
	}
	tsdbStatusDuration.UpdateDuration(startTime)
	return nil
}

func tsdbStatusWithMatches(matches []string, etf []storage.TagFilter, date uint64, topN int, deadline searchutils.Deadline) (*storage.TSDBStatus, error) {
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}
	tagFilterss = addEnforcedFiltersToTagFilterss(tagFilterss, etf)
	if len(tagFilterss) == 0 {
		logger.Panicf("BUG: tagFilterss must be non-empty")
	}
	start := int64(date*secsPerDay) * 1000
	end := int64(date*secsPerDay+secsPerDay) * 1000
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	status, err := netstorage.GetTSDBStatusWithFilters(deadline, sq, topN)
	if err != nil {
		return nil, err
	}
	return status, nil
}

var tsdbStatusDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/status/tsdb"}`)

// LabelsHandler processes /api/v1/labels request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names
func LabelsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	matches := getMatchesFromRequest(r)
	var labels []string
	if len(matches) == 0 && len(etf) == 0 {
		if len(r.Form["start"]) == 0 && len(r.Form["end"]) == 0 {
			var err error
			labels, err = netstorage.GetLabels(deadline)
			if err != nil {
				return fmt.Errorf("cannot obtain labels: %w", err)
			}
		} else {
			ct := startTime.UnixNano() / 1e6
			end, err := searchutils.GetTime(r, "end", ct)
			if err != nil {
				return err
			}
			start, err := searchutils.GetTime(r, "start", end-defaultStep)
			if err != nil {
				return err
			}
			tr := storage.TimeRange{
				MinTimestamp: start,
				MaxTimestamp: end,
			}
			labels, err = netstorage.GetLabelsOnTimeRange(tr, deadline)
			if err != nil {
				return fmt.Errorf("cannot obtain labels on time range: %w", err)
			}
		}
	} else {
		// Extended functionality that allows filtering by label filters and time range
		// i.e. /api/v1/labels?match[]=foobar{baz="abc"}&start=...&end=...
		if len(matches) == 0 {
			matches = []string{"{__name__!=''}"}
		}
		ct := startTime.UnixNano() / 1e6
		end, err := searchutils.GetTime(r, "end", ct)
		if err != nil {
			return err
		}
		start, err := searchutils.GetTime(r, "start", end-defaultStep)
		if err != nil {
			return err
		}
		labels, err = labelsWithMatches(matches, etf, start, end, deadline)
		if err != nil {
			return fmt.Errorf("cannot obtain labels for match[]=%q, start=%d, end=%d: %w", matches, start, end, err)
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteLabelsResponse(bw, labels)
	if err := bw.Flush(); err != nil {
		return err
	}
	labelsDuration.UpdateDuration(startTime)
	return nil
}

func labelsWithMatches(matches []string, etf []storage.TagFilter, start, end int64, deadline searchutils.Deadline) ([]string, error) {
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}
	if start >= end {
		end = start + defaultStep
	}
	tagFilterss = addEnforcedFiltersToTagFilterss(tagFilterss, etf)
	if len(tagFilterss) == 0 {
		logger.Panicf("BUG: tagFilterss must be non-empty")
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	m := make(map[string]struct{})
	if end-start > 24*3600*1000 {
		// It is cheaper to call SearchMetricNames on time ranges exceeding a day.
		mns, err := netstorage.SearchMetricNames(sq, deadline)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch time series for %q: %w", sq, err)
		}
		for _, mn := range mns {
			for _, tag := range mn.Tags {
				m[string(tag.Key)] = struct{}{}
			}
		}
		if len(mns) > 0 {
			m["__name__"] = struct{}{}
		}
	} else {
		rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch data for %q: %w", sq, err)
		}
		var mLock sync.Mutex
		err = rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
			mLock.Lock()
			for _, tag := range rs.MetricName.Tags {
				m[string(tag.Key)] = struct{}{}
			}
			m["__name__"] = struct{}{}
			mLock.Unlock()
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error when data fetching: %w", err)
		}
	}
	labels := make([]string, 0, len(m))
	for label := range m {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels, nil
}

var labelsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels"}`)

// SeriesCountHandler processes /api/v1/series/count request.
func SeriesCountHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	deadline := searchutils.GetDeadlineForStatusRequest(r, startTime)
	n, err := netstorage.GetSeriesCount(deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain series count: %w", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteSeriesCountResponse(bw, n)
	if err := bw.Flush(); err != nil {
		return err
	}
	seriesCountDuration.UpdateDuration(startTime)
	return nil
}

var seriesCountDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series/count"}`)

// SeriesHandler processes /api/v1/series request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
func SeriesHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	// Do not set start to searchutils.minTimeMsecs by default as Prometheus does,
	// since this leads to fetching and scanning all the data from the storage,
	// which can take a lot of time for big storages.
	// It is better setting start as end-defaultStep by default.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/91
	start, err := searchutils.GetTime(r, "start", end-defaultStep)
	if err != nil {
		return err
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)

	tagFilterss, err := getTagFilterssFromRequest(r)
	if err != nil {
		return err
	}
	if start >= end {
		end = start + defaultStep
	}
	sq := storage.NewSearchQuery(start, end, tagFilterss)
	if end-start > 24*3600*1000 {
		// It is cheaper to call SearchMetricNames on time ranges exceeding a day.
		mns, err := netstorage.SearchMetricNames(sq, deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch time series for %q: %w", sq, err)
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		bw := bufferedwriter.Get(w)
		defer bufferedwriter.Put(bw)
		resultsCh := make(chan *quicktemplate.ByteBuffer)
		go func() {
			for i := range mns {
				bb := quicktemplate.AcquireByteBuffer()
				writemetricNameObject(bb, &mns[i])
				resultsCh <- bb
			}
			close(resultsCh)
		}()
		// WriteSeriesResponse must consume all the data from resultsCh.
		WriteSeriesResponse(bw, resultsCh)
		if err := bw.Flush(); err != nil {
			return err
		}
		seriesDuration.UpdateDuration(startTime)
		return nil
	}
	rss, err := netstorage.ProcessSearchQuery(sq, false, deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	resultsCh := make(chan *quicktemplate.ByteBuffer)
	doneCh := make(chan error)
	go func() {
		err := rss.RunParallel(func(rs *netstorage.Result, workerID uint) error {
			if err := bw.Error(); err != nil {
				return err
			}
			bb := quicktemplate.AcquireByteBuffer()
			writemetricNameObject(bb, &rs.MetricName)
			resultsCh <- bb
			return nil
		})
		close(resultsCh)
		doneCh <- err
	}()
	// WriteSeriesResponse must consume all the data from resultsCh.
	WriteSeriesResponse(bw, resultsCh)
	if err := bw.Flush(); err != nil {
		return err
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during data fetching: %w", err)
	}
	seriesDuration.UpdateDuration(startTime)
	return nil
}

var seriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series"}`)

// QueryHandler processes /api/v1/query request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
func QueryHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := searchutils.GetTime(r, "time", ct)
	if err != nil {
		return err
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	step, err := searchutils.GetDuration(r, "step", lookbackDelta)
	if err != nil {
		return err
	}
	if step <= 0 {
		step = defaultStep
	}
	deadline := searchutils.GetDeadlineForQuery(r, startTime)

	if len(query) > maxQueryLen.N {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	if childQuery, windowExpr, offsetExpr := promql.IsMetricSelectorWithRollup(query); childQuery != "" {
		window := windowExpr.Duration(step)
		offset := offsetExpr.Duration(step)
		start -= offset
		end := start
		start = end - window
		// Do not include data point with a timestamp matching the lower boundary of the window as Prometheus does.
		start++
		if end < start {
			end = start
		}
		if err := exportHandler(w, []string{childQuery}, etf, start, end, "promapi", 0, false, deadline); err != nil {
			return fmt.Errorf("error when exporting data for query=%q on the time range (start=%d, end=%d): %w", childQuery, start, end, err)
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}
	if childQuery, windowExpr, stepExpr, offsetExpr := promql.IsRollup(query); childQuery != "" {
		newStep := stepExpr.Duration(step)
		if newStep > 0 {
			step = newStep
		}
		window := windowExpr.Duration(step)
		offset := offsetExpr.Duration(step)
		start -= offset
		end := start
		start = end - window
		if err := queryRangeHandler(startTime, w, childQuery, start, end, step, r, ct, etf); err != nil {
			return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", childQuery, start, end, step, err)
		}
		queryDuration.UpdateDuration(startTime)
		return nil
	}

	queryOffset := getLatencyOffsetMilliseconds()
	if !searchutils.GetBool(r, "nocache") && ct-start < queryOffset && start-ct < queryOffset {
		// Adjust start time only if `nocache` arg isn't set.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/241
		startPrev := start
		start = ct - queryOffset
		queryOffset = startPrev - start
	} else {
		queryOffset = 0
	}
	ec := promql.EvalConfig{
		Start:              start,
		End:                start,
		Step:               step,
		QuotedRemoteAddr:   httpserver.GetQuotedRemoteAddr(r),
		Deadline:           deadline,
		LookbackDelta:      lookbackDelta,
		RoundDigits:        getRoundDigits(r),
		EnforcedTagFilters: etf,
	}
	result, err := promql.Exec(&ec, query, true)
	if err != nil {
		return fmt.Errorf("error when executing query=%q for (time=%d, step=%d): %w", query, start, step, err)
	}
	if queryOffset > 0 {
		for i := range result {
			timestamps := result[i].Timestamps
			for j := range timestamps {
				timestamps[j] += queryOffset
			}
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteQueryResponse(bw, result)
	if err := bw.Flush(); err != nil {
		return err
	}
	queryDuration.UpdateDuration(startTime)
	return nil
}

var queryDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query"}`)

// QueryRangeHandler processes /api/v1/query_range request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
func QueryRangeHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	ct := startTime.UnixNano() / 1e6
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := searchutils.GetTime(r, "start", ct-defaultStep)
	if err != nil {
		return err
	}
	end, err := searchutils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	step, err := searchutils.GetDuration(r, "step", defaultStep)
	if err != nil {
		return err
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return err
	}
	if err := queryRangeHandler(startTime, w, query, start, end, step, r, ct, etf); err != nil {
		return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", query, start, end, step, err)
	}
	queryRangeDuration.UpdateDuration(startTime)
	return nil
}

func queryRangeHandler(startTime time.Time, w http.ResponseWriter, query string, start, end, step int64, r *http.Request, ct int64, etf []storage.TagFilter) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	mayCache := !searchutils.GetBool(r, "nocache")
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}

	// Validate input args.
	if len(query) > maxQueryLen.N {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
	}
	if start > end {
		end = start + defaultStep
	}
	if err := promql.ValidateMaxPointsPerTimeseries(start, end, step); err != nil {
		return err
	}
	if mayCache {
		start, end = promql.AdjustStartEnd(start, end, step)
	}

	ec := promql.EvalConfig{
		Start:              start,
		End:                end,
		Step:               step,
		QuotedRemoteAddr:   httpserver.GetQuotedRemoteAddr(r),
		Deadline:           deadline,
		MayCache:           mayCache,
		LookbackDelta:      lookbackDelta,
		RoundDigits:        getRoundDigits(r),
		EnforcedTagFilters: etf,
	}
	result, err := promql.Exec(&ec, query, false)
	if err != nil {
		return fmt.Errorf("cannot execute query: %w", err)
	}
	if step < maxStepForPointsAdjustment.Milliseconds() {
		queryOffset := getLatencyOffsetMilliseconds()
		if ct-queryOffset < end {
			result = adjustLastPoints(result, ct-queryOffset, ct+step)
		}
	}

	// Remove NaN values as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	result = removeEmptyValuesAndTimeseries(result)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteQueryRangeResponse(bw, result)
	if err := bw.Flush(); err != nil {
		return err
	}
	return nil
}

func removeEmptyValuesAndTimeseries(tss []netstorage.Result) []netstorage.Result {
	dst := tss[:0]
	for i := range tss {
		ts := &tss[i]
		hasNaNs := false
		for _, v := range ts.Values {
			if math.IsNaN(v) {
				hasNaNs = true
				break
			}
		}
		if !hasNaNs {
			// Fast path: nothing to remove.
			if len(ts.Values) > 0 {
				dst = append(dst, *ts)
			}
			continue
		}

		// Slow path: remove NaNs.
		srcTimestamps := ts.Timestamps
		dstValues := ts.Values[:0]
		dstTimestamps := ts.Timestamps[:0]
		for j, v := range ts.Values {
			if math.IsNaN(v) {
				continue
			}
			dstValues = append(dstValues, v)
			dstTimestamps = append(dstTimestamps, srcTimestamps[j])
		}
		ts.Values = dstValues
		ts.Timestamps = dstTimestamps
		if len(ts.Values) > 0 {
			dst = append(dst, *ts)
		}
	}
	return dst
}

var queryRangeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query_range"}`)

var nan = math.NaN()

// adjustLastPoints substitutes the last point values on the time range (start..end]
// with the previous point values, since these points may contain incomplete values.
func adjustLastPoints(tss []netstorage.Result, start, end int64) []netstorage.Result {
	for i := range tss {
		ts := &tss[i]
		values := ts.Values
		timestamps := ts.Timestamps
		j := len(timestamps) - 1
		if j >= 0 && timestamps[j] > end {
			// It looks like the `offset` is used in the query, which shifts time range beyond the `end`.
			// Leave such a time series as is, since it is unclear which points may be incomplete in it.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/625
			continue
		}
		for j >= 0 && timestamps[j] > start {
			j--
		}
		j++
		lastValue := nan
		if j > 0 {
			lastValue = values[j-1]
		}
		for j < len(timestamps) && timestamps[j] <= end {
			values[j] = lastValue
			j++
		}
	}
	return tss
}

func getMaxLookback(r *http.Request) (int64, error) {
	d := maxLookback.Milliseconds()
	if d == 0 {
		d = maxStalenessInterval.Milliseconds()
	}
	return searchutils.GetDuration(r, "max_lookback", d)
}

func addEnforcedFiltersToTagFilterss(dstTfss [][]storage.TagFilter, enforcedFilters []storage.TagFilter) [][]storage.TagFilter {
	if len(dstTfss) == 0 {
		return [][]storage.TagFilter{
			enforcedFilters,
		}
	}
	for i := range dstTfss {
		dstTfss[i] = append(dstTfss[i], enforcedFilters...)
	}
	return dstTfss
}

func getTagFilterssFromMatches(matches []string) ([][]storage.TagFilter, error) {
	tagFilterss := make([][]storage.TagFilter, 0, len(matches))
	for _, match := range matches {
		tagFilters, err := promql.ParseMetricSelector(match)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %q: %w", match, err)
		}
		tagFilterss = append(tagFilterss, tagFilters)
	}
	return tagFilterss, nil
}

func getTagFilterssFromRequest(r *http.Request) ([][]storage.TagFilter, error) {
	matches := getMatchesFromRequest(r)
	if len(matches) == 0 {
		return nil, fmt.Errorf("missing `match[]` query arg")
	}
	tagFilterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}
	etf, err := searchutils.GetEnforcedTagFiltersFromRequest(r)
	if err != nil {
		return nil, err
	}
	tagFilterss = addEnforcedFiltersToTagFilterss(tagFilterss, etf)
	return tagFilterss, nil
}

func getMatchesFromRequest(r *http.Request) []string {
	matches := r.Form["match[]"]
	// This is needed for backwards compatibility
	matches = append(matches, r.Form["match"]...)
	return matches
}

func getRoundDigits(r *http.Request) int {
	s := r.FormValue("round_digits")
	if len(s) == 0 {
		return 100
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 100
	}
	return n
}

func getLatencyOffsetMilliseconds() int64 {
	d := latencyOffset.Milliseconds()
	if d <= 1000 {
		d = 1000
	}
	return d
}

// QueryStatsHandler returns query stats at `/api/v1/status/top_queries`
func QueryStatsHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("cannot parse form values: %w", err)
	}
	topN := 20
	topNStr := r.FormValue("topN")
	if len(topNStr) > 0 {
		n, err := strconv.Atoi(topNStr)
		if err != nil {
			return fmt.Errorf("cannot parse `topN` arg %q: %w", topNStr, err)
		}
		topN = n
	}
	maxLifetimeMsecs, err := searchutils.GetDuration(r, "maxLifetime", 10*60*1000)
	if err != nil {
		return fmt.Errorf("cannot parse `maxLifetime` arg: %w", err)
	}
	maxLifetime := time.Duration(maxLifetimeMsecs) * time.Millisecond
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	querystats.WriteJSONQueryStats(bw, topN, maxLifetime)
	if err := bw.Flush(); err != nil {
		return err
	}
	queryStatsDuration.UpdateDuration(startTime)
	return nil
}

var queryStatsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/status/top_queries"}`)
