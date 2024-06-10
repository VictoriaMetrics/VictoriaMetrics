---
sort: 6
weight: 6
menu:
  docs:
    parent: 'victoriametrics'
    weight: 6
title: vmbackup
aliases:
  - /vmbackup.html
---
# vmbackup

`vmbackup` creates VictoriaMetrics data backups from [instant snapshots](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots).

`vmbackup` supports incremental and full backups. Incremental backups are created automatically if the destination path already contains data from the previous backup.
Full backups can be accelerated with `-origin` pointing to an already existing backup on the same remote storage. In this case `vmbackup` makes server-side copy for the shared
data between the existing backup and new backup. It saves time and costs on data transfer.

Backup process can be interrupted at any time. It is automatically resumed from the interruption point when restarting `vmbackup` with the same args.

Backed up data can be restored with [vmrestore](https://docs.victoriametrics.com/vmrestore/).

See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for more details.

See also [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager/) tool built on top of `vmbackup`. This tool simplifies
creation of hourly, daily, weekly and monthly backups.

## Supported storage types

`vmbackup` supports the following `-dst` storage types:

* [GCS](https://cloud.google.com/storage/). Example: `gs://<bucket>/<path/to/backup>`
* [S3](https://aws.amazon.com/s3/). Example: `s3://<bucket>/<path/to/backup>`
* [Azure Blob Storage](https://azure.microsoft.com/en-us/products/storage/blobs/). Example: `azblob://<container>/<path/to/backup>`
* Any S3-compatible storage such as [MinIO](https://github.com/minio/minio), [Ceph](https://docs.ceph.com/en/pacific/radosgw/s3/) or [Swift](https://platform.swiftstack.com/docs/admin/middleware/s3_middleware.html). See [these docs](#advanced-usage) for details.
* Local filesystem. Example: `fs://</absolute/path/to/backup>`. Note that `vmbackup` prevents from storing the backup into the directory pointed by `-storageDataPath` command-line flag, since this directory should be managed solely by VictoriaMetrics or `vmstorage`.

## Use cases

### Regular backups

Regular backup can be performed with the following command:

```sh
./vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshot.createURL=http://localhost:8428/snapshot/create -dst=gs://<bucket>/<path/to/new/backup>
```

* `</path/to/victoria-metrics-data>` - path to VictoriaMetrics data pointed by `-storageDataPath` command-line flag in single-node VictoriaMetrics or in cluster `vmstorage`.
  There is no need to stop VictoriaMetrics for creating backups since they are performed from immutable [instant snapshots](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots).
* `http://victoriametrics:8428/snapshot/create` is the url for creating snapshots according to [these docs](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots). `vmbackup` creates a snapshot by querying the provided `-snapshot.createURL`, then performs the backup and then automatically removes the created snapshot.
* `<bucket>` is an already existing name for [GCS bucket](https://cloud.google.com/storage/docs/creating-buckets).
* `<path/to/new/backup>` is the destination path where new backup will be placed.

### Regular backups with server-side copy from existing backup

If the destination GCS bucket already contains the previous backup at `-origin` path, then new backup can be accelerated
with the following command:

```sh
./vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshot.createURL=http://localhost:8428/snapshot/create -dst=gs://<bucket>/<path/to/new/backup> -origin=gs://<bucket>/<path/to/existing/backup>
```

It saves time and network bandwidth costs by performing server-side copy for the shared data from the `-origin` to `-dst`.
Typical object storage just creates new names for already existing objects when performing server-side copy,
so this operation should be fast and inexpensive. Unfortunately, there are object storage systems such as [S3 Glacier](https://aws.amazon.com/s3/storage-classes/glacier/),
which make full copies for the copied objects during server-side copy. This may significantly slow down server-side copy
and make it very expensive.

### Incremental backups

Incremental backups are performed if `-dst` points to an already existing backup. In this case only new data is uploaded to remote storage.
It saves time and network bandwidth costs when working with big backups:

```sh
./vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshot.createURL=http://localhost:8428/snapshot/create -dst=gs://<bucket>/<path/to/existing/backup>
```

### Smart backups

Smart backups mean storing full daily backups into `YYYYMMDD` folders and creating incremental hourly backup into `latest` folder:

* Run the following command every hour:

```sh
./vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshot.createURL=http://localhost:8428/snapshot/create -dst=gs://<bucket>/latest
```

This command creates an [instant snapshot](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots)
and uploads it to `gs://<bucket>/latest`. It uploads only the changed data (aka incremental backup). This saves network bandwidth costs and time
when backing up large amounts of data.

* Run the following command once a day:

```sh
./vmbackup -origin=gs://<bucket>/latest -dst=gs://<bucket>/<YYYYMMDD>
```

This command makes [server-side copy](#server-side-copy-of-the-existing-backup) of the backup from `gs://<bucket>/latest` to `gs://<bucket>/<YYYYMMDD>`,
were `<YYYYMMDD>` is the current date like `20240125`. Server-side copy of the backup should be fast on most object storage systems,
since it just creates new names for already existing objects. The server-side copy can be slow on some object storage systems
such as [S3 Glacier](https://aws.amazon.com/s3/storage-classes/glacier/), since they may perform full object copy instead of creating
new names for already existing objects. This may be slow and expensive.

The `smart backups` approach described above saves network bandwidth costs on hourly backups (since they are incremental)
and allows recovering data from either the last hour (the  `latest` backup) or from any day (`YYYYMMDD` backups).

Note that hourly backup shouldn't run when creating daily backup.

Do not forget to remove old backups when they are no longer needed in order to save storage costs.

See also [vmbackupmanager tool](https://docs.victoriametrics.com/vmbackupmanager/) for automating smart backups.

### Server-side copy of the existing backup

Sometimes it is needed to make server-side copy of the existing backup. This can be done by specifying the source backup path via `-origin` command-line flag,
while the destination path for backup copy must be specified via `-dst` command-line flag. For example, the following command copies backup
from `gs://bucket/foo` to `gs://bucket/bar`:

```sh
./vmbackup -origin=gs://bucket/foo -dst=gs://bucket/bar
```

The `-origin` and `-dst` must point to the same object storage bucket or to the same filesystem.

The server-side backup copy is usually performed at much faster speed comparing to the usual backup, since backup data isn't transferred
between the remote storage and locally running `vmbackup` tool. Object storage systems usually just make new names for already existing
objects during server-side copy. Unfortunately there are systems such as [S3 Glacier](https://aws.amazon.com/s3/storage-classes/glacier/),
which perform full object copy during server-side copying. This may be slow and expensive.

If the `-dst` already contains some data, then its' contents is synced with the `-origin` data. This allows making incremental server-side copies of backups.

## How does it work?

The backup algorithm is the following:

1. Create a snapshot by querying the provided `-snapshot.createURL`
1. Collect information about files in the created snapshot, in the `-dst` and in the `-origin`.
1. Determine which files in `-dst` are missing in the created snapshot, and delete them. These are usually small files, which are already merged into bigger files in the snapshot.
1. Determine which files in the created snapshot are missing in `-dst`. These are usually small new files and bigger merged files.
1. Determine which files from step 3 exist in the `-origin`, and perform server-side copy of these files from `-origin` to `-dst`.
   These are usually the biggest and the oldest files, which are shared between backups.
1. Upload the remaining files from step 3 from the created snapshot to `-dst`.
1. Delete the created snapshot.

The algorithm splits source files into 1 GiB chunks in the backup. Each chunk is stored as a separate file in the backup.
Such splitting balances between the number of files in the backup and the amounts of data that needs to be re-transferred after temporary errors.

`vmbackup` relies on [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282) properties:

* All the files in the snapshot are immutable.
* Old files are periodically merged into new files.
* Smaller files have higher probability to be merged.
* Consecutive snapshots share many identical files.

These properties allow performing fast and cheap incremental backups and server-side copying from `-origin` paths.
See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for more details.
`vmbackup` can work improperly or slowly when these properties are violated.

## Troubleshooting

* If the backup is slow, then try setting higher value for `-concurrency` flag. This will increase the number of concurrent workers that upload data to backup storage.
* If `vmbackup` eats all the network bandwidth or CPU, then either decrease the `-concurrency` command-line flag value or set `-maxBytesPerSecond` command-line flag value to lower value.
* If `vmbackup` consumes all the CPU on systems with big number of CPU cores, then try running it with `-filestream.disableFadvise` command-line flag.
* If `vmbackup` has been interrupted due to temporary error, then just restart it with the same args. It will resume the backup process.
* Backups created from [single-node VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/) cannot be restored
  at [cluster VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/) and vice versa.

## Advanced usage


### Providing credentials as a file

Obtaining credentials from a file.

Add flag `-credsFilePath=/etc/credentials` with the following content:

- for S3 (AWS, MinIO or other S3 compatible storages):
    
     ```sh
     [default]
     aws_access_key_id=theaccesskey
     aws_secret_access_key=thesecretaccesskeyvalue
    ```

- for GCP cloud storage:
    
    ```json
    {
           "type": "service_account",
           "project_id": "project-id",
           "private_key_id": "key-id",
           "private_key": "-----BEGIN PRIVATE KEY-----\nprivate-key\n-----END PRIVATE KEY-----\n",
           "client_email": "service-account-email",
           "client_id": "client-id",
           "auth_uri": "https://accounts.google.com/o/oauth2/auth",
           "token_uri": "https://accounts.google.com/o/oauth2/token",
           "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
           "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/service-account-email"
    }
    ```

### Providing credentials via env variables 

Obtaining credentials from env variables.
- For AWS S3 compatible storages set env variable `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. 
  Also you can set env variable `AWS_SHARED_CREDENTIALS_FILE` with path to credentials file.
- For GCE cloud storage set env variable `GOOGLE_APPLICATION_CREDENTIALS` with path to credentials file.
- For Azure storage either set env variables `AZURE_STORAGE_ACCOUNT_NAME` and `AZURE_STORAGE_ACCOUNT_KEY`, or `AZURE_STORAGE_ACCOUNT_CONNECTION_STRING`.

Please, note that `vmbackup` will use credentials provided by cloud providers metadata service [when applicable](https://docs.victoriametrics.com/vmbackup/#using-cloud-providers-metadata-service).

### Using cloud providers metadata service

`vmbackup` and `vmbackupmanager` will automatically use cloud providers metadata service in order to obtain credentials if they are running in cloud environment
and credentials are not explicitly provided via flags or env variables.

### Providing credentials in Kubernetes

The simplest way to provide credentials in Kubernetes is to use [Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
and inject them into the pod as environment variables. For example, the following secret can be used for AWS S3 credentials:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vmbackup-credentials
data:
  access_key: key
  secret_key: secret
```
And then it can be injected into the pod as environment variables:
```yaml
...
env:
- name: AWS_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      key: access_key
      name: vmbackup-credentials
- name: AWS_SECRET_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      key: secret_key
      name: vmbackup-credentials
...
```

A more secure way is to use IAM roles to provide tokens for pods instead of managing credentials manually. 

For AWS deployments it will be required to configure [IAM roles for service accounts](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).
In order to use IAM roles for service accounts with `vmbackup` or `vmbackupmanager` it is required to create ServiceAccount with IAM role mapping:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: monitoring-backups
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::{ACCOUNT_ID}:role/{ROLE_NAME}
```
And [configure pod to use service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/).
After this `vmbackup` and `vmbackupmanager` will automatically use IAM role for service account in order to obtain credentials.

For GCP deployments it will be required to configure [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity).
In order to use Workload Identity with `vmbackup` or `vmbackupmanager` it is required to create ServiceAccount with Workload Identity annotation:
```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: monitoring-backups
  annotations:
    iam.gke.io/gcp-service-account: {sa_name}@{project_name}.iam.gserviceaccount.com
```
And [configure pod to use service account](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/).
After this `vmbackup` and `vmbackupmanager` will automatically use Workload Identity for service account in order to obtain credentials.

### Using custom S3 endpoint

Usage with s3 custom url endpoint. It is possible to use `vmbackup` with s3 compatible storages like minio, cloudian, etc.
You have to add a custom url endpoint via flag:

- for MinIO
    ```sh
      -customS3Endpoint=http://localhost:9000
    ```

- for aws gov region
    ```sh
      -customS3Endpoint=https://s3-fips.us-gov-west-1.amazonaws.com
    ```

### Permanent deletion of objects in S3-compatible storages

`vmbackup` and [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager/) use standard delete operation
for S3-compatible object storage when performing [incremental backups](#incremental-backups).
This operation removes only the current version of the object. This works OK in most cases.

Sometimes it is needed to remove all the versions of an object. In this case pass `-deleteAllObjectVersions` command-line flag to `vmbackup`.

Alternatively, it is possible to use object storage lifecycle rules to remove non-current versions of objects automatically.
Refer to the respective documentation for your object storage provider for more details.

### Command-line flags

Run `vmbackup -help` in order to see all the available options:

```sh
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
  -dst string
     Where to put the backup on the remote storage. Example: gs://bucket/path/to/backup, s3://bucket/path/to/backup, azblob://container/path/to/backup or fs:///path/to/local/backup/dir
     -dst can point to the previous backup. In this case incremental backup is performed, i.e. only changed data is uploaded
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
     Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
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
  -httpListenAddr string
     TCP address for exporting metrics at /metrics page (default ":8420")
  -internStringCacheExpireDuration duration
     The expiry duration for caches for interned strings. See https://en.wikipedia.org/wiki/String_interning . See also -internStringMaxLen and -internStringDisableCache (default 6m0s)
  -internStringDisableCache
     Whether to disable caches for interned strings. This may reduce memory usage at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringCacheExpireDuration and -internStringMaxLen
  -internStringMaxLen int
     The maximum length for strings to intern. A lower limit may save memory at the cost of higher CPU usage. See https://en.wikipedia.org/wiki/String_interning . See also -internStringDisableCache and -internStringCacheExpireDuration (default 500)
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
  -maxBytesPerSecond size
     The maximum upload speed. There is no limit if it is set to 0
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedBytes size
     Allowed size of system memory VictoriaMetrics caches may occupy. This option overrides -memory.allowedPercent if set to a non-zero value. Too low a value may increase the cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache resulting in higher disk IO usage
     Supports the following optional suffixes for size values: KB, MB, GB, TB, KiB, MiB, GiB, TiB (default 0)
  -memory.allowedPercent float
     Allowed percent of system memory VictoriaMetrics caches may occupy. See also -memory.allowedBytes. Too low a value may increase cache miss rate usually resulting in higher CPU and disk IO usage. Too high a value may evict too much data from the OS page cache which will result in higher disk IO usage (default 60)
  -metrics.exposeMetadata
     Whether to expose TYPE and HELP metadata at the /metrics page, which is exposed at -httpListenAddr . The metadata may be needed when the /metrics page is consumed by systems, which require this information. For example, Managed Prometheus in Google Cloud - https://cloud.google.com/stackdriver/docs/managed-prometheus/troubleshooting#missing-metric-type
  -metricsAuthKey value
     Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings
     Flag value can be read from the given file when using -metricsAuthKey=file:///abs/path/to/file or -metricsAuthKey=file://./relative/path/to/file . Flag value can be read from the given http/https url when using -metricsAuthKey=http://host/path or -metricsAuthKey=https://host/path
  -mtls array
     Whether to require valid client certificate for https requests to the corresponding -httpListenAddr . This flag works only if -tls flag is set. See also -mtlsCAFile . This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports array of values separated by comma or specified via multiple flags.
     Empty values are set to false.
  -mtlsCAFile array
     Optional path to TLS Root CA for verifying client certificates at the corresponding -httpListenAddr when -mtls is enabled. By default the host system TLS Root CA is used for client certificate verification. This flag is available only in Enterprise binaries. See https://docs.victoriametrics.com/enterprise/
     Supports an array of values separated by comma or specified via multiple flags.
     Value can contain comma inside single-quoted or double-quoted string, {}, [] and () braces.
  -origin string
     Optional origin directory on the remote storage with old backup for server-side copying when performing full backup. This speeds up full backups
  -pprofAuthKey value
     Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides httpAuth.* settings
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
  -s3ForcePathStyle
     Prefixing endpoint with bucket name when set false, true by default. (default true)
  -s3StorageClass string
     The Storage Class applied to objects uploaded to AWS S3. Supported values are: GLACIER, DEEP_ARCHIVE, GLACIER_IR, INTELLIGENT_TIERING, ONEZONE_IA, OUTPOSTS, REDUCED_REDUNDANCY, STANDARD, STANDARD_IA.
     See https://docs.aws.amazon.com/AmazonS3/latest/userguide/storage-class-intro.html
  -s3TLSInsecureSkipVerify
     Whether to skip TLS verification when connecting to the S3 endpoint.
  -snapshot.createURL string
     VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup. Example: http://victoriametrics:8428/snapshot/create . There is no need in setting -snapshotName if -snapshot.createURL is set
  -snapshot.deleteURL string
     VictoriaMetrics delete snapshot url. Optional. Will be generated from -snapshot.createURL if not provided. All created snapshots will be automatically deleted. Example: http://victoriametrics:8428/snapshot/delete
  -snapshot.tlsCAFile string
     Optional path to TLS CA file to use for verifying connections to -snapshotCreateURL. By default, system CA is used
  -snapshot.tlsCertFile string
     Optional path to client-side TLS certificate file to use when connecting to -snapshotCreateURL
  -snapshot.tlsInsecureSkipVerify
     Whether to skip tls verification when connecting to -snapshotCreateURL
  -snapshot.tlsKeyFile string
     Optional path to client-side TLS certificate key to use when connecting to -snapshotCreateURL
  -snapshot.tlsServerName string
     Optional TLS server name to use for connections to -snapshotCreateURL. By default, the server name from -snapshotCreateURL is used
  -snapshotName string
     Name for the snapshot to backup. See https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-work-with-snapshots. There is no need in setting -snapshotName if -snapshot.createURL is set
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

## How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) - see `vmutils-*` archives there.

### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.22.
1. Run `make vmbackup` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmbackup` binary and puts it into the `bin` folder.

### Production build

1. [Install docker](https://docs.docker.com/install/).
1. Run `make vmbackup-prod` from the root folder of [the repository](https://github.com/VictoriaMetrics/VictoriaMetrics).
   It builds `vmbackup-prod` binary and puts it into the `bin` folder.

### Building docker images

Run `make package-vmbackup`. It builds `victoriametrics/vmbackup:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmbackup`.

The base docker image is [alpine](https://hub.docker.com/_/alpine) but it is possible to use any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```sh
ROOT_IMAGE=scratch make package-vmbackup
```
