---
title: Prometheus
weight: 1
menu:
  docs:
    parent: "vmctl"
    identifier: "vmctl-prometheus"
    weight: 1
---
`vmctl` can migrate historical data from Prometheus to VictoriaMetrics by reading [Prometheus snapshot](https://prometheus.io/docs/prometheus/latest/querying/api/#snapshot).
See `./vmctl prometheus --help` for details and full list of flags. Also see Prometheus [related articles](https://docs.victoriametrics.com/victoriametrics/vmctl/#articles).

To start migration, take [a snapshot of Prometheus data](https://www.robustperception.io/taking-snapshots-of-prometheus-data)
and made it accessible for vmctl in the same filesystem:
```sh
./vmctl prometheus \
  --vm-addr=<victoriametrics-addr>:8428 \
  --prom-snapshot=/path/to/snapshot
```

_See how to configure [--vm-addr](https://docs.victoriametrics.com/victoriametrics/vmctl/#configuring-victoriametrics)._

As soon as required flags are provided and healthchecks are done, `vmctl` will start exploring Prometheus snapshot.
It fetches all available blocks from the snapshot, reads the metadata and prints stats for discovered data:
```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --vm-addr=http://localhost:8428
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 14;
  blocks skipped: 0;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1582409128139 (2020-02-22T22:05:28Z);
  samples: 32549106;
  series: 27289.
Found 14 blocks to import. Continue? [Y/n] y
14 / 14 [-------------------------------------------------------------------------------------------] 100.00% 0 p/s
2020/02/23 15:50:03 Import finished!
2020/02/23 15:50:03 VictoriaMetrics importer stats:
  idle duration: 6.152953029s;
  time spent while importing: 44.908522491s;
  total samples: 32549106;
  samples/s: 724786.84;
  total bytes: 669.1 MB;
  bytes/s: 14.9 MB;
  import requests: 323;
  import requests retries: 0;
2020/02/23 15:50:03 Total time: 51.077451066s
```

## Filtering

Filtering by time is configured via flags `--prom-filter-time-start` and `--prom-filter-time-end` in RFC3339 format.
The filter is applied twice: to drop blocks on exploration phase and to filter timeseries within the blocks:
```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --prom-filter-time-start=2020-02-07T00:07:01Z \
  --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Found 2 blocks to import. Continue? [Y/n] y
```

Filtering by labels is configured with following flags:
- `--prom-filter-label` - the label name, e.g. `__name__` or `instance`;
- `--prom-filter-label-value` - the regular expression to filter the label value. By default, matches all `.*`

For example:
```sh
./vmctl prometheus --prom-snapshot=/path/to/snapshot \
  --prom-filter-label="__name__" \
  --prom-filter-label-value="promhttp.*" \
  --prom-filter-time-start=2020-02-07T00:07:01Z \
  --prom-filter-time-end=2020-02-11T00:07:01Z
Prometheus import mode
Prometheus snapshot stats:
  blocks found: 2;
  blocks skipped: 12;
  min time: 1581288163058 (2020-02-09T22:42:43Z);
  max time: 1581328800000 (2020-02-10T10:00:00Z);
  samples: 1657698;
  series: 3930.
Found 2 blocks to import. Continue? [Y/n] y
14 / 14 [-----------------------------------------------------------------------------------------------] 100.00% ? p/s
2020/02/23 15:51:07 Import finished!
2020/02/23 15:51:07 VictoriaMetrics importer stats:
  idle duration: 0s;
  time spent while importing: 37.415461ms;
  total samples: 10128;
  samples/s: 270690.24;
  total bytes: 195.2 kB;
  bytes/s: 5.2 MB;
  import requests: 2;
  import requests retries: 0;
2020/02/23 15:51:07 Total time: 7.153158218s
```

## Configuration

In Prometheus mode, vmctl reads data from the given snapshot using Prometheus library. Its performance for is limited
by the library performance, by the disk IO, and by `--prom-concurrency` - the number of concurrent readers. Prefer
setting `--prom-concurrency` to the number of available to vmctl CPU cores.

See general [vmctl migration tips](https://docs.victoriametrics.com/victoriametrics/vmctl/#migration-tips).

See `./vmctl prometheus --help` for details and full list of flags:
```shellhelp
   --prom-snapshot value            Path to Prometheus snapshot. Pls see for details https://www.robustperception.io/taking-snapshots-of-prometheus-data
   --prom-concurrency value         Number of concurrently running snapshot readers (default: 1)
   --prom-filter-time-start value   The time filter in RFC3339 format to select timeseries with timestamp equal or higher than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-time-end value     The time filter in RFC3339 format to select timeseries with timestamp equal or lower than provided value. E.g. '2020-01-01T20:07:00Z'
   --prom-filter-label value        Prometheus label name to filter timeseries by. E.g. '__name__' will filter timeseries by name.
   --prom-filter-label-value value  Prometheus regular expression to filter label from "prom-filter-label" flag. (default: ".*")
```