package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/remoteread"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/prometheus/prometheus/prompb"
)

type remotereadProcessor struct {
	filter remoteReadFilter

	dst *vm.Importer
	src *remoteread.Client

	cc            int
	checkSrcAlive bool
}

type remoteReadFilter struct {
	label      string
	labelValue string
	timeStart  *time.Time
	timeEnd    *time.Time
	chunk      string
}

func (rrp *remotereadProcessor) run(ctx context.Context, silent, verbose bool) error {
	if rrp.filter.timeEnd == nil {
		t := time.Now()
		rrp.filter.timeEnd = &t
	}
	ranges, err := stepper.SplitDateRange(*rrp.filter.timeStart, *rrp.filter.timeEnd, rrp.filter.chunk)
	if err != nil {
		return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
	}

	question := fmt.Sprintf("Split defined times into %d ranges to import. Continue?", len(ranges))
	if !silent && !prompt(question) {
		return nil
	}

	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing ranges"), len(ranges))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer func() {
		barpool.Stop()
		log.Println("Import finished!")
		log.Print(rrp.dst.Stats())
	}()

	rangeC := make(chan *remoteread.Filter)
	errCh := make(chan error)
	rrp.dst.ResetStats()

	// Some remote storages doesn't support prometheus health API
	if rrp.checkSrcAlive {
		if err := rrp.src.Ping(); err != nil {
			return fmt.Errorf("data source not ready: %s", err)
		}
	}

	if err := rrp.dst.Ping(); err != nil {
		return fmt.Errorf("destination source not ready: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(rrp.cc)
	for i := 0; i < rrp.cc; i++ {
		go func() {
			defer wg.Done()
			for r := range rangeC {
				if err := rrp.do(ctx, r); err != nil {
					errCh <- fmt.Errorf("request failed for: %s", err)
					return
				}
				bar.Increment()
			}
		}()
	}

	for _, r := range ranges {
		select {
		case infErr := <-errCh:
			return fmt.Errorf("remote read error: %s", infErr)
		case vmErr := <-rrp.dst.Errors():
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		case rangeC <- &remoteread.Filter{
			StartTimestampMs: r[0].UnixMilli(),
			EndTimestampMs:   r[1].UnixMilli(),
			Label:            rrp.filter.label,
			LabelValue:       rrp.filter.labelValue}:
		}
	}

	close(rangeC)
	wg.Wait()
	rrp.dst.Close()
	close(errCh)
	// drain import errors channel
	for vmErr := range rrp.dst.Errors() {
		if vmErr.Err != nil {
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	return nil
}

func (rrp *remotereadProcessor) do(ctx context.Context, filter *remoteread.Filter) error {
	return rrp.src.Read(ctx, filter, func(series prompb.TimeSeries) error {
		ts := convertTimeseries(series)
		if err := rrp.dst.Input(ts); err != nil {
			return fmt.Errorf(
				"failed to read data for time range start: %d, end: %d, %s",
				filter.StartTimestampMs, filter.EndTimestampMs, err)
		}
		return nil
	})
}

func convertTimeseries(series prompb.TimeSeries) *vm.TimeSeries {
	labelPairs := make([]vm.LabelPair, 0, len(series.Labels))
	for _, label := range series.Labels {
		labelPairs = append(labelPairs, vm.LabelPair{Name: label.Name, Value: label.Value})
	}

	n := len(series.Samples)
	values := make([]float64, 0, n)
	timestamps := make([]int64, 0, n)
	for _, sample := range series.Samples {
		values = append(values, sample.Value)
		timestamps = append(timestamps, sample.Timestamp)
	}

	return &vm.TimeSeries{
		LabelPairs: labelPairs,
		Timestamps: timestamps,
		Values:     values,
	}
}
