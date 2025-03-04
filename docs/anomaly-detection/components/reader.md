---
title: Reader
weight: 2
menu:
  docs:
    parent: "vmanomaly-components"
    weight: 2
aliases:
  - /anomaly-detection/components/reader.html
---

VictoriaMetrics Anomaly Detection (`vmanomaly`) primarily uses [VmReader](#vm-reader) to ingest data. This reader focuses on fetching time-series data directly from VictoriaMetrics with the help of powerful [MetricsQL](https://docs.victoriametrics.com/metricsql/) expressions for aggregating, filtering and grouping your data, ensuring seamless integration and efficient data handling.

Future updates will introduce additional readers, expanding the range of data sources `vmanomaly` can work with.


## VM reader

> **Note**: There is backward-compatible change{{% available_from "v1.13.0" anomaly %}} of [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/reader?highlight=queries#vm-reader) arg of [VmReader](#vm-reader). New format allows to specify per-query parameters, like `step` to reduce amount of data read from VictoriaMetrics TSDB and to allow config flexibility. Please see [per-query parameters](#per-query-parameters) section for the details.

Old format like

```yaml
# other config sections ...
reader:
  class: 'vm'
  datasource_url: 'http://localhost:8428'  # source victoriametrics/prometheus
  sampling_period: "10s"  # set it <= min(infer_every) in schedulers section
  queries:
    # old format {query_alias: query_expr}, prior to 1.13, will be converted to a new format automatically
    vmb: 'avg(vm_blocks)'
```

will be converted to a new one with a warning raised in logs:

```yaml
# other config sections ...
reader:
  class: 'vm'
  datasource_url: 'http://localhost:8428'  # source victoriametrics/prometheus
  sampling_period: '10s'
  queries:
    # old format {query_alias: query_expr}, prior to 1.13, will be converted to a new format automatically
    vmb:
      expr: 'avg(vm_blocks)'  # initial MetricsQL expression
      step: '10s'  # individual step for this query, will be filled with `sampling_period` from the root level
      data_range: ['-inf', 'inf']  # by default, no constraints applied on data range
      tz: 'UTC'  # by default, tz-free data is used throughout the model lifecycle
      # new query-level arguments will be added in backward-compatible way in future releases
```

### Per-query parameters

There is change{{% available_from "v1.13.0" anomaly %}} of [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/reader?highlight=queries#vm-reader) arg format. Now each query alias supports the next (sub)fields, which *override reader-level parameters*, if set:

- `expr` (string): MetricsQL/PromQL expression that defines an input for VmReader. As accepted by `/query_range?query=%s`. i.e. `avg(vm_blocks)`

- `step` (string): query-level frequency of the points returned, i.e. `30s`. Will be converted to `/query_range?step=%s` param (in seconds). Useful to optimize total amount of data read from VictoriaMetrics, where different queries may have **different frequencies for different [machine learning models](https://docs.victoriametrics.com/anomaly-detection/components/models)** to run on.

    > **Note**: if not set explicitly (or if older config style prior to [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130)) is used, then it is set to reader-level `sampling_period` arg.

    > **Note**: having **different** individual `step` args for queries (i.e. `30s` for `q1` and `2m` for `q2`) is not yet supported for [multivariate model](https://docs.victoriametrics.com/anomaly-detection/components/models/#multivariate-models) if you want to run it on several queries simultaneously (i.e. setting [`queries`](https://docs.victoriametrics.com/anomaly-detection/components/models/#queries) arg of a model to [`q1`, `q2`]).

- `data_range`{{% available_from "v1.15.1" anomaly %}} (list[float | string]): It allows defining **valid** data ranges for input per individual query in `queries`, resulting in:
  - **High anomaly scores** (>1) when the *data falls outside the expected range*, indicating a data constraint violation.
  - **Lowest anomaly scores** (=0) when the *model's predictions (`yhat`) fall outside the expected range*, meaning uncertain predictions.

  > **Note**: if not set explicitly (or if older config style prior to [v1.13.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1130)) is used, then it is set to reader-level `data_range` arg{{% available_from "v1.18.1" anomaly %}}

- `max_points_per_query`{{% available_from "v1.17.0" anomaly %}} (int): Optional arg, overrides how `search.maxPointsPerTimeseries` flag{{% available_from "v1.14.1" anomaly %}} impacts `vmanomaly` on splitting long `fit_window` [queries](https://docs.victoriametrics.com/anomaly-detection/components/reader/?highlight=queries#vm-reader) into smaller sub-intervals. This helps users avoid hitting the `search.maxQueryDuration` limit for individual queries by distributing initial query across multiple subquery requests with minimal overhead. Set less than `search.maxPointsPerTimeseries` if hitting `maxQueryDuration` limits. If set on a query-level, it overrides the global `max_points_per_query` (reader-level).

- `tz`{{% available_from "v1.18.0" anomaly %}} (string): this optional argument enables timezone specification per query, overriding the reader’s default `tz`. This setting helps to account for local timezone shifts, such as [DST](https://en.wikipedia.org/wiki/Daylight_saving_time), in models that are sensitive to seasonal variations (e.g., [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile)).

- `tenant_id` {{% available_from "v1.19.0" anomaly %}} (string): this optional argument enables tenant-level separation for queries (e.g. `query1` to get the data from tenant "0:0", `query2` - from tenant "1:0"). It works as follows:
  - if *not set, inherits* reader-level `tenant_id`
  - if *set, overrides* reader-level `tenant_id`
  - *raises config validation error*, if *reader-level is not set* and *query-level is found* (mixing of VictoriaMetrics [single-node](https://docs.victoriametrics.com/single-server-victoriametrics/) and [cluster](https://docs.victoriametrics.com/cluster-victoriametrics/) is prohibited in a single config)
  - *raises config validation warning*, if `writer.tenant_id` is not explicitly set to `multitenant` when reader uses tenants, meaning [VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/) will be used for data querying.
  - also *raises config validation error* if a set of `reader.queries` for [multivariate models](https://docs.victoriametrics.com/anomaly-detection/components/models/#multivariate-models) has *different* tenant_ids (meaning tenant data is mixed, and special labels like `vm_project_id`, `vm_account_id` will have [ambiguous values](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy-via-labels))

  > **Note:** The recommended approach for using per-query `tenant_id`s is to set both `reader.tenant_id` and `writer.tenant_id` to `multitenant`. See [this section](https://docs.victoriametrics.com/anomaly-detection/components/writer/#multitenancy-support) for more details. Configurations where `reader.tenant_id` equals `writer.tenant_id` and is not `multitenant` are also considered safe, provided there is a single, DISTINCT `tenant_id` defined in the reader (either at the reader level or the query level, if set).

### Per-query config example
```yaml
reader:
  class: 'vm'
  sampling_period: '1m'
  max_points_per_query: 10000
  data_range: [0, 'inf']
  tenant_id: 'multitenant'
  # other reader params ...
  queries:
    ingestion_rate_t1:
      expr: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
      step: '2m'  # overrides global `sampling_period` of 1m
      data_range: [10, 'inf']  # meaning only positive values > 10 are expected, i.e. a value `y` < 10 will trigger anomaly score > 1
      max_points_per_query: 5000 # overrides reader-level value of 10000 for `ingestion_rate` query
      tz: 'America/New_York'  # to override reader-wise `tz`
      tenant_id: '1:0'  # overriding tenant_id to isolate data
    ingestion_rate_t2:
      expr: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
      step: '2m'  # overrides global `sampling_period` of 1m
      data_range: [10, 'inf']  # meaning only positive values > 10 are expected, i.e. a value `y` < 10 will trigger anomaly score > 1
      max_points_per_query: 5000 # overrides reader-level value of 10000 for `ingestion_rate` query
      tz: 'America/New_York'  # to override reader-wise `tz`
      tenant_id: '2:0'  # overriding tenant_id to isolate data
```

### Config parameters

<table class="params">
    <thead>
        <tr>
            <th>Parameter</th>
            <th>Example</th>
            <th><span style="white-space: nowrap;">Description</span></th>  
        </tr>
    </thead>
    <tbody>
        <tr>
            <td>

<span style="white-space: nowrap;">`class`</span>
            </td>
            <td>

`reader.vm.VmReader` (or `vm`{{% available_from "v1.13.0" anomaly %}})
            </td>
            <td>
Name of the class needed to enable reading from VictoriaMetrics or Prometheus. VmReader is the default option, if not specified.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`queries`</span>
            </td>
            <td>
See [per-query config example](#per-query-config-example) above
            </td>
            <td>
See [per-query config section](#per-query-parameters) above
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`datasource_url`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`http://localhost:8481/`</span>
            </td>
            <td>
Datasource URL address
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tenant_id`</span>
            </td>
            <td>

`0:0`, `multitenant`
            </td>
            <td>
For VictoriaMetrics Cluster version only, tenants are identified by `accountID` or `accountID:projectID`. Starting from [v1.16.2](https://docs.victoriametrics.com/anomaly-detection/changelog/#v1162), `multitenant` [endpoint](https://docs.victoriametrics.com/cluster-victoriametrics/?highlight=reads#multitenancy-via-labels) is supported, to execute queries over multiple [tenants](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy). See VictoriaMetrics Cluster [multitenancy docs](https://docs.victoriametrics.com/cluster-victoriametrics/#multitenancy)
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`sampling_period`</span>
            </td>
            <td>
`1h`
            </td>
            <td>
Frequency of the points returned. Will be converted to `/query_range?step=%s` param (in seconds). **Required** since [v1.9.0](https://docs.victoriametrics.com/anomaly-detection/changelog/#v190).
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`query_range_path`</span>
            </td>
            <td>

<span style="white-space: nowrap;">`/api/v1/query_range`</span>
            </td>
            <td>
Performs PromQL/MetricsQL range query
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`health_path`</span>
            </td>
            <td>

`health`
            </td>
            <td>
Absolute or relative URL address where to check availability of the datasource.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`user`</span>
            </td>
            <td>

`USERNAME`
            </td>
            <td>
BasicAuth username
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`password`</span>
            </td>
            <td>

`PASSWORD`
            </td>
            <td>
BasicAuth password
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`timeout`</span>
            </td>
            <td>

`30s`
            </td>
            <td>
Timeout for the requests, passed as a string
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`verify_tls`</span>
            </td>
            <td>

`false`
            </td>
            <td>
Verify TLS certificate. If `False`, it will not verify the TLS certificate. 
If `True`, it will verify the certificate using the system's CA store. 
If a path to a CA bundle file (like `ca.crt`), it will verify the certificate using the provided CA bundle.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tls_cert_file`</span>
            </td>
            <td>

`path/to/cert.crt`
            </td>
            <td>
Path to a file with the client certificate, i.e. `client.crt`{{% available_from "v1.16.3" anomaly %}}.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tls_key_file`</span>
            </td>
            <td>

`path/to/key.crt`
            </td>
            <td>
Path to a file with the client certificate key, i.e. `client.key`{{% available_from "v1.16.3" anomaly %}}.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`bearer_token`</span>
            </td>
            <td>

`token`
            </td>
            <td>
Token is passed in the standard format with header: `Authorization: bearer {token}`
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`bearer_token_file`</span>
            </td>
            <td>

`path_to_file`
            </td>
            <td>
Path to a file, which contains token, that is passed in the standard format with header: `Authorization: bearer {token}`{{% available_from "v1.15.9" anomaly %}}.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`extra_filters`</span>
            </td>
            <td>

`[]`
            </td>
            <td>
List of strings with series selector. See: [Prometheus querying API enhancements](https://docs.victoriametrics.com/##prometheus-querying-api-enhancements)
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`query_from_last_seen_timestamp`</span>
            </td>
            <td>

`False`
            </td>
            <td>
If True, then query will be performed from the last seen timestamp for a given series. If False, then query will be performed from the start timestamp, based on a schedule period. Defaults to `False`. Useful for `infer` stages in case there were skipped `infer` calls prior to given.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`latency_offset`</span>
            </td>
            <td>

`1ms`
            </td>
            <td>
It allows overriding the default `-search.latencyOffset`{{% available_from "v1.15.1" anomaly %}} [flag of VictoriaMetrics](https://docs.victoriametrics.com/?highlight=search.latencyOffset#list-of-command-line-flags) (30s). The default value is set to 1ms, which should help in cases where `sampling_frequency` is low (10-60s) and `sampling_frequency` equals `infer_every` in the [PeriodicScheduler](https://docs.victoriametrics.com/anomaly-detection/components/scheduler/?highlight=infer_every#periodic-scheduler). This prevents users from receiving `service - WARNING - [Scheduler [scheduler_alias]] No data available for inference.` warnings in logs and allows for consecutive `infer` calls without gaps. To restore the old behavior, set it equal to your `-search.latencyOffset` [flag value](https://docs.victoriametrics.com/?highlight=search.latencyOffset#list-of-command-line-flags).
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`max_points_per_query`</span>
            </td>
            <td>

`10000`
            </td>
            <td>
Optional arg{{% available_from "v1.17.0" anomaly %}} overrides how `search.maxPointsPerTimeseries` flag{{% available_from "v1.14.1" anomaly %}} impacts `vmanomaly` on splitting long `fit_window` [queries](https://docs.victoriametrics.com/anomaly-detection/components/reader/?highlight=queries#vm-reader) into smaller sub-intervals. This helps users avoid hitting the `search.maxQueryDuration` limit for individual queries by distributing initial query across multiple subquery requests with minimal overhead. Set less than `search.maxPointsPerTimeseries` if hitting `maxQueryDuration` limits. You can also set it on [per-query](#per-query-parameters) basis to override this global one.
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`tz`</span>
            </td>
            <td>

`UTC`
            </td>
            <td>
Optional argument{{% available_from "v1.18.0" anomaly %}} specifies the [IANA](https://nodatime.org/TimeZones) timezone to account for local shifts, like [DST](https://en.wikipedia.org/wiki/Daylight_saving_time), in models sensitive to seasonal patterns (e.g., [`ProphetModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#prophet) or [`OnlineQuantileModel`](https://docs.victoriametrics.com/anomaly-detection/components/models/#online-seasonal-quantile)). Defaults to `UTC` if not set and can be overridden on a [per-query basis](#per-query-parameters).
            </td>
        </tr>
        <tr>
            <td>

<span style="white-space: nowrap;">`data_range`</span>
            </td>
            <td>

`["-inf", "inf"]`
            </td>
            <td>
Optional argument{{% available_from "v1.18.1" anomaly %}} allows defining **valid** data ranges for input of all the queries in `queries`. Defaults to `["-inf", "inf"]` if not set and can be overridden on a [per-query basis](#per-query-parameters).
            </td>
        </tr>
    </tbody>
</table>

Config file example:

```yaml
reader:
  class: "vm"  # or "reader.vm.VmReader" until v1.13.0
  datasource_url: "https://play.victoriametrics.com/"
  tenant_id: '0:0'
  tz: 'America/New_York'
  data_range: [1, 'inf']  # reader-level
  queries:
    ingestion_rate:
      expr: 'sum(rate(vm_rows_inserted_total[5m])) by (type) > 0'
      step: '1m' # can override reader-level `sampling_period` on per-query level
      data_range: [0, 'inf']  # if set, overrides reader-level data_range
      tz: 'Australia/Sydney'  # if set, overrides reader-level tz
      # tenant_id: '1:0'  # if set, overrides reader-level tenant_id
  sampling_period: '1m'
  query_from_last_seen_timestamp: True  # false by default
  latency_offset: '1ms'
```

### mTLS protection

`vmanomaly` supports [mutual TLS (mTLS)](https://en.wikipedia.org/wiki/Mutual_authentication){{% available_from "v1.16.3" anomaly %}} for secure communication across its components, including [VmReader](https://docs.victoriametrics.com/anomaly-detection/components/reader/#vm-reader), [VmWriter](https://docs.victoriametrics.com/anomaly-detection/components/writer/#vm-writer), and [Monitoring/Push](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#push-config-parameters). This allows for mutual authentication between the client and server when querying or writing data to [VictoriaMetrics Enterprise, configured for mTLS](https://docs.victoriametrics.com/#mtls-protection).

mTLS ensures that both the client and server verify each other's identity using certificates, which enhances security by preventing unauthorized access. 

To configure mTLS, the following parameters can be set in the [config](#config-parameters):
- `verify_tls`: If set to a string, it functions like the `-mtlsCAFile` command-line argument of VictoriaMetrics, specifying the CA bundle to use. Set to `True` to use the system's default certificate store.
- `tls_cert_file`: Specifies the path to the client certificate, analogous to the `-tlsCertFile` argument of VictoriaMetrics.
- `tls_key_file`: Specifies the path to the client certificate key, similar to the `-tlsKeyFile` argument of VictoriaMetrics.

These options allow you to securely interact with mTLS-enabled VictoriaMetrics endpoints.

Example configuration to enable mTLS with custom certificates:

```yaml
reader:
  class: "vm"
  datasource_url: "https://your-victoriametrics-instance-with-mtls"
  # tenant_id: "0:0" uncomment and set for cluster version
  queries:
    vm_blocks_example:
      expr: 'avg(rate(vm_blocks[5m]))'
      step: 30s
  sampling_period: 30s
  verify_tls: "path/to/ca.crt"  # path to CA bundle for TLS verification
  tls_cert_file: "path/to/client.crt"  # path to the client certificate
  tls_key_file:  "path/to/client.key"  # path to the client certificate key
  # additional reader parameters ...

# other config sections, like models, schedulers, writer, ...
```


### Healthcheck metrics

`VmReader` exposes [several healthchecks metrics](https://docs.victoriametrics.com/anomaly-detection/components/monitoring/#reader-behaviour-metrics).
