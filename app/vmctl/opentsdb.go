package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/cheggaaa/pb/v3"
)

type otsdbProcessor struct {
	oc      *opentsdb.Client
	im      *vm.Importer
	otsdbcc int
	vmcc    int
}

type queryObj struct {
	Series    opentsdb.Meta
	Rt        opentsdb.Retention
	Tr        opentsdb.TimeRange
	StartTime int64
}

func newOtsdbProcessor(oc *opentsdb.Client, im *vm.Importer, otsdbcc int, vmcc int) *otsdbProcessor {
	if otsdbcc < 1 {
		otsdbcc = 1
	}
	if vmcc < 1 {
		vmcc = 1
	}
	return &otsdbProcessor{
		oc:      oc,
		im:      im,
		otsdbcc: otsdbcc,
		vmcc:    vmcc,
	}
}

func (op *otsdbProcessor) run(silent bool) error {
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
	if !silent && !prompt(question) {
		return nil
	}

	startTime := time.Now().Unix()
	queryRanges := 0
	for _, rt := range op.oc.Retentions {
		queryRanges += len(rt.QueryRanges)
	}
	for _, metric := range metrics {
		log.Println(fmt.Sprintf("Starting work on %s", metric))
		serieslist, err := op.oc.FindSeries(metric)
		if err != nil {
			return fmt.Errorf("couldn't retrieve series list for %s : %s", metric, err)
		}
		/*
			Create channels for collecting/processing series and errors
			We'll create them per metric to reduce pressure against OpenTSDB
		*/
		seriesCh := make(chan queryObj)
		errCh := make(chan error)
		bar := pb.StartNew(len(serieslist) * queryRanges)
		var wg sync.WaitGroup
		wg.Add(op.otsdbcc)
		for i := 0; i < op.otsdbcc; i++ {
			go func() {
				defer wg.Done()
				for s := range seriesCh {
					if err := op.do(s); err != nil {
						errCh <- fmt.Errorf("couldn't retrieve series for %s : %s", metric, err)
						return
					}
					bar.Increment()
				}
			}()
		}
		// log.Println(fmt.Sprintf("Found %d series for %s", len(serieslist), metric))
		for _, series := range serieslist {
			for _, rt := range op.oc.Retentions {
				for _, tr := range rt.QueryRanges {
					select {
					case otsdbErr := <-errCh:
						return fmt.Errorf("opentsdb error: %s", otsdbErr)
					case vmErr := <-op.im.Errors():
						return fmt.Errorf("Import process failed: \n%s", wrapErr(vmErr))
					default:
						seriesCh <- queryObj{
							Series: series, Rt: rt,
							Tr: tr, StartTime: startTime}
					}
				}
			}
		}
		// Drain channels per metric
		close(seriesCh)
		close(errCh)
		wg.Wait()
		op.im.Close()
		for vmErr := range op.im.Errors() {
			return fmt.Errorf("Import process failed: \n%s", wrapErr(vmErr))
		}
		bar.Finish()
	}
	log.Println("Import finished!")
	return nil
}

func (op *otsdbProcessor) do(s queryObj) error {

	start := s.StartTime - s.Tr.Start
	end := s.StartTime - s.Tr.End
	data, err := op.oc.GetData(s.Series, s.Rt, start, end)
	if err != nil {
		return fmt.Errorf("failed to collect data for %v in %v:%v", s.Series, s.Rt, s.Tr)
	}
	if len(data.Timestamps) < 1 {
		return nil
	}
	// log.Println("Found %d stats for %v", len(data.Timestamps), seriesMeta)
	labels := make([]vm.LabelPair, len(data.Tags))
	for k, v := range data.Tags {
		labels = append(labels, vm.LabelPair{Name: k, Value: v})
	}
	op.im.Input() <- &vm.TimeSeries{
		Name:       data.Metric,
		LabelPairs: labels,
		Timestamps: data.Timestamps,
		Values:     data.Values,
	}
	return nil
}
