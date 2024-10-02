package prometheus

import (
	"flag"
	"fmt"
	"math"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/querystats"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bufferedwriter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/metrics"
	"github.com/VictoriaMetrics/metricsql"
	"github.com/valyala/fastjson/fastfloat"
)

var (
	latencyOffset = flag.Duration("search.latencyOffset", time.Second*30, "The time when data points become visible in query results after the collection. "+
		"It can be overridden on per-query basis via latency_offset arg. "+
		"Too small value can result in incomplete last points for query results")
	maxQueryLen = flagutil.NewBytes("search.maxQueryLen", 16*1024, "The maximum search query length in bytes")
	maxLookback = flag.Duration("search.maxLookback", 0, "Synonym to -search.lookback-delta from Prometheus. "+
		"The value is dynamically detected from interval between time series datapoints if not set. It can be overridden on per-query basis via max_lookback arg. "+
		"See also '-search.maxStalenessInterval' flag, which has the same meaning due to historical reasons")
	maxStalenessInterval = flag.Duration("search.maxStalenessInterval", 0, "The maximum interval for staleness calculations. "+
		"By default, it is automatically calculated from the median interval between samples. This flag could be useful for tuning "+
		"Prometheus data model closer to Influx-style data model. See https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness for details. "+
		"See also '-search.setLookbackToStep' flag")
	setLookbackToStep = flag.Bool("search.setLookbackToStep", false, "Whether to fix lookback interval to 'step' query arg value. "+
		"If set to true, the query model becomes closer to InfluxDB data model. If set to true, then -search.maxLookback and -search.maxStalenessInterval are ignored")
	maxStepForPointsAdjustment = flag.Duration("search.maxStepForPointsAdjustment", time.Minute, "The maximum step when /api/v1/query_range handler adjusts "+
		"points with timestamps closer than -search.latencyOffset to the current time. The adjustment is needed because such points may contain incomplete data")

	maxUniqueTimeseries = flag.Int("search.maxUniqueTimeseries", 300e3, "The maximum number of unique time series, which can be selected during /api/v1/query and /api/v1/query_range queries. This option allows limiting memory usage")
	maxFederateSeries   = flag.Int("search.maxFederateSeries", 1e6, "The maximum number of time series, which can be returned from /federate. This option allows limiting memory usage")
	maxExportSeries     = flag.Int("search.maxExportSeries", 10e6, "The maximum number of time series, which can be returned from /api/v1/export* APIs. This option allows limiting memory usage")
	maxTSDBStatusSeries = flag.Int("search.maxTSDBStatusSeries", 10e6, "The maximum number of time series, which can be processed during the call to /api/v1/status/tsdb. This option allows limiting memory usage")
	maxSeriesLimit      = flag.Int("search.maxSeries", 30e3, "The maximum number of time series, which can be returned from /api/v1/series. This option allows limiting memory usage")
	maxDeleteSeries     = flag.Int("search.maxDeleteSeries", 1e6, "The maximum number of time series, which can be deleted using /api/v1/admin/tsdb/delete_series. This option allows limiting memory usage")
	maxLabelsAPISeries  = flag.Int("search.maxLabelsAPISeries", 1e6, "The maximum number of time series, which could be scanned when searching for the matching time series "+
		"at /api/v1/labels and /api/v1/label/.../values. This option allows limiting memory usage and CPU usage. See also -search.maxLabelsAPIDuration, "+
		"-search.maxTagKeys, -search.maxTagValues and -search.ignoreExtraFiltersAtLabelsAPI")
	maxPointsPerTimeseries = flag.Int("search.maxPointsPerTimeseries", 30e3, "The maximum points per a single timeseries returned from /api/v1/query_range. "+
		"This option doesn't limit the number of scanned raw samples in the database. The main purpose of this option is to limit the number of per-series points "+
		"returned to graphing UI such as VMUI or Grafana. There is no sense in setting this limit to values bigger than the horizontal resolution of the graph. "+
		"See also -search.maxResponseSeries")
	ignoreExtraFiltersAtLabelsAPI = flag.Bool("search.ignoreExtraFiltersAtLabelsAPI", false, "Whether to ignore match[], extra_filters[] and extra_label query args at "+
		"/api/v1/labels and /api/v1/label/.../values . This may be useful for decreasing load on VictoriaMetrics when extra filters "+
		"match too many time series. The downside is that superfluous labels or series could be returned, which do not match the extra filters. "+
		"See also -search.maxLabelsAPISeries and -search.maxLabelsAPIDuration")
)

