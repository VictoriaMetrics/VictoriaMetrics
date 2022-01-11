package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

const (
	globalSilent  = "s"
	globalVerbose = "verbose"
)

var (
	globalFlags = []cli.Flag{
		&cli.BoolFlag{
			Name:  globalSilent,
			Value: false,
			Usage: "Whether to run in silent mode. If set to true no confirmation prompts will appear.",
		},
		&cli.BoolFlag{
			Name:  globalVerbose,
			Value: false,
			Usage: "Whether to enable verbosity in logs output.",
		},
	}
)

const (
	vmAddr               = "vm-addr"
	vmUser               = "vm-user"
	vmPassword           = "vm-password"
	vmAccountID          = "vm-account-id"
	vmConcurrency        = "vm-concurrency"
	vmCompress           = "vm-compress"
	vmBatchSize          = "vm-batch-size"
	vmSignificantFigures = "vm-significant-figures"
	vmRoundDigits        = "vm-round-digits"

	// also used in vm-native
	vmExtraLabel = "vm-extra-label"
	vmRateLimit  = "vm-rate-limit"
)

var (
	vmFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  vmAddr,
			Value: "http://localhost:8428",
			Usage: "VictoriaMetrics address to perform import requests. \n" +
				"Should be the same as --httpListenAddr value for single-node version or vminsert component. \n" +
				"When importing into the clustered version do not forget to set additionally --vm-account-id flag. \n" +
				"Please note, that `vmctl` performs initial readiness check for the given address by checking `/health` endpoint.",
		},
		&cli.StringFlag{
			Name:    vmUser,
			Usage:   "VictoriaMetrics username for basic auth",
			EnvVars: []string{"VM_USERNAME"},
		},
		&cli.StringFlag{
			Name:    vmPassword,
			Usage:   "VictoriaMetrics password for basic auth",
			EnvVars: []string{"VM_PASSWORD"},
		},
		&cli.StringFlag{
			Name: vmAccountID,
			Usage: "AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). \n" +
				"AccountID is required when importing into the clustered version of VictoriaMetrics. \n" +
				"It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. \n" +
				"If projectID isn't set, then it equals to 0",
		},
		&cli.UintFlag{
			Name:  vmConcurrency,
			Usage: "Number of workers concurrently performing import requests to VM",
			Value: 2,
		},
		&cli.BoolFlag{
			Name:  vmCompress,
			Value: true,
			Usage: "Whether to apply gzip compression to import requests",
		},
		&cli.IntFlag{
			Name:  vmBatchSize,
			Value: 200e3,
			Usage: "How many samples importer collects before sending the import request to VM",
		},
		&cli.IntFlag{
			Name:  vmSignificantFigures,
			Value: 0,
			Usage: "The number of significant figures to leave in metric values before importing. " +
				"See https://en.wikipedia.org/wiki/Significant_figures. Zero value saves all the significant figures. " +
				"This option may be used for increasing on-disk compression level for the stored metrics. " +
				"See also --vm-round-digits option",
		},
		&cli.IntFlag{
			Name:  vmRoundDigits,
			Value: 100,
			Usage: "Round metric values to the given number of decimal digits after the point. " +
				"This option may be used for increasing on-disk compression level for the stored metrics",
		},
		&cli.StringSliceFlag{
			Name:  vmExtraLabel,
			Value: nil,
			Usage: "Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flag" +
				"will have priority. Flag can be set multiple times, to add few additional labels.",
		},
		&cli.Int64Flag{
			Name: vmRateLimit,
			Usage: "Optional data transfer rate limit in bytes per second.\n" +
				"By default the rate limit is disabled. It can be useful for limiting load on configured via '--vmAddr' destination.",
		},
	}
)

const (
	otsdbAddr        = "otsdb-addr"
	otsdbConcurrency = "otsdb-concurrency"
	otsdbQueryLimit  = "otsdb-query-limit"
	otsdbOffsetDays  = "otsdb-offset-days"
	otsdbHardTSStart = "otsdb-hard-ts-start"
	otsdbRetentions  = "otsdb-retentions"
	otsdbFilters     = "otsdb-filters"
	otsdbNormalize   = "otsdb-normalize"
	otsdbMsecsTime   = "otsdb-msecstime"
)

