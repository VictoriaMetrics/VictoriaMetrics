package main

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
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
	vmDisableProgressBar = "vm-disable-progress-bar"

	// also used in vm-native
	vmExtraLabel = "vm-extra-label"
	vmRateLimit  = "vm-rate-limit"

	vmInterCluster = "vm-intercluster"
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
				"By default, the rate limit is disabled. It can be useful for limiting load on configured via '--vmAddr' destination.",
		},
		&cli.BoolFlag{
			Name:  vmDisableProgressBar,
			Usage: "Whether to disable progress bar per each worker during the import.",
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
	influxSkipDatabaseLabel         = "influx-skip-database-label"
	influxPrometheusMode            = "influx-prometheus-mode"
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
		&cli.BoolFlag{
			Name:  influxSkipDatabaseLabel,
			Usage: "Wether to skip adding the label 'db' to timeseries.",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  influxPrometheusMode,
			Usage: "Wether to restore the original timeseries name previously written from Prometheus to InfluxDB v1 via remote_write.",
			Value: false,
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
	vmNativeFilterMatch       = "vm-native-filter-match"
	vmNativeFilterTimeStart   = "vm-native-filter-time-start"
	vmNativeFilterTimeEnd     = "vm-native-filter-time-end"
	vmNativeFilterTimeReverse = "vm-native-filter-time-reverse"
	vmNativeStepInterval      = "vm-native-step-interval"

	vmNativeDisableBinaryProtocol     = "vm-native-disable-binary-protocol"
	vmNativeDisableHTTPKeepAlive      = "vm-native-disable-http-keep-alive"
	vmNativeDisablePerMetricMigration = "vm-native-disable-per-metric-migration"

	vmNativeSrcAddr        = "vm-native-src-addr"
	vmNativeSrcUser        = "vm-native-src-user"
	vmNativeSrcPassword    = "vm-native-src-password"
	vmNativeSrcHeaders     = "vm-native-src-headers"
	vmNativeSrcBearerToken = "vm-native-src-bearer-token"

	vmNativeDstAddr        = "vm-native-dst-addr"
	vmNativeDstUser        = "vm-native-dst-user"
	vmNativeDstPassword    = "vm-native-dst-password"
	vmNativeDstHeaders     = "vm-native-dst-headers"
	vmNativeDstBearerToken = "vm-native-dst-bearer-token"
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
			Name:     vmNativeFilterTimeStart,
			Usage:    "The time filter may contain different timestamp formats. See more details here https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#timestamp-formats",
			Required: true,
		},
		&cli.StringFlag{
			Name:  vmNativeFilterTimeEnd,
			Usage: "The time filter may contain different timestamp formats. See more details here https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#timestamp-formats",
		},
		&cli.StringFlag{
			Name: vmNativeStepInterval,
			Usage: fmt.Sprintf("The time interval to split the migration into steps. For example, to migrate 1y of data with '--%s=month' vmctl will execute it in 12 separate requests from the beginning of the time range to its end. To reverse the order use '--%s'. Requires setting '--%s'. Valid values are '%s','%s','%s','%s','%s'.",
				vmNativeStepInterval, vmNativeFilterTimeReverse, vmNativeFilterTimeStart, stepper.StepMonth, stepper.StepWeek, stepper.StepDay, stepper.StepHour, stepper.StepMinute),
			Value: stepper.StepMonth,
		},
		&cli.BoolFlag{
			Name:  vmNativeFilterTimeReverse,
			Usage: fmt.Sprintf("Whether to reverse the order of time intervals split by '--%s' cmd-line flag. When set, the migration will start from the newest to the oldest data.", vmNativeStepInterval),
			Value: false,
		},
		&cli.BoolFlag{
			Name:  vmNativeDisableHTTPKeepAlive,
			Usage: "Disable HTTP persistent connections for requests made to VictoriaMetrics components during export",
			Value: false,
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
			Name: vmNativeSrcHeaders,
			Usage: "Optional HTTP headers to send with each request to the corresponding source address. \n" +
				"For example, --vm-native-src-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding source address. \n" +
				"Multiple headers must be delimited by '^^': --vm-native-src-headers='header1:value1^^header2:value2'",
		},
		&cli.StringFlag{
			Name:  vmNativeSrcBearerToken,
			Usage: "Optional bearer auth token to use for the corresponding `--vm-native-src-addr`",
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
		&cli.StringFlag{
			Name: vmNativeDstHeaders,
			Usage: "Optional HTTP headers to send with each request to the corresponding destination address. \n" +
				"For example, --vm-native-dst-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding destination address. \n" +
				"Multiple headers must be delimited by '^^': --vm-native-dst-headers='header1:value1^^header2:value2'",
		},
		&cli.StringFlag{
			Name:  vmNativeDstBearerToken,
			Usage: "Optional bearer auth token to use for the corresponding `--vm-native-dst-addr`",
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
				"By default, the rate limit is disabled. It can be useful for limiting load on source or destination databases.",
		},
		&cli.BoolFlag{
			Name: vmInterCluster,
			Usage: "Enables cluster-to-cluster migration mode with automatic tenants data migration.\n" +
				fmt.Sprintf(" In this mode --%s flag format is: 'http://vmselect:8481/'. --%s flag format is: http://vminsert:8480/. \n", vmNativeSrcAddr, vmNativeDstAddr) +
				" TenantID will be appended automatically after discovering tenants from src.",
		},
		&cli.UintFlag{
			Name:  vmConcurrency,
			Usage: "Number of workers concurrently performing import requests to VM",
			Value: 2,
		},
		&cli.BoolFlag{
			Name:  vmNativeDisablePerMetricMigration,
			Usage: "Defines whether to disable per-metric migration and migrate all data via one connection. In this mode, vmctl makes less export/import requests, but can't provide a progress bar or retry failed requests.",
			Value: false,
		},
		&cli.BoolFlag{
			Name: vmNativeDisableBinaryProtocol,
			Usage: "Whether to use https://docs.victoriametrics.com/#how-to-export-data-in-json-line-format" +
				"instead of https://docs.victoriametrics.com/#how-to-export-data-in-native-format API." +
				"Binary export/import API protocol implies less network and resource usage, as it transfers compressed binary data blocks." +
				"Non-binary export/import API is less efficient, but supports deduplication if it is configured on vm-native-src-addr side.",
			Value: false,
		},
	}
)

