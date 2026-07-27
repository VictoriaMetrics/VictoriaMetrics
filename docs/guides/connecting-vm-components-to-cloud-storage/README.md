Several VictoriaMetrics components can connect to cloud storage to read or write object data.

The following table shows the supported types of storage for each component:

| Component | AWS S3 and S3-compatible | Google Cloud Storage | Azure Blob Storage |
|-----------|----|----------------------|--------------------|
| [vmbackup](https://docs.victoriametrics.com/victoriametrics/vmbackup/) | ✅ | ✅ | ✅ |
| [vmrestore](https://docs.victoriametrics.com/victoriametrics/vmrestore/) | ✅ | ✅ | ✅ |
| [vmbackupmanager](https://docs.victoriametrics.com/victoriametrics/vmbackupmanager/) | ✅ | ✅ | ✅ |
| [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/) |  ✅ | ✅ | ❌ |

All these components use the same underlying libraries, so the authentication setup is largely the same. The main difference is in the command-line flags: 

- vmalert uses `-s3.*` prefixed flags (e.g., `-s3.credsFilePath`)
- backup and restore tools use unprefixed flags (e.g., `-credsFilePath`)

See the [component reference](https://docs.victoriametrics.com/guides/connecting-vm-components-to-cloud-storage/#per-component-flag-reference) for details.

## Obtaining credentials

You need to supply credentials so the component can connect to the cloud storage. The setup differs by provider; the sections below cover AWS S3, S3‑compatible systems, GCS, and Azure Blob Storage.

### AWS S3

1. In AWS, [create an IAM user](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html) or role with the minimum permissions for the target bucket (see table below).
1. [Create an access key](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html) for that IAM identity.
1. Copy the **Access key ID** and **Secret access key** values. You will use them in the credentials file or environment variables.

| Component       | S3 usage                                                    | Minimum S3 permissions |
|-----------------|-------------------------------------------------------------|------------------------|
| vmalert (Enterprise) | Reads alerting/recording rules from bucket.    | `s3:GetObject`, `s3:ListBucket` on the rules bucket/prefix.|
| vmrestore       | Restores backups from S3 buckets                            | `s3:GetObject`, `s3:ListBucket` on the backup bucket/prefix.|
| vmbackup        | Uploads backups to S3 and may delete old backup objects during maintenance.| `s3:PutObject`, `s3:GetObject`, `s3:ListBucket`, `s3:DeleteObject` on the backup bucket/prefix.|
| vmbackupmanager (Enterprise) | Automates backups using vmbackup behavior.                  | Same as vmbackup. |


### S3-compatible storage (MinIO, Ceph)

Generate access keys using your storage system's admin interface or CLI. The credentials follow the same format as AWS S3.

### Google Cloud Storage

1. Open the Google Cloud Console and go to **IAM & Admin > Service Accounts**.
1. Click **Create service account**, enter a name, and assign a Storage role (for example, Storage Object Admin).
1. Open the service account, go to **Keys**, then click **Add key > Create new key**.
1. Choose **JSON** as the key type and click **Create**.
1. Store the JSON file on the machine running the VictoriaMetrics component.

### Azure Blob Storage

> Azure does not use credential files.

1. Log in to the Azure Portal.
1. Search for and select **Storage accounts**.
1. Click on your specific storage account name. (This is your `AZURE_STORAGE_ACCOUNT_NAME`).
1. In the left menu under **Security + networking**, click **Access keys**.
1. Copy the key value from either **key1** or **key2** (this is your `AZURE_STORAGE_ACCOUNT_KEY`).
1. Define the access keys as environment variables.
    ```sh
    export AZURE_STORAGE_ACCOUNT_NAME=mystorageaccount
    export AZURE_STORAGE_ACCOUNT_KEY=myaccountkey
    ```

## Authenticating with the cloud provider

Provide the credentials as a file or with environment variables, along with the path to the cloud storage bucket. The syntax for the bucket name depends on the cloud provider:

- `s3://`: for AWS S3 and self-hosted S3-compatible storage (MinIO, Ceph)
- `gs://`: Google Cloud Storage
- `azblob://`: Azure Blob Storage

### vmbackup and vmrestore

The following example backups to an AWS S3 bucket using a credentials file:

```sh
vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=s3://victoriametrics-backup/backup01 \
  -credsFilePath=/etc/credentials
```

In order to restore from the same backup from AWS S3:

```sh
vmrestore \
  -src=s3://victoriametrics-backup/backup01 \
  -storageDataPath=/data \
  -credsFilePath=/etc/credentials

```

> To use non-AWS S3 buckets such as MinIO or Ceph, you must [supply the `-customS3Endpoint` argument](https://docs.victoriametrics.com/guides/connecting-vm-components-to-cloud-storage/#s3-compatible).

Alternatively, you can set the access keys as environment variables instead of using a credential file:

```sh
export AWS_ACCESS_KEY_ID=YOUR_AWS_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_AWS_ACCESS_KEY

vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=s3://victoriametrics-backup/backup01

vmrestore \
  -src=s3://victoriametrics-backup/backup01 \
  -storageDataPath=/data
```


Backups on Google Cloud Storage use the `gs://` prefix in the destination:

```sh
vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=gs://victoriametrics-backup/backup01 \
  -credsFilePath=/etc/credentials
```

You can restore this backup with:

```sh
vmrestore \
  -src=gs://victoriametrics-backup/backup01 \
  -storageDataPath=/data \
  -credsFilePath=/etc/credentials
```

On Google Cloud, you can define the path to the JSON credential file with `GOOGLE_APPLICATION_CREDENTIALS`. For example:

```sh
export GOOGLE_APPLICATION_CREDENTIALS=/etc/credentials

vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=gs://victoriametrics-backup/backup01

vmrestore \
  -src=gs://victoriametrics-backup/backup01 \
  -storageDataPath=/data
```

For Azure Blob Storage, use the `azblob://` prefix and rely on environment variables instead of `-credsFilePath`.

```sh
export AZURE_STORAGE_ACCOUNT_NAME=myaccount
export AZURE_STORAGE_ACCOUNT_KEY=mykey

vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=azblob://victoriametrics-backup/backup01

vmrestore \
  -src=azblob://victoriametrics-backup/backup01 \
  -storageDataPath=/data
```

### vmbackupmanager 

> vmbackupmanager only works in the [Enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/) edition.

To manage backups with vmbackupmanager on AWS S3, add the credentials with the `-credsFilePath` flag:

```sh
vmbackupmanager \
  -dst=s3://vmstorage-data/backups \
  -credsFilePath=/etc/credentials \
  -storageDataPath=/vmstorage-data \
  -snapshot.createURL=http://vmstorage:8482/snapshot/create \
  -licenseFile=/etc/vm-license
```

Or define the access keys using environment variables:

```sh
export AWS_ACCESS_KEY_ID=YOUR_AWS_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_AWS_ACCESS_KEY

vmbackupmanager \
  -dst=s3://vmstorage-data/backups \
  -storageDataPath=/vmstorage-data \
  -snapshot.createURL=http://vmstorage:8482/snapshot/create \
  -licenseFile=/etc/vm-license
```

> To use non-AWS S3 buckets, you must [supply the `-customS3Endpoint` argument](#s3-compatible).

Automated backups on Google Cloud Storage take the following form:

```sh
vmbackupmanager \
  -dst=gs://vmstorage-data/backups \
  -credsFilePath=/etc/credentials \
  -storageDataPath=/vmstorage-data \
  -snapshot.createURL=http://vmstorage:8482/snapshot/create \
  -licenseFile=/etc/vm-license
```

As with vmbackup and vmrestore, you can also define the path to the JSON credential file with `GOOGLE_APPLICATION_CREDENTIALS`:

```sh
export GOOGLE_APPLICATION_CREDENTIALS=/etc/credentials

vmbackupmanager \
  -dst=gs://vmstorage-data/backups \
  -storageDataPath=/vmstorage-data \
  -snapshot.createURL=http://vmstorage:8482/snapshot/create \
  -licenseFile=/etc/vm-license
```

vmbackupmanager can also use Azure Blob Storage by defining environment variables:

```sh
export AZURE_STORAGE_ACCOUNT_NAME=mystorageaccount
export AZURE_STORAGE_ACCOUNT_KEY=myaccountkey

vmbackupmanager \
  -dst=azblob://vmstorage-data/backups \
  -storageDataPath=/vmstorage-data \
  -snapshot.createURL=http://vmstorage:8482/snapshot/create \
  -licenseFile=/etc/vm-license
```

### vmalert

> - vmalert cloud storage command line flags are prefixed with `-s3.` for S3 buckets *and* Google Cloud Storage.
> - Cloud storage only works in the [Enterprise](https://docs.victoriametrics.com/victoriametrics/enterprise/) edition.

Read alerting rules from an S3 bucket. The `-rule` flag accepts a prefix, so it matches all files starting with `alerts_` in the `rules` folder:

```sh
vmalert \
  -rule=s3://my-alert-bucket/rules/alerts_ \
  -s3.credsFilePath=/etc/vmalert/aws-credentials \
  -datasource.url=http://vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager:9093 \
  -licenseFile=/etc/vm-license
```

Instead of a credential file, you can supply the access keys using environment variables:

```sh
export AWS_ACCESS_KEY_ID=YOUR_AWS_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=YOUR_SECRET_AWS_ACCESS_KEY

vmalert \
  -rule=s3://my-alert-bucket/rules/alerts_ \
  -datasource.url=http://vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager:9093 \
  -licenseFile=/etc/vm-license
```

> To use non-AWS S3 buckets, you must [supply the `-s3.customEndpoint` argument](#s3-compatible).

To read rules from Google Cloud Storage:

```sh
vmalert \
  -rule=gs://my-alert-bucket/rules/alerts_ \
  -s3.credsFilePath=/etc/vmalert/gcp-service-account.json \
  -datasource.url=http://vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager:9093 \
  -licenseFile=/etc/vm-license
```

If you prefer, you can supply the path to the JSON credential file with the `GOOGLE_APPLICATION_CREDENTIALS` environment variable:

```sh
export GOOGLE_APPLICATION_CREDENTIALS=/etc/credentials

vmalert \
  -rule=gs://my-alert-bucket/rules/alerts_ \
  -datasource.url=http://vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager:9093 \
  -licenseFile=/etc/vm-license
```

## Credentials files format

The file format depends on the storage provider.

### S3 credentials

The file uses the standard AWS shared credentials format used by the [AWS CLI](https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html) and [AWS SDKs](https://docs.aws.amazon.com/sdkref/latest/guide/file-format.html):

```ini
[default]
aws_access_key_id=YOUR_AWS_ACCESS_KEY
aws_secret_access_key=YOUR_AWS_SECRET_ACCESS_KEY
```

You can define multiple profiles in a single file:

```ini
[default]
aws_access_key_id=DEFAULT_ACCESS_KEY
aws_secret_access_key=DEFAULT_SECRET_KEY

[monitoring]
aws_access_key_id=MONITORING_ACCESS_KEY
aws_secret_access_key=MONITORING_SECRET_KEY

[alerts]
aws_access_key_id=ALERTS_ACCESS_KEY
aws_secret_access_key=ALERTS_SECRET_KEY
```

Use the `-configProfile` flag (or `-s3.configProfile` in vmalert) to select a non-default profile:

```sh
-configProfile=alerts
```

You can separate credentials from other configuration settings. Put credentials in one file:

```ini
[default]
aws_access_key_id=DEFAULT_ACCESS_KEY
aws_secret_access_key=DEFAULT_SECRET_KEY
```

And non-sensitive settings in another:

```ini
[default]
region=us-east-1
```

Then pass both files:

```sh
-configFilePath=/etc/aws-config \
-credsFilePath=/etc/credentials
```

### GCS credentials file format

The file is the JSON key downloaded from Google Cloud Console. Its content looks like this:

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

This is the standard service account key format defined by [Google Cloud IAM](https://developers.google.com/workspace/guides/create-credentials).

### Azure Blob Storage

Azure does not support credentials via file. Use environment variables instead.

```sh
export AZURE_STORAGE_ACCOUNT_NAME=mystorageaccount
export AZURE_STORAGE_ACCOUNT_KEY=myaccountkey
```

## Self-hosted and S3-compatible endpoints

For S3-compatible storage such as MinIO or Ceph, set a custom endpoint with the `-customS3Endpoint` flag  for vmbackup, vmrestore, and vmbackupmanager. For example:

```sh
vmbackup \
  -storageDataPath=/data \
  -snapshot.createURL=http://localhost:8428/snapshot/create \
  -dst=s3://victoriametrics-backup/backup01 \
  -customS3Endpoint=http://minio.example.local:9000
```

On vmalert, use the `-s3.customEndpoint` flag instead:

```sh
vmalert \
  -rule=s3://my-alert-bucket/rules/alerts_ \
  -s3.customEndpoint=http://minio.example.local:9000 \
  -s3.credsFilePath=/etc/vmalert/aws-credentials \
  -datasource.url=http://vmselect:8481/select/0/prometheus \
  -notifier.url=http://alertmanager:9093 \
  -licenseFile=/etc/vm-license
```

### Addressing S3-compatible buckets {#s3-compatible}

When connecting to non-AWS S3-compatible buckets, there is an additional flag you might need to configure:

- `-s3ForcePathStyle`: on vmbackupmanager, vmbackup, and vmrestore.
- `-s3.forcePathStyle`: on vmalert.

The flag changes the expected URL pattern for a bucket.

| Flag value | Address-style | Example | Use with |
|------------|---------------|---------|----------|
| `true` (default) | Path-style | `https://endpoint/bucket/key` |  MinIO, Ceph, most S3-compatible storages |
| `false`        | Virtual host-style | `https://bucket.endpoint/key` | [Aliyun OSS](https://www.aliyun.com/product/oss) and other endpoints that require it |

> The flag only takes effect when you use a custom endpoint (`-customS3Endpoint` or `-s3.customEndpoint` on vmalert). When connecting to real AWS S3, the SDK handles addressing automatically.

## Per-component flag reference

The table below shows how the same concept maps to different flag names across components.

| Concept | vmalert | vmbackup, vmrestore, and vmbackupmanager |
|---|---|---|---|---|
| Credentials file | `-s3.credsFilePath` | `-credsFilePath` |
| Config file | `-s3.configFilePath` | `-configFilePath` |
| Profile selection | `-s3.configProfile` | `-configProfile` |
| Custom endpoint | `-s3.customEndpoint` | `-customS3Endpoint` |
| Force path style | `-s3.forcePathStyle` | `-s3ForcePathStyle` |
| TLS insecure | N/A | `-s3TLSInsecureSkipVerify` |
| Storage class | N/A | `-s3StorageClass` |



