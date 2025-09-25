---
title: OpenTSDB
weight: 6
menu:
  docs:
    parent: "integrations-vm"
    weight: 6
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive inserts via 
[telnet put protocol](http://opentsdb.net/docs/build/html/api_telnet/put.html) and [HTTP /api/put requests](http://opentsdb.net/docs/build/html/api_http/put.html)
for ingesting OpenTSDB data. The same protocol is used for [ingesting data in KairosDB](https://kairosdb.github.io/docs/PushingData.html).

See full list of OpenTSDB-related configuration flags by running:
```sh
/path/to/victoria-metrics-prod --help | grep opentsdb
```

## Sending data via telnet

Enable OpenTSDB receiver in VictoriaMetrics by setting `-opentsdbListenAddr` command line flag:
```sh
/path/to/victoria-metrics-prod -opentsdbListenAddr=:4242
```

Send data to the given address from OpenTSDB-compatible agents.

Example for writing data with OpenTSDB protocol to local VictoriaMetrics using `nc`:
```sh
echo "put foo.bar.baz `date +%s` 123 tag1=value1 tag2=value2" | nc -N localhost 4242
```

An arbitrary number of lines delimited by `\n` (aka newline char) can be sent in one go.
After that the data may be read via [/api/v1/export](https://docs.victoriametrics.com/victoriametrics/#how-to-export-data-in-json-line-format) endpoint:
```sh
curl -G 'http://localhost:8428/api/v1/export' -d 'match=foo.bar.baz'
```

The `/api/v1/export` endpoint should return the following response:
```json
{"metric":{"__name__":"foo.bar.baz","tag1":"value1","tag2":"value2"},"values":[123],"timestamps":[1560277292000]}
```

## Sending data via HTTP

Enable HTTP server for OpenTSDB `/api/put` requests by setting `-opentsdbHTTPListenAddr` command line flag:
```sh
/path/to/victoria-metrics-prod -opentsdbHTTPListenAddr=:4242
```

Send data to the given address from OpenTSDB-compatible agents.

Example for writing a single data point:
```sh
curl -H 'Content-Type: application/json' -d '{"metric":"x.y.z","value":45.34,"tags":{"t1":"v1","t2":"v2"}}' http://localhost:4242/api/put
```

Example for writing multiple data points in a single request:
```sh
curl -H 'Content-Type: application/json' -d '[{"metric":"foo","value":45.34},{"metric":"bar","value":43}]' http://localhost:4242/api/put
```

After that the data may be read via [/api/v1/export](https://docs.victoriametrics.com/victoriametrics/#how-to-export-data-in-json-line-format) endpoint:
```sh
curl -G 'http://localhost:8428/api/v1/export' -d 'match[]=x.y.z' -d 'match[]=foo' -d 'match[]=bar'
```

The `/api/v1/export` endpoint should return the following response:

```json
{"metric":{"__name__":"foo"},"values":[45.34],"timestamps":[1566464846000]}
{"metric":{"__name__":"bar"},"values":[43],"timestamps":[1566464846000]}
{"metric":{"__name__":"x.y.z","t1":"v1","t2":"v2"},"values":[45.34],"timestamps":[1566464763000]}
```

Extra labels may be added to all the imported time series by passing `extra_label=name=value` query args.
For example, `/api/put?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.