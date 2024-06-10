---
sort: 13
weight: 13
title: VMSingle
menu:
  docs:
    parent: "operator-custom-resources"
    weight: 13
aliases:
  - /operator/resources/vmsingle.html
---

# VMSingle

`VMSingle` represents database for storing metrics.
The `VMSingle` CRD declaratively defines a [single-node VM](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html)
installation to run in a Kubernetes cluster.

For each `VMSingle` resource, the Operator deploys a properly configured `Deployment` in the same namespace.
The VMSingle `Pod`s are configured to mount an empty dir or `PersistentVolumeClaimSpec` for storing data.
Deployment update strategy set to [recreate](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#recreate-deployment).
No more than one replica allowed.

For each `VMSingle` resource, the Operator adds `Service` and `VMServiceScrape` in the same namespace prefixed with name from `VMSingle.metadata.name`.

## Specification

You can see the full actual specification of the `VMSingle` resource in the **[API docs -> VMSingle](../api.md#vmsingle)**.

If you can't find necessary field in the specification of the custom resource,
see [Extra arguments section](./README.md#extra-arguments).

Also, you can check out the [examples](#examples) section.

## High availability

`VMSingle` doesn't support high availability by default, for such purpose
use [`VMCluster`](./vmcluster.md) instead or duplicate the setup.

## Version management

To set `VMSingle` version add `spec.image.tag` name from [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  image:
    repository: victoriametrics/victoria-metrics
    tag: v1.93.4
    pullPolicy: Always
  # ...
```

Also, you can specify `imagePullSecrets` if you are pulling images from private repo:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  image:
    repository: victoriametrics/victoria-metrics
    tag: v1.93.4
    pullPolicy: Always
  imagePullSecrets:
    - name: my-repo-secret
# ...
```

## Resource management

You can specify resources for each `VMSingle` resource in the `spec` section of the `VMSingle` CRD.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: vmsingle-resources-example
spec:
    # ...
    resources:
        requests:
          memory: "64Mi"
          cpu: "250m"
        limits:
          memory: "128Mi"
          cpu: "500m"
    # ...
```

If these parameters are not specified, then,
by default all `VMSingle` pods have resource requests and limits from the default values of the following [operator parameters](../configuration.md):

- `VM_VMSINGLEDEFAULT_RESOURCE_LIMIT_MEM` - default memory limit for `VMSingle` pods,
- `VM_VMSINGLEDEFAULT_RESOURCE_LIMIT_CPU` - default memory limit for `VMSingle` pods,
- `VM_VMSINGLEDEFAULT_RESOURCE_REQUEST_MEM` - default memory limit for `VMSingle` pods,
- `VM_VMSINGLEDEFAULT_RESOURCE_REQUEST_CPU` - default memory limit for `VMSingle` pods.

These default parameters will be used if:

- `VM_VMSINGLEDEFAULT_USEDEFAULTRESOURCES` is set to `true` (default value),
- `VMSingle` CR doesn't have `resources` field in `spec` section.

Field `resources` in `VMSingle` spec have higher priority than operator parameters.

If you set `VM_VMSINGLEDEFAULT_USEDEFAULTRESOURCES` to `false` and don't specify `resources` in `VMSingle` CRD,
then `VMSingle` pods will be created without resource requests and limits.

Also, you can specify requests without limits - in this case default values for limits will not be used.

## Enterprise features

VMSingle supports features from [VictoriaMetrics Enterprise](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise):

- [Downsampling](https://docs.victoriametrics.com/#downsampling) 
- [Multiple retentions / Retention filters](https://docs.victoriametrics.com/#retention-filters)
- [Backup automation](https://docs.victoriametrics.com/vmbackupmanager.html)

For using Enterprise version of [vmsingle](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html)
you need to change version of `VMSingle` to version with `-enterprise` suffix using [Version management](#version-management).

All the enterprise apps require `-eula` command-line flag to be passed to them.
This flag acknowledges that your usage fits one of the cases listed on [this page](https://docs.victoriametrics.com/enterprise.html#victoriametrics-enterprise).
So you can use [extraArgs](./README.md#extra-arguments) for passing this flag to `VMSingle`.

### Downsampling

After that you can pass [Downsampling](https://docs.victoriametrics.com/#downsampling)
flag to `VMSingle` with [extraArgs](./README.md#extra-arguments) too.

Here are complete example for [Downsampling](https://docs.victoriametrics.com/#downsampling):
 
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: vmsingle-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmsingle
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmsingle enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
    
    # using enterprise features: Downsampling
    # more details about downsampling you can read on https://docs.victoriametrics.com/#downsampling
    downsampling.period: 30d:5m,180d:1h,1y:6h,2y:1d

  # ...other fields...
```

### Retention filters

The same method is used to enable retention filters - here are complete example for [Retention filters](https://docs.victoriametrics.com/#retention-filters).

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: vmsingle-ent-example
spec:
  # enabling enterprise features
  image:
    # enterprise version of vmsingle
    tag: v1.93.5-enterprise
  extraArgs:
    # should be true and means that you have the legal right to run a vmsingle enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    eula: true
    
    # using enterprise features: Retention filters
    # more details about retention filters you can read on https://docs.victoriametrics.com/#retention-filters
    retentionFilter: '{team="juniors"}:3d,{env=~"dev|staging"}:30d'

  # ...other fields...
```

### Backup automation

You can check [vmbackupmanager documentation](https://docs.victoriametrics.com/vmbackupmanager.html) for backup automation.
It contains a description of the service and its features. This section covers vmbackumanager integration in vmoperator.

`VMSingle` has built-in backup configuration, it uses `vmbackupmanager` - proprietary tool for backups.
It supports incremental backups (hourly, daily, weekly, monthly) with popular object storages (aws s3, google cloud storage).

Here is a complete example for backup configuration:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:

  vmBackup:
    # should be true and means that you have the legal right to run a vmsingle enterprise
    # that can either be a signed contract or an email with confirmation to run the service in a trial period
    # https://victoriametrics.com/legal/esa/
    acceptEULA: true

    # using enterprise features: Backup automation
    # more details about backup automation you can read on https://docs.victoriametrics.com/vmbackupmanager.html      
    destination: "s3://your_bucket/folder"
    credentialsSecret:
      name: remote-storage-keys
      key: credentials

  # ...other fields...

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
``` 

You can read more about backup configuration options and mechanics [here](https://docs.victoriametrics.com/vmbackupmanager.html)

Possible configuration options for backup crd can be found at [link](../api.md#vmbackup)

#### Restoring backups

There are several ways to restore with [vmrestore](https://docs.victoriametrics.com/vmrestore.html) or [vmbackupmanager](https://docs.victoriametrics.com/vmbackupmanager.html).

##### Manually mounting disk

You have to stop `VMSingle` by scaling it replicas to zero and manually restore data to the database directory.

Steps:

1. Edit `VMSingle` CRD, set `replicaCount: 0`
1. Wait until database stops
1. SSH to some server, where you can mount `VMSingle` disk and mount it manually
1. Restore files with `vmrestore`
1. Umount disk
1. Edit `VMSingle` CRD, set `replicaCount: 1`
1. Wait database start

##### Using VMRestore init container

1. Add init container with `vmrestore` command to `VMSingle` CRD, example:
    ```yaml
    apiVersion: operator.victoriametrics.com/v1beta1
    kind: VMSingle
    metadata:
      name: example-vmsingle
    spec:
    
      vmBackup:
        # should be true and means that you have the legal right to run a vmsingle enterprise
        # that can either be a signed contract or an email with confirmation to run the service in a trial period
        # https://victoriametrics.com/legal/esa/
        acceptEULA: true
    
        # using enterprise features: Backup automation
        # more details about backup automation you can read on https://docs.victoriametrics.com/vmbackupmanager.html      
        destination: "s3://your_bucket/folder"
        credentialsSecret:
          name: remote-storage-keys
          key: credentials
          
      extraArgs:
        runOnStart: "true"
        
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
    
      # ...other fields...
    ```
1. Apply it, and db will be restored from S3
1. Remove `initContainers` and apply CRD.

Note that using `VMRestore` will require adjusting `src` for each pod because restore will be handled per-pod.

##### Using VMBackupmanager init container

Using VMBackupmanager restore in Kubernetes environment is described [here](https://docs.victoriametrics.com/vmbackupmanager.html#how-to-restore-in-kubernetes).

Advantages of using `VMBackupmanager` include:

- Automatic adjustment of `src` for each pod when backup is requested
- Graceful handling of case when no restore is required - `VMBackupmanager` will exit with successful status code and won't prevent pod from starting

## Examples

```yaml
kind: VMSingle
metadata:
  name: vmsingle-example
spec:
  retentionPeriod: "12"
  removePvcAfterDelete: true
  storage:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 50Gi
  extraArgs:
    dedup.minScrapeInterval: 60s
  resources:
    requests:
      memory: 500Mi
      cpu: 500m
    limits:
      memory: 10Gi
      cpu: 5
```
