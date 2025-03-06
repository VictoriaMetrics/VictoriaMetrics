package graphite

import (
	"flag"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

var maxGraphiteSeries = flag.Int("search.maxGraphiteSeries", 300e3, "The maximum number of time series, which can be scanned during queries to Graphite Render API. "+
	"See https://docs.victoriametrics.com/#graphite-render-api-usage")

type evalConfig struct {
	startTime   int64
	endTime     int64
	storageStep int64
	deadline    searchutils.Deadline

	currentTime time.Time

	// xFilesFactor is used for determining when consolidateFunc must be applied.
	//
	// 0 means that consolidateFunc should be applied if at least a single non-NaN data point exists on the given step.
	// 1 means that consolidateFunc should be applied if all the data points are non-NaN on the given step.
	xFilesFactor float64

	// Enforced tag filters
	etfs [][]storage.TagFilter

	// originalQuery contains the original query - used for debug logging.
	originalQuery string
}

func (ec *evalConfig) pointsLen(step int64) int {
	return int((ec.endTime - ec.startTime) / step)
}

func (ec *evalConfig) newTimestamps(step int64) []int64 {
	pointsLen := ec.pointsLen(step)
	timestamps := make([]int64, pointsLen)
	ts := ec.startTime
	for i := 0; i < pointsLen; i++ {
		timestamps[i] = ts
		ts += step
	}
	return timestamps
}

type series struct {
	Name       string
	Tags       map[string]string
	Timestamps []int64
	Values     []float64

	// holds current path expression like graphite does.
	pathExpression string

	expr graphiteql.Expr

	// consolidateFunc is applied to raw samples in order to generate data points algined to the given step.
	// see series.consolidate() function for details.
	consolidateFunc aggrFunc

	// xFilesFactor is used for determining when consolidateFunc must be applied.
	//
	// 0 means that consolidateFunc should be applied if at least a single non-NaN data point exists on the given step.
	// 1 means that consolidateFunc should be applied if all the data points are non-NaN on the given step.
	xFilesFactor float64

	step int64
}

func (s *series) consolidate(ec *evalConfig, step int64) {
	aggrFunc := s.consolidateFunc
	if aggrFunc == nil {
		aggrFunc = aggrAvg
	}
	xFilesFactor := s.xFilesFactor
	if s.xFilesFactor <= 0 {
		xFilesFactor = ec.xFilesFactor
	}
	s.summarize(aggrFunc, ec.startTime, ec.endTime, step, xFilesFactor)
}

func (s *series) summarize(aggrFunc aggrFunc, startTime, endTime, step int64, xFilesFactor float64) {
	pointsLen := int((endTime - startTime) / step)
	timestamps := s.Timestamps
	values := s.Values
	dstTimestamps := make([]int64, 0, pointsLen)
	dstValues := make([]float64, 0, pointsLen)
	ts := startTime
	i := 0
	for len(dstTimestamps) < pointsLen {
		tsEnd := ts + step
		j := i
		for j < len(timestamps) && timestamps[j] < tsEnd {
			j++
		}
		if i == j && i > 0 && ts-timestamps[i-1] <= 2000 {
			// The current [ts ... tsEnd) interval has no samples,
			// but the last sample on the previous interval [ts - step ... ts)
			// is closer than 2 seconds to the current interval.
			// Let's consider that this sample belongs to the current interval,
			// since such discrepancy could appear because of small jitter in samples' ingestion.
			i--
		}
		v := aggrFunc.apply(xFilesFactor, values[i:j])
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, v)
		ts = tsEnd
		i = j
	}
	// Do not reuse s.Timestamps and s.Values, since they can be too big
	s.Timestamps = dstTimestamps
	s.Values = dstValues
	s.step = step
}

func execExpr(ec *evalConfig, query string) (nextSeriesFunc, error) {
	expr, err := graphiteql.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", query, err)
	}
	return evalExpr(ec, expr)
}

func evalExpr(ec *evalConfig, expr graphiteql.Expr) (nextSeriesFunc, error) {
	switch t := expr.(type) {
	case *graphiteql.MetricExpr:
		return evalMetricExpr(ec, t)
	case *graphiteql.FuncExpr:
		return evalFuncExpr(ec, t)
	default:
		return nil, fmt.Errorf("unexpected expression type %T; want graphiteql.MetricExpr or graphiteql.FuncExpr; expr: %q", t, t.AppendString(nil))
	}
}

func evalMetricExpr(ec *evalConfig, me *graphiteql.MetricExpr) (nextSeriesFunc, error) {
	tfs := []storage.TagFilter{{
		Key:   []byte("__graphite__"),
		Value: []byte(me.Query),
	}}
	tfss := joinTagFilterss(tfs, ec.etfs)
	sq := storage.NewSearchQuery(ec.startTime, ec.endTime, tfss, *maxGraphiteSeries)
	return newNextSeriesForSearchQuery(ec, sq, me)
}

func newNextSeriesForSearchQuery(ec *evalConfig, sq *storage.SearchQuery, expr graphiteql.Expr) (nextSeriesFunc, error) {
	rss, err := netstorage.ProcessSearchQuery(nil, sq, ec.deadline)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch data for %q: %w", sq, err)
	}
	seriesCh := make(chan *series, cgroup.AvailableCPUs())
	errCh := make(chan error, 1)
	go func() {
		err := rss.RunParallel(nil, func(rs *netstorage.Result, _ uint) error {
			nameWithTags := getCanonicalPath(&rs.MetricName)
			tags := unmarshalTags(nameWithTags)
			s := &series{
				Name:           tags["name"],
				Tags:           tags,
				Timestamps:     append([]int64{}, rs.Timestamps...),
				Values:         append([]float64{}, rs.Values...),
				expr:           expr,
				pathExpression: string(expr.AppendString(nil)),
			}
			s.summarize(aggrAvg, ec.startTime, ec.endTime, ec.storageStep, 0)
			t := timerpool.Get(30 * time.Second)
			select {
			case seriesCh <- s:
			case <-t.C:
				logger.Errorf("resource leak when processing the %s (full query: %s); please report this error to VictoriaMetrics developers",
					expr.AppendString(nil), ec.originalQuery)
			}
			timerpool.Put(t)
			return nil
		})
		close(seriesCh)
		errCh <- err
	}()
	f := func() (*series, error) {
		s := <-seriesCh
		if s != nil {
			return s, nil
		}
		err := <-errCh
		return nil, err
	}
	return f, nil
}

func evalFuncExpr(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	// Do not lowercase the fe.FuncName, since Graphite function names are case-sensitive.
	tf := transformFuncs[fe.FuncName]
	if tf == nil {
		return nil, fmt.Errorf("unknown function %q", fe.FuncName)
	}
	nextSeries, err := tf(ec, fe)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate %s: %w", fe.AppendString(nil), err)
	}
	return nextSeries, nil
}
