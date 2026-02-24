# Security Policy

## Supported Versions

The following versions of VictoriaMetrics receive regular security fixes:

| Version                                                                        | Supported          |
|--------------------------------------------------------------------------------|--------------------|
| [Latest release](https://docs.victoriametrics.com/victoriametrics/changelog/)  | :white_check_mark: |
| [LTS releases](https://docs.victoriametrics.com/victoriametrics/lts-releases/) | :white_check_mark: |
| other releases                                                                 | :x:                |

See [this page](https://victoriametrics.com/security/) for more details.

## Software Bill of Materials (SBOM)

Every VictoriaMetrics container image published to
[Docker Hub](https://hub.docker.com/u/victoriametrics)
and [Quay.io](https://quay.io/organization/victoriametrics)
includes an [SPDX](https://spdx.dev/) SBOM attestation
generated automatically by BuildKit during
`docker buildx build`.

To inspect the SBOM for an image:

```sh
docker buildx imagetools inspect \
  docker.io/victoriametrics/victoria-metrics:latest \
  --format "{{ json .SBOM }}"
```

To scan an image using its SBOM attestation with
[Trivy](https://github.com/aquasecurity/trivy):

```sh
trivy image --sbom-sources oci \
  docker.io/victoriametrics/victoria-metrics:latest
```

## Reporting a Vulnerability

Please report any security issues to <security@victoriametrics.com>