// Default step used if not set.
const defaultStep = 5 * 60 * 1000

// ExpandWithExprs handles the request to /expand-with-exprs
func ExpandWithExprs(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	format := r.FormValue("format")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		httpserver.EnableCORS(w, r)
		WriteExpandWithExprsJSONResponse(bw, query)
	} else {
		WriteExpandWithExprsResponse(bw, query)
	}
	_ = bw.Flush()
}

// PrettifyQuery handles the request /prettify-query
func PrettifyQuery(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("query")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	w.Header().Set("Content-Type", "application/json")
	httpserver.EnableCORS(w, r)

	prettyQuery, err := metricsql.Prettify(query)
	if err != nil {
		fmt.Fprintf(bw, `{"status": "error", "msg": %q}`, err)
	} else {
		fmt.Fprintf(bw, `{"status": "success", "query": %q}`, prettyQuery)
	}
	_ = bw.Flush()
}

// FederateHandler implements /federate . See https://prometheus.io/docs/prometheus/latest/federation/
func FederateHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer federateDuration.UpdateDuration(startTime)

	cp, err := getCommonParams(r, startTime, true)
	if err != nil {
		return err
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	if lookbackDelta <= 0 {
		lookbackDelta = defaultStep
	}
	if cp.IsDefaultTimeRange() {
		cp.start = cp.end - lookbackDelta
	}
	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxFederateSeries)
	rss, err := netstorage.ProcessSearchQuery(nil, sq, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	sw := newScalableWriter(bw)
	err = rss.RunParallel(nil, func(rs *netstorage.Result, workerID uint) error {
		if err := bw.Error(); err != nil {
			return err
		}
		bb := sw.getBuffer(workerID)
		WriteFederate(bb, rs)
		return sw.maybeFlushBuffer(bb)
	})
	if err != nil {
		return fmt.Errorf("error during sending data to remote client: %w", err)
	}
	return sw.flush()
}

var federateDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/federate"}`)

