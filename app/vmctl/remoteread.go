package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/remoteread"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/cheggaaa/pb/v3"
)

var (
	remoteRead                 = flag.Bool("remote-read", false, "Use Prometheus remote read protocol")
	remoteReadUseStream        = flag.Bool("remote-read-use-stream", false, "Defines whether to use SAMPLES or STREAMED_XOR_CHUNKS mode. By default is uses SAMPLES mode. See https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#streamed-chunks")
	remoteReadConcurrency      = flag.Int("remote-read-concurrency", 1, "Number of concurrently running remote read readers")
	remoteReadFilterTimeStart  = flagutil.NewTime("remote-read-filter-time-start", "", "The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'")
	remoteReadFilterTimeEnd    = flagutil.NewTime("remote-read-filter-time-end", "", "The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'")
	remoteReadFilterLabel      = flag.String("remote-read-filter-label", "__name__", "Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.")
	remoteReadFilterLabelValue = flag.String("remote-read-filter-label-value", ".*", "Prometheus regular expression to filter label from \"remote-read-filter-label\" flag.")
	remoteReadStepInterval     = flag.String("remote-read-step-interval", "", fmt.Sprintf("Split export data into chunks. Requires setting --%s. Valid values are %q,%q,%q,%q.", "remote-read-filter-time-start", stepper.StepMonth, stepper.StepDay, stepper.StepHour, stepper.StepMinute))
	remoteReadSrcAddr          = flag.String("remote-read-src-addr", "", "Remote read address to perform read from.")
	remoteReadUser             = flag.String("remote-read-user", "", "Remote read username for basic auth")
	remoteReadPassword         = flag.String("remote-read-password", "", "Remote read password for basic auth")
	remoteReadHTTPTimeout      = flag.Duration("remote-read-http-timeout", 0, "Timeout defines timeout for HTTP write request to remote storage")
	remoteReadHeaders          = flag.String("remote-read-headers", "", "Optional HTTP headers to send with each request to the corresponding remote source storage \n"+
		"For example, --remote-read-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding remote source storage. \n"+
		"Multiple headers must be delimited by '^^': --remote-read-headers='header1:value1^^header2:value2'")
	remoteReadInsecureSkipVerify = flag.Bool("remote-read-insecure-skip-verify", false, "Whether to skip TLS certificate verification when connecting to the remote read address")
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

func remoteReadImport(ctx context.Context, importer *vm.Importer) flagutil.Action {
	return func(args []string) {
		err := flagutil.SetFlagsFromEnvironment()
		if err != nil {
			logger.Fatalf("error set flags from environment variables: %s", err)
		}

		if *remoteReadSrcAddr == "" {
			logger.Fatalf("flag --remote-read-src-addr cannot be empty")
		}
		if *remoteReadStepInterval == "" {
			logger.Fatalf("flag --remote-read-step-interval cannot be empty")
		}

		rr, err := remoteread.NewClient(remoteread.Config{
			Addr:               *remoteReadSrcAddr,
			Username:           *remoteReadUser,
			Password:           *remoteReadPassword,
			Timeout:            *remoteReadHTTPTimeout,
			UseStream:          *remoteReadUseStream,
			Headers:            *remoteReadHeaders,
			LabelName:          *remoteReadFilterLabel,
			LabelValue:         *remoteReadFilterLabelValue,
			InsecureSkipVerify: *remoteReadInsecureSkipVerify,
		})
		if err != nil {
			logger.Fatalf("error create remote read client: %s", err)
		}

		vmCfg := initConfigVM()

		importer, err = vm.NewImporter(vmCfg)
		if err != nil {
			logger.Fatalf("failed to create VM importer: %s", err)
		}

		remoteReadFilterTimeStart.SetLayout(time.RFC3339)
		remoteReadFilterTimeEnd.SetLayout(time.RFC3339)

		rmp := remoteReadProcessor{
			src: rr,
			dst: importer,
			filter: remoteReadFilter{
				timeStart: remoteReadFilterTimeStart.Timestamp,
				timeEnd:   remoteReadFilterTimeEnd.Timestamp,
				chunk:     *remoteReadStepInterval,
			},
			cc: *remoteReadConcurrency,
		}
		if err := rmp.run(ctx, *globalSilent, *globalVerbose); err != nil {
			logger.Fatalf("error run import via remote read protocol: %s", err)
		}
	}
}
