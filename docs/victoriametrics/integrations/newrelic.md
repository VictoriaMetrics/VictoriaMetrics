---
title: NewRelic
weight: 7
menu:
  docs:
    parent: "integrations-vm"
    weight: 7
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive data from 
[NewRelic infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent)
at `/newrelic/infra/v2/metrics/events/bulk` HTTP path.

VictoriaMetrics receives [Events](https://docs.newrelic.com/docs/infrastructure/manage-your-data/data-instrumentation/default-infrastructure-monitoring-data/#infrastructure-events)
from NewRelic agent at the given path, transforms them to [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples)
with extra [data mapping](#data-mapping) before storing the samples to the database.

## Sending data from agent

NewRelic infrastructure agent is configured via `COLLECTOR_URL` and `NRIA_LICENSE_KEY` environment variables for sending 
the collected metrics. The `COLLECTOR_URL` must point to `/newrelic` HTTP endpoint at VictoriaMetrics, 
while the `NRIA_LICENSE_KEY` must contain NewRelic license key (can be obtained at [newrelic.com](https://newrelic.com/signup)).

For example, the following command can be used for running NewRelic infrastructure agent:
```sh
COLLECTOR_URL="http://`<victoriametrics-addr>/newrelic" NRIA_LICENSE_KEY="NEWRELIC_LICENSE_KEY" ./newrelic-infra
```
_Replace `<victoriametrics-addr>` with the VictoriaMetrics hostname or IP address._

For cluster version use vminsert address:
```
http://<vminsert-addr>:8480/insert/<tenant>/newrelic
```
_Replace `<vminsert-addr>` with the hostname or IP address of vminsert service._

If you have more than 1 vminsert, configure [load-balancing](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).
Replace `<tenant>` based on your [multitenancy settings](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy).

## Data mapping

VictoriaMetrics maps [NewRelic Events](https://docs.newrelic.com/docs/infrastructure/manage-your-data/data-instrumentation/default-infrastructure-monitoring-data/#infrastructure-events)
to [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) in the following way:

1. Every numeric field is converted into a raw sample with the corresponding name.
1. The `eventType` and all the other fields with `string` value type are attached to every raw sample as [metric labels](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#labels).
1. The `timestamp` field is used as timestamp for the ingested [raw sample](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples).
   The `timestamp` field may be specified either in seconds or in milliseconds since the [Unix Epoch](https://en.wikipedia.org/wiki/Unix_time).
   If the `timestamp` field is missing, then the raw sample is stored with the current timestamp.

For example, let's import the following NewRelic Events request to VictoriaMetrics:

```json
[
  {
    "Events":[
      {
        "eventType":"SystemSample",
        "entityKey":"macbook-pro.local",
        "cpuPercent":25.056660790748904,
        "cpuUserPercent":8.687987912389374,
        "cpuSystemPercent":16.36867287835953,
        "cpuIOWaitPercent":0,
        "cpuIdlePercent":74.94333920925109,
        "cpuStealPercent":0,
        "loadAverageOneMinute":5.42333984375,
        "loadAverageFiveMinute":4.099609375,
        "loadAverageFifteenMinute":3.58203125
      }
    ]
  }
]
```

Save this JSON into `newrelic.json` file and then use the following command in order to import it into VictoriaMetrics:
```sh
curl -X POST -H 'Content-Type: application/json' --data-binary @newrelic.json http://localhost:8428/newrelic/infra/v2/metrics/events/bulk
```

Let's fetch the ingested data via [data export API](https://docs.victoriametrics.com/victoriametrics/#how-to-export-data-in-json-line-format):
```sh
curl http://localhost:8428/api/v1/export -d 'match={eventType="SystemSample"}'
{"metric":{"__name__":"cpuStealPercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[0],"timestamps":[1697407970000]}
{"metric":{"__name__":"loadAverageFiveMinute","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[4.099609375],"timestamps":[1697407970000]}
{"metric":{"__name__":"cpuIOWaitPercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[0],"timestamps":[1697407970000]}
{"metric":{"__name__":"cpuSystemPercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[16.368672878359],"timestamps":[1697407970000]}
{"metric":{"__name__":"loadAverageOneMinute","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[5.42333984375],"timestamps":[1697407970000]}
{"metric":{"__name__":"cpuUserPercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[8.687987912389],"timestamps":[1697407970000]}
{"metric":{"__name__":"cpuIdlePercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[74.9433392092],"timestamps":[1697407970000]}
{"metric":{"__name__":"loadAverageFifteenMinute","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[3.58203125],"timestamps":[1697407970000]}
{"metric":{"__name__":"cpuPercent","entityKey":"macbook-pro.local","eventType":"SystemSample"},"values":[25.056660790748],"timestamps":[1697407970000]}
```