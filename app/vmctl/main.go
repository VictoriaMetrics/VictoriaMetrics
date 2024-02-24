package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/remoteread"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
)

func main() {
	var (
		err      error
		importer *vm.Importer
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	start := time.Now()
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "VictoriaMetrics command-line tool",
		Version: buildinfo.Version,
		Commands: []*cli.Command{
			{
				Name:  "opentsdb",
				Usage: "Migrate time series from OpenTSDB",
				Flags: mergeFlags(globalFlags, otsdbFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("OpenTSDB import mode")

					// create Transport with given TLS config
					certFile := c.String(otsdbCertFile)
					keyFile := c.String(otsdbKeyFile)
					caFile := c.String(otsdbCAFile)
					serverName := c.String(otsdbServerName)
					insecureSkipVerify := c.Bool(otsdbInsecureSkipVerify)
					addr := c.String(otsdbAddr)

					tr, err := httputils.Transport(addr, certFile, caFile, keyFile, serverName, insecureSkipVerify)
					if err != nil {
						return fmt.Errorf("failed to create Transport: %s", err)
					}
					oCfg := opentsdb.Config{
						Addr:       addr,
						Limit:      c.Int(otsdbQueryLimit),
						Offset:     c.Int64(otsdbOffsetDays),
						HardTS:     c.Int64(otsdbHardTSStart),
						Retentions: c.StringSlice(otsdbRetentions),
						Filters:    c.StringSlice(otsdbFilters),
						Normalize:  c.Bool(otsdbNormalize),
						MsecsTime:  c.Bool(otsdbMsecsTime),
						Transport:  tr,
					}
					otsdbClient, err := opentsdb.NewClient(oCfg)
					if err != nil {
						return fmt.Errorf("failed to create opentsdb client: %s", err)
					}

					vmCfg := initConfigVM(c)
					// disable progress bars since openTSDB implementation
					// does not use progress bar pool
					vmCfg.DisableProgressBar = true
					importer, err := vm.NewImporter(ctx, vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					otsdbProcessor := newOtsdbProcessor(otsdbClient, importer, c.Int(otsdbConcurrency), c.Bool(globalSilent), c.Bool(globalVerbose))
					return otsdbProcessor.run()
				},
			},
			{
				Name:  "influx",
				Usage: "Migrate time series from InfluxDB",
				Flags: mergeFlags(globalFlags, influxFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					// create TLS config
					certFile := c.String(influxCertFile)
					keyFile := c.String(influxKeyFile)
					caFile := c.String(influxCAFile)
					serverName := c.String(influxServerName)
					insecureSkipVerify := c.Bool(influxInsecureSkipVerify)

					tc, err := httputils.TLSConfig(certFile, caFile, keyFile, serverName, insecureSkipVerify)
					if err != nil {
						return fmt.Errorf("failed to create TLS Config: %s", err)
					}

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
						TLSConfig: tc,
					}

					influxClient, err := influx.NewClient(iCfg)
					if err != nil {
						return fmt.Errorf("failed to create influx client: %s", err)
					}

					vmCfg := initConfigVM(c)
					importer, err = vm.NewImporter(ctx, vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					processor := newInfluxProcessor(
						influxClient,
						importer,
						c.Int(influxConcurrency),
						c.String(influxMeasurementFieldSeparator),
						c.Bool(influxSkipDatabaseLabel),
						c.Bool(influxPrometheusMode),
						c.Bool(globalSilent),
						c.Bool(globalVerbose))
					return processor.run()
				},
			},
			{
				Name:  "remote-read",
				Usage: "Migrate time series via Prometheus remote-read protocol",
				Flags: mergeFlags(globalFlags, remoteReadFlags, vmFlags),
				Action: func(c *cli.Context) error {
					rr, err := remoteread.NewClient(remoteread.Config{
						Addr:               c.String(remoteReadSrcAddr),
						Username:           c.String(remoteReadUser),
						Password:           c.String(remoteReadPassword),
						Timeout:            c.Duration(remoteReadHTTPTimeout),
						UseStream:          c.Bool(remoteReadUseStream),
						Headers:            c.String(remoteReadHeaders),
						LabelName:          c.String(remoteReadFilterLabel),
						LabelValue:         c.String(remoteReadFilterLabelValue),
						CertFile:           c.String(remoteReadCertFile),
						KeyFile:            c.String(remoteReadKeyFile),
						CAFile:             c.String(remoteReadCAFile),
						ServerName:         c.String(remoteReadServerName),
						InsecureSkipVerify: c.Bool(remoteReadInsecureSkipVerify),
						DisablePathAppend:  c.Bool(remoteReadDisablePathAppend),
					})
					if err != nil {
						return fmt.Errorf("error create remote read client: %s", err)
					}

					vmCfg := initConfigVM(c)

					importer, err := vm.NewImporter(ctx, vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					rmp := remoteReadProcessor{
						src: rr,
						dst: importer,
						filter: remoteReadFilter{
							timeStart:   c.Timestamp(remoteReadFilterTimeStart),
							timeEnd:     c.Timestamp(remoteReadFilterTimeEnd),
							chunk:       c.String(remoteReadStepInterval),
							timeReverse: c.Bool(remoteReadFilterTimeReverse),
						},
						cc:        c.Int(remoteReadConcurrency),
						isSilent:  c.Bool(globalSilent),
						isVerbose: c.Bool(globalVerbose),
					}
					return rmp.run(ctx)
				},
			},
			{
				Name:  "prometheus",
				Usage: "Migrate time series from Prometheus",
				Flags: mergeFlags(globalFlags, promFlags, vmFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus import mode")

					vmCfg := initConfigVM(c)
					importer, err = vm.NewImporter(ctx, vmCfg)
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
				Flags: mergeFlags(globalFlags, vmNativeFlags),
				Action: func(c *cli.Context) error {
					fmt.Println("VictoriaMetrics Native import mode")

					if c.String(vmNativeFilterMatch) == "" {
						return fmt.Errorf("flag %q can't be empty", vmNativeFilterMatch)
					}

					disableKeepAlive := c.Bool(vmNativeDisableHTTPKeepAlive)

					var srcExtraLabels []string
					srcAddr := strings.Trim(c.String(vmNativeSrcAddr), "/")
					srcInsecureSkipVerify := c.Bool(vmNativeSrcInsecureSkipVerify)
					srcAuthConfig, err := auth.Generate(
						auth.WithBasicAuth(c.String(vmNativeSrcUser), c.String(vmNativeSrcPassword)),
						auth.WithBearer(c.String(vmNativeSrcBearerToken)),
						auth.WithHeaders(c.String(vmNativeSrcHeaders)))
					if err != nil {
						return fmt.Errorf("error initilize auth config for source: %s", srcAddr)
					}
					srcHTTPClient := &http.Client{Transport: &http.Transport{
						DisableKeepAlives: disableKeepAlive,
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: srcInsecureSkipVerify,
						},
					}}

					dstAddr := strings.Trim(c.String(vmNativeDstAddr), "/")
					dstExtraLabels := c.StringSlice(vmExtraLabel)
					dstInsecureSkipVerify := c.Bool(vmNativeDstInsecureSkipVerify)
					dstAuthConfig, err := auth.Generate(
						auth.WithBasicAuth(c.String(vmNativeDstUser), c.String(vmNativeDstPassword)),
						auth.WithBearer(c.String(vmNativeDstBearerToken)),
						auth.WithHeaders(c.String(vmNativeDstHeaders)))
					if err != nil {
						return fmt.Errorf("error initilize auth config for destination: %s", dstAddr)
					}
					dstHTTPClient := &http.Client{Transport: &http.Transport{
						DisableKeepAlives: disableKeepAlive,
						TLSClientConfig: &tls.Config{
							InsecureSkipVerify: dstInsecureSkipVerify,
						},
					}}

					p := vmNativeProcessor{
						rateLimit:    c.Int64(vmRateLimit),
						interCluster: c.Bool(vmInterCluster),
						filter: native.Filter{
							Match:       c.String(vmNativeFilterMatch),
							TimeStart:   c.String(vmNativeFilterTimeStart),
							TimeEnd:     c.String(vmNativeFilterTimeEnd),
							Chunk:       c.String(vmNativeStepInterval),
							TimeReverse: c.Bool(vmNativeFilterTimeReverse),
						},
						src: &native.Client{
							AuthCfg:     srcAuthConfig,
							Addr:        srcAddr,
							ExtraLabels: srcExtraLabels,
							HTTPClient:  srcHTTPClient,
						},
						dst: &native.Client{
							AuthCfg:     dstAuthConfig,
							Addr:        dstAddr,
							ExtraLabels: dstExtraLabels,
							HTTPClient:  dstHTTPClient,
						},
						backoff:                  backoff.New(),
						cc:                       c.Int(vmConcurrency),
						disablePerMetricRequests: c.Bool(vmNativeDisablePerMetricMigration),
						isSilent:                 c.Bool(globalSilent),
						isNative:                 !c.Bool(vmNativeDisableBinaryProtocol),
					}
					return p.run(ctx)
				},
			},
			{
				Name:  "verify-block",
				Usage: "Verifies exported block with VictoriaMetrics Native format",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "gunzip",
						Usage: "Use GNU zip decompression for exported block",
						Value: false,
					},
				},
				Action: func(c *cli.Context) error {
					common.StartUnmarshalWorkers()
					blockPath := c.Args().First()
					isBlockGzipped := c.Bool("gunzip")
					if len(blockPath) == 0 {
						return cli.Exit("you must provide path for exported data block", 1)
					}
					log.Printf("verifying block at path=%q", blockPath)
					f, err := os.OpenFile(blockPath, os.O_RDONLY, 0600)
					if err != nil {
						return cli.Exit(fmt.Errorf("cannot open exported block at path=%q err=%w", blockPath, err), 1)
					}
					var blocksCount atomic.Uint64
					if err := stream.Parse(f, isBlockGzipped, func(block *stream.Block) error {
						blocksCount.Add(1)
						return nil
					}); err != nil {
						return cli.Exit(fmt.Errorf("cannot parse block at path=%q, blocksCount=%d, err=%w", blockPath, blocksCount.Load(), err), 1)
					}
					log.Printf("successfully verified block at path=%q, blockCount=%d", blockPath, blocksCount.Load())
					return nil
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
		cancelCtx()
	}()

	err = app.Run(os.Args)
	if err != nil {
		log.Fatalln(err)
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
		DisableProgressBar: c.Bool(vmDisableProgressBar),
	}
}
