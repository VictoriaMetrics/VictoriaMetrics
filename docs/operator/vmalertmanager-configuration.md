---
sort: 13
---

# Managing configuration for VMAlertmanager

The operator generates a configuration file for `VMAlertmanager` based on user input at the definition of `CRD`.
Generated config stored at `Secret` created by the operator, it has the following name template `vmalertmanager-CRD_NAME-config`.
This configuration file is mounted at `VMAlertmanager` `Pod`. A special side-car container tracks its changes and sends config-reload signals to `alertmanager` container.

## Using secret

Basically, you can use the global configuration defined at manually created `Secret`. This `Secret` must be created before `VMAlertmanager`.

Name of the `Secret` must be defined at `VMAlertmanager` `spec.configSecret` option.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: vmalertmanager-example-alertmanager
  labels:
    app: vm-operator
type: Opaque
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    route:
      receiver: 'webhook'
    receivers:
      - name: 'webhook'
        webhook_configs:
          - url: 'http://alertmanagerwh:30500/'
---
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-alertmanager
spec:
  replicaCount: 2
  configSecret: vmalertmanager-example-alertmanager
```

## Using inline raw config

Also, if there is no secret data at configuration, or you just want to redefine some global variables for `alertmanager`.
You can define configuration at `spec.configRawYaml` section of `VMAlertmanager` configuration:
```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanager
metadata:
  name: example-alertmanager
spec:
  replicaCount: 2
  configRawYaml: |
    global:
       resolve_timeout: 5m
    route:
      receiver: 'default'
      group_interval: 5m
      repeat_interval: 12h
    receivers:
    - name: 'default'
```

If both `configSecret` and `configRawYaml` are defined, only configuration from `configRawYaml` will be used. Values from `configRawYaml` will be ignored.

## Using VMAlertmanagerConfig
`VMAlertmanagerConfig` allows delegating notification configuration to the kubernetes cluster users.
The application owner may configure notifications by defining it at `VMAlertmanagerConfig`.
With the combination of `VMRule` and `VMServiceScrape` it allows delegating configuration observability to application owners, and uses popular `GitOps` practice.

Operator combines `VMAlertmanagerConfigs` into a single configuration file for `VMAlertmanager`.

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanagerConfig
metadata:
  name: example-email-web
  namespace: production
spec:
  route:
    receiver: email
    group_interval: 1m
    routes:
      - receiver: email
        matchers:
          - {severity =~ "warning|critical", app_name = "blog"}
  receivers:
    - name: email
      email_configs:
        - to: some-email@example.com
          from: alerting@example.com
          smarthost: example.com:25
          text: ALARM
```

#### Special Case
VMAlertmanagerConfig has enforced namespace matcher.
Alerts must have a proper namespace label, with the same value as name of namespace for VMAlertmanagerConfig.
It can be disabled, by setting the following value to the VMAlertmanager: spec.disableNamespaceMatcher: true.

## behavior without provided config

If no configuration is provided, operator configures stub configuration with blackhole route.