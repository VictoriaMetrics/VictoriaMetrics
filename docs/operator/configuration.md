---
sort: 4
weight: 4
title: Configuration
menu:
  docs:
    parent: "operator"
    weight: 4
aliases:
  - /operator/configuration.html
---

# Configuration

Operator configured by env variables, list of it can be found 
on [Variables](./vars.md) page.

It defines default configuration options, like images for components, timeouts, features.

In addition, the operator has a special startup mode for outputting all variables, their types and default values.
For instance, with this mode you can know versions of VM components, which are used by default: 

```console
./operator --printDefaults

# This application is configured via the environment. The following environment variables can be used:
# 
# KEY                                                          TYPE                              DEFAULT                                                           REQUIRED    DESCRIPTION
# VM_USECUSTOMCONFIGRELOADER                                   True or False                     false                                                                                                                                                                                   
# VM_CUSTOMCONFIGRELOADERIMAGE                                 String                            victoriametrics/operator:config-reloader-v0.32.0                                                                                                       
# VM_VMALERTDEFAULT_IMAGE                                      String                            victoriametrics/vmalert                                                       
# VM_VMALERTDEFAULT_VERSION                                    String                            v1.93.3                                                                                                                                                 
# VM_VMALERTDEFAULT_USEDEFAULTRESOURCES                        True or False                     true                                                                          
# VM_VMALERTDEFAULT_RESOURCE_LIMIT_MEM                         String                            500Mi                                                                         
# VM_VMALERTDEFAULT_RESOURCE_LIMIT_CPU                         String                            200m                                                                                                                                                                                                                            
# ...
```

You can choose output format for variables with `--printFormat` flag, possible values: `json`, `yaml`, `list` and `table` (default):

```console
.operator --printDefaults --printFormat=json

# {
#     'VM_USECUSTOMCONFIGRELOADER': 'false',
#     'VM_CUSTOMCONFIGRELOADERIMAGE': 'victoriametrics/operator:config-reloader-v0.32.0',
#     'VM_VMALERTDEFAULT_IMAGE': 'victoriametrics/vmalert',
#     'VM_VMALERTDEFAULT_VERSION': 'v1.93.3',
# ...
#     'VM_FORCERESYNCINTERVAL': '60s',
#     'VM_ENABLESTRICTSECURITY': 'true'
# }
```

## Conversion of prometheus-operator objects

You can read detailed instructions about configuring prometheus-objects conversion in [this document](./migration.md).

## Helm-charts

