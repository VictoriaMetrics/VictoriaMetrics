package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

var (
	influxAddr         = flag.String("influx-addr", "http://localhost:8086", "InfluxDB server addr")
	influxUser         = flag.String("influx-user", "", "InfluxDB user")
	influxPassword     = flag.String("influx-password", "", "InfluxDB user password")
	influxDB           = flag.String("influx-database", "", "InfluxDB database")
	influxRetention    = flag.String("influx-retention-policy", "", "InfluxDB retention policy")
	influxChunkSize    = flag.Int("influx-chunk-size", 10_000, "The chunkSize defines max amount of series to be returned in one chunk")
	influxConcurrency  = flag.Int("influx-concurrency", 1, "Number of concurrently running fetch queries to InfluxDB")
	influxFilterSeries = flag.String("influx-filter-series", "", "InfluxDB filter expression to select series. E.g. \"from cpu where arch='x86' AND hostname='host_2753'\".\n"+
		"See for details https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#show-series")

	influxFilterTimeStart           = flag.String("influx-filter-time-start", "", "The time filter to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'")
	influxFilterTimeEnd             = flag.String("influx-filter-time-end", "", "The time filter to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'")
	influxMeasurementFieldSeparator = flag.String("influx-measurement-field-separator", "_", "The {separator} symbol used to concatenate {measurement} and {field} names into series name {measurement}{separator}{field}.")
	influxSkipDatabaseLabel         = flag.Bool("influx-skip-database-label", false, "Whether to skip adding the label 'db' to timeseries.")
	influxPrometheusMode            = flag.Bool("influx-prometheus-mode", false, "Whether to restore the original timeseries name previously written from Prometheus to InfluxDB v1 via remote_write.")
)

type influxProcessor struct {
	ic          *influx.Client
	im          *vm.Importer
	cc          int
	separator   string
	skipDbLabel bool
	promMode    bool
}

func newInfluxProcessor(ic *influx.Client, im *vm.Importer, cc int, separator string, skipDbLabel bool, promMode bool) *influxProcessor {
	if cc < 1 {
		cc = 1
	}
	return &influxProcessor{
		ic:          ic,
		im:          im,
		cc:          cc,
		separator:   separator,
		skipDbLabel: skipDbLabel,
		promMode:    promMode,
	}
}

func (ip *influxProcessor) run(silent, verbose bool) error {
	series, err := ip.ic.Explore()
	if err != nil {
		return fmt.Errorf("explore query failed: %s", err)
	}
	if len(series) < 1 {
		return fmt.Errorf("found no timeseries to import")
	}

	question := fmt.Sprintf("Found %d timeseries to import. Continue?", len(series))
	if !silent && !prompt(question) {
		return nil
	}

	bar := barpool.AddWithTemplate(fmt.Sprintf(barTpl, "Processing series"), len(series))
	if err := barpool.Start(); err != nil {
		return err
	}
	defer barpool.Stop()

	seriesCh := make(chan *influx.Series)
	errCh := make(chan error)
	ip.im.ResetStats()

	var wg sync.WaitGroup
	wg.Add(ip.cc)
	for i := 0; i < ip.cc; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if err := ip.do(s); err != nil {
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
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		case seriesCh <- s:
		}
	}

	close(seriesCh)
	wg.Wait()
	ip.im.Close()
	close(errCh)
	// drain import errors channel
	for vmErr := range ip.im.Errors() {
		if vmErr.Err != nil {
			return fmt.Errorf("import process failed: %s", wrapErr(vmErr, verbose))
		}
	}
	for err := range errCh {
		return fmt.Errorf("import process failed: %s", err)
	}

	log.Println("Import finished!")
	log.Print(ip.im.Stats())
	return nil
}

const dbLabel = "db"
const nameLabel = "__name__"
const valueField = "value"

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
		} else if lp.Name == nameLabel && s.Field == valueField && ip.promMode {
			name = lp.Value
		}
		labels[i] = vm.LabelPair{
			Name:  lp.Name,
			Value: lp.Value,
		}
	}
	if !containsDBLabel && !ip.skipDbLabel {
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
		ts := vm.TimeSeries{
			Name:       name,
			LabelPairs: labels,
			Timestamps: time,
			Values:     values,
		}
		if err := ip.im.Input(&ts); err != nil {
			return err
		}
	}
}

func influxImporter([]string) {
	fmt.Println("InfluxDB import mode")

	ctx, cancel := context.WithCancel(context.Background())
	signalHandler(cancel)

	if *influxDB == "" {
		logger.Fatalf("flag --influx-database cannot be empty")
	}

	iCfg := influx.Config{
		Addr:      *influxAddr,
		Username:  *influxUser,
		Password:  *influxPassword,
		Database:  *influxDB,
		Retention: *influxRetention,
		Filter: influx.Filter{
			Series:    *influxFilterSeries,
			TimeStart: *influxFilterTimeStart,
			TimeEnd:   *influxFilterTimeEnd,
		},
		ChunkSize: *influxChunkSize,
	}
	influxClient, err := influx.NewClient(iCfg)
	if err != nil {
		logger.Fatalf("failed to create influx client: %s", err)
	}

	vmCfg := initConfigVM()
	importer, err := vm.NewImporter(vmCfg)
	if err != nil {
		logger.Fatalf("failed to create VM importer: %s", err)
	}

	go func() {
		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			logger.Errorf("context cancel err: %s\n", err)
		}
		importer.Close()
	}()

	processor := newInfluxProcessor(
		influxClient,
		importer,
		*influxConcurrency,
		*influxMeasurementFieldSeparator,
		*influxSkipDatabaseLabel,
		*influxPrometheusMode)

	if err := processor.run(*globalSilent, *globalVerbose); err != nil {
		logger.Fatalf("error run influx import processor: %s", err)
	}
}
