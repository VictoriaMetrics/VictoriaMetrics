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
	"github.com/cheggaaa/pb/v3"
)

type remoteReadProcessor struct {
	filter remoteReadFilter

	dst *vm.Importer
	src *remoteread.Client

	cc int
}

type remoteReadFilter struct {
	timeStart *time.Time
	timeEnd   *time.Time
	chunk     string
}

func (rrp *remoteReadProcessor) run(ctx context.Context, silent, verbose bool) error {
	rrp.dst.ResetStats()
	if rrp.filter.timeEnd == nil {
		t := time.Now().In(rrp.filter.timeStart.Location())
		rrp.filter.timeEnd = &t
	}
	if rrp.cc < 1 {
		rrp.cc = 1
	}

	ranges, err := stepper.SplitDateRange(*rrp.filter.timeStart, *rrp.filter.timeEnd, rrp.filter.chunk)
	if err != nil {
		return fmt.Errorf("failed to create date ranges for the given time filters: %v", err)
	}

	question := fmt.Sprintf("Selected time range %q - %q will be split into %d ranges according to %q step. Continue?",
		rrp.filter.timeStart.String(), rrp.filter.timeEnd.String(), len(ranges), rrp.filter.chunk)
	if !silent && !prompt(question) {
		return nil
	}

	var bar *pb.ProgressBar
	if !silent {
		bar = barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing ranges"), len(ranges))
		if err := barpool.Start(); err != nil {
			return err
		}
	}
	defer func() {
		if !silent {
			barpool.Stop()
		}
		log.Println("Import finished!")
		log.Print(rrp.dst.Stats())
	}()

	rangeC := make(chan *remoteread.Filter)
	errCh := make(chan error)

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
				if bar != nil {
					bar.Increment()
				}
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
		}:
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

func (rrp *remoteReadProcessor) do(ctx context.Context, filter *remoteread.Filter) error {
	return rrp.src.Read(ctx, filter, func(series *vm.TimeSeries) error {
		if err := rrp.dst.Input(series); err != nil {
			return fmt.Errorf(
				"failed to read data for time range start: %d, end: %d, %s",
				filter.StartTimestampMs, filter.EndTimestampMs, err)
		}
		return nil
	})
}
