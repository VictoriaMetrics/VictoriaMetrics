---
title: Zabbix Connector
weight: 10
menu:
  docs:
    parent: "integrations-vm"
    weight: 10
---

VictoriaMetrics components like **vmagent**, **vminsert** or **single-node** can receive data from 
[Zabbix Connector streaming protocol](https://www.zabbix.com/documentation/current/en/manual/config/export/streaming#protocol)
at `/zabbixconnector/api/v1/history` HTTP path.

## Send data from Zabbix Connector

You must specify `http://<victoriametrics-addr>:8428/zabbixconnector/api/v1/history` in the "URL" parameter when creating a new connector in the [Zabbix WEB interface](https://www.zabbix.com/documentation/current/en/manual/config/export/streaming). Starting from version 7.0, the "Type of information" parameter must be set to "numeric (unsigned)" and "numeric (float)".

Extra labels may be added to all the written time series by passing `extra_label=name=value` query args.
For example, `/zabbixconnector/api/v1/history?extra_label=foo=bar` would add `{foo="bar"}` label to all the ingested metrics.

## Zabbix Connector data mapping

VictoriaMetrics maps [Zabbix Connector streaming protocol](https://www.zabbix.com/documentation/current/en/manual/config/export/streaming#protocol)
to [raw samples](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples) in the following way:

* Discards all item values ​​that do not match the types "numeric (unsigned)" and "numeric (float)".
* JSON path `$.host.host` converts to label value `host`.
* JSON path `$.host.name` converts to label value `hostname`.
* JSON path `$.name` converts to label value `__name__`.
* JSON path `$.value` is used as the metric value.
* JSON path `$.clock` and `$.ns` is used as timestamp for the ingested [raw sample](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#raw-samples).\
timestamp is calculated using the formula: `$.clock`*1e3 + `$.ns`/1e6 (See [Zabbix item structure](https://www.zabbix.com/documentation/current/en/manual/appendix/protocols/real_time_export#item-values) for details)
* If the command line flag `-zabbixconnector.addGroupsValue=<value>` is specified, the elements of the `$.grops` group array will be converted to the label name prefixed with `group_` with the value `<value>`.
* The tag object array `$.item_tags` will be configured as follows:
  * The `tag` element will be used as the tag name, prefixed with `tag_`. The "value" element will be used as the value of the label.
  * If the command line flag `-zabbixconnector.addEmptyTagsValue=<value>` is specified, then tags with empty values ​​will not be ignored. `<value>` will be used as the tag value. By default, tags with empty values ​​are ignored.
  * If the command line flag `-zabbixconnector.addDuplicateTagsSeparator=<value>` is specified, then the values ​​of duplicate tags will be merged into one using the `<value>` delimiter.

For example, let's import the following Zabbix Connector request to VictoriaMetrics:

```json
{"host":{"host":"ZabbixServer","name":"ZabbixServer"},"groups":["servers"],"item_tags":[{"tag":"foo","value":""}],"itemid":44457,"name":"item_1","clock":1673454303,"ns":800155804,"value":0,"type":3}
{"host":{"host":"ZabbixServer","name":"ZabbixServer"},"groups":["servers"],"item_tags":[{"tag":"foo","value":"test"}, {"tag":"foo","value":""}],"itemid":44458,"name":"item_2","clock":1673454303,"ns":832290669,"value":1,"type":3}
{"host":{"host":"ZabbixServer","name":"ZabbixServer"},"groups":["servers"],"item_tags":[{"tag":"bar","value":"test"}],"itemid":44458,"name":"item_3","clock":1673454303,"ns":867770366,"value":123,"type":3}
```

Save this NDJSON into `data.ndjson` file and then use the following command in order to import it into VictoriaMetrics:

```sh
curl -X POST -H 'Content-Type: application/x-ndjson' --data-binary @data.ndjson http://localhost:8428/zabbixconnector/api/v1/history
```
Let's assume vmagent is running with command line flags:
* `-zabbixconnector.addGroupsValue=exists`
* `-zabbixconnector.addEmptyTagsValue=exists`
* `-zabbixconnector.addDuplicateTagsSeparator=,`

Let's fetch the ingested data via [data export API](#how-to-export-data-in-json-line-format):

```sh
curl http://localhost:8428/api/v1/export -d 'match={host="Zabbix server"}'
{"metric":{"__name__":"item_1","host":"ZabbixServer","hostname":"ZabbixServer","group_servers":"exists","tag_foo":"exists"},"values":[0],"timestamps":[1673454303800]}
{"metric":{"__name__":"item_2","host":"ZabbixServer","hostname":"ZabbixServer","group_servers":"exists","tag_foo":"test,exists"},"values":[1],"timestamps":[1673454303832]}
{"metric":{"__name__":"item_3","host":"ZabbixServer","hostname":"ZabbixServer","group_servers":"exists","tag_bar":"test"},"values":[123],"timestamps":[1673454303867]}
```