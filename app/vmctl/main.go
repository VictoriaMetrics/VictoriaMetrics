package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func main() {
	// var importer *vm.Importer

	ctx, cancelCtx := context.WithCancel(context.Background())
	start := time.Now()
	cmd := &flagutil.Command{
		Name:  "vmctl",
		Usage: "VictoriaMetrics command-line tool",
		// Version: buildinfo.Version,
		// Disable `-version` flag to avoid conflict with lib/buildinfo flags
		// see https://github.com/urfave/cli/issues/1560
		// HideVersion: true,
		Subcommands: []*flagutil.Command{
			{
				Name:   "opentsdb",
				Usage:  "Migrate time series from OpenTSDB",
				Action: otsbImport,
			},
			{
				Name:   "influx",
				Usage:  "Migrate time series from InfluxDB",
				Action: influxImporter,
			},
			{
				Name:   "remote-read",
				Usage:  "Migrate time series via Prometheus remote-read protocol",
				Action: remoteReadImport(ctx),
			},
			{
				Name:   "prometheus",
				Usage:  "Migrate time series from Prometheus",
				Action: prometheusImport,
			},
			{
				Name:   "vm-native",
				Usage:  "Migrate time series between VictoriaMetrics installations via native binary format",
				Action: nativeImport(ctx),
			},
			{
				Name:   "verify-block",
				Usage:  "Verifies exported block with VictoriaMetrics Native format",
				Action: verifyBlocks,
			},
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Execution cancelled")
		// if importer != nil {
		// 	importer.Close()
		// }
		cancelCtx()
	}()

	cmd.Run()
	log.Printf("Total time: %v", time.Since(start))
}

func initConfigVM() vm.Config {
	return vm.Config{
		Addr:               *vmAddr,
		User:               *vmUser,
		Password:           *vmPassword,
		Concurrency:        uint8(*vmConcurrency),
		Compress:           *vmCompress,
		AccountID:          *vmAccountID,
		BatchSize:          *vmBatchSize,
		SignificantFigures: *vmSignificantFigures,
		RoundDigits:        *vmRoundDigits,
		ExtraLabels:        *vmExtraLabel,
		RateLimit:          *vmRateLimit,
		DisableProgressBar: *vmDisableProgressBar,
	}
}
