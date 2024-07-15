---
title: Telegraf
weight: 1
sort: 1
menu:
  docs:
    identifier: "telegraf"
    parent: "data-ingestion"
    weight: 1
    # sort: 1
aliases:
  - /data-ingestion/telegraf.html
  - /data-ingestion/Telegraf.html
---
# Telegraf Setup
You will need to add the following output section to a Telegraf configuration file and reload Telegraf to enable shipping data from Telegraf to VictoriaMetrics.
All the options examples below can be combined to fit your use case

To avoid storing Passwords in configuration files you can store as a key value pair in `/etc/default/telegraf` on Linux as follows
```
victoriametrics_url="https://metrics.example.com"
victoriametrics_user="telegraf"
victoriametrics_password="password"
victoriametrics_token="my_token"
```
and they can be referenced in a Telegraf configuration file by prepending the variable name with `$` ex. `$victoriametrics_url` will be translated to `https://metrics.example.com` if it referenced in a Telegraf configuration using the values from `/etc/default/telegraf` in the values seen above.
Otherwise please replace the variables below to fit your setup.

If you want to mimic this behavior on windows please read [Influx Data's blog on storing variables in the registry](https://www.influxdata.com/blog/using-telegraf-on-windows/)

## Minimum Configuration with no Authentication
```toml
[[outputs.influxdb]]
  urls = ["$victoriametrics_url"]
  database = "victoriametrics"
  skip_database_creation = true
  exclude_retention_policy_tag = true
  content_encoding = "gzip"
```


## HTTP Basic Authentication (Username and Password)
This is the same as the minimum configuration, but adds the `username` and `password` options

```toml
[[outputs.influxdb]]
  urls = ["$victoriametrics_url"]
  username = "$victoriametrics_user"
  password = "$victoriametrics_password"
  database = "victoriametrics"
  skip_database_creation = true
  exclude_retention_policy_tag = true
  content_encoding = "gzip"
```

## Bearer Authentication (Token)

This is the same as the minimum configuration but adds the authorization header

```
[[outputs.influxdb]]
  urls = ["$victoriametrics_url"]
  http_headers = {"Authorization" = "Bearer $victoriametrics_token"}
  database = "victoriametrics"
  skip_database_creation = true
  exclude_retention_policy_tag = true
  content_encoding = "gzip"
```

## Route certain metrics
If you only want to route certain metrics to VictoriaMetrics use the `namepass` option with a comma separated listed of the measurements you wish to send to VictoriaMetrics.

```
[[outputs.influxdb]]
  urls = ["$victoriametrics_url"]
  username = "$victoriametrics_user"
  password = "$victoriametrics_password"
  database = "victoriametrics"
  skip_database_creation = true
  exclude_retention_policy_tag = true
  content_encoding = "gzip"
  namepass = ["cpu","disk","measurement1","measurement2"]
```

## Ignore TLS/SSL Certificate errors
This is the same as the minimum configuration but adds `insecure_skip_verify = true` to the configuration to ignore TLS certificate errors.
This is not recommended since it can allow sending metrics to a compromised site.

```
[[outputs.influxdb]]
  urls = ["$victoriametrics_url"]
  username = "$victoriametrics_user"
  password = "$victoriametrics_password"
  database = "victoriametrics"
  skip_database_creation = true
  exclude_retention_policy_tag = true
  content_encoding = "gzip"
  insecure_skip_verify = true
```

# References 
- [Install Telegraf](https://docs.influxdata.com/telegraf/v1/install/)
- [InfluxDBv1 output for Telegraf](https://github.com/influxdata/telegraf/tree/master/plugins/outputs/influxdb)
- [Storing Telegraf variables in the windows registry](https://www.influxdata.com/blog/using-telegraf-on-windows/)
- [Telegraf variables](https://docs.influxdata.com/telegraf/v1/configuration/#example-telegraf-environment-variables)
