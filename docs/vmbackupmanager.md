---
weight: 10
menu:
  docs:
    parent: victoriametrics
    weight: 10
title: vmbackupmanager
aliases:
  - /vmbackupmanager.html
---
# vmbackupmanager

***vmbackupmanager is a part of [enterprise package](https://docs.victoriametrics.com/enterprise/).
It is available for download and evaluation at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
See how to request a free trial license [here](https://victoriametrics.com/products/enterprise/trial/).***

The VictoriaMetrics backup manager automates regular backup procedures. It supports the following backup intervals: **hourly**, **daily**, **weekly** and **monthly**.
Multiple backup intervals may be configured simultaneously. I.e. the backup manager creates hourly backups every hour, while it creates daily backups every day, etc.
Backup manager must have read access to the storage data, so best practice is to install it on the same machine (or as a sidecar) where the storage node is installed.
The backup service makes a backup every hour and puts it to the latest folder and then copies data to the folders
which represent the backup intervals (hourly, daily, weekly and monthly)

The required flags for running the service are as follows:

* `-license` or `-licenseFile` . See [these docs](https://docs.victoriametrics.com/enterprise/#running-victoriametrics-enterprise).
* `-storageDataPath` - path to VictoriaMetrics or vmstorage data path to make backup from.
* `-snapshot.createURL` - VictoriaMetrics creates snapshot URL which will automatically be created during backup. Example: http://victoriametrics:8428/snapshot/create
* `-dst` - backup destination at [the supported storage types](https://docs.victoriametrics.com/vmbackup/#supported-storage-types).
* `-credsFilePath` - path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.
  See [https://cloud.google.com/iam/docs/creating-managing-service-account-keys](https://cloud.google.com/iam/docs/creating-managing-service-account-keys)
  and [https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html](https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html).

Backup schedule is controlled by the following flags:

* `-disableHourly` - disable hourly run. Default false
* `-disableDaily` - disable daily run. Default false
* `-disableWeekly` - disable weekly run. Default false
* `-disableMonthly` - disable monthly run. Default false

By default, all flags are turned on and Backup Manager backups data every hour for every interval (hourly, daily, weekly and monthly).

The backup manager creates the following directory hierarchy at `-dst`:

* `/latest/` - contains the latest backup
* `/hourly/` - contains hourly backups. Each backup is named as `YYYY-MM-DD:HH`
* `/daily/` - contains daily backups. Each backup is named as `YYYY-MM-DD`
* `/weekly/` - contains weekly backups. Each backup is named as `YYYY-WW`
* `/monthly/` - contains monthly backups. Each backup is named as `YYYY-MM`

To get the full list of supported flags please run the following command:

```sh
./vmbackupmanager --help
```

The service creates a **full** backup each run. This means that the system can be restored fully
from any particular backup using [vmrestore](https://docs.victoriametrics.com/vmrestore/).
Backup manager uploads only the data that has been changed or created since the most recent backup
([incremental backup](https://docs.victoriametrics.com/vmbackup/#incremental-backups)).
This reduces the consumed network traffic and the time needed for performing the backup.
See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for details.

*Please take into account that the first backup upload could take a significant amount of time as it needs to upload all the data.*

There are two flags which could help with performance tuning:

* `-maxBytesPerSecond` - the maximum upload speed. There is no limit if it is set to 0
* `-concurrency` - The number of concurrent workers. Higher concurrency may improve upload speed (default 10)

## Example of Usage

GCS and cluster version. You need to have a credentials file in json format with following structure:

credentials.json

```json
{
  "type": "service_account",
  "project_id": "<project>",
  "private_key_id": "",
  "private_key": "-----BEGIN PRIVATE KEY-----\-----END PRIVATE KEY-----\n",
  "client_email": "test@<project>.iam.gserviceaccount.com",
  "client_id": "",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40<project>.iam.gserviceaccount.com"
}

```

Backup manager launched with the following configuration:

```sh
export NODE_IP=192.168.0.10
export VMSTORAGE_ENDPOINT=http://127.0.0.1:8428
./vmbackupmanager -dst=gs://vmstorage-data/$NODE_IP -credsFilePath=credentials.json -storageDataPath=/vmstorage-data -snapshot.createURL=$VMSTORAGE_ENDPOINT/snapshot/create -eula
```

Expected logs in vmbackupmanager:

```sh
info    lib/backup/actions/backup.go:131    server-side copied 81 out of 81 parts from GCS{bucket: "vmstorage-data", dir: "192.168.0.10//latest/"} to GCS{bucket: "vmstorage-data", dir: "192.168.0.10//weekly/2020-34/"} in 2.549833008s
info    lib/backup/actions/backup.go:169    backed up 853315 bytes in 2.882 seconds; deleted 0 bytes; server-side copied 853315 bytes; uploaded 0 bytes
```

Expected logs in vmstorage:

```sh
info    VictoriaMetrics/lib/storage/table.go:146    creating table snapshot of "/vmstorage-data/data"...
info    VictoriaMetrics/lib/storage/storage.go:311    deleting snapshot "/vmstorage-data/snapshots/20200818201959-162C760149895DDA"...
info    VictoriaMetrics/lib/storage/storage.go:319    deleted snapshot "/vmstorage-data/snapshots/20200818201959-162C760149895DDA" in 0.169 seconds
```

The result on the GCS bucket

* The root folder

  ![root folder](vmbackupmanager_root_folder.webp)

* The latest folder

  ![latest folder](vmbackupmanager_latest_folder.webp)

`vmbackupmanager` uses [smart backups](https://docs.victoriametrics.com/vmbackup/#smart-backups) technique in order
to accelerate backups and save both data transfer costs and data copying costs. This includes server-side copy of already existing
objects. Typical object storage systems implement server-side copy by creating new names for already existing objects.
This is very fast and efficient. Unfortunately there are systems such as [S3 Glacier](https://aws.amazon.com/s3/storage-classes/glacier/),
which perform full object copy during server-side copying. This may be slow and expensive.

Please, see [vmbackup docs](https://docs.victoriametrics.com/vmbackup/#advanced-usage) for more examples of authentication with different
storage types.

## Backup Retention Policy

Backup retention policy is controlled by:

* `-keepLastHourly` - keep the last N hourly backups. Disabled by default
* `-keepLastDaily` - keep the last N daily backups. Disabled by default
* `-keepLastWeekly` - keep the last N weekly backups. Disabled by default
* `-keepLastMonthly` - keep the last N monthly backups. Disabled by default

> *Note*: 0 value in every keepLast flag results into deletion of ALL backups for particular type (hourly, daily, weekly and monthly)

> *Note*: retention policy does not enforce removing previous versions of objects in object storages such if versioning is enabled. See [these docs](https://docs.victoriametrics.com/vmbackup/#permanent-deletion-of-objects-in-s3-compatible-storages) for more details.

Let’s assume we have a backup manager collecting daily backups for the past 10 days.

![retention policy daily before retention cycle](vmbackupmanager_rp_daily_1.webp)

We enable backup retention policy for backup manager by using following configuration:

```sh
export NODE_IP=192.168.0.10
export VMSTORAGE_ENDPOINT=http://127.0.0.1:8428
./vmbackupmanager -dst=gs://vmstorage-data/$NODE_IP -credsFilePath=credentials.json -storageDataPath=/vmstorage-data -snapshot.createURL=$VMSTORAGE_ENDPOINT/snapshot/create
-keepLastDaily=3 -eula
```

Expected logs in backup manager on start:

```sh
info    lib/logger/flag.go:20    flag "keepLastDaily" = "3"
```

Expected logs in backup manager during retention cycle:

```sh
info    app/vmbackupmanager/retention.go:106    daily backups to delete [daily/2021-02-13 daily/2021-02-12 daily/2021-02-11 daily/2021-02-10 daily/2021-02-09 daily/2021-02-08 daily/2021-02-07]
```

The result on the GCS bucket. We see only 3 daily backups:

[retention policy daily after retention cycle](vmbackupmanager_rp_daily_2.webp "retention policy daily after retention cycle")

### Protection backups against deletion by retention policy

You can protect any backup against deletion by retention policy with the `vmbackupmanager backups lock` command.

For instance:

```sh
./vmbackupmanager backup lock daily/2021-02-13 -dst=<DST_PATH> -storageDataPath=/vmstorage-data -eula
```

After that the backup won't be deleted by retention policy.
You can view the `locked` attribute in backup list:

```sh
./vmbackupmanager backup list -dst=<DST_PATH> -storageDataPath=/vmstorage-data -eula
```

To remove protection, you can use the command `vmbackupmanager backups unlock`.

For example:

```sh
./vmbackupmanager backup unlock daily/2021-02-13 -dst=<DST_PATH> -storageDataPath=/vmstorage-data -eula
```

## API methods

`vmbackupmanager` exposes the following API methods:

* POST `/api/v1/backups` - schedule/create the backup. Response examples:

  success, status code - 201, body:
  ```json
      {}
  ```

  failure, status code - 400, body:
  ```json
      {"error": "backups <name> is in progress"}
  ```

* GET `/api/v1/backups` - returns list of backups in remote storage.
  Response example:
  ```json
  [{"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00"},{"name":"hourly/2023-04-07:11","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:06+00:00"},{"name":"latest","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:04+00:00"},{"name":"monthly/2023-04","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:10+00:00"},{"name":"weekly/2023-14","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:09+00:00"}]
  ```
  > Note: `created_at` field is in RFC3339 format.
  
* PUT `/api/v1/backups/<BACKUP_NAME>` - update "locked" attribute for backup by name.
  Example request body:
  ```json
  {"locked":true}
  ```
  Example response:
  ```json
  {"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00", "locked": true}
  ```

* POST `/api/v1/restore` - saves backup name to restore when [performing restore](#restore-commands).
  Example request body:
  ```json
  {"backup":"daily/2022-10-06"}
  ```

* GET `/api/v1/restore` - returns backup name from restore mark if it exists.
  Example response:
  ```json
  {"backup":"daily/2022-10-06"}
  ```

* DELETE `/api/v1/restore` - delete restore mark.

## CLI

`vmbackupmanager` exposes CLI commands to work with [API methods](#api-methods) without external dependencies.

Supported commands:
```sh
vmbackupmanager backup 

  vmbackupmanager backup list 
    List backups in remote storage

  vmbackupmanager backup lock
    Locks backup in remote storage against deletion

  vmbackupmanager backup unlock
    Unlocks backup in remote storage for deletion

vmbackupmanager restore
  Restore backup specified by restore mark if it exists

  vmbackupmanager restore get 
    Get restore mark if it exists

  vmbackupmanager restore delete 
    Delete restore mark if it exists

  vmbackupmanager restore create [backup_name]
    Create restore mark
```

By default, CLI commands are using `http://127.0.0.1:8300` endpoint to reach `vmbackupmanager` API.
It can be changed by using flag:
```
-apiURL string
      vmbackupmanager address to perform API requests (default "http://127.0.0.1:8300")
```

### Backup commands

`vmbackupmanager backup list` lists backups in remote storage:
```sh
$ ./vmbackupmanager backup list
[{"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00"},{"name":"hourly/2023-04-07:11","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:06+00:00"},{"name":"latest","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:04+00:00"},{"name":"monthly/2023-04","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:10+00:00"},{"name":"weekly/2023-14","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:09+00:00"}]
```

### Restore commands

Restore commands are used to create, get and delete restore mark.
Restore mark is used by `vmbackupmanager` to store backup name to restore when running restore.


Create restore mark:
```sh
$ ./vmbackupmanager restore create daily/2022-10-06
```

Get restore mark if it exists:
```sh
$ ./vmbackupmanager restore get
{"backup":"daily/2022-10-06"}
```

Delete restore mark if it exists:
```sh
$ ./vmbackupmanager restore delete
```

Perform restore:
```sh
$ /vmbackupmanager-prod restore -dst=gs://vmstorage-data/$NODE_IP -credsFilePath=credentials.json -storageDataPath=/vmstorage-data
```
Note that `vmsingle` or `vmstorage` should be stopped before performing restore.

If restore mark doesn't exist at `storageDataPath`(restore wasn't requested) `vmbackupmanager restore` will exit with successful status code.

### How to restore backup via CLI

1. Run `vmbackupmanager backup list` to get list of available backups:
  ```sh
  $ /vmbackupmanager-prod backup list
  [{"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00"},{"name":"hourly/2023-04-07:11","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:06+00:00"},{"name":"latest","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:04+00:00"},{"name":"monthly/2023-04","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:10+00:00"},{"name":"weekly/2023-14","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:09+00:00"}]
  ```
1. Run `vmbackupmanager restore create` to create restore mark:
    - Use relative path to backup to restore from currently used remote storage:
      ```sh
      $ /vmbackupmanager-prod restore create daily/2023-04-07
      ```
    - Use full path to backup to restore from any remote storage:
      ```sh
      $ /vmbackupmanager-prod restore create azblob://test1/vmbackupmanager/daily/2023-04-07
      ```
1. Stop `vmstorage` or `vmsingle` node
1. Run `vmbackupmanager restore` to restore backup:
  ```sh
  $ /vmbackupmanager-prod restore -credsFilePath=credentials.json -storageDataPath=/vmstorage-data
  ```
1. Start `vmstorage` or `vmsingle` node


### How to restore in Kubernetes

1. Ensure there is an init container with `vmbackupmanager restore` in `vmstorage` or `vmsingle` pod.
   For [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/) deployments it is required to add:
   ```yaml
   vmbackup:
     restore:
       onStart:
         enabled: "true"
   ```
   See operator `VMStorage` schema [here](https://docs.victoriametrics.com/operator/api.html#vmstorage) and `VMSingle` [here](https://docs.victoriametrics.com/operator/api.html#vmsinglespec).
1. Enter container running `vmbackupmanager`
1. Use `vmbackupmanager backup list` to get list of available backups:
  ```sh
  $ /vmbackupmanager-prod backup list
  [{"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00"},{"name":"hourly/2023-04-07:11","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:06+00:00"},{"name":"latest","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:04+00:00"},{"name":"monthly/2023-04","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:10+00:00"},{"name":"weekly/2023-14","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:09+00:00"}]
  ```
1. Use `vmbackupmanager restore create` to create restore mark:
- Use relative path to backup to restore from currently used remote storage:
  ```sh
  $ /vmbackupmanager-prod restore create daily/2023-04-07
  ```
- Use full path to backup to restore from any remote storage:
  ```sh
  $ /vmbackupmanager-prod restore create azblob://test1/vmbackupmanager/daily/2023-04-07
  ```
1. Restart pod

#### Restore cluster into another cluster

These steps are assuming that [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/) is used to manage `VMCluster`.
Clusters here are referred to as `source` and `destination`.

1. Create a new cluster with access to *source* cluster `vmbackupmanager` storage and same number of storage nodes.
   Add the following section in order to enable restore on start (operator `VMStorage` schema can be found [here](https://docs.victoriametrics.com/operator/api.html#vmstorage):
   ```yaml
   vmbackup:
     restore:
       onStart:
         enabled: "true"
   ```
   Note: it is safe to leave this section in the cluster configuration, since it will be ignored if restore mark doesn't exist.
   > Important! Use different `-dst` for *destination* cluster to avoid overwriting backup data of the *source* cluster.
1. Enter container running `vmbackupmanager` in *source* cluster
1. Use `vmbackupmanager backup list` to get list of available backups:
  ```sh
  $ /vmbackupmanager-prod backup list
  [{"name":"daily/2023-04-07","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:07+00:00"},{"name":"hourly/2023-04-07:11","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:06+00:00"},{"name":"latest","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:04+00:00"},{"name":"monthly/2023-04","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:10+00:00"},{"name":"weekly/2023-14","size_bytes":318837,"size":"311.4ki","created_at":"2023-04-07T16:15:09+00:00"}]
  ```
1. Use `vmbackupmanager restore create` to create restore mark at each pod of the *destination* cluster.
   Each pod in *destination* cluster should be restored from backup of respective pod in *source* cluster.
   For example: `vmstorage-source-0` in *source* cluster should be restored from `vmstorage-destination-0` in *destination* cluster.
  ```sh
  $ /vmbackupmanager-prod restore create s3://source_cluster/vmstorage-source-0/daily/2023-04-07
  ```
1. Restart `vmstorage` pods of *destination* cluster. On pod start `vmbackupmanager` will restore data from the specified backup.

## Monitoring

`vmbackupmanager` exports various metrics in Prometheus exposition format at `http://vmbackupmanager:8300/metrics` page. It is recommended setting up regular scraping of this page
either via [vmagent](https://docs.victoriametrics.com/vmagent/) or via Prometheus, so the exported metrics could be analyzed later.

Use the official [Grafana dashboard](https://grafana.com/grafana/dashboards/17798) for `vmbackupmanager` overview.
Graphs on this dashboard contain useful hints - hover the `i` icon in the top left corner of each graph in order to read it.
If you have suggestions for improvements or have found a bug - please open an issue on github or add
a review to the dashboard.

## Configuration

### Flags

Pass `-help` to `vmbackupmanager` in order to see the full list of supported
command-line flags with their descriptions.

The shortlist of configuration flags is the following:

```text
vmbackupmanager performs regular backups according to the provided configs.

subcommands:
 backup: provides auxiliary backup-related commands
 restore: restores backup specified by restore mark if it exists

command-line flags:
  -apiURL string
     vmbackupmanager address to perform API requests (default "http://127.0.0.1:8300")
  -backupInterval duration
     Interval between backups. If set to 0 interval is set to 1h if hourly backups are enabled and 24h otherwise
  -concurrency int
     The number of concurrent workers. Higher concurrency may reduce backup duration (default 10)
  -configFilePath string
     Path to file with S3 configs. Configs are loaded from default location if not set.
     See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
  -configProfile string
     Profile name for S3 configs. If no set, the value of the environment variable will be loaded (AWS_PROFILE or AWS_DEFAULT_PROFILE), or if both not set, DefaultSharedConfigProfile is used
  -credsFilePath string
     Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.
     See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
  -customS3Endpoint string
     Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set
  -deleteAllObjectVersions
     Whether to prune previous object versions when deleting an object. By default, when object storage has versioning enabled deleting the file removes only current version. This option forces removal of all previous versions. See: https://docs.victoriametrics.com/vmbackup/#permanent-deletion-of-objects-in-s3-compatible-storages
  -disableDaily
     Disable daily run. Default false
  -disableHourly
     Disable hourly run. Default false
  -disableMonthly
     Disable monthly run. Default false
  -disableWeekly
     Disable weekly run. Default false
  -dst string
     The root folder of Victoria Metrics backups. Example: gs://bucket/path/to/backup/dir, s3://bucket/path/to/backup/dir or fs:///path/to/local/backup/dir
  -enableTCP6
     Whether to enable IPv6 for listening and dialing. By default, only IPv4 TCP and UDP are used
  -envflag.enable
     Whether to enable reading flags from environment variables in addition to the command line. Command line flag values have priority over values from environment vars. Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     Deprecated, please use -license or -licenseFile flags instead. By specifying this flag, you confirm that you have an enterprise license and accept the ESA https://victoriametrics.com/legal/esa/ . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -filestream.disableFadvise
     Whether to disable fadvise() syscall when reading large data files. The fadvise() syscall prevents from eviction of recently accessed data from OS page cache during background merges and backups. In some rare cases it is better to disable the syscall if it uses too much CPU
  -flagsAuthKey value
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -flagsAuthKey=file:///abs/path/to/file or -flagsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -flagsAuthKey=http://host/path or -flagsAuthKey=https://host/path
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default, mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming connections to -httpListenAddr are closed after the configured timeout. This may help evenly spreading load among a cluster of services behind TCP-level load balancer. Zero value disables closing of incoming connections (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth
  -http.header.csp string
     Value for 'Content-Security-Policy' header, recommended: "default-src 'self'"
  -http.header.frameOptions string
     Value for 'X-Frame-Options' header
  -http.header.hsts string
     Value for 'Strict-Transport-Security' header, recommended: 'max-age=31536000; includeSubDomains'
  -http.idleConnTimeout duration
     Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
     The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
     An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
     Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password value
     Password for HTTP server's Basic Auth. The authentication is disabled if -httpAuth.username is empty
     Flag value can be read from the given file when using -httpAuth.password=file:///abs/path/to/file or -httpAuth.password=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -httpAuth.password=http://host/path or -httpAuth.password=https://host/path
  -httpAuth.username string
     Username for HTTP server's Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr array
     Address to listen for incoming http requests
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -internStringCacheExpireDuration duration
     The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
     Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
     The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
  -keepLastDaily int
     Keep last N daily backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastHourly int
     Keep last N hourly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastMonthly int
     Keep last N monthly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastWeekly int
     Keep last N weekly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -license string
     License key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed via file specified by -licenseFile command-line flag
  -license.forceOffline
     Whether to enable offline verification for VictoriaMetrics Enterprise license key, which has been passed either via -license or via -licenseFile command-line flag. The issued license key must support offline verification feature. Contact info@victoriametrics.com if you need offline license verification. This flag is available only in Enterprise binaries
  -licenseFile string
     Path to file with license key for VictoriaMetrics Enterprise. See https://victoriametrics.com/products/enterprise/ . Trial Enterprise license can be obtained from https://victoriametrics.com/products/enterprise/trial/ . This flag is available only in Enterprise binaries. The license key can be also passed inline via -license command-line flag
  -loggerDisableTimestamps
     Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
     Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit
  -loggerFormat string
     Format for logs. Possible values: default, json (default "default")
  -loggerJSONFields string
     Allows renaming fields in JSON formatted logs. Example: "ts:timestamp,msg:message" renames "ts" to "timestamp" and "msg" to "message". Supported fields: ts, level, caller, msg
  -loggerLevel string
     Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerMaxArgLen int
     The maximum length of a single logged argument. Longer arguments are replaced with 'arg_start..arg_end', where 'arg_start' and 'arg_end' is prefix and suffix of the arg with the length not exceeding -loggerMaxArgLen / 2 (default 1000)
  -loggerOutput string
     Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
     Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
     Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxBytesPerSecond int
     The maximum upload speed. There is no limit if it is set to 0
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pprofAuthKey value
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides -httpAuth.*
     Flag value can be read from the given file when using -pprofAuthKey=file:///abs/path/to/file or -pprofAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -pprofAuthKey=http://host/path or -pprofAuthKey=https://host/path
  -pushmetrics.disableCompression
     Whether to disable request body compression when pushing metrics to every -pushmetrics.url
  -pushmetrics.extraLabel array
     Optional labels to add to metrics pushed to every -pushmetrics.url . For example, -pushmetrics.extraLabel='instance="foo"' adds instance="foo" label to all the metrics pushed to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.header array
     Optional HTTP request header to send to every -pushmetrics.url . For example, -pushmetrics.header='Authorization: Basic foobar' adds 'Authorization: Basic foobar' header to every request to every -pushmetrics.url
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -pushmetrics.interval duration
     Interval for pushing metrics to every -pushmetrics.url (default 10s)
  -pushmetrics.url array
     Optional URL to push metrics exposed at /metrics page. See https://docs.victoriametrics.com/#push-metrics . By default, metrics exposed at /metrics page aren't pushed to any remote storage
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -runOnStart
     Upload backups immediately after start of the service. Otherwise the backup starts on new hour
  -s3ForcePathStyle
     Prefixing endpoint with bucket name when set false, true by default. (default true)
  -s3StorageClass string
     The Storage Class applied to objects uploaded to AWS S3. Supported values are: GLACIER, DEEP_ARCHIVE, GLACIER_IR, INTELLIGENT_TIERING, ONEZONE_IA, OUTPOSTS, REDUCED_REDUNDANCY, STANDARD, STANDARD_IA.
     See https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html
  -s3TLSInsecureSkipVerify
     Whether to skip TLS verification when connecting to the S3 endpoint.
  -snapshot.createURL string
     VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup.Example: http://victoriametrics:8428/snapshot/create
  -snapshot.deleteURL string
     VictoriaMetrics delete snapshot url. Optional. Will be generated from snapshot.createURL if not provided. All created snapshots will be automatically deleted. Example: http://victoriametrics:8428/snapshot/delete
  -storageDataPath string
     Path to VictoriaMetrics data. Must match -storageDataPath from VictoriaMetrics or vmstorage (default "victoria-metrics-data")
  -tls array
     Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. See also -mtls
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -tlsAutocertCacheDir string
     Directory to store TLS certificates issued via Let's Encrypt. Certificates are lost on restarts if this flag isn't set. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -tlsAutocertEmail string
     Contact email for the issued Let's Encrypt TLS certificates. See also -tlsAutocertHosts and -tlsAutocertCacheDir .This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
  -tlsAutocertHosts array
     Optional hostnames for automatic issuing of Let's Encrypt TLS certificates. These hostnames must be reachable at -httpListenAddr . The -httpListenAddr must listen tcp port 443 . The -tlsAutocertHosts overrides -tlsCertFile and -tlsKeyFile . See also -tlsAutocertEmail and -tlsAutocertCacheDir . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsCertFile array
     Path to file with TLS certificate for the corresponding -httpListenAddr if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsCipherSuites array
     Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsKeyFile array
     Path to file with TLS key for the corresponding -httpListenAddr if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -tlsMinVersion array
     Optional minimum TLS version to use for the corresponding -httpListenAddr if -tls is set. Supported values: TLS10, TLS11, TLS12, TLS13
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -version
     Show VictoriaMetrics version
```
