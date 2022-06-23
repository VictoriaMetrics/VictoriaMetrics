---
sort: 10
---

## vmbackupmanager

***vmbackupmanager is a part of [enterprise package](https://victoriametrics.com/products/enterprise/). It is available for download and evaluation at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)***

The VictoriaMetrics backup manager automates regular backup procedures. It supports the following backup intervals: **hourly**, **daily**, **weekly** and **monthly**. Multiple backup intervals may be configured simultaneously. I.e. the backup manager creates hourly backups every hour, while it creates daily backups every day, etc. Backup manager must have read access to the storage data, so best practice is to install it on the same machine (or as a sidecar) where the storage node is installed.
The backup service makes a backup every hour and puts it to the latest folder and then copies data to the folders which represent the backup intervals (hourly, daily, weekly and monthly)

The required flags for running the service are as follows:

* -eula - should be true and means that you have the legal right to run a backup manager. That can either be a signed contract or an email with confirmation to run the service in a trial period
* -storageDataPath - path to VictoriaMetrics or vmstorage data path to make backup from
* -snapshot.createURL - VictoriaMetrics creates snapshot URL which will automatically be created during backup. Example: <http://victoriametrics:8428/snapshot/create>
* -dst - backup destination at s3, gcs or local filesystem
* -credsFilePath - path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set. See [https://cloud.google.com/iam/docs/creating-managing-service-account-keys](https://cloud.google.com/iam/docs/creating-managing-service-account-keys) and [https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html](https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html)

Backup schedule is controlled by the following flags:

* -disableHourly - disable hourly run. Default false
* -disableDaily - disable daily run. Default false
* -disableWeekly - disable weekly run. Default false
* -disableMonthly - disable monthly run. Default false

By default, all flags are turned on and Backup Manager backups data every hour for every interval (hourly, daily, weekly and monthly).

The backup manager creates the following directory hierarchy at **-dst**:

* /latest/ - contains the latest backup
* /hourly/ - contains hourly backups. Each backup is named as *YYYY-MM-DD:HH*
* /daily/ - contains daily backups. Each backup is named as *YYYY-MM-DD*
* /weekly/ - contains weekly backups. Each backup is named as *YYYY-WW*
* /monthly/ - contains monthly backups. Each backup is named as *YYYY-MM*

To get the full list of supported flags please run the following command:

```console
./vmbackupmanager --help
```

The service creates a **full** backup each run. This means that the system can be restored fully from any particular backup using vmrestore. Backup manager uploads only the data that has been changed or created since the most recent backup (incremental backup).

*Please take into account that the first backup upload could take a significant amount of time as it needs to upload all of the data.*

There are two flags which could help with performance tuning:

* -maxBytesPerSecond - the maximum upload speed. There is no limit if it is set to 0
* -concurrency - The number of concurrent workers. Higher concurrency may improve upload speed (default 10)

## Example of Usage

GCS and cluster version. You need to have a credentials file in json format with following structure

credentials.json

```json
{
  "type": "service_account",
  "project_id": "<project>",
  "private_key_id": "",
  "private_key": "-----BEGIN PRIVATE KEY-----\-----END PRIVATE KEY-----\n",
  "client_email": “test@<project>.iam.gserviceaccount.com",
  "client_id": "",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40<project>.iam.gserviceaccount.com"
}

```

Backup manager launched with the following configuration:

```console
export NODE_IP=192.168.0.10
export VMSTORAGE_ENDPOINT=http://127.0.0.1:8428
./vmbackupmanager -dst=gs://vmstorage-data/$NODE_IP -credsFilePath=credentials.json -storageDataPath=/vmstorage-data -snapshot.createURL=$VMSTORAGE_ENDPOINT/snapshot/create -eula 
```

Expected logs in vmbackupmanager:

```console
info    lib/backup/actions/backup.go:131    server-side copied 81 out of 81 parts from GCS{bucket: "vmstorage-data", dir: "192.168.0.10//latest/"} to GCS{bucket: "vmstorage-data", dir: "192.168.0.10//weekly/2020-34/"} in 2.549833008s
info    lib/backup/actions/backup.go:169    backed up 853315 bytes in 2.882 seconds; deleted 0 bytes; server-side copied 853315 bytes; uploaded 0 bytes 
```

Expected logs in vmstorage:

```console
info    VictoriaMetrics/lib/storage/table.go:146    creating table snapshot of "/vmstorage-data/data"...
info    VictoriaMetrics/lib/storage/storage.go:311    deleting snapshot "/vmstorage-data/snapshots/20200818201959-162C760149895DDA"...
info    VictoriaMetrics/lib/storage/storage.go:319    deleted snapshot "/vmstorage-data/snapshots/20200818201959-162C760149895DDA" in 0.169 seconds
```

The result on the GCS bucket

* The root folder
  
  ![root](vmbackupmanager_root_folder.png)

* The latest folder

  ![latest](vmbackupmanager_latest_folder.png)

## Backup Retention Policy

Backup retention policy is controlled by:

* -keepLastHourly - keep the last N hourly backups. Disabled by default
* -keepLastDaily - keep the last N daily backups. Disabled by default
* -keepLastWeekly - keep the last N weekly backups. Disabled by default
* -keepLastMonthly - keep the last N monthly backups. Disabled by default

*Note*: 0 value in every keepLast flag results into deletion ALL backups for particular type (hourly, daily, weekly and monthly)

Let’s assume we have a backup manager collecting daily backups for the past 10 days.

![daily](vmbackupmanager_rp_daily_1.png)

We enable backup retention policy for backup manager by using following configuration:

```console
export NODE_IP=192.168.0.10
export VMSTORAGE_ENDPOINT=http://127.0.0.1:8428
./vmbackupmanager -dst=gs://vmstorage-data/$NODE_IP -credsFilePath=credentials.json -storageDataPath=/vmstorage-data -snapshot.createURL=$VMSTORAGE_ENDPOINT/snapshot/create
-keepLastDaily=3 -eula
```

Expected logs in backup manager on start:

```console
info    lib/logger/flag.go:20    flag "keepLastDaily" = "3"
```

Expected logs in backup manager during retention cycle:

```console
info    app/vmbackupmanager/retention.go:106    daily backups to delete [daily/2021-02-13 daily/2021-02-12 daily/2021-02-11 daily/2021-02-10 daily/2021-02-09 daily/2021-02-08 daily/2021-02-07]
```

The result on the GCS bucket. We see only 3 daily backups:

![daily](vmbackupmanager_rp_daily_2.png)


## Configuration

### Flags

Pass `-help` to `vmbackupmanager` in order to see the full list of supported
command-line flags with their descriptions.

The shortlist of configuration flags is the following:

```
vmbackupmanager performs regular backups according to the provided configs.

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
     Whether to enable IPv6 for listening and dialing. By default only IPv4 TCP and UDP is used
  -envflag.enable
     Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set. See https://docs.victoriametrics.com/#environment-variables for more details
  -envflag.prefix string
     Prefix for environment variables if -envflag.enable is set
  -eula
     By specifying this flag, you confirm that you have an enterprise license and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
  -flagsAuthKey string
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -fs.disableMmap
     Whether to use pread() instead of mmap() for reading data files. By default mmap() is used for 64-bit arches and pread() is used for 32-bit arches, since they cannot read data files bigger than 2^32 bytes in memory. mmap() is usually faster for reading small data chunks than pread()
  -http.connTimeout duration
     Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem (default 2m0s)
  -http.disableResponseCompression
     Disable compression of HTTP responses to save CPU resources. By default compression is enabled to save network bandwidth
  -http.idleConnTimeout duration
     Timeout for incoming idle http connections (default 1m0s)
  -http.maxGracefulShutdownDuration duration
     The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown (default 7s)
  -http.pathPrefix string
     An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus
  -http.shutdownDelay duration
     Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers
  -httpAuth.password string
     Password for HTTP Basic Auth. The authentication is disabled if -httpAuth.username is empty
  -httpAuth.username string
     Username for HTTP Basic Auth. The authentication is disabled if empty. See also -httpAuth.password
  -httpListenAddr string
     Address to listen for http connections (default ":8300")
  -keepLastDaily int
     Keep last N daily backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastHourly int
     Keep last N hourly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastMonthly int
     Keep last N monthly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -keepLastWeekly int
     Keep last N weekly backups. If 0 is specified next retention cycle removes all backups for given time period. (default -1)
  -loggerDisableTimestamps
     Whether to disable writing timestamps in logs
  -loggerErrorsPerSecondLimit int
     Per-second limit on the number of ERROR messages. If more than the given number of errors are emitted per second, the remaining errors are suppressed. Zero values disable the rate limit
  -loggerFormat string
     Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
     Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
     Output for the logs. Supported values: stderr, stdout (default "stderr")
  -loggerTimezone string
     Timezone to use for timestamps in logs. Timezone must be a valid IANA Time Zone. For example: America/New_York, Europe/Berlin, Etc/GMT+3 or Local (default "UTC")
  -loggerWarnsPerSecondLimit int
     Per-second limit on the number of WARN messages. If more than the given number of warns are emitted per second, then the remaining warns are suppressed. Zero values disable the rate limit
  -maxBytesPerSecond int
     The maximum upload speed. There is no limit if it is set to 0
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, KiB, MiB, GiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from OS page cache which will result in higher disk IO usage (default 60)
  -metricsAuthKey string
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -pprofAuthKey string
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
  -runOnStart
     Upload backups immediately after start of the service. Otherwise the backup starts on new hour
  -s3ForcePathStyle
     Prefixing endpoint with bucket name when set false, true by default. (default true)
  -snapshot.createURL string
     VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup.Example: http://victoriametrics:8428/snapshot/create
  -snapshot.deleteURL string
     VictoriaMetrics delete snapshot url. Optional. Will be generated from snapshot.createURL if not provided. All created snaphosts will be automatically deleted.Example: http://victoriametrics:8428/snapshot/delete
  -storageDataPath string
     Path to VictoriaMetrics data. Must match -storageDataPath from VictoriaMetrics or vmstorage (default "victoria-metrics-data")
  -tls
     Whether to enable TLS for incoming HTTP requests at -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set
  -tlsCertFile string
     Path to file with TLS certificate if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated
  -tlsCipherSuites array
     Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants
     Supports an array of values separated by comma or specified via multiple flags.
  -tlsKeyFile string
     Path to file with TLS key if -tls is set. The provided key file is automatically re-read every second, so it can be dynamically updated
  -version
     Show VictoriaMetrics version
```
