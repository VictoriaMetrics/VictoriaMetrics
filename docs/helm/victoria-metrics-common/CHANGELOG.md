# CHANGELOG for `victoria-metrics-common` helm-chart

## Next release

- TODO

## 0.0.15

**Release date:** 2024-10-11

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Display compatibility error message

## 0.0.14

**Release date:** 2024-10-04

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed openshift compatibility templates

## 0.0.13

**Release date:** 2024-09-16

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Do not use image variant if custom image tag is set in `vm.image` template
- Support multiple license flag styles, which are different for vmanomaly and other services

## 0.0.12

**Release date:** 2024-09-16

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Add enterprise to existing variant if enterprise enabled
- Added `vm.enterprise.disabled` template to check if enterprise license is disabled
- Use `service.servicePort` as a port source if flag is not set in `vm.url`

## 0.0.11

**Release date:** 2024-09-11

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added ability to pass extra prefix for `vm.managed.fullname`

## 0.0.10

**Release date:** 2024-09-10

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed protocol extraction with TLS enabled
- Typo fixes
- use appkey as `app` label by default
- support multiple service naming styles for `vm.service`

## 0.0.9

**Release date:** 2024-09-02

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Allow `appKey` argument to be a list to support deeply nested objects
- Added `vm.namespace`, which returns `namespaceOverride` or `global.namespaceOverride` or `Release.Namespace` as a default
- Added `vm.managed.fullname`, which returns default fullname prefixed by `appKey`
- Added `vm.plain.fullname`, which returns default fullname suffixed by `appKey`

## 0.0.8

**Release date:** 2024-08-29

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added `vm.service` for unified service name generation
- Added `vm.url` to construct service base url
- Added `vm.name` for chart name
- Added `vm.fullname` which is actively used in resource name construction
- Added `vm.chart` to construct chart name label value
- Added `vm.labels` for common labels
- Added `vm.sa` for service account name
- Added `vm.release` for release name
- Added `vm.selectorLabels` for common selector labels

## 0.0.7

**Release date:** 2024-08-27

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Support short and long args flags in `vm.args`
- Updated `vm.enterprise.only` error message

## 0.0.6

**Release date:** 2024-08-27

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Changed structure of `vm.args` template output
- Removed `eula` support

## 0.0.5

**Release date:** 2024-08-26

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Fixed `vm.enterprise.only` template to check if at least one of both global.licence.eula and .Values.license.eula are defined
- Convert `vm.args` bool `true` values to flags without values

## 0.0.4

**Release date:** 2024-08-26

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Updated `vm.probe.*` templates to remove Helm 3.14 restriction.
- Added `vm.args` template for cmd args generation

## 0.0.3

**Release date:** 2024-08-25

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Moved license templates from other charts `vm.license.volume`, `vm.license.mount`, `vm.license.flag`
- Moved `vm.compatibility.renderSecurityContext` template
- Fixed a case, when null is passed to a `.Values.global`. See [this issue](https://github.com/VictoriaMetrics/helm-charts/issues/1296)

## 0.0.2

**Release date:** 2024-08-23

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added `vm.port.from.flag` template to extract port from cmd flag listen address.

## 0.0.1

**Release date:** 2024-08-15

![Helm: v3](https://img.shields.io/static/v1?label=Helm&message=v3&color=informational&logo=helm)

- Added `vm.enterprise.only` template to fail rendering if required license arguments weren't set.
- Added `vm.image` template that introduces common chart logic of how to build image name from application variables.
- Added `vm.ingress.port` template to render properly tngress port configuration depending on args type.
- Added `vm.probe.*` templates to render probes params consistently across all templates.