const (
	remoteRead                   = "remote-read"
	remoteReadUseStream          = "remote-read-use-stream"
	remoteReadConcurrency        = "remote-read-concurrency"
	remoteReadFilterTimeStart    = "remote-read-filter-time-start"
	remoteReadFilterTimeEnd      = "remote-read-filter-time-end"
	remoteReadFilterTimeReverse  = "remote-read-filter-time-reverse"
	remoteReadFilterLabel        = "remote-read-filter-label"
	remoteReadFilterLabelValue   = "remote-read-filter-label-value"
	remoteReadStepInterval       = "remote-read-step-interval"
	remoteReadSrcAddr            = "remote-read-src-addr"
	remoteReadUser               = "remote-read-user"
	remoteReadPassword           = "remote-read-password"
	remoteReadHTTPTimeout        = "remote-read-http-timeout"
	remoteReadHeaders            = "remote-read-headers"
	remoteReadInsecureSkipVerify = "remote-read-insecure-skip-verify"
	remoteReadDisablePathAppend  = "remote-read-disable-path-append"
)

var (
	remoteReadFlags = []cli.Flag{
		&cli.IntFlag{
			Name:  remoteReadConcurrency,
			Usage: "Number of concurrently running remote read readers",
			Value: 1,
		},
		&cli.TimestampFlag{
			Name:     remoteReadFilterTimeStart,
			Usage:    "The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'",
			Layout:   time.RFC3339,
			Required: true,
		},
		&cli.TimestampFlag{
			Name:   remoteReadFilterTimeEnd,
			Usage:  "The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'",
			Layout: time.RFC3339,
		},
		&cli.StringFlag{
			Name:  remoteReadFilterLabel,
			Usage: "Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.",
			Value: "__name__",
		},
		&cli.StringFlag{
			Name:  remoteReadFilterLabelValue,
			Usage: fmt.Sprintf("Prometheus regular expression to filter label from %q flag.", remoteReadFilterLabelValue),
			Value: ".*",
		},
		&cli.BoolFlag{
			Name:  remoteRead,
			Usage: "Use Prometheus remote read protocol",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  remoteReadUseStream,
			Usage: "Defines whether to use SAMPLES or STREAMED_XOR_CHUNKS mode. By default, is uses SAMPLES mode. See https://prometheus.io/docs/prometheus/latest/querying/remote_read_api/#streamed-chunks",
			Value: false,
		},
		&cli.StringFlag{
			Name: remoteReadStepInterval,
			Usage: fmt.Sprintf("The time interval to split the migration into steps. For example, to migrate 1y of data with '--%s=month' vmctl will execute it in 12 separate requests from the beginning of the time range to its end. To reverse the order use '--%s'. Requires setting '--%s'. Valid values are '%s','%s','%s','%s','%s'.",
				remoteReadStepInterval, remoteReadFilterTimeReverse, remoteReadFilterTimeStart, stepper.StepMonth, stepper.StepWeek, stepper.StepDay, stepper.StepHour, stepper.StepMinute), Required: true,
		},
		&cli.BoolFlag{
			Name:  remoteReadFilterTimeReverse,
			Usage: fmt.Sprintf("Whether to reverse the order of time intervals split by '--%s' cmd-line flag. When set, the migration will start from the newest to the oldest data.", remoteReadStepInterval),
			Value: false,
		},
		&cli.StringFlag{
			Name:     remoteReadSrcAddr,
			Usage:    "Remote read address to perform read from.",
			Required: true,
		},
		&cli.StringFlag{
			Name:    remoteReadUser,
			Usage:   "Remote read username for basic auth",
			EnvVars: []string{"REMOTE_READ_USERNAME"},
		},
		&cli.StringFlag{
			Name:    remoteReadPassword,
			Usage:   "Remote read password for basic auth",
			EnvVars: []string{"REMOTE_READ_PASSWORD"},
		},
		&cli.DurationFlag{
			Name:  remoteReadHTTPTimeout,
			Usage: "Timeout defines timeout for HTTP requests made by remote read client",
		},
		&cli.StringFlag{
			Name:  remoteReadHeaders,
			Value: "",
			Usage: "Optional HTTP headers to send with each request to the corresponding remote source storage \n" +
				"For example, --remote-read-headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding remote source storage. \n" +
				"Multiple headers must be delimited by '^^': --remote-read-headers='header1:value1^^header2:value2'",
		},
		&cli.BoolFlag{
			Name:  remoteReadInsecureSkipVerify,
			Usage: "Whether to skip TLS certificate verification when connecting to the remote read address",
			Value: false,
		},
		&cli.BoolFlag{
			Name:  remoteReadDisablePathAppend,
			Usage: "Whether to disable automatic appending of the /api/v1/read suffix to --remote-read-src-addr",
			Value: false,
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