// ExportCSVHandler exports data in CSV format from /api/v1/export/csv
func ExportCSVHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer exportCSVDuration.UpdateDuration(startTime)

	cp, err := getExportParams(r, startTime)
	if err != nil {
		return err
	}

	format := r.FormValue("format")
	if len(format) == 0 {
		return fmt.Errorf("missing `format` arg; see https://docs.victoriametrics.com/#how-to-export-csv-data")
	}
	fieldNames := strings.Split(format, ",")
	reduceMemUsage := httputils.GetBool(r, "reduce_mem_usage")

	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxExportSeries)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	sw := newScalableWriter(bw)
	writeCSVLine := func(xb *exportBlock, workerID uint) error {
		if len(xb.timestamps) == 0 {
			return nil
		}
		bb := sw.getBuffer(workerID)
		WriteExportCSVLine(bb, xb, fieldNames)
		return sw.maybeFlushBuffer(bb)
	}
	doneCh := make(chan error, 1)
	if !reduceMemUsage {
		rss, err := netstorage.ProcessSearchQuery(nil, sq, cp.deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
		}
		go func() {
			err := rss.RunParallel(nil, func(rs *netstorage.Result, workerID uint) error {
				if err := bw.Error(); err != nil {
					return err
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = &rs.MetricName
				xb.timestamps = rs.Timestamps
				xb.values = rs.Values
				if err := writeCSVLine(xb, workerID); err != nil {
					return err
				}
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			doneCh <- err
		}()
	} else {
		go func() {
			err := netstorage.ExportBlocks(nil, sq, cp.deadline, func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange, workerID uint) error {
				if err := bw.Error(); err != nil {
					return err
				}
				if err := b.UnmarshalData(); err != nil {
					return fmt.Errorf("cannot unmarshal block during export: %w", err)
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = mn
				xb.timestamps, xb.values = b.AppendRowsWithTimeRangeFilter(xb.timestamps[:0], xb.values[:0], tr)
				if err := writeCSVLine(xb, workerID); err != nil {
					return err
				}
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			doneCh <- err
		}()
	}
	err = <-doneCh
	if err != nil {
		return fmt.Errorf("error during sending the exported csv data to remote client: %w", err)
	}
	return sw.flush()
}

var exportCSVDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export/csv"}`)

// ExportNativeHandler exports data in native format from /api/v1/export/native.
func ExportNativeHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer exportNativeDuration.UpdateDuration(startTime)

	cp, err := getExportParams(r, startTime)
	if err != nil {
		return err
	}

	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxExportSeries)
	w.Header().Set("Content-Type", "VictoriaMetrics/native")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	sw := newScalableWriter(bw)

	// Marshal tr
	trBuf := make([]byte, 0, 16)
	trBuf = encoding.MarshalInt64(trBuf, cp.start)
	trBuf = encoding.MarshalInt64(trBuf, cp.end)
	_, _ = bw.Write(trBuf)

	// Marshal native blocks.
	err = netstorage.ExportBlocks(nil, sq, cp.deadline, func(mn *storage.MetricName, b *storage.Block, _ storage.TimeRange, workerID uint) error {
		if err := bw.Error(); err != nil {
			return err
		}
		bb := sw.getBuffer(workerID)
		dst := bb.B
		tmpBuf := bbPool.Get()
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

		bb.B = dst
		return sw.maybeFlushBuffer(bb)
	})
	if err != nil {
		return fmt.Errorf("error during sending native data to remote client: %w", err)
	}
	return sw.flush()
}

var exportNativeDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export/native"}`)

var bbPool bytesutil.ByteBufferPool

// ExportHandler exports data in raw format from /api/v1/export.
func ExportHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer exportDuration.UpdateDuration(startTime)

	cp, err := getExportParams(r, startTime)
	if err != nil {
		return err
	}
	format := r.FormValue("format")
	maxRowsPerLine := int(fastfloat.ParseInt64BestEffort(r.FormValue("max_rows_per_line")))
	reduceMemUsage := httputils.GetBool(r, "reduce_mem_usage")
	if err := exportHandler(nil, w, cp, format, maxRowsPerLine, reduceMemUsage); err != nil {
		return fmt.Errorf("error when exporting data on the time range (start=%d, end=%d): %w", cp.start, cp.end, err)
	}
	return nil
}

var exportDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/export"}`)

func exportHandler(qt *querytracer.Tracer, w http.ResponseWriter, cp *commonParams, format string, maxRowsPerLine int, reduceMemUsage bool) error {
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	sw := newScalableWriter(bw)
	writeLineFunc := func(xb *exportBlock, workerID uint) error {
		bb := sw.getBuffer(workerID)
		WriteExportJSONLine(bb, xb)
		return sw.maybeFlushBuffer(bb)
	}
	contentType := "application/stream+json; charset=utf-8"
	if format == "prometheus" {
		contentType = "text/plain; charset=utf-8"
		writeLineFunc = func(xb *exportBlock, workerID uint) error {
			bb := sw.getBuffer(workerID)
			WriteExportPrometheusLine(bb, xb)
			return sw.maybeFlushBuffer(bb)
		}
	} else if format == "promapi" {
		WriteExportPromAPIHeader(bw)
		var firstLineOnce atomic.Bool
		var firstLineSent atomic.Bool
		writeLineFunc = func(xb *exportBlock, workerID uint) error {
			bb := sw.getBuffer(workerID)
			// Use Load() in front of CompareAndSwap() in order to avoid slow inter-CPU synchronization
			// in fast path after the first line has been already sent.
			if !firstLineOnce.Load() && firstLineOnce.CompareAndSwap(false, true) {
				// Send the first line to sw.bw
				WriteExportPromAPILine(bb, xb)
				_, err := sw.bw.Write(bb.B)
				bb.Reset()
				firstLineSent.Store(true)
				return err
			}
			for !firstLineSent.Load() {
				// Busy wait until the first line is sent to sw.bw
				runtime.Gosched()
			}
			bb.B = append(bb.B, ',')
			WriteExportPromAPILine(bb, xb)
			return sw.maybeFlushBuffer(bb)
		}
	}
	if maxRowsPerLine > 0 {
		writeLineFuncOrig := writeLineFunc
		writeLineFunc = func(xb *exportBlock, workerID uint) error {
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
				if err := writeLineFuncOrig(xb, workerID); err != nil {
					return err
				}
			}
			xb.values = valuesOrig
			xb.timestamps = timestampsOrig
			return nil
		}
	}

	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxExportSeries)
	w.Header().Set("Content-Type", contentType)

	doneCh := make(chan error, 1)
	if !reduceMemUsage {
		rss, err := netstorage.ProcessSearchQuery(qt, sq, cp.deadline)
		if err != nil {
			return fmt.Errorf("cannot fetch data for %q: %w", sq, err)
		}
		qtChild := qt.NewChild("background export format=%s", format)
		go func() {
			err := rss.RunParallel(qtChild, func(rs *netstorage.Result, workerID uint) error {
				if err := bw.Error(); err != nil {
					return err
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = &rs.MetricName
				xb.timestamps = rs.Timestamps
				xb.values = rs.Values
				if err := writeLineFunc(xb, workerID); err != nil {
					return err
				}
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			qtChild.Done()
			doneCh <- err
		}()
	} else {
		qtChild := qt.NewChild("background export format=%s", format)
		go func() {
			err := netstorage.ExportBlocks(qtChild, sq, cp.deadline, func(mn *storage.MetricName, b *storage.Block, tr storage.TimeRange, workerID uint) error {
				if err := bw.Error(); err != nil {
					return err
				}
				if err := b.UnmarshalData(); err != nil {
					return fmt.Errorf("cannot unmarshal block during export: %w", err)
				}
				xb := exportBlockPool.Get().(*exportBlock)
				xb.mn = mn
				xb.timestamps, xb.values = b.AppendRowsWithTimeRangeFilter(xb.timestamps[:0], xb.values[:0], tr)
				if len(xb.timestamps) > 0 {
					if err := writeLineFunc(xb, workerID); err != nil {
						return err
					}
				}
				xb.reset()
				exportBlockPool.Put(xb)
				return nil
			})
			qtChild.Done()
			doneCh <- err
		}()
	}
	err := <-doneCh
	if err != nil {
		return fmt.Errorf("cannot send data to remote client: %w", err)
	}
	if err := sw.flush(); err != nil {
		return fmt.Errorf("cannot send data to remote client: %w", err)
	}
	if format == "promapi" {
		WriteExportPromAPIFooter(bw, qt)
	}
	return bw.Flush()
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
	New: func() any {
		return &exportBlock{}
	},
}

// DeleteHandler processes /api/v1/admin/tsdb/delete_series prometheus API request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#delete-series
func DeleteHandler(startTime time.Time, r *http.Request) error {
	defer deleteDuration.UpdateDuration(startTime)

	cp, err := getCommonParams(r, startTime, true)
	if err != nil {
		return err
	}
	if !cp.IsDefaultTimeRange() {
		return fmt.Errorf("start=%d and end=%d args aren't supported. Remove these args from the query in order to delete all the matching metrics", cp.start, cp.end)
	}
	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxDeleteSeries)
	deletedCount, err := netstorage.DeleteSeries(nil, sq, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot delete time series: %w", err)
	}
	if deletedCount > 0 {
		promql.ResetRollupResultCache()
	}
	return nil
}

var deleteDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/admin/tsdb/delete_series"}`)

// LabelValuesHandler processes /api/v1/label/<labelName>/values request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#querying-label-values
func LabelValuesHandler(qt *querytracer.Tracer, startTime time.Time, labelName string, w http.ResponseWriter, r *http.Request) error {
	defer labelValuesDuration.UpdateDuration(startTime)

	cp, err := getCommonParamsForLabelsAPI(r, startTime, false)
	if err != nil {
		return err
	}
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxLabelsAPISeries)
	labelValues, err := netstorage.LabelValues(qt, labelName, sq, limit, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain values for label %q: %w", labelName, err)
	}

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteLabelValuesResponse(bw, labelValues, qt)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("canot flush label values to remote client: %w", err)
	}
	return nil
}

var labelValuesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/label/{}/values"}`)

const secsPerDay = 3600 * 24

// TSDBStatusHandler processes /api/v1/status/tsdb request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#tsdb-stats
//
// It can accept `match[]` filters in order to narrow down the search.
func TSDBStatusHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer tsdbStatusDuration.UpdateDuration(startTime)

	cp, err := getCommonParams(r, startTime, false)
	if err != nil {
		return err
	}
	cp.deadline = searchutils.GetDeadlineForStatusRequest(r, startTime)

	date := fasttime.UnixDate()
	dateStr := r.FormValue("date")
	if len(dateStr) > 0 {
		if dateStr == "0" {
			date = 0
		} else {
			t, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				return fmt.Errorf("cannot parse `date` arg %q: %w", dateStr, err)
			}
			date = uint64(t.Unix()) / secsPerDay
		}
	}
	focusLabel := r.FormValue("focusLabel")
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
	start := int64(date*secsPerDay) * 1000
	end := int64((date+1)*secsPerDay)*1000 - 1
	sq := storage.NewSearchQuery(start, end, cp.filterss, *maxTSDBStatusSeries)
	status, err := netstorage.TSDBStatus(qt, sq, focusLabel, topN, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain tsdb stats: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteTSDBStatusResponse(bw, status, qt)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot send tsdb status response to remote client: %w", err)
	}
	return nil
}

var tsdbStatusDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/status/tsdb"}`)

// LabelsHandler processes /api/v1/labels request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#getting-label-names
func LabelsHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer labelsDuration.UpdateDuration(startTime)

	cp, err := getCommonParamsForLabelsAPI(r, startTime, false)
	if err != nil {
		return err
	}
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}
	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxLabelsAPISeries)
	labels, err := netstorage.LabelNames(qt, sq, limit, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain labels: %w", err)
	}

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteLabelsResponse(bw, labels, qt)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot send labels response to remote client: %w", err)
	}
	return nil
}

var labelsDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/labels"}`)

// SeriesCountHandler processes /api/v1/series/count request.
func SeriesCountHandler(startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer seriesCountDuration.UpdateDuration(startTime)

	deadline := searchutils.GetDeadlineForStatusRequest(r, startTime)
	n, err := netstorage.SeriesCount(nil, deadline)
	if err != nil {
		return fmt.Errorf("cannot obtain series count: %w", err)
	}
	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	WriteSeriesCountResponse(bw, n)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot send series count response to remote client: %w", err)
	}
	return nil
}

var seriesCountDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series/count"}`)

// SeriesHandler processes /api/v1/series request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
func SeriesHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer seriesDuration.UpdateDuration(startTime)

	// Do not set start to httputils.minTimeMsecs by default as Prometheus does,
	// since this leads to fetching and scanning all the data from the storage,
	// which can take a lot of time for big storages.
	// It is better setting start as end-defaultStep by default.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/91
	cp, err := getCommonParamsForLabelsAPI(r, startTime, true)
	if err != nil {
		return err
	}
	limit, err := httputils.GetInt(r, "limit")
	if err != nil {
		return err
	}

	sq := storage.NewSearchQuery(cp.start, cp.end, cp.filterss, *maxSeriesLimit)
	metricNames, err := netstorage.SearchMetricNames(qt, sq, cp.deadline)
	if err != nil {
		return fmt.Errorf("cannot fetch time series for %q: %w", sq, err)
	}
	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	if limit > 0 && limit < len(metricNames) {
		metricNames = metricNames[:limit]
	}
	qtDone := func() {
		qt.Donef("start=%d, end=%d", cp.start, cp.end)
	}
	WriteSeriesResponse(bw, metricNames, qt, qtDone)
	return bw.Flush()
}

var seriesDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/series"}`)

