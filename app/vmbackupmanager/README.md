## vmbackupmanager

***vmbackupmanager is a part of [enterprise package](https://victoriametrics.com/products/enterprise/). It is available for download and evaluation at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)***

The VictoriaMetrics backup manager automates regular backup procedures. It supports the following backup intervals: **hourly**, **daily**, **weekly** and **monthly**. Multiple backup intervals may be configured simultaneously. I.e. the backup manager creates hourly backups every hour, while it creates daily backups every day, etc. Backup manager must have read access to the storage data, so best practice is to install it on the same machine (or as a sidecar) where the storage node is installed.
The backup service makes a backup every hour and puts it to the latest folder and then copies data to the folders which represent the backup intervals (hourly, daily, weekly and monthly)

The required flags for running the service are as follows:

* -eula - should be true and means that you have the legal right to run a backup manager. That can either be a signed contract or an email with confirmation to run the service in a trial period
* -storageDataPath - path to VictoriaMetrics or vmstorage data path to make backup from
* -snapshot.createURL - VictoriaMetrics creates snapshot URL which will automatically be created during backup. Example: http://victoriametrics:8428/snapshot/create
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

- The root folder
  
  ![root](vmbackupmanager_root_folder.png)

- The latest folder

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
