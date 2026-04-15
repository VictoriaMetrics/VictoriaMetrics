package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	vmetrics "github.com/VictoriaMetrics/metrics"
	"github.com/cheggaaa/pb/v3"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

type otsdbProcessor struct {
	oc        *opentsdb.Client
	im        *vm.Importer
	otsdbcc   int
	isVerbose bool
}

type queryObj struct {
	Series    opentsdb.Meta
	Rt        opentsdb.RetentionMeta
	Tr        opentsdb.TimeRange
	StartTime int64
}

func newOtsdbProcessor(oc *opentsdb.Client, im *vm.Importer, otsdbcc int, verbose bool) *otsdbProcessor {
	if otsdbcc < 1 {
		otsdbcc = 1
	}
	return &otsdbProcessor{
		oc:        oc,
		im:        im,
		otsdbcc:   otsdbcc,
		isVerbose: verbose,
	}
}

func (op *otsdbProcessor) run(ctx context.Context) error {
	log.Println("Loading all metrics from OpenTSDB for filters: ", op.oc.Filters)
	var metrics []string
	for _, filter := range op.oc.Filters {
		q := fmt.Sprintf("%s/api/suggest?type=metrics&q=%s&max=%d", op.oc.Addr, filter, op.oc.Limit)
		m, err := op.oc.FindMetrics(q)
		if err != nil {
			return fmt.Errorf("metric discovery failed for %q: %s", q, err)
		}
		metrics = append(metrics, m...)
	}
	if len(metrics) < 1 {
		return fmt.Errorf("found no timeseries to import with filters %q", op.oc.Filters)
	}

	question := fmt.Sprintf("Found %d metrics to import. Continue?", len(metrics))
	if !prompt(ctx, question) {
		return nil
	}

	op.im.ResetStats()
	var startTime int64
	if op.oc.HardTS != 0 {
		startTime = op.oc.HardTS
	} else {
		startTime = time.Now().Unix()
	}
	queryRanges := 0
	// pre-calculate the number of query ranges we'll be processing
	for _, rt := range op.oc.Retentions {
		queryRanges += len(rt.QueryRanges)
	}
	for _, metric := range metrics {
		log.Printf("Starting work on %s", metric)
		serieslist, err := op.oc.FindSeries(metric)
		if err != nil {
			return fmt.Errorf("couldn't retrieve series list for %s : %s", metric, err)
		}
		/*
			Create channels for collecting/processing series and errors
			We'll create them per metric to reduce pressure against OpenTSDB

			Limit the size of seriesCh so we can't get too far ahead of actual processing
		*/
		seriesCh := make(chan queryObj, op.otsdbcc)
		errCh := make(chan error)
		// we're going to make serieslist * queryRanges queries, so we should represent that in the progress bar
		otsdbSeriesTotal.Add(len(serieslist) * queryRanges)
		bar := pb.StartNew(len(serieslist) * queryRanges)
		var wg sync.WaitGroup
		for range op.otsdbcc {
			wg.Go(func() {
				for s := range seriesCh {
					if err := op.do(s); err != nil {
						otsdbErrorsTotal.Inc()
						errCh <- fmt.Errorf("couldn't retrieve series for %s : %s", metric, err)
						return
					}
					otsdbSeriesProcessed.Inc()
					bar.Increment()
				}
			})
		}
		runErr := op.sendQueries(ctx, serieslist, seriesCh, errCh, startTime)

		// Always drain channels and wait for workers to prevent goroutine leaks
		close(seriesCh)
		wg.Wait()
		close(errCh)
		// check for any lingering errors on the query side
		for otsdbErr := range errCh {
			if runErr == nil {
				runErr = fmt.Errorf("import process failed: \n%s", otsdbErr)
			}
		}
		bar.Finish()
		if runErr != nil {
			return runErr
		}
		log.Print(op.im.Stats())
	}
	op.im.Close()
	for vmErr := range op.im.Errors() {
		if vmErr.Err != nil {
			otsdbErrorsTotal.Inc()
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, op.isVerbose))
		}
	}
	log.Println("Import finished!")
	log.Print(op.im.Stats())
	return nil
}

// sendQueries iterates over all series and retention ranges, sending queries to workers.
// It returns early if ctx is canceled or an error is received.
func (op *otsdbProcessor) sendQueries(ctx context.Context, serieslist []opentsdb.Meta, seriesCh chan<- queryObj, errCh <-chan error, startTime int64) error {
	for _, series := range serieslist {
		for _, rt := range op.oc.Retentions {
			for _, tr := range rt.QueryRanges {
				select {
				case <-ctx.Done():
					return fmt.Errorf("context canceled: %s", ctx.Err())
				case otsdbErr := <-errCh:
					otsdbErrorsTotal.Inc()
					return fmt.Errorf("opentsdb error: %s", otsdbErr)
				case vmErr := <-op.im.Errors():
					return fmt.Errorf("import process failed: %s", wrapErr(vmErr, op.isVerbose))
				case seriesCh <- queryObj{
					Tr: tr, StartTime: startTime,
					Series: series, Rt: opentsdb.RetentionMeta{
						FirstOrder:  rt.FirstOrder,
						SecondOrder: rt.SecondOrder,
						AggTime:     rt.AggTime,
					}}:
				}
			}
		}
	}
	return nil
}

func (op *otsdbProcessor) do(s queryObj) error {
	start := s.StartTime - s.Tr.Start
	end := s.StartTime - s.Tr.End
	data, err := op.oc.GetData(s.Series, s.Rt, start, end, op.oc.MsecsTime)
	if err != nil {
		return fmt.Errorf("failed to collect data for %v in %v:%v :: %v", s.Series, s.Rt, s.Tr, err)
	}
	if len(data.Timestamps) < 1 || len(data.Values) < 1 {
		log.Printf("no data found for %v in %v:%v...skipping", s.Series, s.Rt, s.Tr)
		return nil
	}
	labels := make([]vm.LabelPair, 0, len(data.Tags))
	for k, v := range data.Tags {
		labels = append(labels, vm.LabelPair{Name: k, Value: v})
	}
	ts := vm.TimeSeries{
		Name:       data.Metric,
		LabelPairs: labels,
		Timestamps: data.Timestamps,
		Values:     data.Values,
	}
	return op.im.Input(&ts)
}

var (
	otsdbSeriesTotal     = vmetrics.NewCounter(`vmctl_opentsdb_migration_series_total`)
	otsdbSeriesProcessed = vmetrics.NewCounter(`vmctl_opentsdb_migration_series_processed`)
	otsdbErrorsTotal     = vmetrics.NewCounter(`vmctl_opentsdb_migration_errors_total`)
)
