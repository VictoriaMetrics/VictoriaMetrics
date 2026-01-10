---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file has to be manually updated during feature work in PR, make docs-update-flags command could be used peridically to ensure the flags in sync. -->
```shellhelp
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -gcp.pubsub.publish.byteThreshold int
     Publish a batch when its size in bytes reaches this value. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 1000000)
  -gcp.pubsub.publish.countThreshold int
     Publish a batch when it has this many messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 100)
  -gcp.pubsub.publish.credentialsFile string
     Path to file with GCP credentials to use for PubSub client. If not set, default credentials will be used (see Workload Identity for K8S or https://cloud.google.com/docs/authentication/application-default-credentials). See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -gcp.pubsub.publish.delayThreshold duration
     Publish a non-empty batch after this delay has passed. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 10ms)
  -gcp.pubsub.publish.maxOutstandingBytes int
     The maximum size of buffered messages to be published. If less than or equal to zero, this is disabled. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default -1)
  -gcp.pubsub.publish.maxOutstandingMessages int
     The maximum number of buffered messages to be published. If less than or equal to zero, this is disabled. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 100)
  -gcp.pubsub.publish.timeout duration
     The maximum time that the client will attempt to publish a bundle of messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#writing-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 1m0s)
  -gcp.pubsub.subscribe.credentialsFile string
     Path to file with GCP credentials to use for PubSub client. If not set, default credentials are used (see Workload Identity for K8S or https://cloud.google.com/docs/authentication/application-default-credentials ). See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -gcp.pubsub.subscribe.defaultMessageFormat string
     Default message format if -gcp.pubsub.subscribe.topicSubscription.messageFormat is missing. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default "promremotewrite")
  -gcp.pubsub.subscribe.topicSubscription array
     GCP PubSub topic subscription in the format: projects/<project-id>/subscriptions/<subscription-name>. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -gcp.pubsub.subscribe.topicSubscription.concurrency array
     The number of concurrently processed messages for topic subscription specified via -gcp.pubsub.subscribe.topicSubscription flag. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 0)
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to default value.
  -gcp.pubsub.subscribe.topicSubscription.isGzipped array
     Enables gzip decompression for messages payload at the corresponding -gcp.pubsub.subscribe.topicSubscription. Only prometheus, jsonline, graphite and influx formats accept gzipped messages. See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -gcp.pubsub.subscribe.topicSubscription.messageFormat array
     Message format for the corresponding -gcp.pubsub.subscribe.topicSubscription. Valid formats: influx, prometheus, promremotewrite, graphite, jsonline . See https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic array
     Kafka topic names for data consumption. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.basicAuth.password array
     Optional basic auth password for -kafka.consumer.topic.  Must be used in conjunction with any supported auth methods for kafka client, specified by flag -kafka.consumer.topic.options='security.protocol=SASL_SSL;sasl.mechanisms=PLAIN' . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.basicAuth.username array
     Optional basic auth username for -kafka.consumer.topic. Must be used in conjunction with any supported auth methods for kafka client, specified by flag -kafka.consumer.topic.options='security.protocol=SASL_SSL;sasl.mechanisms=PLAIN' . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.brokers array
     List of brokers to connect for given topic, e.g. -kafka.consumer.topic.broker=host-1:9092;host-2:9092 . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.concurrency array
     Configures consumer concurrency for topic specified via -kafka.consumer.topic flag. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default 1)
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to default value.
  -kafka.consumer.topic.defaultFormat string
     Expected data format in the topic if -kafka.consumer.topic.format is skipped. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/ (default "promremotewrite")
  -kafka.consumer.topic.format array
     data format for corresponding kafka topic. Valid formats: influx, prometheus, promremotewrite, graphite, jsonline and opentelemetry. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.groupID array
     Defines group.id for topic. See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -kafka.consumer.topic.isGzipped array
     Enables gzip setting for topic messages payload. Only prometheus, jsonline, graphite and influx formats accept gzipped messages.See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -kafka.consumer.topic.options array
     Optional key=value;key1=value2 settings for topic consumer. See full configuration options at https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md . See https://docs.victoriametrics.com/victoriametrics/integrations/kafka/#reading-metrics . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -license string
     License key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed via file specified by -licenseFile command-line flag
  -license.forceOffline
     Whether to enable offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification. This flag is available only in Enterprise binaries
  -licenseFile string
     Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed inline via -license command-line flag
  -licenseFile.reloadInterval duration
     Interval for reloading the license file specified via -licenseFile. See https://victoriametrics.com/products/enterprise/ . This flag is available only in Enterprise binaries (default 1h0m0s)
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/victoriametrics/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Each array item can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
```
