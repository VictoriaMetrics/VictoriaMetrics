---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file should not be updated manually. Run make docs-update-flags while preparing a new release to sync flags in docs from actual binaries. -->
```shellhelp
NAME:
   vmctl - VictoriaMetrics command-line tool

USAGE:
   vmctl [global options] command [command options]


COMMANDS:
   opentsdb      Migrate time series from OpenTSDB
   influx        Migrate time series from InfluxDB
   remote-read   Migrate time series via Prometheus remote-read protocol
   prometheus    Migrate time series from Prometheus
   vm-native     Migrate time series between VictoriaMetrics installations
   verify-block  Verifies exported block with VictoriaMetrics Native format
   help, h       Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```