var (
	otsdbFlags = []cli.Flag{
		&cli.StringFlag{
			Name:     otsdbAddr,
			Value:    "http://localhost:4242",
			Required: true,
			Usage:    "OpenTSDB server addr",
		},
		&cli.IntFlag{
			Name:  otsdbConcurrency,
			Usage: "Number of concurrently running fetch queries to OpenTSDB per metric",
			Value: 1,
		},
		&cli.StringSliceFlag{
			Name:     otsdbRetentions,
			Value:    nil,
			Required: true,
			Usage: "Retentions patterns to collect on. Each pattern should describe the aggregation performed " +
				"for the query, the row size (in HBase) that will define how long each individual query is, " +
				"and the time range to query for. e.g. sum-1m-avg:1h:3d. " +
				"The first time range defined should be a multiple of the row size in HBase. " +
				"e.g. if the row size is 2 hours, 4h is good, 5h less so. We want each query to land on unique rows.",
		},
		&cli.StringSliceFlag{
			Name:  otsdbFilters,
			Value: cli.NewStringSlice("a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"),
			Usage: "Filters to process for discovering metrics in OpenTSDB",
		},
		&cli.Int64Flag{
			Name:  otsdbOffsetDays,
			Usage: "Days to offset our 'starting' point for collecting data from OpenTSDB",
			Value: 0,
		},
		&cli.Int64Flag{
			Name:  otsdbHardTSStart,
			Usage: "A specific timestamp to start from, will override using an offset",
			Value: 0,
		},
		/*
			because the defaults are set *extremely* low in OpenTSDB (10-25 results), we will
			set a larger default limit, but still allow a user to increase/decrease it
		*/
		&cli.IntFlag{
			Name:  otsdbQueryLimit,
			Usage: "Result limit on meta queries to OpenTSDB (affects both metric name and tag value queries, recommended to use a value exceeding your largest series)",
			Value: 100e6,
		},
		&cli.BoolFlag{
			Name:  otsdbMsecsTime,
			Value: false,
			Usage: "Whether OpenTSDB is writing values in milliseconds or seconds",
		},
		&cli.BoolFlag{
			Name:  otsdbNormalize,
			Value: false,
			Usage: "Whether to normalize all data received to lower case before forwarding to VictoriaMetrics",
		},
	}
)

const (
	influxAddr                      = "influx-addr"
	influxUser                      = "influx-user"
	influxPassword                  = "influx-password"
	influxDB                        = "influx-database"
	influxRetention                 = "influx-retention-policy"
	influxChunkSize                 = "influx-chunk-size"
	influxConcurrency               = "influx-concurrency"
	influxFilterSeries              = "influx-filter-series"
	influxFilterTimeStart           = "influx-filter-time-start"
	influxFilterTimeEnd             = "influx-filter-time-end"
	influxMeasurementFieldSeparator = "influx-measurement-field-separator"
)

var (
	influxFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  influxAddr,
			Value: "http://localhost:8086",
			Usage: "InfluxDB server addr",
		},
		&cli.StringFlag{
			Name:    influxUser,
			Usage:   "InfluxDB user",
			EnvVars: []string{"INFLUX_USERNAME"},
		},
		&cli.StringFlag{
			Name:    influxPassword,
			Usage:   "InfluxDB user password",
			EnvVars: []string{"INFLUX_PASSWORD"},
		},
		&cli.StringFlag{
			Name:     influxDB,
			Usage:    "InfluxDB database",
			Required: true,
		},
		&cli.StringFlag{
			Name:  influxRetention,
			Usage: "InfluxDB retention policy",
			Value: "autogen",
		},
		&cli.IntFlag{
			Name:  influxChunkSize,
			Usage: "The chunkSize defines max amount of series to be returned in one chunk",
			Value: 10e3,
		},
		&cli.IntFlag{
			Name:  influxConcurrency,
			Usage: "Number of concurrently running fetch queries to InfluxDB",
			Value: 1,
		},
		&cli.StringFlag{
			Name: influxFilterSeries,
			Usage: "InfluxDB filter expression to select series. E.g. \"from cpu where arch='x86' AND hostname='host_2753'\".\n" +
				"See for details https://docs.influxdata.com/influxdb/v1.7/query_language/schema_exploration#show-series",
		},
		&cli.StringFlag{
			Name:  influxFilterTimeStart,
			Usage: "The time filter to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name:  influxFilterTimeEnd,
			Usage: "The time filter to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name:  influxMeasurementFieldSeparator,
			Usage: "The {separator} symbol used to concatenate {measurement} and {field} names into series name {measurement}{separator}{field}.",
			Value: "_",
		},
	}
)

