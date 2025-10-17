---
title: Kafka
weight: 9
menu:
  docs:
    parent: "integrations-vm"
    weight: 9
---

> This integration is supported only in [Enterprise version](https://docs.victoriametrics.com/victoriametrics/enterprise/) of vmagent.

[vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) can read/write metrics from/to Kafka.

## Reading metrics

`vmagent` can read metrics in various formats from Kafka messages.
Use `-kafka.consumer.topic.defaultFormat` or `-kafka.consumer.topic.format` command-line flags to configure the expected format:

* `promremotewrite` - [Prometheus remote_write](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write).
  Messages in this format can be sent by vmagent - see [these docs](#writing-metrics).
* `influx` - [InfluxDB line protocol format](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/).
* `prometheus` - [Prometheus text exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format)
  and [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md).
* `graphite` - [Graphite plaintext format](https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol).
* `jsonline` - [JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-json-line-format).
* `opentelemetry`{{% available_from "v1.128.0" %}}  - [Opentelemetry format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#sending-data-via-opentelemetry)

For Kafka messages in the `promremotewrite` format, `vmagent` will automatically detect whether they are using [the Prometheus remote write protocol](https://prometheus.io/docs/specs/remote_write_spec/#protocol)
or [the VictoriaMetrics remote write protocol](https://docs.victoriametrics.com/victoriametrics/vmagent/#victoriametrics-remote-write-protocol), and handle them accordingly.

 vmagent performs manual commit for each processed kafka message in order to guarantee message delivery. This behavior could be changed with flag `-kafka.consumer.topic.options='enable.auto.commit'`, in this scenario
kafka client will automatically commit offset based on value of `auto.commit.interval.ms=5000` (5s by default).

Every Kafka message may contain multiple lines in `influx`, `prometheus`, `graphite` and `jsonline` format delimited by `\n`.

`vmagent` consumes messages from Kafka topics specified via `-kafka.consumer.topic` command-line flag. 
Multiple topics can be specified by passing multiple `-kafka.consumer.topic` command-line flags to `vmagent`.

`vmagent` consumes messages from Kafka brokers specified via `-kafka.consumer.topic.brokers` command-line flag.
Multiple brokers can be specified per each `-kafka.consumer.topic` by passing a list of brokers delimited by `;`.
For example:
```sh
./bin/vmagent 
      -kafka.consumer.topic='topic-a' 
      -kafka.consumer.topic.brokers='host1:9092;host2:9092' 
      -kafka.consumer.topic='topic-b' 
      -kafka.consumer.topic.brokers='host3:9092;host4:9092'
```

This command starts `vmagent` which reads messages from `topic-a` at `host1:9092` and `host2:9092` brokers and messages
from `topic-b` at `host3:9092` and `host4:9092` brokers.

When using YAML configuration (e.g. [Helm charts](https://github.com/VictoriaMetrics/helm-charts) or [Kubernetes operator](https://docs.victoriametrics.com/operator/))
keys provided in `extraArgs` **must be unique**. To achieve the same configuration as in the example above, use the following configuration:
```yaml
extraArgs:
  "kafka.consumer.topic": "topic-a,topic-b"
  "kafka.consumer.topic.brokers": "host1:9092;host2:9092,host3:9092;host4:9092"
```
Note that list of brokers for the same topic is separated by `;` and different groups of brokers are separated by `,`.

The following command starts `vmagent`, which reads metrics in InfluxDB line protocol format from Kafka broker at `localhost:9092`
from the topic `metrics-by-telegraf` and sends them to remote storage at `http://localhost:8428/api/v1/write`:
```sh
./bin/vmagent -remoteWrite.url=http://localhost:8428/api/v1/write \
       -kafka.consumer.topic.brokers=localhost:9092 \
       -kafka.consumer.topic.format=influx \
       -kafka.consumer.topic=metrics-by-telegraf \
       -kafka.consumer.topic.groupID=some-id
```

It is expected that [Telegraf](https://github.com/influxdata/telegraf) sends metrics to the `metrics-by-telegraf` topic with the following config:

```yaml
[[outputs.kafka]]
brokers = ["localhost:9092"]
topic = "influx"
data_format = "influx"
```

`vmagent` buffers messages read from Kafka topic on local disk if the remote storage at `-remoteWrite.url` cannot
keep up with the data ingestion rate. Buffering can be disabled via `-remoteWrite.disableOnDiskQueue` cmd-line flags.
See more about [disabling on-disk persistence](https://docs.victoriametrics.com/victoriametrics/vmagent/#disabling-on-disk-persistence).

See also [how to write metrics to multiple distinct tenants](https://docs.victoriametrics.com/victoriametrics/vmagent/#multitenancy).

### Consumer command-line flags 

```sh
  -kafka.consumer.topic array
        Kafka topic names for data consumption. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.basicAuth.password array
        Optional basic auth password for -kafka.consumer.topic.  Must be used in conjunction with any supported auth methods for kafka client, specified by flag -kafka.consumer.topic.options='security.protocol=SASL_SSL;sasl.mechanisms=PLAIN' . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.basicAuth.username array
        Optional basic auth username for -kafka.consumer.topic. Must be used in conjunction with any supported auth methods for kafka client, specified by flag -kafka.consumer.topic.options='security.protocol=SASL_SSL;sasl.mechanisms=PLAIN' . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.brokers array
        List of brokers to connect for given topic, e.g. -kafka.consumer.topic.broker=host-1:9092;host-2:9092 . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.concurrency array
        Configures consumer concurrency for topic specified via -kafka.consumer.topic flag. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 1)
        Supports array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.defaultFormat string
        Expected data format in the topic if -kafka.consumer.topic.format is skipped. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default "promremotewrite")
  -kafka.consumer.topic.format array
        data format for corresponding kafka topic. Valid formats: influx, prometheus, promremotewrite, graphite, jsonline and opentelemetry. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.groupID array
        Defines group.id for topic. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.isGzipped array
        Enables gzip setting for topic messages payload. Only prometheus, jsonline, graphite and influx formats accept gzipped messages.See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports array of values separated by comma or specified via multiple flags.
  -kafka.consumer.topic.options array
        Optional key=value;key1=value2 settings for topic consumer. See full configuration options at https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
```

## Writing metrics

`vmagent` writes data to Kafka with `at-least-once` semantics if `-remoteWrite.url` contains e.g. Kafka URL. 
For example, if `vmagent` is started with `-remoteWrite.url=kafka://localhost:9092/?topic=prom-rw`,
then it will send Prometheus remote_write messages to Kafka bootstrap server at `localhost:9092` with the topic `prom-rw`.
These messages can be read later from Kafka by another `vmagent` - see [how to read metrics from kafka](#reading-metrics).

Additional Kafka options can be passed as query params to `-remoteWrite.url`. For instance, `kafka://localhost:9092/?topic=prom-rw&client.id=my-favorite-id`
sets `client.id` Kafka option to `my-favorite-id`. The full list of Kafka options is available [here](https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md).

By default, `vmagent` sends compressed messages using Google's Snappy, as defined in [the Prometheus remote write protocol](https://prometheus.io/docs/specs/remote_write_spec/#protocol).
To switch to [the VictoriaMetrics remote write protocol](https://docs.victoriametrics.com/victoriametrics/vmagent/#victoriametrics-remote-write-protocol)
and reduce network bandwidth, simply set the `-remoteWrite.forceVMProto=true` flag. It is also possible to adjust 
the compression level for the VictoriaMetrics remote write protocol using the `-remoteWrite.vmProtoCompressLevel` command-line flag.

By default, `vmagent` uses a single producer per topic. This can be changed with setting `kafka://localhost:9092/?concurrency=<int>`,
where `<int>` is an integer defining the number additional workers. It could improve throughput in networks with high latency.
Or if Kafka brokers located at different region/availability-zone.

### Estimating message size and rate

If you are migrating from remote write to Kafka, the request rate and request body size of remote write can roughly 
correspond to the message rate and size of Kafka.

vmagent organizes scraped/ingested data into **blocks**. A block contains multiple time series and samples.
Each block is compressed with Snappy or ZSTD before being sent out by the remote write or the Kafka producer.

To get the request rate of remote write (as the estimated produce rate of Kafka), use the following MetricsQL:
```metricsql
sum(rate(vmagent_remotewrite_requests_total{}[1m])) 
```

Similarly, the average size of the compressed block of remote write (serving as the estimated message size of Kafka) is as follows:
```metricsql
sum(rate(vmagent_remotewrite_conn_bytes_written_total{}[1m]))
 / 
sum(rate(vmagent_remotewrite_requests_total{}[1m])) 
```

Please note that the remote write body and Kafka message need to use the same compression algorithm to serve as
estimation references. See more in [the VictoriaMetrics remote write protocol](https://docs.victoriametrics.com/victoriametrics/vmagent/#victoriametrics-remote-write-protocol).

### Kafka broker authorization and authentication

Two types of auth are supported:

* sasl with username and password:

```sh
./bin/vmagent -remoteWrite.url='kafka://localhost:9092/?topic=prom-rw&security.protocol=SASL_SSL&sasl.mechanisms=PLAIN' \
    -remoteWrite.basicAuth.username=user \
    -remoteWrite.basicAuth.password=password
```

* tls certificates:

```sh
./bin/vmagent -remoteWrite.url='kafka://localhost:9092/?topic=prom-rw&security.protocol=SSL' \
    -remoteWrite.tlsCAFile=/opt/ca.pem \
    -remoteWrite.tlsCertFile=/opt/cert.pem \
    -remoteWrite.tlsKeyFile=/opt/key.pem
```
