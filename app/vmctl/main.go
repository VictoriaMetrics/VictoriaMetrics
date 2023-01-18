package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	globalSilent  = flag.Bool("s", false, "Whether to run in silent mode. If set to true no confirmation prompts will appear.")
	globalVerbose = flag.Bool("verbose", false, "Whether to enable verbosity in logs output.")
)

func main() {

	ctx, cancelCtx := context.WithCancel(context.Background())
	start := time.Now()
	cmd := &flagutil.Command{
		Name:  "vmctl",
		Usage: "VictoriaMetrics command-line tool",
		Subcommands: []*flagutil.Command{
			{
				Name:   "opentsdb",
				Usage:  "Migrate time series from OpenTSDB",
				Action: otsdbImport,
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
		cancelCtx()
	}()

	cmd.Run()
	log.Printf("Total time: %v", time.Since(start))
}
