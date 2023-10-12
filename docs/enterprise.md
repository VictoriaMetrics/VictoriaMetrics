---
sort: 99
weight: 99
title: VictoriaMetrics Enterprise
menu:
  docs:
    parent: "victoriametrics"
    weight: 99
aliases:
- /enterprise.html
---

# VictoriaMetrics Enterprise

VictoriaMetrics components are provided in two kinds - [community edition](https://victoriametrics.com/products/open-source/)
and [enterprise edition](https://victoriametrics.com/products/enterprise/).

VictoriaMetrics community components are open source and are free to use - see [the source code](https://github.com/VictoriaMetrics/VictoriaMetrics/)
and [the license](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE).

The use of VictoriaMetrics enterprise components is permitted in the following cases:

- Evaluation use in non-production setups. Please, request trial license [here](https://victoriametrics.com/products/enterprise/).
  Trial key will be sent to your email after filling the trial form.
  Download components from usual places - [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) and [docker hub](https://hub.docker.com/u/victoriametrics).
  Enterprise binaries and packages have `enterprise` suffix in their names.

- Production use if you have a valid enterprise contract or valid permit from VictoriaMetrics company.
  [Contact us](mailto:sales@victoriametrics.com) if you need such contract.

- [Managed VictoriaMetrics](https://docs.victoriametrics.com/managed-victoriametrics/) is built on top of enterprise binaries of VictoriaMetrics.

See [running VictoriaMetrics enterprise](#running-victoriametrics-enterprise) for details on how to run VictoriaMetrics enterprise.

## VictoriaMetrics enterprise features

VictoriaMetrics enterprise includes [all the features of the community edition](https://docs.victoriametrics.com/#prominent-features),
plus the following additional features:

- [Downsampling](https://docs.victoriametrics.com/#downsampling) - this feature allows reducing storage costs
  and increasing performance for queries over historical data.
- [Multiple retentions](https://docs.victoriametrics.com/#retention-filters) - this feature allows reducing storage costs
  by specifying different retentions to different datasets.
- [Automatic discovery of vmstorage nodes](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#automatic-vmstorage-discovery) -
  this feature allows updating the list of `vmstorage` nodes at `vminsert` and `vmselect` without the need to restart these services.
- [Backup automation](https://docs.victoriametrics.com/vmbackupmanager.html).
- [Advanced per-tenant stats](https://docs.victoriametrics.com/PerTenantStatistic.html).
- [Advanced auth and rate limiter](https://docs.victoriametrics.com/vmgateway.html).
- [mTLS for cluster components](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#mtls-protection).
- [Kafka integration](https://docs.victoriametrics.com/vmagent.html#kafka-integration).
- [Multitenant support in vmalert](https://docs.victoriametrics.com/vmalert.html#multitenancy).
- [Ability to read alerting and recording rules from object storage](https://docs.victoriametrics.com/vmalert.html#reading-rules-from-object-storage).
- [Ability to filter incoming requests by IP at vmauth](https://docs.victoriametrics.com/vmauth.html#ip-filters).
- [Anomaly Detection Service](https://docs.victoriametrics.com/vmanomaly.html).

On top of this, enterprise package of VictoriaMetrics includes the following important Enterprise features:

- First-class consulting and technical support provided by the core dev team.
- [Monitoring of monitoring](https://victoriametrics.com/products/mom/) - this feature allows forecasting
  and preventing possible issues in VictoriaMetrics setups.
- [Enterprise security compliance](https://victoriametrics.com/security/).
- Prioritizing of feature requests from Enterprise customers.

[Contact us](mailto:info@victoriametrics.com) if you are interested in VictoriaMetrics enterprise.

## Running VictoriaMetrics enterprise

There are several ways to run VictoriaMetrics enterprise:
- [Binary releases](#binary-releases)
- [Docker images](#docker-images)
- [Helm charts](#helm-charts)
- [Kubernetes operator](#kubernetes-operator)

### Binary releases

Binary releases of VictoriaMetrics enterprise are available [here](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
Enterprise binaries and packages have `enterprise` suffix in their names. For example, `victoria-metrics-linux-amd64-v1.94.0-enterprise.tar.gz`.

In order to run binary release of VictoriaMetrics enterprise, download the release for your OS and unpack it.
Then run `victoria-metrics-enterprise` binary from the unpacked directory.

Before v1.94.0 all the enterprise apps required `-eula` command-line flag to be passed to them.
This flag acknowledges that your usage fits one of the cases listed above.

After v1.94.0 either `-eula` flag or the following flags are used:
```console
  -license string
     enterprise license key. This flag is available only in VictoriaMetrics enterprise. Documentation - https://docs.victoriametrics.com/enterprise.html, for more information, visit  https://victoriametrics.com/products/enterprise/ . To request a trial license, go to https://victoriametrics.com/products/enterprise/trial/
  -license.forceOffline
     enables offline license verification. License keys issued must support this feature. Contact our support team for license keys with offline check support.
  -licenseFile string
     path to file with enterprise license key. This flag is available only in VictoriaMetrics enterprise. Documentation - https://docs.victoriametrics.com/enterprise.html, for more information, visit  https://victoriametrics.com/products/enterprise/ . To request a trial license, go to https://victoriametrics.com/products/enterprise/trial/
```

For example, the following command runs `victoria-metrics-enterprise` binary with the specified license:
```console
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.94.0/victoria-metrics-linux-amd64-v1.94.0-enterprise.tar.gz
tar -xzf victoria-metrics-linux-amd64-v1.94.0-enterprise.tar.gz
./victoria-metrics-prod -license={VM_KEY_VALUE}
```

Alternatively, the license can be specified via `-license-file` command-line flag:
```console
./victoria-metrics-prod -license-file=/path/to/license/file
```

The license file must contain the license key.

### Docker images

Docker images for VictoriaMetrics enterprise are available [here](https://hub.docker.com/u/victoriametrics).
Enterprise docker images have `enterprise` suffix in their names. For example, `victoriametrics/victoria-metrics:v1.94.0-enteprise`.

In order to run docker image of VictoriaMetrics enterprise component it is required to provide the license key command-line
flag similar to the one described in the previous section.

For example, the following command runs [VictoriaMetrics single-node](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html) docker image with the specified license:
```console
docker run --name=victoria-metrics victoriametrics/victoria-metrics:v1.94.0-enteprise -license={VM_KEY_VALUE}
```

Alternatively, the license can be specified via `-license-file` command-line flag:
```console
docker run --name=victoria-metrics -v /vm-license:/vm-license  victoriametrics/victoria-metrics:v1.94.0-enteprise -license-file=/vm-license
```

Example docker-compose configuration:
```yaml
version: "3.5"
services:
  victoriametrics:
    container_name: victoriametrics
    image: victoriametrics/victoria-metrics:v1.94.0
    ports:
      - 8428:8428
    volumes:
      - vmdata:/storage
      - /vm-license:/vm-license
    command:
      - "--storageDataPath=/storage"
      - "--license-file=/vm-license"
volumes:
  vmdata: {}
```
Note, that example assumes that license file is located at `/vm-license` path on the host.

### Helm charts

Helm charts for VictoriaMetrics components are available [here](https://github.com/VictoriaMetrics/helm-charts).

In order to run VictoriaMetrics enterprise helm chart it is required to provide the license key via `license` value in `values.yaml` file 
and adjust the image tag to the enterprise one. 

For example, the following values file for [VictoriaMetrics single-node chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-single)
is used to provide key in plain-text:
```yaml
server:
  image:
    tag: v1.94.0-enterprise 

license:
  key: {VM_KEY_VALUE}
```

In order to provide key via existing secret, the following values file is used:
```yaml
server:
  image:
    tag: v1.94.0-enterprise 

license:
  secret:
    name: vm-license
    key: license
```

Example secret with license key:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vm-license
type: Opaque
data:
  license: {BASE64_ENCODED_LICENSE_KEY}
```

Or create secret via `kubectl`:
```console
kubectl create secret generic vm-license --from-literal=license={VM_KEY_VALUE}
```

### Kubernetes operator

VictoriaMetrics enterprise components can be deployed via [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/).
In order to use enterprise components it is required to provide the license key via `license` field and adjust the image tag to the enterprise one.

For example, the following custom resource for [VictoriaMetrics single-node](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html) 
is used to provide key in plain-text:
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  retentionPeriod: "1"
  license:
    key: "VM_KEY_VALUE"
  image:
    tag: v1.94.0-enterprise 
```

In order to provide key via existing secret, the following custom resource is used:
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  retentionPeriod: "1"
  license:
    keyRef:
      name: vm-license
      key: license
  image:
    tag: v1.94.0-enterprise 
```

Example secret with license key:
```yaml

apiVersion: v1
kind: Secret
metadata:
  name: vm-license
type: Opaque
data:
  license: {BASE64_ENCODED_LICENSE_KEY}
```

Or create secret via `kubectl`:
```console
kubectl create secret generic vm-license --from-literal=license={VM_KEY_VALUE}
```

See full list of CRD specifications [here](https://docs.victoriametrics.com/operator/api.html).

## Monitoring license expiration

All victoria metrics enterprise components expose the following metrics:
```
vm_license_expires_at 1694304000
vm_license_expires_in_seconds 1592720
```
Please, refer to monitoring section of each component for details on how to scrape these metrics.

`vm_license_expires_at` is the expiration date in unix timestamp format.
`vm_license_expires_in_seconds` is the amount of seconds until the license expires.

Example alerts for [vmalert](https://docs.victoriametrics.com/vmalert.html):
{% raw %}
```yaml
groups:
  - name: vm-license
    # note the `job` label and update accordingly to your setup
    rules:
      - alert: LicenseExpiresInLessThan30Days
        expr: vm_license_expires_in_seconds < 30 * 24 * 3600
        labels:
          severity: warning
        annotations:
          summary: "{{ $labels.job }} instance {{ $labels.instance }} license expires in less than 30 days"
          description: "{{ $labels.instance }} of job {{ $labels.job }} license expires in {{ $value | humanizeDuration }}. 
            Please make sure to update the license before it expires."

      - alert: LicenseExpiresInLessThan7Days
        expr: vm_license_expires_in_seconds < 7 * 24 * 3600
        labels:
          severity: critical
        annotations:
          summary: "{{ $labels.job }} instance {{ $labels.instance }} license expires in less than 7 days"
          description: "{{ $labels.instance }} of job {{ $labels.job }} license expires in {{ $value | humanizeDuration }}. 
            Please make sure to update the license before it expires."
```
{% endraw %}