In [helm-charts](https://github.com/VictoriaMetrics/helm-charts) some important configuration parameters are implemented as separate flags in `values.yaml`:

### victoria-metrics-k8s-stack

For possible values refer to [parameters](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-k8s-stack#parameters).

Also, checkout [here possible ENV variables](./vars.md) to configure operator behaviour.
ENV variables can be set in the `victoria-metrics-operator.env` section.

```yaml
# values.yaml

victoria-metrics-operator:
  image:
    # -- Image repository
    repository: victoriametrics/operator
    # -- Image tag
    tag: v0.35.0
    # -- Image pull policy
    pullPolicy: IfNotPresent

  # -- Tells helm to remove CRD after chart remove
  cleanupCRD: true
  cleanupImage:
    repository: gcr.io/google_containers/hyperkube
    tag: v1.18.0
    pullPolicy: IfNotPresent

  operator:
    # -- By default, operator converts prometheus-operator objects.
    disable_prometheus_converter: false
    # -- Compare-options and sync-options for prometheus objects converted by operator for properly use with ArgoCD
    prometheus_converter_add_argocd_ignore_annotations: false
    # -- Enables ownership reference for converted prometheus-operator objects,
    # it will remove corresponding victoria-metrics objects in case of deletion prometheus one.
    enable_converter_ownership: false
    # -- By default, operator creates psp for its objects.
    psp_auto_creation_enabled: true
    # -- Enables custom config-reloader, bundled with operator.
    # It should reduce  vmagent and vmauth config sync-time and make it predictable.
    useCustomConfigReloader: false

  # -- extra settings for the operator deployment. full list Ref: [https://github.com/VictoriaMetrics/operator/blob/master/vars.md](https://github.com/VictoriaMetrics/operator/blob/master/vars.md)
  env:
    # -- default version for vmsingle
    - name: VM_VMSINGLEDEFAULT_VERSION
      value: v1.43.0
    # -- container registry name prefix, e.g. docker.io
    - name: VM_CONTAINERREGISTRY
      value: ""
    # -- image for custom reloader (see the useCustomConfigReloader parameter)
    - name: VM_CUSTOMCONFIGRELOADERIMAGE
      value: victoriametrics/operator:config-reloader-v0.32.0

  # By default, the operator will watch all the namespaces
  # If you want to override this behavior, specify the namespace it needs to watch separated by a comma.
  # Ex: my_namespace1,my_namespace2
  watchNamespace: ""

  # Count of operator instances (can be increased for HA mode)
  replicaCount: 1

  # -- VM operator log level
  # -- possible values: info and error.
  logLevel: "info"

  # -- Resource object
  resources:
    {}
    # limits:
    #   cpu: 120m
    #   memory: 320Mi
    # requests:
    #   cpu: 80m
    #   memory: 120Mi
```

### victoria-metrics-operator

For possible values refer to [parameters](https://github.com/VictoriaMetrics/helm-charts/tree/master/charts/victoria-metrics-operator#parameters).

Also, checkout [here possible ENV variables](./vars.md) to configure operator behaviour.
ENV variables can be set in the `env` section.

```yaml
# values.yaml

image:
  # -- Image repository
  repository: victoriametrics/operator
  # -- Image tag
  tag: v0.35.0
  # -- Image pull policy
  pullPolicy: IfNotPresent

operator:
  # -- By default, operator converts prometheus-operator objects.
  disable_prometheus_converter: false
  # -- Compare-options and sync-options for prometheus objects converted by operator for properly use with ArgoCD
  prometheus_converter_add_argocd_ignore_annotations: false
  # -- Enables ownership reference for converted prometheus-operator objects,
  # it will remove corresponding victoria-metrics objects in case of deletion prometheus one.
  enable_converter_ownership: false
  # -- By default, operator creates psp for its objects.
  psp_auto_creation_enabled: true
  # -- Enables custom config-reloader, bundled with operator.
  # It should reduce  vmagent and vmauth config sync-time and make it predictable.
  useCustomConfigReloader: false

# -- extra settings for the operator deployment. full list Ref: [https://github.com/VictoriaMetrics/operator/blob/master/vars.md](https://github.com/VictoriaMetrics/operator/blob/master/vars.md)
env:
  # -- default version for vmsingle
  - name: VM_VMSINGLEDEFAULT_VERSION
    value: v1.43.0
  # -- container registry name prefix, e.g. docker.io
  - name: VM_CONTAINERREGISTRY
    value: ""
  # -- image for custom reloader (see the useCustomConfigReloader parameter)
  - name: VM_CUSTOMCONFIGRELOADERIMAGE
    value: victoriametrics/operator:config-reloader-v0.32.0

# By default, the operator will watch all the namespaces
# If you want to override this behavior, specify the namespace it needs to watch separated by a comma.
# Ex: my_namespace1,my_namespace2
watchNamespace: ""

# Count of operator instances (can be increased for HA mode)
replicaCount: 1

# -- VM operator log level
# -- possible values: info and error.
logLevel: "info"

# -- Resource object
resources:
  {}
  # limits:
  #   cpu: 120m
  #   memory: 320Mi
  # requests:
  #   cpu: 80m
  #   memory: 120Mi
```

## Namespaced mode

By default, the operator will watch all namespaces, but it can be configured to watch only specific namespace.

If you want to override this behavior, specify the namespace:

- in the `WATCH_NAMESPACE` environment variable.
- in the `watchNamespace` field in the `values.yaml` file of helm-charts.

The operator supports only single namespace for watching.

You can find example of RBAC manifests for single-namespace mode in 
[this file](https://github.com/VictoriaMetrics/operator/blob/master/config/examples/operator_rbac_for_single_namespace.yaml).

## Monitoring of cluster components

By default, operator creates [VMServiceScrape](./resources/vmservicescrape.md) 
object for each component that it manages.

You can disable this behaviour with `VM_DASABLESELFSERVICASCRAPECREATION` environment variable:

```shell
VM_DASABLESELFSERVICASCRAPECREATION=false
```

Also, you can override default configuration for self-scraping with `ServiceScrapeSpec` field in each deployable resource 
(`vmcluster/select`, `vmcluster/insert`, `vmcluster/storage`, `vmagent`, `vmalert`, `vmalertmanager`, `vmauth`, `vmsingle`):

## CRD Validation

Operator supports validation admission webhook [docs](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)

It checks resources configuration and returns errors to caller before resource will be created at kubernetes api.
This should reduce errors and simplify debugging.

Validation hooks at operator side must be enabled with flags:

```console
./operator
    --webhook.enable
    # optional configuration for certDir and tls names.
    --webhook.certDir=/tmp/k8s-webhook-server/serving-certs/
    --webhook.keyName=tls.key
    --webhook.certName=tls.crt
```

You have to mount correct certificates at give directory.
It can be simplified with cert-manager and kustomize command:

```console
kustomize build config/deployments/webhook/
```

### Requirements

- Valid certificate with key must be provided to operator
- Valid CABundle must be added to the `ValidatingWebhookConfiguration`

### Useful links

- [k8s admission webhooks](https://banzaicloud.com/blog/k8s-admission-webhooks/)
- [olm webhooks](https://docs.openshift.com/container-platform/4.5/operators/user/olm-webhooks.html)
