---
weight: 80
title: Enterprise
description: "Enterprise features. Downsampling, backup automation, multi-tenancy, advanced security."
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

VictoriaMetrics, [VictoriaLogs](https://docs.victoriametrics.com/victorialogs/) and [VictoriaTraces](https://docs.victoriametrics.com/victoriatraces/) components are provided
in two kinds - [Community edition](https://victoriametrics.com/products/open-source/) and [Enterprise edition](https://victoriametrics.com/products/enterprise/).

The community components are open source and are free to use:

- See [VictoriaMetrics source code](https://github.com/VictoriaMetrics/VictoriaMetrics/) and [VictoriaMetrics license](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/LICENSE).
- See [VictoriaLogs source code](https://github.com/VictoriaMetrics/VictoriaLogs/) and [VictoriaLogs license](https://github.com/VictoriaMetrics/VictoriaLogs/blob/master/LICENSE).
- See [VictoriaTraces source code](https://github.com/VictoriaMetrics/VictoriaTraces/) and [VictoriaLogs license](https://github.com/VictoriaMetrics/VictoriaTraces/blob/master/LICENSE).

Both open-source and enterprise components are available at the following places:

- Binary executables are available at the corresponding releases pages:
  - [releases page for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
  - [releases page for VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs/releases/latest)
  - [releases page for VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaTraces/releases/latest)
- Docker images are available at [Docker Hub](https://hub.docker.com/u/victoriametrics) and [Quay](https://quay.io/organization/victoriametrics).

Enterprise executables and Docker images have `enterprise` suffix in their names and tags.

## Valid cases for VictoriaMetrics Enterprise

The use of Enterprise components is permitted in the following cases:

- Evaluation use in non-production setups. Pass the `-eula` command-line flag {% available_from "#" %} to accept the
  [ESA](https://victoriametrics.com/legal/esa/) as described [in these docs](#running-victoriametrics-enterprise).

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

- Stable releases with long-term support, which contains important bugfixes and security fixes. See [these docs](https://docs.victoriametrics.com/victorialogs/lts-releases/).
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

Enterprise components are available in the following forms:

- [Binary releases](https://docs.victoriametrics.com/victoriametrics/enterprise/#binary-releases)
- [Docker images](https://docs.victoriametrics.com/victoriametrics/enterprise/#docker-images)
- [Helm charts](https://docs.victoriametrics.com/victoriametrics/enterprise/#helm-charts)
- [Kubernetes operator](https://docs.victoriametrics.com/victoriametrics/enterprise/#kubernetes-operator)

### Binary releases

It is allowed to run Enterprise components in [cases listed here](https://docs.victoriametrics.com/victoriametrics/enterprise/#valid-cases-for-victoriametrics-enterprise).

Binary releases of Enterprise components are available at [the releases page for VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest),
[the releases page for VictoriaLogs](https://github.com/VictoriaMetrics/VictoriaLogs/releases/latest)
and [the releases page for VictoriaTraces](https://github.com/VictoriaMetrics/VictoriaTraces/releases/latest).
Enterprise binaries and packages have `enterprise` suffix in their names. For example, `victoria-metrics-linux-amd64-v1.155.0-enterprise.tar.gz`.

In order to run binary release of Enterprise component, please download the `*-enterprise.tar.gz` archive for your OS and architecture
from the corresponding releases page and unpack it. Then run the unpacked binary.

All the Enterprise components require specifying the `-eula` command-line flag {% available_from "#" %} to accept the [ESA](https://victoriametrics.com/legal/esa/).

LTS lines support `-eula` flag starting from releases v1.136.x and v1.148.x. If you need another Enterprise version, please [contact the Sales team](mailto:sales@victoriametrics.com).

For example, the following command runs VictoriaMetrics Enterprise binary:

```sh
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v1.155.0/victoria-metrics-linux-amd64-v1.155.0-enterprise.tar.gz
tar -xzf victoria-metrics-linux-amd64-v1.155.0-enterprise.tar.gz
./victoria-metrics-prod -eula
```

### Docker images

It is allowed to run Enterprise components in [cases listed here](https://docs.victoriametrics.com/victoriametrics/enterprise/#valid-cases-for-victoriametrics-enterprise).

Docker images for Enterprise components are available at [VictoriaMetrics Docker Hub](https://hub.docker.com/u/victoriametrics) and [VictoriaMetrics Quay](https://quay.io/organization/victoriametrics).
Enterprise docker images have `enterprise` suffix in their names. For example, `victoriametrics/victoria-metrics:v1.155.0-enterprise`.

In order to run Docker image of VictoriaMetrics Enterprise component, pass the `-eula` command-line flag {% available_from "#" %} to accept the [ESA](https://victoriametrics.com/legal/esa/).

For example, the following command runs VictoriaMetrics Enterprise Docker image:

```sh
docker run --name=victoria-metrics victoriametrics/victoria-metrics:v1.155.0-enterprise -eula
```

Example docker-compose configuration:

```yaml
version: "3.5"
services:
  victoriametrics:
    container_name: victoriametrics
    image: victoriametrics/victoria-metrics:v1.155.0-enterprise
    ports:
      - 8428:8428
    volumes:
      - vmdata:/storage
    command:
      - "-storageDataPath=/storage"
      - "-eula"
volumes:
  vmdata: {}
```

### Helm charts

It is allowed to run Enterprise components in cases [listed here](https://docs.victoriametrics.com/victoriametrics/enterprise/#valid-cases-for-victoriametrics-enterprise).

Helm charts for Enterprise components are available in our VictoriaMetrics [Helm repository](https://github.com/VictoriaMetrics/helm-charts).

In order to run Enterprise helm chart it is required to accept the [ESA](https://victoriametrics.com/legal/esa/) and adjust the image tag to the Enterprise one
as described in the [docker-images](https://docs.victoriametrics.com/victoriametrics/enterprise/#docker-images) section.

For example, the following `values` file for [VictoriaMetrics single-node chart](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-single)
is used to pass `-eula` command-line flag {% available_from "#" %} to accept the [ESA](https://victoriametrics.com/legal/esa/):

```yaml
server:
  image:
    tag: v1.155.0-enterprise
  extraArgs:
    eula: "true"
```

### Kubernetes operator

It is allowed to run Enterprise components in [cases listed here](https://docs.victoriametrics.com/victoriametrics/enterprise/#valid-cases-for-victoriametrics-enterprise).

Enterprise components can be deployed via [VictoriaMetrics operator](https://docs.victoriametrics.com/operator/).
In order to use Enterprise components it is required to accept the [ESA](https://victoriametrics.com/legal/esa/) and adjust the image tag to the enterprise one.

For example, the following custom resource for [VictoriaMetrics single-node](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/)
is used to pass `-eula` command-line flag {% available_from "#" %} to accept the [ESA](https://victoriametrics.com/legal/esa/):

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMSingle
metadata:
  name: example-vmsingle
spec:
  retentionPeriod: "1"
  image:
    tag: v1.155.0-enterprise
  extraArgs:
    eula: "true"
```

See the full list of the CRD specifications in the [Operator API](https://docs.victoriametrics.com/operator/api/).

### FIPS Compatibility

VictoriaMetrics Enterprise supports [FIPS 140-3](https://en.wikipedia.org/wiki/FIPS_140-3) compliant mode starting with version {{% available_from "v1.118.0" %}},
using the [Go FIPS 140-3 Cryptographic Module](https://go.dev/blog/fips140). This ensures all cryptographic operations use a validated FIPS module.

Builds are available for amd64 and arm64 architectures.

Example archive:

`victoria-metrics-linux-amd64-v1.155.0-enterprise.tar.gz`

Includes:

- `victoria-metrics-prod` (standard)
- `victoria-metrics-fips` (FIPS-compatible)

Example Docker image:

`victoriametrics/victoria-metrics:v1.155.0-enterprise-fips` – uses the FIPS-compatible binary and based on `scratch` image.
---

Section below contains backward-compatible anchors for links that were moved or renamed.

###### Updating the license key

License key is no longer needed to run VictoriaMetrics Enterprise. Please see [how to run VictoriaMetrics enterprise](#running-victoriametrics-enterprise).

###### What Happens to Licensed Components When a License Expires

License key is no longer needed to run VictoriaMetrics Enterprise. Please see [how to run VictoriaMetrics enterprise](#running-victoriametrics-enterprise).

###### Monitoring license expiration

License key is no longer needed to run VictoriaMetrics Enterprise. Please see [how to run VictoriaMetrics enterprise](#running-victoriametrics-enterprise).