package main

import (
	"flag"
	"log"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	globalSilent  = flag.Bool("s", false, "Whether to run in silent mode. If set to true no confirmation prompts will appear.")
	globalVerbose = flag.Bool("verbose", false, "Whether to enable verbosity in logs output.")
)

func main() {
	start := time.Now()
	cmd := &flagutil.Command{
		Name:   "vmctl",
		Usage:  "VictoriaMetrics command-line tool",
		Action: func(args []string) {},
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
				Action: remoteReadImport,
			},
			{
				Name:   "prometheus",
				Usage:  "Migrate time series from Prometheus",
				Action: prometheusImport,
			},
			{
				Name:   "vm-native",
				Usage:  "Migrate time series between VictoriaMetrics installations via native binary format",
				Action: nativeImport,
			},
			{
				Name:   "verify-block",
				Usage:  "Verifies exported block with VictoriaMetrics Native format",
				Action: verifyBlocks,
			},
		},
	}

	cmd.Run()
	log.Printf("Total time: %v", time.Since(start))
}
