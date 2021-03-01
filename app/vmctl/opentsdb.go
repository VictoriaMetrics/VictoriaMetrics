package main

import (
	"fmt"
	"log"
	//"sync"
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

	//seriesCh := make(chan *opentsdb.Meta)
	//errCh := make(chan error)
	//var wg sync.WaitGroup
	//wg.Add(op.otsdbcc)
	startTime := time.Now().Unix()
	for _, metric := range metrics {
		log.Println(fmt.Sprintf("Starting work on %s", metric))
		serieslist, err := op.oc.FindSeries(metric)
		if err != nil {
			return fmt.Errorf("couldn't retrieve series list for %s : %s", metric, err)
		}
		bar := pb.StartNew(len(serieslist))
		// log.Println(fmt.Sprintf("Found %d series for %s", len(serieslist), metric))
		/*for _, series := range serieslist {
			seriesCh <- series
		}*/
		for _, series := range serieslist {
			for _, rt := range op.oc.Retentions {
				for _, tr := range rt.QueryRanges {
					err = op.do(series, rt, tr, startTime)
					if err != nil {
						return fmt.Errorf("couldn't retrieve series for %s : %s", metric, err)
					}
					// log.Println(fmt.Sprintf("Processed %d-%d for %s", tr.Start, tr.End, series))
					/*for i := 0; i < op.otsdbcc; i++ {
						defer wg.Done()

					}*/
				}
			}
			bar.Increment()
		}
		bar.Finish()
	}
	log.Println("Import finished!")
	return nil
}

func (op *otsdbProcessor) do(seriesMeta opentsdb.Meta, rt opentsdb.Retention, tr opentsdb.TimeRange, now int64) error {
	start := now - tr.Start
	end := now - tr.End
	data, err := op.oc.GetData(seriesMeta, rt, start, end)
	if err != nil {
		return fmt.Errorf("failed to collect data for %v in %v:%v", seriesMeta, rt, tr)
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
