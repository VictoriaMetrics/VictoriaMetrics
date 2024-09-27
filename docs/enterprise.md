---
weight: 99
title: Enterprise
menu:
  docs:
    identifier: vm-enterprise
    parent: 'victoriametrics'
    weight: 99
aliases:
- /enterprise.html
---
VictoriaMetrics components are provided in two kinds - [Community edition](https://victoriametrics.com/products/open-source/)
and [Enterprise edition](https://victoriametrics.com/products/enterprise/).

VictoriaMetrics community components are open source and are free to use - see [the source code](https://github.com/VictoriaMetrics/VictoriaMetrics/)
and [the license](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE).

VictoriaMetrics Enterprise components are available in binary form at [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
and at [docker hub](https://hub.docker.com/u/victoriametrics). Enterprise binaries and packages have `enterprise` suffix in their names.

## Valid cases for VictoriaMetrics Enterprise

The use of VictoriaMetrics Enterprise components is permitted in the following cases:

- Evaluation use in non-production setups. Please, request trial license [here](https://victoriametrics.com/products/enterprise/trial/)
  and then pass it via `-license` or `-licenseFile` command-line flags as described [in these docs](#running-victoriametrics-enterprise).

- Production use if you have a valid enterprise contract or valid permit from VictoriaMetrics company.
  Please contact us via [this page](https://victoriametrics.com/products/enterprise/) if you are interested in such a contract.

- [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/) is built on top of VictoriaMetrics Enterprise.

See [these docs](#running-victoriametrics-enterprise) for details on how to run VictoriaMetrics enterprise.

## VictoriaMetrics enterprise features

VictoriaMetrics Enterprise includes [all the features of the community edition](https://docs.victoriametrics.com/#prominent-features),
plus the following additional features:

- Stable releases with long-term support, which contains important bugfixes and security fixes. See [these docs](https://docs.victoriametrics.com/lts-releases/).
- First-class consulting and technical support provided by the core VictoriaMetrics dev team.
- [Monitoring of monitoring](https://victoriametrics.com/products/mom/) - this feature allows forecasting
  and preventing possible issues in VictoriaMetrics setups.
- [Enterprise security compliance](https://victoriametrics.com/security/).
- Prioritizing of feature requests from Enterprise customers.

On top of this, Enterprise package of VictoriaMetrics includes the following features:

- [Downsampling](https://docs.victoriametrics.com/#downsampling) - this feature allows reducing storage costs
  and increasing performance for queries over historical data.
- [Multiple retentions](https://docs.victoriametrics.com/#retention-filters) - this feature allows reducing storage costs
  by specifying different retentions for different datasets.
- [Automatic discovery of vmstorage nodes](https://docs.victoriametrics.com/cluster-victoriametrics/#automatic-vmstorage-discovery) -
  this feature allows updating the list of `vmstorage` nodes at `vminsert` and `vmselect` without the need to restart these services.
- [Anomaly Detection Service](https://docs.victoriametrics.com/anomaly-detection) - this feature allows automation and simplification of your alerting rules, covering [complex anomalies](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/) found in metrics data.
- [Backup automation](https://docs.victoriametrics.com/vmbackupmanager/).
- [Advanced per-tenant stats](https://docs.victoriametrics.com/pertenantstatistic/).
- [Advanced auth and rate limiter](https://docs.victoriametrics.com/vmgateway/).
- [Automatic issuing of TLS certificates](https://docs.victoriametrics.com/#automatic-issuing-of-tls-certificates).
- [mTLS for all the VictoriaMetrics components](https://docs.victoriametrics.com/#mtls-protection).
- [mTLS for communications between cluster components](https://docs.victoriametrics.com/cluster-victoriametrics/#mtls-protection).
- [mTLS-based request routing](https://docs.victoriametrics.com/vmauth/#mtls-based-request-routing).
- [Kafka integration](https://docs.victoriametrics.com/vmagent/#kafka-integration).
- [Google PubSub integration](https://docs.victoriametrics.com/vmagent/#google-pubsub-integration).
- [Multitenant support in vmalert](https://docs.victoriametrics.com/vmalert/#multitenancy).
- [Ability to read alerting and recording rules from Object Storage](https://docs.victoriametrics.com/vmalert/#reading-rules-from-object-storage).
- [Ability to filter incoming requests by IP at vmauth](https://docs.victoriametrics.com/vmauth/#ip-filters).

Contact us via [this page](https://victoriametrics.com/products/enterprise/) if you are interested in VictoriaMetrics Enterprise.

## Running VictoriaMetrics Enterprise

VictoriaMetrics Enterprise components are available in the following forms:

- [Binary releases](#binary-releases)
- [Docker images](#docker-images)
- [Helm charts](#helm-charts)
- [Kubernetes operator](#kubernetes-operator)

### Binary releases

It is allowed to run VictoriaMetrics Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Binary releases of VictoriaMetrics Enterprise are available [at the releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest).
Enterprise binaries and packages have `enterprise` suffix in their names. For example, `victoria-metrics-linux-amd64-v1.102.0-enterprise.tar.gz`.

In order to run binary release of VictoriaMetrics Enterprise component, please download the `*-enterprise.tar.gz` archive for your OS and architecture
from the [releases page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) and unpack it. Then run the unpacked binary.

All the VictoriaMetrics Enterprise components require specifying the following command-line flags:

* `-license` - this flag accepts VictoriaMetrics Enterprise license key, which can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/)
* `-licenseFile` - this flag accepts a path to file with VictoriaMetrics Enterprise license key,
  which can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/) . Use either `-license` or `-licenseFile`, but not both.
* `-licenseFile.reloadInterval` - specifies the interval for checking the license file for updates. The default value is 1 hour. If the license file is updated, the new license key is read from the file.
* `-license.forceOffline` - enables offline verification of VictoriaMetrics Enterprise license key. Contact us via [this page](https://victoriametrics.com/products/enterprise/)
  if you need license key, which can be verified offline without the need to connect to VictoriaMetrics license server.

For example, the following command runs VictoriaMetrics Enterprise binary with the Enterprise license
obtained at [this page](https://victoriametrics.com/products/enterprise/trial/):

```sh
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.102.0/victoria-metrics-linux-amd64-v1.102.0-enterprise.tar.gz
tar -xzf victoria-metrics-linux-amd64-v1.102.0-enterprise.tar.gz
./victoria-metrics-prod -license=BASE64_ENCODED_LICENSE_KEY
```

Alternatively, VictoriaMetrics Enterprise license can be stored in the file and then referred via `-licenseFile` command-line flag:

```sh
./victoria-metrics-prod -licenseFile=/path/to/vm-license
```

### Docker images

It is allowed to run VictoriaMetrics Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Docker images for VictoriaMetrics Enterprise are available [at VictoriaMetrics DockerHub](https://hub.docker.com/u/victoriametrics).
Enterprise docker images have `enterprise` suffix in their names. For example, `victoriametrics/victoria-metrics:v1.102.0-enterprise`.

In order to run Docker image of VictoriaMetrics Enterprise component, it is required to provide the license key via command-line
flag as described [here](#binary-releases).

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following command runs VictoriaMetrics Enterprise Docker image with the specified license key:

```sh
docker run --name=victoria-metrics victoriametrics/victoria-metrics:v1.102.0-enterprise -license=BASE64_ENCODED_LICENSE_KEY
```

Alternatively, the license code can be stored in the file and then referred via `-licenseFile` command-line flag:

```sh
docker run --name=victoria-metrics -v /vm-license:/vm-license  victoriametrics/victoria-metrics:v1.102.0-enterprise -licenseFile=/path/to/vm-license
```

Example docker-compose configuration:
```yaml
version: "3.5"
services:
  victoriametrics:
    container_name: victoriametrics
    image: victoriametrics/victoria-metrics:v1.102.0
    ports:
      - 8428:8428
    volumes:
      - vmdata:/storage
      - /vm-license:/vm-license
    command:
      - "-storageDataPath=/storage"
      - "-licenseFile=/vm-license"
volumes:
  vmdata: {}
```

The example assumes that the license file is stored at `/vm-license` on the host.

### Helm charts

It is allowed to run VictoriaMetrics Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Helm charts for VictoriaMetrics Enterprise components are available [here](https://github.com/VictoriaMetrics/helm-charts).

In order to run VictoriaMetrics Enterprise helm chart it is required to provide the license key via `license` value in `values.yaml` file
and adjust the image tag to the Enterprise one as described [here](#docker-images).

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following `values` file for [VictoriaMetrics single-node chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-single)
is used to provide key in plain-text:

```yaml
server:
  image:
    tag: v1.102.0-enterprise

license:
  key: {BASE64_ENCODED_LICENSE_KEY}
```

In order to provide key via existing secret, the following values file is used:

```yaml
server:
  image:
    tag: v1.102.0-enterprise

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
```sh
kubectl create secret generic vm-license --from-literal=license={BASE64_ENCODED_LICENSE_KEY}
```

Note that license key provided by using secret is mounted in a file. This allows to perform updates of the license without the need to restart the pod.

### Kubernetes operator

It is allowed to run VictoriaMetrics Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

VictoriaMetrics Enterprise components can be deployed via [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/).
In order to use Enterprise components it is required to provide the license key via `license` field and adjust the image tag to the enterprise one.

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following custom resource for [VictoriaMetrics single-node](https://docs.victoriametrics.com/single-server-victoriametrics/) 
is used to provide key in plain-text:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  retentionPeriod: "1"
  license:
    key: {BASE64_ENCODED_LICENSE_KEY}
  image:
    tag: v1.102.0-enterprise
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
    tag: v1.102.0-enterprise
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
```sh
kubectl create secret generic vm-license --from-literal=license={BASE64_ENCODED_LICENSE_KEY}
```

Note that license key provided by using secret is mounted in a file. This allows to perform updates of the license without the need to restart the pod.
See full list of CRD specifications [here](https://docs.victoriametrics.com/operator/api.html).

## Monitoring license expiration

All the VictoriaMetrics Enterprise components expose the following metrics at the `/metrics` page:

* `vm_license_expires_at` - license expiration date in unix timestamp format
* `vm_license_expires_in_seconds` - the number of seconds left until the license expires

Example alerts for [vmalert](https://docs.victoriametrics.com/vmalert/) based on these metrics:

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
