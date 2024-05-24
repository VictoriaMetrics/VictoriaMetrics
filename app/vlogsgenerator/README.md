# vlogsgenerator

Logs generator for [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/).

## How to build vlogsgenerator?

Run `make vlogsgenerator` from the repository root. This builds `bin/vlogsgenerator` binary.

## How run vlogsgenerator?

`vlogsgenerator` generates logs in [JSON line format](https://jsonlines.org/) suitable for the ingestion
via [`/insert/jsonline` endpoint at VictoriaLogs](https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api).

By default it writes the generated logs into `stdout`. For example, the following command writes generated logs to `stdout`:

```
bin/vlogsgenerator
```

It is possible to redirect the generated logs to file. For example, the following command writes the generated logs to `logs.json` file:

```
bin/vlogsgenerator > logs.json
```

The generated logs at `logs.json` file can be inspected with the following command:

```
head logs.json | jq .
```

Below is an example output:

```json
{
  "_time": "2024-05-08T14:34:00.854Z",
  "_msg": "message for the stream 8 and worker 0; ip=185.69.136.129; uuid=b4fe8f1a-c93c-dea3-ba11-5b9f0509291e; u64=8996587920687045253",
  "host": "host_8",
  "worker_id": "0",
  "run_id": "f9b3deee-e6b6-7f56-5deb-1586e4e81725",
  "const_0": "some value 0 8",
  "const_1": "some value 1 8",
  "const_2": "some value 2 8",
  "var_0": "some value 0 12752539384823438260",
  "dict_0": "warn",
  "dict_1": "info",
  "u8_0": "6",
  "u16_0": "35202",
  "u32_0": "1964973739",
  "u64_0": "4810489083243239145",
  "float_0": "1.868",
  "ip_0": "250.34.75.125",
  "timestamp_0": "1799-03-16T01:34:18.311Z",
  "json_0": "{\"foo\":\"bar_3\",\"baz\":{\"a\":[\"x\",\"y\"]},\"f3\":NaN,\"f4\":32}"
}
{
  "_time": "2024-05-08T14:34:00.854Z",
  "_msg": "message for the stream 9 and worker 0; ip=164.244.254.194; uuid=7e8373b1-ce0d-1ce7-8e96-4bcab8955598; u64=13949903463741076522",
  "host": "host_9",
  "worker_id": "0",
  "run_id": "f9b3deee-e6b6-7f56-5deb-1586e4e81725",
  "const_0": "some value 0 9",
  "const_1": "some value 1 9",
  "const_2": "some value 2 9",
  "var_0": "some value 0 5371555382075206134",
  "dict_0": "INFO",
  "dict_1": "FATAL",
  "u8_0": "219",
  "u16_0": "31459",
  "u32_0": "3918836777",
  "u64_0": "6593354256620219850",
  "float_0": "1.085",
  "ip_0": "253.151.88.158",
  "timestamp_0": "2042-10-05T16:42:57.082Z",
  "json_0": "{\"foo\":\"bar_5\",\"baz\":{\"a\":[\"x\",\"y\"]},\"f3\":NaN,\"f4\":27}"
}
```

The `run_id` field uniquely identifies every `vlogsgenerator` invocation.

### How to write logs to VictoriaLogs?

The generated logs can be written directly to VictoriaLogs by passing the address of [`/insert/jsonline` endpoint](https://docs.victoriametrics.com/victorialogs/data-ingestion/#json-stream-api)
to `-addr` command-line flag. For example, the following command writes the generated logs to VictoriaLogs running at `localhost`:

```
bin/vlogsgenerator -addr=http://localhost:9428/insert/jsonline
```

### Configuration

`vlogsgenerator` accepts various command-line flags, which can be used for configuring the number and the shape of the generated logs.
These flags can be inspected by running `vlogsgenerator -help`. Below are the most interesting flags:

* `-start` - starting timestamp for generating logs. Logs are evenly generated on the [`-start` ... `-end`] interval.
* `-end` - ending timestamp for generating logs. Logs are evenly generated on the [`-start` ... `-end`] interval.
* `-activeStreams` - the number of active [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields) to generate.
* `-logsPerStream` - the number of log entries to generate per each log stream. Log entries are evenly distributed on the [`-start` ... `-end`] interval.

The total number of generated logs can be calculated as `-activeStreams` * `-logsPerStream`.

For example, the following command generates `1_000_000` log entries on the time range `[2024-01-01 - 2024-02-01]` across `100`
[log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields), where every logs stream contains `10_000` log entries,
and writes them to `http://localhost:9428/insert/jsonline`:

```
bin/vlogsgenerator \
  -start=2024-01-01 -end=2024-02-01 \
  -activeStreams=100 \
  -logsPerStream=10_000 \
  -addr=http://localhost:9428/insert/jsonline
```

### Churn rate

It is possible to generate churn rate for active [log streams](https://docs.victoriametrics.com/victorialogs/keyconcepts/#stream-fields)
by specifying `-totalStreams` command-line flag bigger than `-activeStreams`. For example, the following command generates
logs for `1000` total streams, while the number of active streams equals to `100`. This means that at every time there are logs for `100` streams,
but these streams change over the given [`-start` ... `-end`] time range, so the total number of streams on the given time range becomes `1000`:

```
bin/vlogsgenerator \
  -start=2024-01-01 -end=2024-02-01 \
  -activeStreams=100 \
  -totalStreams=1_000 \
  -logsPerStream=10_000 \
  -addr=http://localhost:9428/insert/jsonline
```

In this case the total number of generated logs equals to `-totalStreams` * `-logsPerStream` = `10_000_000`.

### Benchmark tuning

By default `vlogsgenerator` generates and writes logs by a single worker. This may limit the maximum data ingestion rate during benchmarks.
The number of workers can be changed via `-workers` command-line flag. For example, the following command generates and writes logs with `16` workers:

```
bin/vlogsgenerator \
  -start=2024-01-01 -end=2024-02-01 \
  -activeStreams=100 \
  -logsPerStream=10_000 \
  -addr=http://localhost:9428/insert/jsonline \
  -workers=16
```

### Output statistics

Every 10 seconds `vlogsgenerator` writes statistics about the generated logs into `stderr`. The frequency of the generated statistics can be adjusted via `-statInterval` command-line flag.
For example, the following command writes statistics every 2 seconds:

```
bin/vlogsgenerator \
  -start=2024-01-01 -end=2024-02-01 \
  -activeStreams=100 \
  -logsPerStream=10_000 \
  -addr=http://localhost:9428/insert/jsonline \
  -statInterval=2s
```
