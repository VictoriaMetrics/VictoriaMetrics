---
title: Google PubSub
weight: 8
menu:
  docs:
    parent: "integrations-vm"
    weight: 8
---

> This integration is supported only in [Enterprise version](https://docs.victoriametrics.com/victoriametrics/enterprise/) of vmagent.

[vmagent](https://docs.victoriametrics.com/victoriametrics/vmagent/) can read/write metrics from/to [Google PubSub](https://cloud.google.com/pubsub).

## Reading metrics

`vmagent` can read metrics in various formats from Google PubSub messages.
Use `-gcp.pubsub.subscribe.defaultMessageFormat` and `-gcp.pubsub.subscribe.topicSubscription.messageFormat` command-line flags to configure the expected format:
* `promremotewrite` - [Prometheus remote_write](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write).
  Messages in this format can be sent by vmagent - see [these docs](#writing-metrics).
* `influx` - [InfluxDB line protocol format](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/).
* `prometheus` - [Prometheus text exposition format](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-based-format)
  and [OpenMetrics format](https://github.com/OpenObservability/OpenMetrics/blob/master/specification/OpenMetrics.md).
* `graphite` - [Graphite plaintext format](https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol).
* `jsonline` - [JSON line format](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-json-line-format).

Every PubSub message may contain multiple lines in `influx`, `prometheus`, `graphite` and `jsonline` format delimited by `\n`.

`vmagent` consumes messages from PubSub topic subscriptions specified by `-gcp.pubsub.subscribe.topicSubscription` command-line flag.
Multiple topics can be specified by passing multiple `-gcp.pubsub.subscribe.topicSubscription` command-line flags to `vmagent`.

`vmagent` uses a standard Google authorization mechanism for topic access. It's possible to specify credentials directly 
via `-gcp.pubsub.subscribe.credentialsFile` command-line flag. 

The following command configures `vmagent` to read metrics in [InfluxDB line protocol format](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/)
from PubSub `projects/victoriametrics-vmagent-pub-sub-test/subscriptions/telegraf-testing` and send them to the remote storage at `http://localhost:8428/api/v1/write`:
```sh
./bin/vmagent -remoteWrite.url=http://localhost:8428/api/v1/write \
       -gcp.pubsub.subscribe.topicSubscription=projects/victoriametrics-vmagent-pub-sub-test/subscriptions/telegraf-testing \
       -gcp.pubsub.subscribe.topicSubscription.messageFormat=influx
```

It is expected that [Telegraf](https://github.com/influxdata/telegraf) sends metrics to the `telegraf-testing` topic 
at the `victoriametrics-vmagent-pub-sub-test` project with the following config:
```yaml
[[outputs.cloud_pubsub]]
  project = "victoriametrics-vmagent-pub-sub-test"
  topic = "telegraf-testing"
  data_format = "influx"
```

`vmagent` buffers messages read from Google PubSub topic on local disk if the remote storage at `-remoteWrite.url` cannot
keep up with the data ingestion rate. Buffering can be disabled via `-remoteWrite.disableOnDiskQueue` cmd-line flags. 
See more about [disabling on-disk persistence](https://docs.victoriametrics.com/victoriametrics/vmagent/#disabling-on-disk-persistence).

See also [how to write metrics to multiple distinct tenants](https://docs.victoriametrics.com/victoriametrics/vmagent/#multitenancy).

### Multiple topics

`vmagent` can read messages from different topics in different formats. For example, the following command starts `vmagent` that reads plaintext
[Influx](https://docs.influxdata.com/influxdb/cloud/reference/syntax/line-protocol/) messages from `telegraf-testing` topic
and gzipped [JSON line](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#json-line-format) messages from `json-line-testing` topic:
```sh
./bin/vmagent -remoteWrite.url=http://localhost:8428/api/v1/write \
       -gcp.pubsub.subscribe.topicSubscription=projects/victoriametrics-vmagent-pub-sub-test/subscriptions/telegraf-testing \
       -gcp.pubsub.subscribe.topicSubscription.messageFormat=influx \
       -gcp.pubsub.subscribe.topicSubscription.isGzipped=false \
       -gcp.pubsub.subscribe.topicSubscription=projects/victoriametrics-vmagent-pub-sub-test/subscriptions/json-line-testing \
       -gcp.pubsub.subscribe.topicSubscription.messageFormat=jsonline \
       -gcp.pubsub.subscribe.topicSubscription.isGzipped=true
```

### Consumer command-line flags

```sh
  -gcp.pubsub.subscribe.credentialsFile string
        Path to file with GCP credentials to use for PubSub client. If not set, default credentials are used (see Workload Identity for K8S or https://cloud.google.com/docs/authentication/application-default-credentials ). See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -gcp.pubsub.subscribe.defaultMessageFormat string
        Default message format if -gcp.pubsub.subscribe.topicSubscription.messageFormat is missing. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default "promremotewrite")
  -gcp.pubsub.subscribe.topicSubscription array
        GCP PubSub topic subscription in the format: projects/<project-id>/subscriptions/<subscription-name>. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
  -gcp.pubsub.subscribe.topicSubscription.concurrency array
        The number of concurrently processed messages for topic subscription specified via -gcp.pubsub.subscribe.topicSubscription flag. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 0)
        Supports array of values separated by comma or specified via multiple flags.
  -gcp.pubsub.subscribe.topicSubscription.isGzipped array
        Enables gzip decompression for messages payload at the corresponding -gcp.pubsub.subscribe.topicSubscription. Only prometheus, jsonline, graphite and influx formats accept gzipped messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports array of values separated by comma or specified via multiple flags.
  -gcp.pubsub.subscribe.topicSubscription.messageFormat array
        Message format for the corresponding -gcp.pubsub.subscribe.topicSubscription. Valid formats: influx, prometheus, promremotewrite, graphite, jsonline . See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        Supports an array of values separated by comma or specified via multiple flags.
```

## Writing metrics

`vmagent` writes data into Google PubSub if `-remoteWrite.url` command-line flag starts with `pubsub:` prefix.
For example, `-remoteWrite.url=pubsub:projects/victoriametrics-vmagent-publish-test/topics/testing-pubsub-push`.

These messages can be read later from Google PubSub by another `vmagent` instance - see [these docs](#reading-metrics) for details.

`vmagent` uses a standard Google authorization mechanism for PubSub topic access. 
Custom auth credentials can be specified via `-gcp.pubsub.subscribe.credentialsFile` command-line flag.

### Producer command-line flags 

```sh
  -gcp.pubsub.publish.byteThreshold int
        Publish a batch when its size in bytes reaches this value. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 1000000)
  -gcp.pubsub.publish.countThreshold int
        Publish a batch when it has this many messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 100)
  -gcp.pubsub.publish.credentialsFile string
        Path to file with GCP credentials to use for PubSub client. If not set, default credentials will be used (see Workload Identity for K8S or https://cloud.google.com/docs/authentication/application-default-credentials). See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -gcp.pubsub.publish.delayThreshold value
        Publish a non-empty batch after this delay has passed. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        The following optional suffixes are supported: s (second), h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 10ms)
  -gcp.pubsub.publish.maxOutstandingBytes int
        The maximum size of buffered messages to be published. If less than or equal to zero, this is disabled. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default -1)
  -gcp.pubsub.publish.maxOutstandingMessages int
        The maximum number of buffered messages to be published. If less than or equal to zero, this is disabled. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 100)
  -gcp.pubsub.publish.timeout value
        The maximum time that the client will attempt to publish a bundle of messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
        The following optional suffixes are supported: s (second), h (hour), d (day), w (week), y (year). If suffix isn't set, then the duration is counted in months (default 60s)
```