const (
	promSnapshot         = "prom-snapshot"
	promConcurrency      = "prom-concurrency"
	promFilterTimeStart  = "prom-filter-time-start"
	promFilterTimeEnd    = "prom-filter-time-end"
	promFilterLabel      = "prom-filter-label"
	promFilterLabelValue = "prom-filter-label-value"
)

var (
	promFlags = []cli.Flag{
		&cli.StringFlag{
			Name:     promSnapshot,
			Usage:    "Path to Prometheus snapshot. Pls see for details https://www.robustperception.io/taking-snapshots-of-prometheus-data",
			Required: true,
		},
		&cli.IntFlag{
			Name:  promConcurrency,
			Usage: "Number of concurrently running snapshot readers",
			Value: 1,
		},
		&cli.StringFlag{
			Name:  promFilterTimeStart,
			Usage: "The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name:  promFilterTimeEnd,
			Usage: "The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name:  promFilterLabel,
			Usage: "Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.",
		},
		&cli.StringFlag{
			Name:  promFilterLabelValue,
			Usage: fmt.Sprintf("Prometheus regular expression to filter label from %q flag.", promFilterLabel),
			Value: ".*",
		},
	}
)

const (
	vmNativeFilterMatch     = "vm-native-filter-match"
	vmNativeFilterTimeStart = "vm-native-filter-time-start"
	vmNativeFilterTimeEnd   = "vm-native-filter-time-end"

	vmNativeSrcAddr     = "vm-native-src-addr"
	vmNativeSrcUser     = "vm-native-src-user"
	vmNativeSrcPassword = "vm-native-src-password"

	vmNativeDstAddr     = "vm-native-dst-addr"
	vmNativeDstUser     = "vm-native-dst-user"
	vmNativeDstPassword = "vm-native-dst-password"
)

var (
	vmNativeFlags = []cli.Flag{
		&cli.StringFlag{
			Name: vmNativeFilterMatch,
			Usage: "Time series selector to match series for export. For example, select {instance!=\"localhost\"} will " +
				"match all series with \"instance\" label different to \"localhost\".\n" +
				" See more details here https://github.com/VictoriaMetrics/VictoriaMetrics#how-to-export-data-in-native-format",
			Value: `{__name__!=""}`,
		},
		&cli.StringFlag{
			Name:  vmNativeFilterTimeStart,
			Usage: "The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name:  vmNativeFilterTimeEnd,
			Usage: "The time filter may contain either unix timestamp in seconds or RFC3339 values. E.g. '2020-01-01T20:07:00Z'",
		},
		&cli.StringFlag{
			Name: vmNativeSrcAddr,
			Usage: "VictoriaMetrics address to perform export from. \n" +
				" Should be the same as --httpListenAddr value for single-node version or vmselect component." +
				" If exporting from cluster version see https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format",
			Required: true,
		},
		&cli.StringFlag{
			Name:    vmNativeSrcUser,
			Usage:   "VictoriaMetrics username for basic auth",
			EnvVars: []string{"VM_NATIVE_SRC_USERNAME"},
		},
		&cli.StringFlag{
			Name:    vmNativeSrcPassword,
			Usage:   "VictoriaMetrics password for basic auth",
			EnvVars: []string{"VM_NATIVE_SRC_PASSWORD"},
		},
		&cli.StringFlag{
			Name: vmNativeDstAddr,
			Usage: "VictoriaMetrics address to perform import to. \n" +
				" Should be the same as --httpListenAddr value for single-node version or vminsert component." +
				" If importing into cluster version see https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format",
			Required: true,
		},
		&cli.StringFlag{
			Name:    vmNativeDstUser,
			Usage:   "VictoriaMetrics username for basic auth",
			EnvVars: []string{"VM_NATIVE_DST_USERNAME"},
		},
		&cli.StringFlag{
			Name:    vmNativeDstPassword,
			Usage:   "VictoriaMetrics password for basic auth",
			EnvVars: []string{"VM_NATIVE_DST_PASSWORD"},
		},
		&cli.StringSliceFlag{
			Name:  vmExtraLabel,
			Value: nil,
			Usage: "Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flag" +
				"will have priority. Flag can be set multiple times, to add few additional labels.",
		},
		&cli.Int64Flag{
			Name: vmRateLimit,
			Usage: "Optional data transfer rate limit in bytes per second.\n" +
				"By default the rate limit is disabled. It can be useful for limiting load on source or destination databases.",
		},
	}
)

func mergeFlags(flags ...[]cli.Flag) []cli.Flag {
	var result []cli.Flag
	for _, f := range flags {
		result = append(result, f...)
	}
	return result
}