// QueryHandler processes /api/v1/query request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
func QueryHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer queryDuration.UpdateDuration(startTime)

	ct := startTime.UnixNano() / 1e6
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	mayCache := !httputils.GetBool(r, "nocache")
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := httputils.GetTime(r, "time", ct)
	if err != nil {
		return err
	}
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}
	step, err := httputils.GetDuration(r, "step", lookbackDelta)
	if err != nil {
		return err
	}
	if step <= 0 {
		step = defaultStep
	}

	if len(query) > maxQueryLen.IntN() {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
	}
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return err
	}
	if childQuery, windowExpr, offsetExpr := promql.IsMetricSelectorWithRollup(query); childQuery != "" {
		window, err := windowExpr.NonNegativeDuration(step)
		if err != nil {
			return fmt.Errorf("cannot parse lookbehind window in square brackets at %s: %w", query, err)
		}
		offset := offsetExpr.Duration(step)
		start -= offset
		end := start
		start = end - window
		// Do not include data point with a timestamp matching the lower boundary of the window as Prometheus does.
		start++
		if end < start {
			end = start
		}

		tagFilterss, err := getTagFilterssFromMatches([]string{childQuery})
		if err != nil {
			return err
		}
		filterss := searchutils.JoinTagFilterss(tagFilterss, etfs)

		cp := &commonParams{
			deadline: deadline,
			start:    start,
			end:      end,
			filterss: filterss,
		}
		if err := exportHandler(qt, w, cp, "promapi", 0, false); err != nil {
			return fmt.Errorf("error when exporting data for query=%q on the time range (start=%d, end=%d): %w", childQuery, start, end, err)
		}
		return nil
	}
	if childQuery, windowExpr, stepExpr, offsetExpr := promql.IsRollup(query); childQuery != "" {
		newStep, err := stepExpr.NonNegativeDuration(step)
		if err != nil {
			return fmt.Errorf("cannot parse step in square brackets at %s: %w", query, err)
		}
		if newStep > 0 {
			step = newStep
		}
		window, err := windowExpr.NonNegativeDuration(step)
		if err != nil {
			return fmt.Errorf("cannot parse lookbehind window in square brackets at %s: %w", query, err)
		}
		offset := offsetExpr.Duration(step)
		start -= offset
		end := start
		start = end - window
		if err := queryRangeHandler(qt, startTime, w, childQuery, start, end, step, r, ct, etfs); err != nil {
			return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", childQuery, start, end, step, err)
		}
		return nil
	}

	queryOffset, err := getLatencyOffsetMilliseconds(r)
	if err != nil {
		return err
	}
	if !httputils.GetBool(r, "nocache") && ct-start < queryOffset && start-ct < queryOffset {
		// Adjust start time only if `nocache` arg isn't set.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/241
		startPrev := start
		start = ct - queryOffset
		queryOffset = startPrev - start
	} else {
		queryOffset = 0
	}
	qs := &promql.QueryStats{}
	ec := &promql.EvalConfig{
		Start:               start,
		End:                 start,
		Step:                step,
		MaxPointsPerSeries:  *maxPointsPerTimeseries,
		MaxSeries:           *maxUniqueTimeseries,
		QuotedRemoteAddr:    httpserver.GetQuotedRemoteAddr(r),
		Deadline:            deadline,
		MayCache:            mayCache,
		LookbackDelta:       lookbackDelta,
		RoundDigits:         getRoundDigits(r),
		EnforcedTagFilterss: etfs,
		GetRequestURI: func() string {
			return httpserver.GetRequestURI(r)
		},

		QueryStats: qs,
	}
	result, err := promql.Exec(qt, ec, query, true)
	if err != nil {
		return fmt.Errorf("error when executing query=%q for (time=%d, step=%d): %w", query, start, step, err)
	}
	if queryOffset > 0 {
		for i := range result {
			r := &result[i]
			// Do not modify r.Timestamps, since they may be shared among multiple series.
			// Make a copy instead.
			timestamps := append([]int64{}, r.Timestamps...)
			for j := range timestamps {
				timestamps[j] += queryOffset
			}
			r.Timestamps = timestamps
		}
	}

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	qtDone := func() {
		qt.Donef("query=%s, time=%d: series=%d", query, start, len(result))
	}

	WriteQueryResponse(bw, result, qt, qtDone, qs)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot flush query response to remote client: %w", err)
	}
	return nil
}

var queryDuration = metrics.NewSummary(`vm_request_duration_seconds{path="/api/v1/query"}`)

// QueryRangeHandler processes /api/v1/query_range request.
//
// See https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries
func QueryRangeHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, r *http.Request) error {
	defer queryRangeDuration.UpdateDuration(startTime)

	ct := startTime.UnixNano() / 1e6
	query := r.FormValue("query")
	if len(query) == 0 {
		return fmt.Errorf("missing `query` arg")
	}
	start, err := httputils.GetTime(r, "start", ct-defaultStep)
	if err != nil {
		return err
	}
	end, err := httputils.GetTime(r, "end", ct)
	if err != nil {
		return err
	}
	step, err := httputils.GetDuration(r, "step", defaultStep)
	if err != nil {
		return err
	}
	etfs, err := searchutils.GetExtraTagFilters(r)
	if err != nil {
		return err
	}
	if err := queryRangeHandler(qt, startTime, w, query, start, end, step, r, ct, etfs); err != nil {
		return fmt.Errorf("error when executing query=%q on the time range (start=%d, end=%d, step=%d): %w", query, start, end, step, err)
	}
	return nil
}

