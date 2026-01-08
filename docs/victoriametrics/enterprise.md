---
weight: 80
title: Enterprise
menu:
  docs:
    identifier: vm-enterprise
    parent: 'victoriametrics'
    weight: 80
tags:
  - metrics
  - enterprise
aliases:
- /enterprise.html
- /enterprise/index.html
- /enterprise/
---

VictoriaMetrics and [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) components are provided
in two kinds - [Community edition](https://victoriametrics.com/products/open-source/) and [Enterprise edition](https://victoriametrics.com/products/enterprise/).

VictoriaMetrics and VictoriaLogs community components are open source and are free to use:

- See [VictoriaMetrics source code](https://github.com/VictoriaMetrics/VictoriaMetrics/) and [VictoriaMetrics license](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE).
- See [VictoriaLogs source code](https://github.com/VictoriaMetrics/VictoriaLogs/) and [VictoriaLogs license](https://github.com/VictoriaMetrics/VictoriaLogs/blob/master/LICENSE).

Enterprise components of VictoriaMetrics and VictoriaLogs are available at the following places:

- Binary executables are available at [the releases page for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
  and [the release page for VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs/releases/latest).
- Docker images are available at [Docker Hub](https://hub.docker.com/u/victoriametrics) and [Quay](https://quay.io/organization/victoriametrics).

Enterprise executables and Docker images have `enterprise` suffix in their names and tags.

## Valid cases for VictoriaMetrics Enterprise

The use of Enterprise components of VictoriaMetrics and VictoriaLogs is permitted in the following cases:

- Evaluation use in non-production setups. Please, request a [trial license](https://victoriametrics.com/products/enterprise/trial/)
  and then pass it via `-license` or `-licenseFile` command-line flags as described [in these docs](#running-victoriametrics-enterprise).

- Production use if you have a valid enterprise contract or valid permit from VictoriaMetrics company.
  Please contact us via [this page](https://victoriametrics.com/products/enterprise/) if you are interested in such a contract.

- [VictoriaMetrics Cloud](https://docs.victoriametrics.com/victoriametrics-cloud/) is built on top of VictoriaMetrics Enterprise.

See [these docs](#running-victoriametrics-enterprise) for details on how to run Enterprise components of VictoriaMetrics and VictoriaLogs.

## VictoriaMetrics Enterprise features

VictoriaMetrics Enterprise includes [all the features of the community edition](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prominent-features),
plus the following additional features:

- Stable releases with long-term support, which contains important bugfixes and security fixes. See [these docs](https://docs.victoriametrics.com/victoriametrics/lts-releases/).
- First-class consulting and technical support provided by the core VictoriaMetrics dev team.
- [Monitoring of monitoring](https://victoriametrics.com/products/mom/) - this feature allows forecasting
  and preventing possible issues in VictoriaMetrics setups.
- [Enterprise security compliance](https://victoriametrics.com/security/).
- Prioritizing of feature requests from Enterprise customers.

On top of this, Enterprise package of VictoriaMetrics includes the following features:

- [Downsampling](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#downsampling) - this feature allows reducing storage costs
  and increasing performance for queries over historical data.
- [Multiple retentions](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#retention-filters) - this feature allows reducing storage costs
  by specifying different retentions for different datasets.
- [Automatic discovery of vmstorage nodes](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#automatic-vmstorage-discovery) -
  this feature allows updating the list of `vmstorage` nodes at `vminsert` and `vmselect` without the need to restart these services.
- [Anomaly Detection Service](https://docs.victoriametrics.com/anomaly-detection/) - this feature allows automation and simplification of your alerting rules, covering [complex anomalies](https://victoriametrics.com/blog/victoriametrics-anomaly-detection-handbook-chapter-2/) found in metrics data.
- [Backup automation](https://docs.victoriametrics.com/victoriametrics/vmbackupmanager/).
- [Advanced per-tenant stats](https://docs.victoriametrics.com/victoriametrics/pertenantstatistic/).
- [Query execution stats](https://docs.victoriametrics.com/victoriametrics/query-stats/).
- [Advanced auth and rate limiter](https://docs.victoriametrics.com/victoriametrics/vmgateway/).
- [Automatic issuing of TLS certificates](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#automatic-issuing-of-tls-certificates).
- [mTLS for all the VictoriaMetrics components](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#mtls-protection).
- [mTLS for communications between cluster components](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#mtls-protection).
- [mTLS-based request routing](https://docs.victoriametrics.com/victoriametrics/vmauth/#mtls-based-request-routing).
- [Kafka integration](https://docs.victoriametrics.com/victoriametrics/integrations/kafka/).
- [Google PubSub integration](https://docs.victoriametrics.com/victoriametrics/integrations/pubsub/).
- [Multitenant support in vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/#multitenancy).
- [Ability to read alerting and recording rules from Object Storage](https://docs.victoriametrics.com/victoriametrics/vmalert/#reading-rules-from-object-storage).
- [Ability to filter incoming requests by IP at vmauth](https://docs.victoriametrics.com/victoriametrics/vmauth/#ip-filters).
- [FIPS 140-3 compatible builds](https://docs.victoriametrics.com/victoriametrics/enterprise/#fips-compatibility).

Contact us via [this page](https://victoriametrics.com/products/enterprise/) if you are interested in VictoriaMetrics Enterprise.

## VictoriaLogs Enterprise features

VictoriaLogs enterprise includes [all the features of the community edition](https://docs.victoriametrics.com/victorialogs/),
plus the following additional features:

- First-class consulting and technical support provided by the core VictoriaMetrics dev team.
- [Monitoring of monitoring](https://victoriametrics.com/products/mom/) - this feature allows forecasting
  and preventing possible issues in VictoriaLogs setups.
- [Enterprise security compliance](https://victoriametrics.com/security/).
- Prioritizing of feature requests from Enterprise customers.

On top of this, Enterprise package of VictoriaLogs includes the following features:

- [Automatic issuing of TLS certificates](https://docs.victoriametrics.com/victorialogs/#automatic-issuing-of-tls-certificates).
- [mTLS for all the VictoriaLogs components](https://docs.victoriametrics.com/victorialogs/#mtls).
- [mTLS for communications between cluster components](https://docs.victoriametrics.com/victorialogs/cluster/#mtls).

Contact us via [this page](https://victoriametrics.com/products/enterprise/) if you are interested in VictoriaLogs Enterprise.

## Running VictoriaMetrics Enterprise

Enterprise components of VictoriaMetrics and VictoriaLogs are available in the following forms:

- [Binary releases](#binary-releases)
- [Docker images](#docker-images)
- [Helm charts](#helm-charts)
- [Kubernetes operator](#kubernetes-operator)

### Binary releases

It is allowed to run VictoriaMetrics and VictoriaLogs Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Binary releases of Enterprise components are available at [the releases page for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
and [the releases page for VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs/releases/latest).
Enterprise binaries and packages have `enterprise` suffix in their names. For example, `victoria-metrics-linux-amd64-v1.133.0-enterprise.tar.gz`.

In order to run binary release of Enterprise component, please download the `*-enterprise.tar.gz` archive for your OS and architecture
from the corresponding releases page and unpack it. Then run the unpacked binary.

All the Enterprise components of VictoriaMetrics and VictoriaLogs require specifying the following command-line flags:

- `-license` - this flag accepts VictoriaMetrics Enterprise license key, which can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/)
- `-licenseFile` - this flag accepts a path to file with VictoriaMetrics Enterprise license key,
  which can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/) . Use either `-license` or `-licenseFile`, but not both.
- `-licenseFile.reloadInterval` - specifies the interval for checking the license file for updates. The default value is 1 hour. If the license file is updated, the new license key is read from the file.
- `-license.forceOffline` - enables offline verification of VictoriaMetrics Enterprise license key. Contact us via [this page](https://victoriametrics.com/products/enterprise/)
  if you need license key, which can be verified offline without the need to connect to VictoriaMetrics license server.

For example, the following command runs VictoriaMetrics Enterprise binary with the Enterprise license
obtained at [this page](https://victoriametrics.com/products/enterprise/trial/):

```sh
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.133.0/victoria-metrics-linux-amd64-v1.133.0-enterprise.tar.gz
tar -xzf victoria-metrics-linux-amd64-v1.133.0-enterprise.tar.gz
./victoria-metrics-prod -license=BASE64_ENCODED_LICENSE_KEY
```

Alternatively, VictoriaMetrics Enterprise license can be stored in the file and then referred via `-licenseFile` command-line flag:

```sh
./victoria-metrics-prod -licenseFile=/path/to/vm-license
```

### Docker images

It is allowed to run VictoriaMetrics and VictoriaLogs Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Docker images for Enterprise components are available at [VictoriaMetrics Docker Hub](https://hub.docker.com/u/victoriametrics) and [VictoriaMetrics Quay](https://quay.io/organization/victoriametrics).
Enterprise docker images have `enterprise` suffix in their names. For example, `victoriametrics/victoria-metrics:v1.133.0-enterprise`.

In order to run Docker image of VictoriaMetrics Enterprise component, it is required to provide the license key via the command-line
flag as described in the [binary-releases](#binary-releases) section.

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following command runs VictoriaMetrics Enterprise Docker image with the specified license key:

```sh
docker run --name=victoria-metrics victoriametrics/victoria-metrics:v1.133.0-enterprise -license=BASE64_ENCODED_LICENSE_KEY
```

Alternatively, the license code can be stored in the file and then referred via `-licenseFile` command-line flag:

```sh
docker run --name=victoria-metrics -v /vm-license:/vm-license  victoriametrics/victoria-metrics:v1.133.0-enterprise -licenseFile=/path/to/vm-license
```

Example docker-compose configuration:

```yaml
version: "3.5"
services:
  victoriametrics:
    container_name: victoriametrics
    image: victoriametrics/victoria-metrics:v1.133.0
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

It is allowed to run VictoriaMetrics and VictoriaLogs Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Helm charts for Enterprise components are available in our VictoriaMetrics [Helm repository](https://github.com/VictoriaMetrics/helm-charts).

In order to run Enterprise helm chart it is required to provide the license key via `license` value in `values.yaml` file
and adjust the image tag to the Enterprise one as described in the [docker-images](#docker-images) section.

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following `values` file for [VictoriaMetrics single-node chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-single)
is used to provide the license key in plain-text:

```yaml
server:
  image:
    tag: v1.133.0-enterprise

license:
  key: {BASE64_ENCODED_LICENSE_KEY}
```

In order to provide the license key via existing secret, the following values file is used:

```yaml
server:
  image:
    tag: v1.133.0-enterprise

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

Note that the license key provided by using secret is mounted in a file. This allows to perform updates of the license without the need to restart the pod.

### Kubernetes operator

It is allowed to run VictoriaMetrics and VictoriaLogs Enterprise components in [cases listed here](#valid-cases-for-victoriametrics-enterprise).

Enterprise components can be deployed via [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/).
In order to use Enterprise components it is required to provide the license key via `license` field and adjust the image tag to the enterprise one.

Enterprise license key can be obtained at [this page](https://victoriametrics.com/products/enterprise/trial/).

For example, the following custom resource for [VictoriaMetrics single-node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
is used to provide the license key in plain-text:

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
    tag: v1.133.0-enterprise
```

In order to provide the license key via an existing secret, the following custom resource is used:

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
    tag: v1.133.0-enterprise
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

Note that the license key provided by using a secret is mounted as a file. This allows updates to the license without the need to restart the pod.
See the full list of the CRD specifications in the [Operator API](https://docs.victoriametrics.com/operator/api/).

### Updating the license key

Updating the license key for VictoriaMetrics and VictoriaLogs Enterprise components depends on the way
the license key is provided to the component:
- If the license key is provided via `-license` command-line flag, then the component should be restarted
  with the new license key.
- If the license key is provided via `-licenseFile` command-line flag, then the license file should be updated
  with the new license key. The component will automatically reload the license file at the interval specified
  via `-licenseFile.reloadInterval` command-line flag (1 hour by default) and apply the new license key without the need to restart the component.
- If the license key is provided via Kubernetes secret, then the secret should be updated
  with the new license key. The component will automatically reload the license file at the interval specified
  via `-licenseFile.reloadInterval` command-line flag (1 hour by default) and apply the new license key without the need to restart the component.
- If the license key is provided via Helm chart value, then the corresponding `values.yaml` file
  should be updated with the new license key and then the Helm chart should be upgraded via `helm upgrade` command.
  This will restart the component with the new license key.
- If the license key is provided via Kubernetes operator custom resource, then the corresponding custom resource
  should be updated with the new license key. This will restart the component with the new license key.

### FIPS Compatibility

VictoriaMetrics Enterprise supports [FIPS 140-3](https://en.wikipedia.org/wiki/FIPS_140-3) compliant mode starting with version {{% available_from "v1.118.0" %}},
using the [Go FIPS 140-3 Cryptographic Module](https://go.dev/blog/fips140). This ensures all cryptographic operations use a validated FIPS module.

Builds are available for amd64 and arm64 architectures.

Example archive:

`victoria-metrics-linux-amd64-v1.133.0-enterprise.tar.gz`

Includes:

- `victoria-metrics-prod` (standard)
- `victoria-metrics-fips` (FIPS-compatible)

Example Docker image:

`victoriametrics/victoria-metrics:v1.133.0-enterprise-fips` – uses the FIPS-compatible binary and based on `scratch` image.

## Monitoring license expiration

All the VictoriaMetrics and VictoriaLogs Enterprise components expose the following metrics at the `/metrics` page:

- `vm_license_expires_at` - license expiration date in unix timestamp format
- `vm_license_expires_in_seconds` - the number of seconds left until the license expires

Example alerts for [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/) based on these metrics:

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
