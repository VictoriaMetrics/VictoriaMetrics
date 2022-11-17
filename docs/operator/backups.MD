---
sort: 5
---

# Backups

## vmbackupmanager

More details about `vmbackupmanager` can be found [here](https://docs.victoriametrics.com/vmbackupmanager.html).

### vmbackupmanager is proprietary software. 

Before using it, you must have signed contract and accept EULA https://victoriametrics.com/assets/VM_EULA.pdf

## Usage examples

`VMSingle` and `VMCluster` has built-in backup configuration, it uses `vmbackupmanager` - proprietary tool for backups.
It supports incremental backups (hours, daily, etc) with popular object storages (aws s3, google cloud storage).

You can enable it with the simple configuration, define secret


```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: remote-storage-keys
type: Opaque
stringData:
  credentials: |-
    [default]
    aws_access_key_id = your_access_key_id
    aws_secret_access_key = your_secret_access_key
---
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  # Add fields here
  retentionPeriod: "1"
  vmBackup:
    # This is Enterprise Package feature you need to have signed contract to use it
    # and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
    acceptEULA: true
    destination: "s3://your_bucket/folder"
    credentialsSecret:
      name: remote-storage-keys
      key: credentials
---
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMCluster
metadata:
  name: example-vmcluster-persistent
spec:
  retentionPeriod: "4"
  replicationFactor: 2
  vmstorage:
    replicaCount: 2
    vmBackup:
      # This is Enterprise Package feature you need to have signed contract to use it
      # and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
      acceptEULA: true
      destination: "s3://your_bucket/folder"
      credentialsSecret:
        name: remote-storage-keys
        key: credentials
``` 


NOTE: for cluster version operator adds suffix for destination: `"s3://your_bucket/folder"`, it becomes `"s3://your_bucket/folder/$(POD_NAME)"`. 
It's needed to make consistent backups for each storage node.

You can read more about backup configuration options and mechanics [here](https://docs.victoriametrics.com/vmbackup.html)

Possible configuration options for backup crd can be found at [link](https://docs.victoriametrics.com/operator/api.html#vmbackup)
 
 
## Restoring backups

There are several ways to restore with [vmrestore](https://docs.victoriametrics.com/vmrestore.html) or [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager.html).
 

### Manually mounting disk
You have to stop `VMSingle` by scaling it replicas to zero and manually restore data to the database directory.

Steps:
1. edit `VMSingle` CRD, set replicaCount: 0
2. wait until database stops
3. ssh to some server, where you can mount `VMSingle` disk and mount it manually
4. restore files with `vmrestore`
5. umount disk
6. edit `VMSingle` CRD, set replicaCount: 1
7. wait database start
 
### Using VMRestore init container

1. add init container with vmrestore command to `VMSingle` CRD, example:

    ```yaml
    apiVersion: operator.victoriametrics.com/v1beta1
    kind: VMSingle
    metadata:
     name: vmsingle-restored
     namespace: monitoring-system
    spec:
     initContainers:
     - name: vmrestore
       image: victoriametrics/vmrestore:latest
       volumeMounts:
       - mountPath: /victoria-metrics-data
         name: data
       - mountPath: /etc/vm/creds
         name: secret-remote-storage-keys
         readOnly: true
       args:
       - -storageDataPath=/victoria-metrics-data
       - -src=s3://your_bucket/folder/latest
       - -credsFilePath=/etc/vm/creds/credentials
     vmBackup:
       # This is Enterprise Package feature you need to have signed contract to use it
       # and accept the EULA https://victoriametrics.com/assets/VM_EULA.pdf
      acceptEULA: true
      destination: "s3://your_bucket/folder"
      extraArgs:
          runOnStart: "true"
      image:
          repository: victoriametrics/vmbackupmanager
          tag: v1.83.0-enterprise
      credentialsSecret:
       name: remote-storage-keys
       key: credentials
    ```
2. apply it, and db will be restored from s3

3. remove initContainers and apply crd.

### Using VMBackupmanager init container

Using VMBackupmanager restore in Kubernetes environment is described [here](https://docs.victoriametrics.com/vmbackupmanager.html#how-to-restore-in-kubernetes).
