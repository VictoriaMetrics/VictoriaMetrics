package main

import (
	"context"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/remoteread"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/influx"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/opentsdb"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/native/stream"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
)

func main() {
	var (
		err      error
		importer *vm.Importer
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	start := time.Now()
	beforeFn := func(c *cli.Context) error {
		isSilent = c.Bool(globalSilent)
		if c.Bool(globalDisableProgressBar) {
			barpool.Disable(true)
		}
		netutil.EnableIPv6()
		return nil
	}
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "VictoriaMetrics command-line tool",
		Version: buildinfo.Version,
		Commands: []*cli.Command{
			{
				Name:   "opentsdb",
				Usage:  "Migrate time series from OpenTSDB",
				Flags:  mergeFlags(globalFlags, otsdbFlags, vmFlags),
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					fmt.Println("OpenTSDB import mode")

					// create Transport with given TLS config
					certFile := c.String(otsdbCertFile)
					keyFile := c.String(otsdbKeyFile)
					caFile := c.String(otsdbCAFile)
					serverName := c.String(otsdbServerName)
					insecureSkipVerify := c.Bool(otsdbInsecureSkipVerify)
					addr := c.String(otsdbAddr)
					if err := httputil.CheckURL(addr); err != nil {
						return fmt.Errorf("invalid -%s: %w", otsdbAddr, err)
					}

					tr, err := promauth.NewTLSTransport(certFile, keyFile, caFile, serverName, insecureSkipVerify, "vmctl_opentsdb")
					if err != nil {
						return fmt.Errorf("failed to create transport for -%s=%q: %s", otsdbAddr, addr, err)
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

					vmCfg, err := initConfigVM(c)
					if err != nil {
						return fmt.Errorf("failed to init VM configuration: %s", err)
					}

					importer, err := vm.NewImporter(ctx, vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					otsdbProcessor := newOtsdbProcessor(otsdbClient, importer, c.Int(otsdbConcurrency), c.Bool(globalVerbose))
					return otsdbProcessor.run(ctx)
				},
			},
			{
				Name:   "influx",
				Usage:  "Migrate time series from InfluxDB",
				Flags:  mergeFlags(globalFlags, influxFlags, vmFlags),
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					// create TLS config
					certFile := c.String(influxCertFile)
					keyFile := c.String(influxKeyFile)
					caFile := c.String(influxCAFile)
					serverName := c.String(influxServerName)
					insecureSkipVerify := c.Bool(influxInsecureSkipVerify)

					tc, err := promauth.NewTLSConfig(certFile, keyFile, caFile, serverName, insecureSkipVerify)
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

					vmCfg, err := initConfigVM(c)
					if err != nil {
						return fmt.Errorf("failed to init VM configuration: %s", err)
					}

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
						c.Bool(globalVerbose))
					return processor.run(ctx)
				},
			},
			{
				Name:   "remote-read",
				Usage:  "Migrate time series via Prometheus remote-read protocol",
				Flags:  mergeFlags(globalFlags, remoteReadFlags, vmFlags),
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					fmt.Println("Remote-read import mode")

					addr := c.String(remoteReadSrcAddr)
					if err := httputil.CheckURL(addr); err != nil {
						return fmt.Errorf("invalid -%s: %w", remoteReadSrcAddr, err)
					}

					// create TLS config
					certFile := c.String(remoteReadCertFile)
					keyFile := c.String(remoteReadKeyFile)
					caFile := c.String(remoteReadCAFile)
					serverName := c.String(remoteReadServerName)
					insecureSkipVerify := c.Bool(remoteReadInsecureSkipVerify)

					tr, err := promauth.NewTLSTransport(certFile, keyFile, caFile, serverName, insecureSkipVerify, "vmctl_remoteread")
					if err != nil {
						return fmt.Errorf("failed to create transport for -%s=%q: %s", remoteReadSrcAddr, addr, err)
					}

					// Backwards compatible default values if none provided by user
					rrLabelNames := c.StringSlice(remoteReadFilterLabel)
					rrLabelValues := c.StringSlice(remoteReadFilterLabelValue)
					if len(rrLabelNames) == 0 && len(rrLabelValues) == 0 {
						rrLabelNames = []string{"__name__"}
						rrLabelValues = []string{".*"}
					}

					rr, err := remoteread.NewClient(remoteread.Config{
						Addr:              addr,
						Transport:         tr,
						Username:          c.String(remoteReadUser),
						Password:          c.String(remoteReadPassword),
						Timeout:           c.Duration(remoteReadHTTPTimeout),
						UseStream:         c.Bool(remoteReadUseStream),
						Headers:           c.String(remoteReadHeaders),
						LabelNames:        rrLabelNames,
						LabelValues:       rrLabelValues,
						DisablePathAppend: c.Bool(remoteReadDisablePathAppend),
					})
					if err != nil {
						return fmt.Errorf("error create remote read client: %s", err)
					}

					vmCfg, err := initConfigVM(c)
					if err != nil {
						return fmt.Errorf("failed to init VM configuration: %s", err)
					}

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
						isVerbose: c.Bool(globalVerbose),
					}
					return rmp.run(ctx)
				},
			},
			{
				Name:   "prometheus",
				Usage:  "Migrate time series from Prometheus",
				Flags:  mergeFlags(globalFlags, promFlags, vmFlags),
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus import mode")

					vmCfg, err := initConfigVM(c)
					if err != nil {
						return fmt.Errorf("failed to init VM configuration: %s", err)
					}

					importer, err = vm.NewImporter(ctx, vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					promCfg := prometheus.Config{
						Snapshot:     c.String(promSnapshot),
						TemporaryDir: c.String(promTemporaryDirPath),
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
						cl:        cl,
						im:        importer,
						cc:        c.Int(promConcurrency),
						isVerbose: c.Bool(globalVerbose),
					}
					return pp.run(ctx)
				},
			},
			{
				Name:   "vm-native",
				Usage:  "Migrate time series between VictoriaMetrics installations",
				Flags:  mergeFlags(globalFlags, vmNativeFlags),
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					fmt.Println("VictoriaMetrics Native import mode")

					if c.String(vmNativeFilterMatch) == "" {
						return fmt.Errorf("flag %q can't be empty", vmNativeFilterMatch)
					}

					bfRetries := c.Int(vmNativeBackoffRetries)
					bfFactor := c.Float64(vmNativeBackoffFactor)
					bfMinDuration := c.Duration(vmNativeBackoffMinDuration)
					bf, err := backoff.New(bfRetries, bfFactor, bfMinDuration)
					if err != nil {
						return fmt.Errorf("failed to create backoff object: %s", err)
					}

					disableKeepAlive := c.Bool(vmNativeDisableHTTPKeepAlive)

					var srcExtraLabels []string
					srcAddr := strings.Trim(c.String(vmNativeSrcAddr), "/")
					srcAuthConfig, err := auth.Generate(
						auth.WithBasicAuth(c.String(vmNativeSrcUser), c.String(vmNativeSrcPassword)),
						auth.WithBearer(c.String(vmNativeSrcBearerToken)),
						auth.WithHeaders(c.String(vmNativeSrcHeaders)))
					if err != nil {
						return fmt.Errorf("error initialize auth config for source: %s", srcAddr)
					}

					// create TLS config
					srcCertFile := c.String(vmNativeSrcCertFile)
					srcKeyFile := c.String(vmNativeSrcKeyFile)
					srcCAFile := c.String(vmNativeSrcCAFile)
					srcServerName := c.String(vmNativeSrcServerName)
					srcInsecureSkipVerify := c.Bool(vmNativeSrcInsecureSkipVerify)

					srcTC, err := promauth.NewTLSConfig(srcCertFile, srcKeyFile, srcCAFile, srcServerName, srcInsecureSkipVerify)
					if err != nil {
						return fmt.Errorf("failed to create TLS Config: %s", err)
					}

					trSrc := httputil.NewTransport(false, "vmctl_src")
					trSrc.DisableKeepAlives = disableKeepAlive
					trSrc.TLSClientConfig = srcTC

					srcHTTPClient := &http.Client{
						Transport: trSrc,
					}

					dstAddr := strings.Trim(c.String(vmNativeDstAddr), "/")
					dstExtraLabels := c.StringSlice(vmExtraLabel)
					dstAuthConfig, err := auth.Generate(
						auth.WithBasicAuth(c.String(vmNativeDstUser), c.String(vmNativeDstPassword)),
						auth.WithBearer(c.String(vmNativeDstBearerToken)),
						auth.WithHeaders(c.String(vmNativeDstHeaders)))
					if err != nil {
						return fmt.Errorf("error initialize auth config for destination: %s", dstAddr)
					}

					// create TLS config
					dstCertFile := c.String(vmNativeDstCertFile)
					dstKeyFile := c.String(vmNativeDstKeyFile)
					dstCAFile := c.String(vmNativeDstCAFile)
					dstServerName := c.String(vmNativeDstServerName)
					dstInsecureSkipVerify := c.Bool(vmNativeDstInsecureSkipVerify)

					dstTC, err := promauth.NewTLSConfig(dstCertFile, dstKeyFile, dstCAFile, dstServerName, dstInsecureSkipVerify)
					if err != nil {
						return fmt.Errorf("failed to create TLS Config: %s", err)
					}

					trDst := httputil.NewTransport(false, "vmctl_dst")
					trDst.DisableKeepAlives = disableKeepAlive
					trDst.TLSClientConfig = dstTC

					dstHTTPClient := &http.Client{
						Transport: trDst,
					}

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
						backoff:                  bf,
						cc:                       c.Int(vmConcurrency),
						disablePerMetricRequests: c.Bool(vmNativeDisablePerMetricMigration),
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
				Before: beforeFn,
				Action: func(c *cli.Context) error {
					protoparserutil.StartUnmarshalWorkers()
					blockPath := c.Args().First()
					encoding := ""
					if c.Bool("gunzip") {
						encoding = "gzip"
					}
					if len(blockPath) == 0 {
						return cli.Exit("you must provide path for exported data block", 1)
					}
					log.Printf("verifying block at path=%q", blockPath)
					f, err := os.OpenFile(blockPath, os.O_RDONLY, 0600)
					if err != nil {
						return cli.Exit(fmt.Errorf("cannot open exported block at path=%q err=%w", blockPath, err), 1)
					}
					defer f.Close()
					var blocksCount atomic.Uint64
					if err := stream.Parse(f, encoding, func(_ *stream.Block) error {
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

func initConfigVM(c *cli.Context) (vm.Config, error) {
	addr := c.String(vmAddr)
	if err := httputil.CheckURL(addr); err != nil {
		return vm.Config{}, fmt.Errorf("invalid -%s: %w", vmAddr, err)
	}

	// create Transport with given TLS config
	certFile := c.String(vmCertFile)
	keyFile := c.String(vmKeyFile)
	caFile := c.String(vmCAFile)
	serverName := c.String(vmServerName)
	insecureSkipVerify := c.Bool(vmInsecureSkipVerify)

	tr, err := promauth.NewTLSTransport(certFile, keyFile, caFile, serverName, insecureSkipVerify, "vmctl_client")
	if err != nil {
		return vm.Config{}, fmt.Errorf("failed to create transport for -%s=%q: %s", vmAddr, addr, err)
	}

	bfRetries := c.Int(vmBackoffRetries)
	bfFactor := c.Float64(vmBackoffFactor)
	bfMinDuration := c.Duration(vmBackoffMinDuration)
	bf, err := backoff.New(bfRetries, bfFactor, bfMinDuration)
	if err != nil {
		return vm.Config{}, fmt.Errorf("failed to create backoff object: %s", err)
	}

	return vm.Config{
		Addr:               addr,
		Transport:          tr,
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
		Backoff:            bf,
	}, nil
}