func queryRangeHandler(qt *querytracer.Tracer, startTime time.Time, w http.ResponseWriter, query string,
	start, end, step int64, r *http.Request, ct int64, etfs [][]storage.TagFilter) error {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	mayCache := !httputils.GetBool(r, "nocache")
	lookbackDelta, err := getMaxLookback(r)
	if err != nil {
		return err
	}

	// Validate input args.
	if len(query) > maxQueryLen.IntN() {
		return fmt.Errorf("too long query; got %d bytes; mustn't exceed `-search.maxQueryLen=%d` bytes", len(query), maxQueryLen.N)
	}
	if start > end {
		end = start + defaultStep
	}
	if err := promql.ValidateMaxPointsPerSeries(start, end, step, *maxPointsPerTimeseries); err != nil {
		return fmt.Errorf("%w; (see -search.maxPointsPerTimeseries command-line flag)", err)
	}
	if mayCache {
		start, end = promql.AdjustStartEnd(start, end, step)
	}

	qs := &promql.QueryStats{}
	ec := &promql.EvalConfig{
		Start:               start,
		End:                 end,
		Step:                step,
		MaxPointsPerSeries:  *maxPointsPerTimeseries,
		MaxSeries:           *maxUniqueTimeseries,
		QuotedRemoteAddr:    httpserver.GetQuotedRemoteAddr(r),
		Deadline:            deadline,
		MayCache:            mayCache,
		LookbackDelta:       lookbackDelta,
		RoundDigits:         getRoundDigits(r),
		EnforcedTagFilterss: etfs,
		GetRequestURI: func() string {
			return httpserver.GetRequestURI(r)
		},

		QueryStats: qs,
	}
	result, err := promql.Exec(qt, ec, query, false)
	if err != nil {
		return err
	}
	if step < maxStepForPointsAdjustment.Milliseconds() {
		queryOffset, err := getLatencyOffsetMilliseconds(r)
		if err != nil {
			return err
		}
		if ct-queryOffset < end {
			result = adjustLastPoints(result, ct-queryOffset, ct+step)
		}
	}

	// Remove NaN values as Prometheus does.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/153
	result = removeEmptyValuesAndTimeseries(result)

	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	qtDone := func() {
		qt.Donef("start=%d, end=%d, step=%d, query=%q: series=%d", start, end, step, query, len(result))
	}
	WriteQueryRangeResponse(bw, result, qt, qtDone, qs)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot send query range response to remote client: %w", err)
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
		// Do not re-use ts.Timestamps for dstTimestamps, since ts.Timestamps
		// may be shared among multiple time series.
		dstTimestamps := make([]int64, 0, len(ts.Timestamps))
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
	maxLookback, err := httputils.GetDuration(r, "max_lookback", d)
	if err != nil {
		return 0, err
	}
	d = maxLookback
	if *setLookbackToStep {
		step, err := httputils.GetDuration(r, "step", d)
		if err != nil {
			return 0, err
		}
		d = step
	}
	return d, nil
}

