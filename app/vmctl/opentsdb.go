package main

import (
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/cheggaaa/pb/v3"
)

type otsdbProcessor struct {
	oc        *opentsdb.Client
	im        *vm.Importer
	otsdbcc	int
	vmcc        int
}

func newOtsdbProcessor(oc *opentsdb.Client, im *vm.Importer, otsdbcc int, vmcc int) *otsdbProcessor {
	if otsdbcc < 1 {
		otsdbcc = 1
	}
	if vmcc < 1 {
		vmcc = 1
	}
	return &otsdbProcessor{
		oc:        oc,
		im:        im,
		otsdbcc:        otsdbcc,
		vmcc:        vmcc,
	}
}


func (op *otsdbProcessor) run(silent bool) error {
	var metrics []string
	for _, filter := range op.oc.Filters {
		m, err := op.oc.FindMetrics(filter)
		if err != nil {
			return fmt.Errorf("metric discovery failed: %s", err)
		}
		for _, mt := range m {
			metrics = append(metrics, mt)
		}
	}
	if len(metrics) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}

	question := fmt.Sprintf("Found %d metrics to import. Continue?", len(metrics))
	if !silent && !prompt(question) {
		return nil
	}

	bar := pb.StartNew(len(metrics))

	seriesCh := make(chan *opentsdb.Meta)
	errCh := make(chan error)
	var wg sync.WaitGroup
	wg.Add(op.otsdbcc)
	for _, metric := range metrics {
	}
	return nil
}
	/*
	seriesCh := make(chan *influx.Series)
	errCh := make(chan error)
	ip.im.ResetStats()

	var wg sync.WaitGroup
	wg.Add(op.cc)
	for i := 0; i < op.cc; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if err := op.do(s); err != nil {
					errCh <- fmt.Errorf("request failed for %q.%q: %s", s.Measurement, s.Field, err)
					return
				}
				bar.Increment()
			}
		}()
	}

	// any error breaks the import
	for _, s := range series {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("influx error: %s", infErr)
		case vmErr := <-ip.im.Errors():
			return fmt.Errorf("Import process failed: \n%s", wrapErr(vmErr))
		case seriesCh <- s:
		}
	}

	close(seriesCh)
	wg.Wait()
	ip.im.Close()
	// drain import errors channel
	for vmErr := range ip.im.Errors() {
		return fmt.Errorf("Import process failed: \n%s", wrapErr(vmErr))
	}
	bar.Finish()
	log.Println("Import finished!")
	log.Print(ip.im.Stats())
	return nil
}

func (ip *influxProcessor) do(s *influx.Series) error {
	cr, err := ip.ic.FetchDataPoints(s)
	if err != nil {
		return fmt.Errorf("failed to fetch datapoints: %s", err)
	}
	defer func() {
		_ = cr.Close()
	}()
	var name string
	if s.Measurement != "" {
		name = fmt.Sprintf("%s%s%s", s.Measurement, ip.separator, s.Field)
	} else {
		name = s.Field
	}

	labels := make([]vm.LabelPair, len(s.LabelPairs))
	var containsDBLabel bool
	for i, lp := range s.LabelPairs {
		if lp.Name == dbLabel {
			containsDBLabel = true
			break
		}
		labels[i] = vm.LabelPair{
			Name:  lp.Name,
			Value: lp.Value,
		}
	}
	if !containsDBLabel {
		labels = append(labels, vm.LabelPair{
			Name:  dbLabel,
			Value: ip.ic.Database(),
		})
	}

	for {
		time, values, err := cr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		// skip empty results
		if len(time) < 1 {
			continue
		}
		ip.im.Input() <- &vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: time,
			Values:     values,
		}
	}
}*/
