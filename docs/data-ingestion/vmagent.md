---
title: Vmagent
weight: 1
sort: 1
menu:
  docs:
    identifier: "vmagent"
    parent: "data-ingestion"
    weight: 1
aliases:
  - /data-ingestion/vmagent.html
  - /data-ingestion/VMAgent.html
---
# Vmagent Setup
`vmagent` is a small agent that can receive data in any format VictoriaMetrics Single and `vminsert` can ingest as well scrape Prometheus endpoints with the same job definitions.
This section of the documentation only covers forwarding data from `vmagent` to another destination.
For Information about `vmagent` as well as quickstart guide please refer to the [vmagent documentation](https://docs.victoriametrics.com/vmagent/)

In any of the examples below you can add `insert/<tenant_id>/` to the URL path if you are sending metrics to vminsert.
For example the remote write URL would change from

```
https://<victoriametrics_url>/prometheus/api/v1/write
```

to

```
https://<victoriametrics_url>/insert/<tenant_id>/prometheus/api/v1/write
```


## Sending data to VictoriaMetrics without authentication
This requires setting the `-remote.url` flag in the command line arguments for vmagent

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>:<victoriametrics_port>/api/v1/write
```

## Sending data to VictoriaMetrics with basic authentication

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>:<victoriametrics_port>/api/v1/write -remoteWrite.basicAuth.username=<username> -remoteWrite.basicAuth.password=<password>
```

The basic authentication password can be placed in a file accessed via `-remoteWrite.passwordFile`.
The file needs to be readable by the user `vmagent` is running as
The password file should look like


```
<password>
```

and the vmagent command will look like 


## Sending data to VictoriaMetrics with bearer Authentication

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>:<victoriametrics_port>/api/v1/write -remoteWrite.bearerToken=<token>
```

The token can be placed in a file accessed via `-remoteWrite.bearerTokenFile`.
The file needs to be readable by the user `vmagent` is running as
The token file should look like


```
<token>

```

and the vmagent command will look like 

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>:<victoriametrics_port>/api/v1/write -remoteWrite.bearerTokenFile=/path/to/tokenfile

```


## Ignore TLS/SSL errors between vmagent and the destination

If you are using Self signed certificates you can either certificates issues using the `-remoteWrite.tlsInsecureSkipVerify`, which is a security risk, or use `-remoteWrite.tlsCAFile` to point to a file containing the self signed CA certificate. 

### Ignore TLS/SSL errors

```sh
/path/to/vmagent -remoteWrite.url=https://<victoriametrics_url>:<victoriametrics_port>/api/v1/write -remoteWrite.bearerToken=<token> -remoteWrite.tlsInsecureSkipVerify
```


## References
- [vmagent docs]()
- [vmagent commandline flags]()