func getTagFilterssFromMatches(matches []string) ([][]storage.TagFilter, error) {
	tfss := make([][]storage.TagFilter, 0, len(matches))
	for _, match := range matches {
		tfssLocal, err := searchutils.ParseMetricSelector(match)
		if err != nil {
			return nil, fmt.Errorf("cannot parse matches[]=%s: %w", match, err)
		}
		tfss = append(tfss, tfssLocal...)
	}
	return tfss, nil
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

func getLatencyOffsetMilliseconds(r *http.Request) (int64, error) {
	d := latencyOffset.Milliseconds()
	if d < 0 {
		// Zero latency offset may be useful for some use cases.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2061#issuecomment-1299109836
		d = 0
	}
	return httputils.GetDuration(r, "latency_offset", d)
}

// QueryStatsHandler returns query stats at `/api/v1/status/top_queries`
func QueryStatsHandler(w http.ResponseWriter, r *http.Request) error {
	topN := 20
	topNStr := r.FormValue("topN")
	if len(topNStr) > 0 {
		n, err := strconv.Atoi(topNStr)
		if err != nil {
			return fmt.Errorf("cannot parse `topN` arg %q: %w", topNStr, err)
		}
		topN = n
	}
	maxLifetimeMsecs, err := httputils.GetDuration(r, "maxLifetime", 10*60*1000)
	if err != nil {
		return fmt.Errorf("cannot parse `maxLifetime` arg: %w", err)
	}
	maxLifetime := time.Duration(maxLifetimeMsecs) * time.Millisecond
	w.Header().Set("Content-Type", "application/json")
	bw := bufferedwriter.Get(w)
	defer bufferedwriter.Put(bw)
	querystats.WriteJSONQueryStats(bw, topN, maxLifetime)
	if err := bw.Flush(); err != nil {
		return fmt.Errorf("cannot send query stats response to client: %w", err)
	}
	return nil
}

// commonParams contains common parameters for all /api/v1/* handlers
//
// timeout, start, end, match[], extra_label, extra_filters[]
type commonParams struct {
	deadline         searchutils.Deadline
	start            int64
	end              int64
	currentTimestamp int64
	filterss         [][]storage.TagFilter
}

func (cp *commonParams) IsDefaultTimeRange() bool {
	return cp.start == 0 && cp.currentTimestamp-cp.end < 1000
}

// getExportParams obtains common params from r, which are used in /api/v1/export* handlers
//
// - timeout
// - start
// - end
// - match[]
// - extra_label
// - extra_filters[]
func getExportParams(r *http.Request, startTime time.Time) (*commonParams, error) {
	cp, err := getCommonParams(r, startTime, true)
	if err != nil {
		return nil, err
	}
	cp.deadline = searchutils.GetDeadlineForExport(r, startTime)
	return cp, nil
}

func getCommonParamsForLabelsAPI(r *http.Request, startTime time.Time, requireNonEmptyMatch bool) (*commonParams, error) {
	cp, err := getCommonParamsInternal(r, startTime, requireNonEmptyMatch, true)
	if err != nil {
		return nil, err
	}
	if cp.start == 0 {
		cp.start = cp.end - defaultStep
	}
	cp.deadline = searchutils.GetDeadlineForLabelsAPI(r, startTime)
	return cp, nil
}

// getCommonParams obtains common params from r, which are used in /api/v1/* handlers:
//
// - timeout
// - start
// - end
// - match[]
// - extra_label
// - extra_filters[]
func getCommonParams(r *http.Request, startTime time.Time, requireNonEmptyMatch bool) (*commonParams, error) {
	return getCommonParamsInternal(r, startTime, requireNonEmptyMatch, false)
}

func getCommonParamsInternal(r *http.Request, startTime time.Time, requireNonEmptyMatch, isLabelsAPI bool) (*commonParams, error) {
	deadline := searchutils.GetDeadlineForQuery(r, startTime)
	start, err := httputils.GetTime(r, "start", 0)
	if err != nil {
		return nil, err
	}
	ct := startTime.UnixNano() / 1e6
	end, err := httputils.GetTime(r, "end", ct)
	if err != nil {
		return nil, err
	}
	// Limit the `end` arg to the current time +2 days in the same way
	// as it is limited during data ingestion.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/ea06d2fd3ccbbb6aa4480ab3b04f7b671408be2a/lib/storage/table.go#L378
	// This should fix possible timestamp overflow - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2669
	maxTS := startTime.UnixNano()/1e6 + 2*24*3600*1000
	if end > maxTS {
		end = maxTS
	}
	if end < start {
		end = start
	}
	matches := append([]string{}, r.Form["match[]"]...)
	matches = append(matches, r.Form["match"]...)
	if requireNonEmptyMatch && len(matches) == 0 {
		return nil, fmt.Errorf("missing `match[]` arg")
	}
	filterss, err := getTagFilterssFromMatches(matches)
	if err != nil {
		return nil, err
	}

	if len(filterss) > 0 || !isLabelsAPI || !*ignoreExtraFiltersAtLabelsAPI {
		// If matches isn't empty, then there is no sense in ignoring extra filters
		// even if ignoreExtraLabelsAtLabelsAPI is set, since extra filters won't slow down
		// the query - they can only improve query performance by reducing the number
		// of matching series at the storage level.
		etfs, err := searchutils.GetExtraTagFilters(r)
		if err != nil {
			return nil, err
		}
		filterss = searchutils.JoinTagFilterss(filterss, etfs)
	}

	cp := &commonParams{
		deadline:         deadline,
		start:            start,
		end:              end,
		currentTimestamp: ct,
		filterss:         filterss,
	}
	return cp, nil
}

type scalableWriter struct {
	bw *bufferedwriter.Writer
	m  sync.Map
}

func newScalableWriter(bw *bufferedwriter.Writer) *scalableWriter {
	return &scalableWriter{
		bw: bw,
	}
}

func (sw *scalableWriter) getBuffer(workerID uint) *bytesutil.ByteBuffer {
	v, ok := sw.m.Load(workerID)
	if !ok {
		v = &bytesutil.ByteBuffer{}
		sw.m.Store(workerID, v)
	}
	return v.(*bytesutil.ByteBuffer)
}

func (sw *scalableWriter) maybeFlushBuffer(bb *bytesutil.ByteBuffer) error {
	if len(bb.B) < 1024*1024 {
		return nil
	}
	_, err := sw.bw.Write(bb.B)
	bb.Reset()
	return err
}

func (sw *scalableWriter) flush() error {
	sw.m.Range(func(_, v any) bool {
		bb := v.(*bytesutil.ByteBuffer)
		_, err := sw.bw.Write(bb.B)
		return err == nil
	})
	return sw.bw.Flush()
}
