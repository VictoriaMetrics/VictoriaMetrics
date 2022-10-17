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
	filter    remoteReadFilter
	rateLimit int64

	dst *vm.Importer
	src *remoteread.Client

	cc int
}

type remoteReadFilter struct {
	label      string
	labelValue string
	timeStart  string
	timeEnd    string
	chunk      string
}

func (f remoteReadFilter) startTimeParsed() (*time.Time, error) {
	startOfRange, err := time.Parse(time.RFC3339, f.timeStart)
	if err != nil {
		return nil, err
	}
	return &startOfRange, nil
}

func (f remoteReadFilter) endTimeParsed() (*time.Time, error) {
	if f.timeEnd == "" {
		t := time.Now()
		return &t, nil
	}
	endOfRange, err := time.Parse(time.RFC3339, f.timeEnd)
	if err != nil {
		return nil, err
	}
	return &endOfRange, nil
}

func (rrp *remotereadProcessor) run(ctx context.Context, silent, verbose bool) error {

	startOfRange, err := rrp.filter.startTimeParsed()
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeStart, rrp.filter.timeStart, time.RFC3339, err)
	}

	endOfRange, err := rrp.filter.endTimeParsed()
	if err != nil {
		return fmt.Errorf("failed to parse %s, provided: %s, expected format: %s, error: %v", vmNativeFilterTimeEnd, rrp.filter.timeEnd, time.RFC3339, err)
	}

	ranges, err := stepper.SplitDateRange(*startOfRange, *endOfRange, rrp.filter.chunk)
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
			Min:        r[0].UnixMilli(),
			Max:        r[1].UnixMilli(),
			Label:      rrp.filter.label,
			LabelValue: rrp.filter.labelValue}:
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
		imts := convertTimeseries(series)
		if err := rrp.dst.Input(imts); err != nil {
			tStart := time.Unix(0, filter.Min*int64(time.Millisecond))
			tEnd := time.Unix(0, filter.Max*int64(time.Millisecond))
			return fmt.Errorf("failed to read data for time range start: %s, end: %s, %s", tStart, tEnd, err)
		}
		return nil
	})
}

func convertTimeseries(series prompb.TimeSeries) *vm.TimeSeries {
	var ts vm.TimeSeries
	for _, label := range series.Labels {
		ts.LabelPairs = append(ts.LabelPairs, vm.LabelPair{Name: label.Name, Value: label.Value})
	}
	for _, sample := range series.Samples {
		ts.Values = append(ts.Values, sample.Value)
		ts.Timestamps = append(ts.Timestamps, sample.Timestamp)
	}
	return &ts
}
