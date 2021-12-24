package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/urfave/cli/v2"
)

func main() {
	var (
		err      error
		importer *vm.Importer
	)

	start := time.Now()
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "VictoriaMetrics command-line tool",
		Version: buildinfo.Version,
		Commands: []*cli.Command{
			{
				Name:  "opentsdb",
				Usage: "Migrate timeseries from OpenTSDB",
				Flags: mergeFlags(globalFlags, otsdbFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("OpenTSDB import mode")

					oCfg := opentsdb.Config{
						Addr:       c.String(otsdbAddr),
						Limit:      c.Int(otsdbQueryLimit),
						Offset:     c.Int64(otsdbOffsetDays),
						HardTS:     c.Int64(otsdbHardTSStart),
						Retentions: c.StringSlice(otsdbRetentions),
						Filters:    c.StringSlice(otsdbFilters),
						Normalize:  c.Bool(otsdbNormalize),
						MsecsTime:  c.Bool(otsdbMsecsTime),
					}
					otsdbClient, err := opentsdb.NewClient(oCfg)
					if err != nil {
						return fmt.Errorf("failed to create opentsdb client: %s", err)
					}

					vmCfg := initConfigVM(c)
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					otsdbProcessor := newOtsdbProcessor(otsdbClient, importer, c.Int(otsdbConcurrency))
					return otsdbProcessor.run(c.Bool(globalSilent), c.Bool(globalVerbose))
				},
			},
			{
				Name:  "influx",
				Usage: "Migrate timeseries from InfluxDB",
				Flags: mergeFlags(globalFlags, influxFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					iCfg := influx.Config{
						Addr:      c.String(influxAddr),
						Username:  c.String(influxUser),
						Password:  c.String(influxPassword),
						Database:  c.String(influxDB),
						Retention: c.String(influxRetention),
						Filter: influx.Filter{
							Series:    c.String(influxFilterSeries),
							TimeStart: c.String(influxFilterTimeStart),
							TimeEnd:   c.String(influxFilterTimeEnd),
						},
						ChunkSize: c.Int(influxChunkSize),
					}
					influxClient, err := influx.NewClient(iCfg)
					if err != nil {
						return fmt.Errorf("failed to create influx client: %s", err)
					}

					vmCfg := initConfigVM(c)
					importer, err = vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					processor := newInfluxProcessor(influxClient, importer,
						c.Int(influxConcurrency), c.String(influxMeasurementFieldSeparator))
					return processor.run(c.Bool(globalSilent), c.Bool(globalVerbose))
				},
			},
			{
				Name:  "prometheus",
				Usage: "Migrate timeseries from Prometheus",
				Flags: mergeFlags(globalFlags, promFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus import mode")

					vmCfg := initConfigVM(c)
					importer, err = vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					promCfg := prometheus.Config{
						Snapshot: c.String(promSnapshot),
						Filter: prometheus.Filter{
							TimeMin:    c.String(promFilterTimeStart),
							TimeMax:    c.String(promFilterTimeEnd),
							Label:      c.String(promFilterLabel),
							LabelValue: c.String(promFilterLabelValue),
						},
					}
					cl, err := prometheus.NewClient(promCfg)
					if err != nil {
						return fmt.Errorf("failed to create prometheus client: %s", err)
					}
					pp := prometheusProcessor{
						cl: cl,
						im: importer,
						cc: c.Int(promConcurrency),
					}
					return pp.run(c.Bool(globalSilent), c.Bool(globalVerbose))
				},
			},
			{
				Name:  "vm-native",
				Usage: "Migrate time series between VictoriaMetrics installations via native binary format",
				Flags: vmNativeFlags,
				Action: func(c *cli.Context) error {
					fmt.Println("VictoriaMetrics Native import mode")

					if c.String(vmNativeFilterMatch) == "" {
						return fmt.Errorf("flag %q can't be empty", vmNativeFilterMatch)
					}

					p := vmNativeProcessor{
						rateLimit: c.Int64(vmRateLimit),
						filter: filter{
							match:     c.String(vmNativeFilterMatch),
							timeStart: c.String(vmNativeFilterTimeStart),
							timeEnd:   c.String(vmNativeFilterTimeEnd),
						},
						src: &vmNativeClient{
							addr:     strings.Trim(c.String(vmNativeSrcAddr), "/"),
							user:     c.String(vmNativeSrcUser),
							password: c.String(vmNativeSrcPassword),
						},
						dst: &vmNativeClient{
							addr:        strings.Trim(c.String(vmNativeDstAddr), "/"),
							user:        c.String(vmNativeDstUser),
							password:    c.String(vmNativeDstPassword),
							extraLabels: c.StringSlice(vmExtraLabel),
						},
					}
					return p.run()
				},
			},
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Execution cancelled")
		if importer != nil {
			importer.Close()
		}
	}()

	err = app.Run(os.Args)
	if err != nil {
		log.Println(err)
	}
	log.Printf("Total time: %v", time.Since(start))
}

func initConfigVM(c *cli.Context) vm.Config {
	return vm.Config{
		Addr:               c.String(vmAddr),
		User:               c.String(vmUser),
		Password:           c.String(vmPassword),
		Concurrency:        uint8(c.Int(vmConcurrency)),
		Compress:           c.Bool(vmCompress),
		AccountID:          c.String(vmAccountID),
		BatchSize:          c.Int(vmBatchSize),
		SignificantFigures: c.Int(vmSignificantFigures),
		RoundDigits:        c.Int(vmRoundDigits),
		ExtraLabels:        c.StringSlice(vmExtraLabel),
		RateLimit:          c.Int64(vmRateLimit),
	}
}
