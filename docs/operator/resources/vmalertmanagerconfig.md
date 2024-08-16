---
weight: 4
title: VMAlertmanagerConfig
menu:
  docs:
    identifier: operator-cr-vmalertmanagerconfig
    parent: operator-cr
    weight: 4
aliases:
  - /operator/resources/vmalertmanagerconfig/
  - /operator/resources/vmalertmanagerconfig/index.html
---
The `VMAlertmanagerConfig` provides way to configure [VMAlertmanager](https://docs.victoriametrics.com/operator/resources/vmalertmanager)
configuration with CRD. It allows to define different configuration parts, which will be merged by operator into config.

It behaves like other config parts - `VMServiceScrape` and etc.

Read [Usage](#usage) and [Special case](#special-case) before using.

## Specification

You can see the full actual specification of the `VMAlertmanagerConfig` resource in
the **[API docs -> VMAlertmanagerConfig](https://docs.victoriametrics.com/operator/api#vmalertmanagerconfig)**.

Also, you can check out the [examples](#examples) section.

## Validation

 The operator performs validation of `VMAlertmanagerConfig`. In case of any misconfiguration it adds corresponding validation fail message into `status.lastSyncError` field and updates `status.status` field to `failed` status.

 For example `VMAlertmanagerConfig` without required fields:

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanagerConfig
metadata:
  name: invalid-config
  namespace: default
spec:
  receivers:
  - name: blackhole
  - name: pagerduty
    pagerduty_configs:
    - url: http://example
  route:
    group_by:
    - alertname
    receiver: blackhole
    routes:
    - matchers:
      - alertname="pd"
      receiver: pagerduty
status:
  lastErrorParentAlertmanagerName: default/example-alertmanager
  lastSyncError: 'receiver at idx=2 is invalid: at idx=0 pagerduty_configs one of
    ''routing_key'' or ''service_key'' must be configured'
  lastSyncErrorTimestamp: 1722950290
  status: failed
```

## Usage

`VMAlertmanagerConfig` allows delegating notification configuration to the kubernetes cluster users.
The application owner may configure notifications by defining it at `VMAlertmanagerConfig`.

With the combination of `VMRule` and `VMServiceScrape` it allows delegating configuration observability to application owners, and uses popular `GitOps` practice.

Operator combines `VMAlertmanagerConfig`s into a single configuration file for `VMAlertmanager`.

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

It can be disabled, by setting the following value to the VMAlertmanager: `spec.disableNamespaceMatcher: true`.

## Examples

```yaml
apiVersion: operator.victoriametrics.com/v1beta1
kind: VMAlertmanagerConfig
metadata:
  name: example
  namespace: default
spec:
  inhibit_rules:
    - equals: []
      target_matchers: []
      source_matchers: []
  route:
    routes:
      - receiver: webhook
        continue: true
    receiver: email
    group_by: []
    continue: false
    matchers:
      - job = "alertmanager"
    group_wait: 30s
    group_interval: 45s
    repeat_interval: 1h
  mute_time_intervals:
    - name: base
      time_intervals:
        - times:
            - start_time: ""
              end_time: ""
          weekdays: []
          days_of_month: []
          months: []
          years: []
  receivers:
      email_configs: []
      webhook_configs:
        - url: http://some-other-wh
      pagerduty_configs: []
      pushover_configs: []
      slack_configs: []
      opsgenie_configs: []
      victorops_configs: []
      wechat_configs: []
```
