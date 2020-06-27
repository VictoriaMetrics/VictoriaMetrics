## vmbackup

`vmbackup` creates VictoriaMetrics data backups from [instant snapshots](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots).

Supported storage systems for backups:

* [GCS](https://cloud.google.com/storage/). Example: `gcs://<bucket>/<path/to/backup>`
* [S3](https://aws.amazon.com/s3/). Example: `s3://<bucket>/<path/to/backup>`
* Any S3-compatible storage such as [MinIO](https://github.com/minio/minio), [Ceph](https://docs.ceph.com/docs/mimic/radosgw/s3/) or [Swift](https://www.swiftstack.com/docs/admin/middleware/s3_middleware.html). See `-customS3Endpoint` command-line flag.
* Local filesystem. Example: `fs://</absolute/path/to/backup>`

Incremental backups and full backups are supported. Incremental backups are created automatically if the destination path already contains data from the previous backup.
Full backups can be sped up with `-origin` pointing to already existing backup on the same remote storage. In this case `vmbackup` makes server-side copy for the shared
data between the existing backup and new backup. This saves time and costs on data transfer.

Backup process can be interrupted at any time. It is automatically resumed from the interruption point when restarting `vmbackup` with the same args.

Backed up data can be restored with [vmrestore](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmrestore/README.md).

See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for more details.

See also [vmbackuper](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/466) tool built on top of `vmbackup`. This tool simplifies
creation of hourly, daily, weekly and monthly backups.


### Use cases

#### Regular backups

Regular backup can be performed with the following command:

```
vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshotName=<local-snapshot> -dst=gcs://<bucket>/<path/to/new/backup>
```

* `</path/to/victoria-metrics-data>` - path to VictoriaMetrics data pointed by `-storageDataPath` command-line flag in single-node VictoriaMetrics or in cluster `vmstorage`.
  There is no need to stop VictoriaMetrics for creating backups, since they are performed from immutable [instant snapshots](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots).
* `<local-snapshot>` is the snapshot to backup. See [how to create instant snapshots](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots).
* `<bucket>` is already existing name for [GCS bucket](https://cloud.google.com/storage/docs/creating-buckets).
* `<path/to/new/backup>` is the destination path where new backup will be placed.


#### Regular backups with server-side copy from existing backup

If the destination GCS bucket already contains the previous backup at `-origin` path, then new backup can be sped up
with the following command:

```
vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshotName=<local-snapshot> -dst=gcs://<bucket>/<path/to/new/backup> -origin=gcs://<bucket>/<path/to/existing/backup>
```

This saves time and network bandwidth costs by performing server-side copy for the shared data from the `-origin` to `-dst`.


#### Incremental backups

Incremental backups are performed if `-dst` points to already existing backup. In this case only new data is uploaded to remote storage.
This saves time and network bandwidth costs when working with big backups:

```
vmbackup -storageDataPath=</path/to/victoria-metrics-data> -snapshotName=<local-snapshot> -dst=gcs://<bucket>/<path/to/existing/backup>
```


#### Smart backups

Smart backups mean storing full daily backups into `YYYYMMDD` folders and creating incremental hourly backup into `latest` folder:

* Run the following command every hour:

```
vmbackup -snapshotName=<latest-snapshot> -dst=gcs://<bucket>/latest
```

Where `<latest-snapshot>` is the latest [snapshot](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots).
The command will upload only changed data to `gcs://<bucket>/latest`.

* Run the following command once a day:

```
vmbackup -snapshotName=<daily-snapshot> -dst=gcs://<bucket>/<YYYYMMDD> -origin=gcs://<bucket>/latest
```

Where `<daily-snapshot>` is the snapshot for the last day `<YYYYMMDD>`.


This apporach saves network bandwidth costs on hourly backups (since they are incremental) and allows recovering data from either the last hour (`latest` backup)
or from any day (`YYYYMMDD` backups). Note that hourly backup shouldn't run when creating daily backup.

Do not forget removing old snapshots and backups when they are no longer needed for saving storage costs.

See also [vmbackuper tool](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/466) for automating smart backups.


### How does it work?

The backup algorithm is the following:

1. Collect information about files in the `-snapshotName`, in the `-dst` and in the `-origin`.
2. Determine files in `-dst`, which are missing in `-snapshotName`, and delete them. These are usually small files, which are already merged into bigger files in the snapshot.
3. Determine files from `-snapshotName`, which are missing in `-dst`. These are usually small new files and bigger merged files.
4. Determine files from step 3, which exist in the `-origin`, and perform server-side copy of these files from `-origin` to `-dst`.
   This are usually the biggest and the oldest files, which are shared between backups.
5. Upload the remaining files from setp 3 from `-snapshotName` to `-dst`.

The algorithm splits source files into 100MB chunks in the backup. Each chunk is stored as a separate file in the backup.
Such splitting minimizes the amounts of data to re-transfer after temporary errors.

`vmbackup` relies on [instant snapshot](https://medium.com/@valyala/how-victoriametrics-makes-instant-snapshots-for-multi-terabyte-time-series-data-e1f3fb0e0282) properties:

- All the files in the snapshot are immutable.
- Old files are periodically merged into new files.
- Smaller files have higher probability to be merged.
- Consecutive snapshots share many identical files.

These properties allow performing fast and cheap incremental backups and server-side copying from `-origin` paths.
See [this article](https://medium.com/@valyala/speeding-up-backups-for-big-time-series-databases-533c1a927883) for more details.
`vmbackup` can work improperly or slowly when these properties are violated.


### Troubleshooting

* If the backup is slow, then try setting higher value for `-concurrency` flag. This will increase the number of concurrent workers that upload data to backup storage.
* If `vmbackup` eats all the network bandwidth, then set `-maxBytesPerSecond` to the desired value.
* If `vmbackup` has been interrupted due to temporary error, then just restart it with the same args. It will resume the backup process.
* Backups created from [single-node VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md) cannot be restored
  at [cluster VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md) and vice versa.


### Advanced usage

Run `vmbackup -help` in order to see all the available options:

```
  -concurrency int
    	The number of concurrent workers. Higher concurrency may reduce backup duration (default 10)
  -configFilePath string
    	Path to file with S3 configs. Configs are loaded from default location if not set.
    	See https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
  -configProfile string
    	Profile name for S3 configs (default "default")
  -credsFilePath string
    	Path to file with GCS or S3 credentials. Credentials are loaded from default locations if not set.
    	See https://cloud.google.com/iam/docs/creating-managing-service-account-keys and https://docs.aws.amazon.com/general/latest/gr/aws-security-credentials.html
  -customS3Endpoint string
    	Custom S3 endpoint for use with S3-compatible storages (e.g. MinIO). S3 is used if not set
  -dst string
    	Where to put the backup on the remote storage. Example: gcs://bucket/path/to/backup/dir, s3://bucket/path/to/backup/dir or fs:///path/to/local/backup/dir
    	-dst can point to the previous backup. In this case incremental backup is performed, i.e. only changed data is uploaded
  -envflag.enable
    	Whether to enable reading flags from environment variables additionally to command line. Command line flag values have priority over values from environment vars. Flags are read only from command line if this flag isn't set
  -envflag.prefix string
    	Prefix for environment variables if -envflag.enable is set
  -fs.disableMmap
    	Whether to use pread() instead of mmap() for reading data files
  -loggerFormat string
    	Format for logs. Possible values: default, json (default "default")
  -loggerLevel string
    	Minimum level of errors to log. Possible values: INFO, WARN, ERROR, FATAL, PANIC (default "INFO")
  -loggerOutput string
    	Output for the logs. Supported values: stderr, stdout (default "stderr")
  -maxBytesPerSecond int
    	The maximum upload speed. There is no limit if it is set to 0
  -memory.allowedPercent float
    	Allowed percent of system memory VictoriaMetrics caches may occupy. Too low value may increase cache miss rate, which usually results in higher CPU and disk IO usage. Too high value may evict too much data from OS page cache, which will result in higher disk IO usage (default 60)
  -origin string
    	Optional origin directory on the remote storage with old backup for server-side copying when performing full backup. This speeds up full backups
  -snapshot.createURL string
    	VictoriaMetrics create snapshot url. When this is given a snapshot will automatically be created during backup.Example: http://victoriametrics:8428/snaphsot/create
  -snapshot.deleteURL string
    	VictoriaMetrics delete snapshot url. Optional. Will be generated from snapshotCreateURL if not provided. All created snaphosts will be automatically deleted.Example: http://victoriametrics:8428/snaphsot/delete
  -snapshotName string
    	Name for the snapshot to backup. See https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots
  -storageDataPath string
    	Path to VictoriaMetrics data. Must match -storageDataPath from VictoriaMetrics or vmstorage (default "victoria-metrics-data")
  -version
    	Show VictoriaMetrics version
```


### How to build from sources

It is recommended using [binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) - see `vmutils-*` archives there.


#### Development build

1. [Install Go](https://golang.org/doc/install). The minimum supported version is Go 1.13.
2. Run `make vmbackup` from the root folder of the repository.
   It builds `vmbackup` binary and puts it into the `bin` folder.

#### Production build

1. [Install docker](https://docs.docker.com/install/).
2. Run `make vmbackup-prod` from the root folder of the repository.
   It builds `vmbackup-prod` binary and puts it into the `bin` folder.

#### Building docker images

Run `make package-vmbackup`. It builds `victoriametrics/vmbackup:<PKG_TAG>` docker image locally.
`<PKG_TAG>` is auto-generated image tag, which depends on source code in the repository.
The `<PKG_TAG>` may be manually set via `PKG_TAG=foobar make package-vmbackup`.

By default the image is built on top of [alpine](https://hub.docker.com/_/alpine) image. It is possible to build the package on top of any other base image
by setting it via `<ROOT_IMAGE>` environment variable. For example, the following command builds the image on top of [scratch](https://hub.docker.com/_/scratch) image:

```bash
ROOT_IMAGE=scratch make package-vmbackup
```
