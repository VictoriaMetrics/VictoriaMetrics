# CHANGELOG

## Next release

### Fixes

### Features

<a name="v0.36.0"></a>
## [v0.36.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.36.0) - 23 Aug 2023

### Breaking changes

- **[vmalert](https://docs.victoriametrics.com/operator/api.html#vmalert): Field `OAuth2` was renamed to `oauth2` due to compatibility issue. If you defined `OAuth2` with below fields in vmalert objects using operator before v0.36.0, these fields must be reapplied with new tag `oauth2` after upgrading. See [this issue](https://github.com/VictoriaMetrics/operator/issues/522) and [this PR](https://github.com/VictoriaMetrics/operator/pull/689) for details.**
  - **Affected fields:**
    - **`VMAlert.spec.datasource.OAuth2` -> `VMAlert.spec.datasource.oauth2`,**
    - **`VMAlert.spec.notifier.OAuth2` -> `VMAlert.spec.notifier.oauth2`,**
    - **`VMAlert.spec.notifiers[].OAuth2` -> `VMAlert.spec.notifiers[].oauth2`,**
    - **`VMAlert.spec.remoteRead.OAuth2` -> `VMAlert.spec.remoteRead.oauth2`,**
    - **`VMAlert.spec.remoteWrite.OAuth2` -> `VMAlert.spec.remoteWrite.oauth2`,**

- **[vmalert](https://docs.victoriametrics.com/operator/api.html#vmalert): Field `bearerTokenFilePath` was renamed to `bearerTokenFile` due to compatibility issue. If you defined `bearerTokenFilePath` with below fields in vmalert objects using operator before v0.36.0, these fields must be reapplied with new tag `bearerTokenFile` after upgrading. See [this issue](https://github.com/VictoriaMetrics/operator/issues/522) and [this PR](https://github.com/VictoriaMetrics/operator/pull/688/) for details.**
  - **Affected fields:**
    - **`VMAlert.spec.datasource.bearerTokenFilePath` --> `VMAlert.spec.datasource.bearerTokenFile`,**
    - **`VMAlert.spec.notifier.bearerTokenFilePath` --> `VMAlert.spec.notifier.bearerTokenFile`,**
    - **`VMAlert.spec.notifiers[].bearerTokenFile` --> `VMAlert.spec.notifiers[].bearerTokenFile`,**
    - **`VMAlert.spec.remoteRead.bearerTokenFilePath` --> `VMAlert.spec.remoteRead.bearerTokenFile`,**
    - **`VMAlert.spec.remoteWrite.bearerTokenFilePath` --> `VMAlert.spec.remoteWrite.bearerTokenFile`.**

### Fixes

- operator set resource requests for config-reloader container by default. See [this PR](https://github.com/VictoriaMetrics/operator/pull/695/) for details.
- fix `attachMetadata` value miscovert for scrape objects. See [this issue](https://github.com/VictoriaMetrics/operator/issues/697) and [this PR](https://github.com/VictoriaMetrics/operator/pull/698) for details.
- fix volumeClaimTemplates change check for objects that generate statefulset, like vmstorage, vmselect. Before, the statefulset won't be recreated if additional `claimTemplates` object changed. See [this issue](https://github.com/VictoriaMetrics/operator/issues/507) and [this PR](https://github.com/VictoriaMetrics/operator/pull/719) for details.
- [vmalert](https://docs.victoriametrics.com/operator/api.html#vmalert): fix `tlsCAFile` argument value generation when using secret or configMap. See [this issue](https://github.com/VictoriaMetrics/operator/issues/699) and [this PR](https://github.com/VictoriaMetrics/operator/issues/699) for details.
- [vmalertmanager](https://docs.victoriametrics.com/operator/api.html#vmalertmanager): fix default request memory and apply default resources if not set. See [this issue](https://github.com/VictoriaMetrics/operator/issues/706) and [this PR](https://github.com/VictoriaMetrics/operator/pull/710) for details.
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): fix missing additional VolumeClaimTemplates when using `ClaimTemplates` under StatefulMode.

### Features

- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): add [example config](https://github.com/VictoriaMetrics/operator/blob/master/config/examples/vmagent_stateful_with_sharding.yaml) for vmagent statefulmode.
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent)/[vmsingle](https://docs.victoriametrics.com/operator/api.html#vmsingle): adapt new features in streaming aggregation:
  - support `streamAggr.dropInput`, see [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4243) for details;
  - support list for `match` parameter, see [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4635) for details;
  - support `staleness_interval`, see [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4667) for details.
- [vmcluster](https://docs.victoriametrics.com/operator/api.html#vmagent): add [example config](https://github.com/VictoriaMetrics/operator/blob/master/config/examples/vmcluster_with_additional_claim.yaml) for cluster with custom storage claims.
- [vmrule](https://docs.victoriametrics.com/operator/api.html#vmrule): support `update_entries_limit` field in rules, refer to [alerting rules](https://docs.victoriametrics.com/vmalert.html#alerting-rules). See [this PR](https://github.com/VictoriaMetrics/operator/pull/691) for details.
- [vmrule](https://docs.victoriametrics.com/operator/api.html#vmrule): support `keep_firing_for` field in rules, refer to [alerting rules](https://docs.victoriametrics.com/vmalert.html#alerting-rules). See [this PR](https://github.com/VictoriaMetrics/operator/pull/711) for details.
- [vmoperator parameters](https://docs.victoriametrics.com/operator/vars.html): Add option `VM_ENABLESTRICTSECURITY` and enable strict security context by default. See [this issue](https://github.com/VictoriaMetrics/operator/issues/637), [this](https://github.com/VictoriaMetrics/operator/pull/692/) and [this](https://github.com/VictoriaMetrics/operator/pull/712) PR for details.
- [vmoperator parameters](https://docs.victoriametrics.com/operator/vars.html): change option `VM_PSPAUTOCREATEENABLED` default value from `true` to `false` cause PodSecurityPolicy already got deprecated since [kubernetes v1.25](https://kubernetes.io/docs/reference/using-api/deprecation-guide/#psp-v125). See [this pr](https://github.com/VictoriaMetrics/operator/pull/726) for details.

[Changes][v0.36.0]

<a name="v0.35.1"></a>
## [v0.35.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.35.1) - 12 Jul 2023

### Fixes

- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): fixes regression with remoteWrite authorization (basicAuth/token). When `UseCustomConfigReloader` option was set, operator incorrectly rendered mounts for `vmagent` container. https://github.com/VictoriaMetrics/operator/commit/f2b8cf701a33f91cef19848c857fd6efb7db59dd

[Changes][v0.35.1]


<a name="v0.35.0"></a>
## [v0.35.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.35.0) - 03 Jul 2023

### Fixes

* [vmuser](https://docs.victoriametrics.com/operator/api.html#vmuser): fix vmselect url_map in vmuser. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/655). Thanks [@Haleygo](https://github.com/Haleygo)
* [vmalert](https://docs.victoriametrics.com/operator/api.html#vmalert): correctly set default port for vmauth components discovery. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/658). Thanks [@Haleygo](https://github.com/Haleygo)
* [vmuser](https://docs.victoriametrics.com/operator/api.html#vmuser): remove rate limit on delete. In https://github.com/VictoriaMetrics/operator/pull/672. Thanks [@Haleygo](https://github.com/Haleygo)
* [vmcluster](https://docs.victoriametrics.com/operator/api.html#vmcluster): fix spec change check. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/677). Thanks [@Haleygo](https://github.com/Haleygo)
* Correctly publish multi-arch release at https://github.com/VictoriaMetrics/operator/pull/681. Thanks [@Haleygo](https://github.com/Haleygo)

### Features

* [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): add validation when generate static scrape config. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/677). Thanks [@Haleygo](https://github.com/Haleygo)
* [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): add validation for slack receiver url. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/661). Thanks [@Haleygo](https://github.com/Haleygo)
* [vmauth](https://docs.victoriametrics.com/operator/api.html#vmauth)/[vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): implement configuration initiation for custom config reloader. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/619). Thanks [@Haleygo](https://github.com/Haleygo)
* add more generators  Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/668
* [vmsingle](https://docs.victoriametrics.com/operator/api.html#vmsingle): add status field. See [this issue for details](https://github.com/VictoriaMetrics/operator/issues/670). Thanks [@Haleygo](https://github.com/Haleygo)

[Changes][v0.35.0]


<a name="v0.34.1"></a>
## [v0.34.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.34.1) - 29 May 2023

### Fixes

- [vmcluster](https://docs.victoriametrics.com/operator/api.html#vmcluster): fail fast on misconfigured or missing kubernetes pods. It should prevent rare bug with cascade pod deletion. See this [issue](https://github.com/VictoriaMetrics/operator/issues/643) for details
- [vmauth](https://docs.victoriametrics.com/operator/api.html#vmauth)/[vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): correctly renders initConfig image with global container registry domain. See this [issue](https://github.com/VictoriaMetrics/operator/issues/654) for details.
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): correctly set RBAC permissions for single namespace mode and custom config reloader image. See this [issue](https://github.com/VictoriaMetrics/operator/issues/653) for details.

[Changes][v0.34.1]


<a name="v0.34.0"></a>
## [v0.34.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.34.0) - 24 May 2023

### Breaking changes

* **[Operator]: allows to properly run operator with single namespace. It changes default behavior with WATCH_NAMESPACE param is set.  Operator will no longer make any calls for cluster wide resources and create only single namespace config for `VMAgent`. https://github.com/VictoriaMetrics/operator/issues/641**

### Fixes

- [vmnodescrape](https://docs.victoriametrics.com/operator/api.html#vmnodescrape): fixed selectors for Exists and NotExists operators with empty label Thanks [@Amper](https://github.com/Amper) in https://github.com/VictoriaMetrics/operator/pull/646
- [vmrule](https://docs.victoriametrics.com/operator/api.html#vmrule): Add config for vmrule in validating webhook Thanks in https://github.com/VictoriaMetrics/operator/pull/650
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): skips misconfigured objects with missed secret references: https://github.com/VictoriaMetrics/operator/issues/648
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): correctly renders initContainer for configuration download: https://github.com/VictoriaMetrics/operator/issues/649

### Features

- [vmalertmanager](https://docs.victoriametrics.com/operator/api.html#vmalertmanager): Bump alertmanager to v0.25.0 Thanks [@tamcore](https://github.com/tamcore) in https://github.com/VictoriaMetrics/operator/pull/636
- [vmcluster](https://docs.victoriametrics.com/operator/api.html#vmcluster): added `clusterNativePort` field to VMSelect/VMInsert for multi-level cluster setup ([#634](https://github.com/VictoriaMetrics/operator/issues/634)) Thanks [@Amper](https://github.com/Amper) in https://github.com/VictoriaMetrics/operator/pull/639
- [vmrule](https://docs.victoriametrics.com/operator/api.html#vmrule): add notifierHeader field in vmrule spec Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/622
- [vmpodscrape](https://docs.victoriametrics.com/operator/api.html#vmpodscrape): adds FilterRunning option as prometheus does in https://github.com/VictoriaMetrics/operator/pull/640
- [vmauth](https://docs.victoriametrics.com/operator/api.html#vmauth): adds latest features in https://github.com/VictoriaMetrics/operator/pull/642

[Changes][v0.34.0]


<a name="v0.33.0"></a>
## [v0.33.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.33.0) - 19 Apr 2023

### Fixes

- [vmalert](https://docs.victoriametrics.com/operator/api.html#vmalert): skip bad rules and improve logging for rules exceed max configmap size https://github.com/VictoriaMetrics/operator/commit/bb754d5c20bb371a197cd6ff5afac1ba86a4d92b
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): fixed error with headers in VMAlertmanagerConfig.Receivers.EmailConfigs.Headers unmarshalling. Thanks [@Amper](https://github.com/Amper) in https://github.com/VictoriaMetrics/operator/pull/610
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): fixed keepInput setting for streaming aggregation. Thanks [@Amper](https://github.com/Amper) in https://github.com/VictoriaMetrics/operator/pull/618
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): fix webhook config maxAlerts not work. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/625
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): Remove single quotes from remote write headers. Thanks [@axelsccp](https://github.com/axelsccp) in https://github.com/VictoriaMetrics/operator/pull/613
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): fix parse route error and some comments. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/630
- [vmuser](https://docs.victoriametrics.com/operator/api.html#vmuser): properly removes finalizers for objects https://github.com/VictoriaMetrics/operator/commit/8f10113920a353f21fbcc8637076905f2e57bb34

### Features

- [vmalertmanager](https://docs.victoriametrics.com/operator/api.html#vmalertmanager): add option to disable route continue enforce. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/621
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): support set require_tls to false. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/624
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): add sanity check. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/627
- Makefile: bump Alpine base image to latest v3.17.3. Thanks [@denisgolius](https://github.com/denisgolius) in https://github.com/VictoriaMetrics/operator/pull/628
- [vmalertmanagerconfig](https://docs.victoriametrics.com/operator/api.html#vmalertmanagerconfig): support sound field in pushover config. Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/631
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent)/[vmauth](https://docs.victoriametrics.com/operator/api.html#vmauth): download initial config with initContainer https://github.com/VictoriaMetrics/operator/commit/612e7c8f40659731e7938ef9556eb088c67eb4b7

[Changes][v0.33.0]


<a name="v0.32.1"></a>
## [v0.32.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.32.1) - 16 Mar 2023

### Fixes

- config: fixes typo at default vm apps version https://github.com/VictoriaMetrics/operator/issues/608
- [vmsingle](https://docs.victoriametrics.com/operator/api.html#vmsingle): conditionally adds stream aggregation config https://github.com/VictoriaMetrics/operator/commit/4a0ca54113afcde439ca4c77e22d3ef1c0d36241

[Changes][v0.32.1]


<a name="v0.32.0"></a>
## [v0.32.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.32.0) - 15 Mar 2023

### Fixes

- security: builds docker image with latest `alpine` base image and go `v1.20`.

### Features

- [vmauth](https://docs.victoriametrics.com/operator/api.html#vmauth): automatically configures `proxy-protocol` client and `reloadAuthKey` for `config-reloader` container. https://github.com/VictoriaMetrics/operator/commit/611819233bf595a4dbd04b07d7be24b7e994379c
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): adds `scrapeTimeout` global configuration for `VMAgent` https://github.com/VictoriaMetrics/operator/commit/d1d5024c6befa0961f8d56c82a0554935a4b1878
- [vmagent](https://docs.victoriametrics.com/operator/api.html#vmagent): adds [streaming aggregation](https://docs.victoriametrics.com/stream-aggregation.html) for `remoteWrite` targets https://github.com/VictoriaMetrics/operator/commit/b8baa6c2b72bdda64ebfcc9c3d86d846cd9b3c98 Thanks [@Amper](https://github.com/Amper)
- [vmsingle](https://docs.victoriametrics.com/operator/api.html#vmsingle): adds [streaming aggregation](https://docs.victoriametrics.com/stream-aggregation.html) as global configuration for database https://github.com/VictoriaMetrics/operator/commit/b8baa6c2b72bdda64ebfcc9c3d86d846cd9b3c98 Thanks [@Amper](https://github.com/Amper)

[Changes][v0.32.0]


<a name="v0.31.0"></a>
## [v0.31.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.31.0) - 02 Mar 2023

### Fixes

- hpa: Fix hpa object since v2beta deprecated in 1.26+ Thanks [@Haleygo](https://github.com/Haleygo) in https://github.com/VictoriaMetrics/operator/pull/593
- api: adds missing generated client CRD entities https://github.com/VictoriaMetrics/operator/issues/599

### Features

- [vmalertmanager](https://docs.victoriametrics.com/operator/api.html#vmalertmanager): Add support of vmalertmanager.spec.templates and autoreload dirs for templates and configmaps  thanks [@Amper](https://github.com/Amper) https://github.com/VictoriaMetrics/operator/issues/590 https://github.com/VictoriaMetrics/operator/issues/592
- [vmalertmanager](https://docs.victoriametrics.com/operator/api.html#vmalertmanager): Add support "%SHARD_NUM%" placeholder for vmagent sts/deployment  Thanks [@Amper](https://github.com/Amper) https://github.com/VictoriaMetrics/operator/issues/508

[Changes][v0.31.0]


<a name="v0.30.4"></a>
## [v0.30.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.30.4) - 27 Jan 2023

### Fixes

- vmalertmanagerconfig: properly build `name` setting for  `mute_time_intervals`. It must be uniq https://github.com/VictoriaMetrics/operator/commit/4db1c89abd5360a119e68874d51c27872265acb6
- vmcluster: add `dedupMinScrape` only if replicationFactor > 1. It must improve overall cluster perfomance. Thanks [@hagen1778](https://github.com/hagen1778) https://github.com/VictoriaMetrics/operator/commit/837d6e71c6298e5a44c3f73f85235560aec4ee60
- controllers/vmalert: do not delete annotations from created secret. Thanks [@zoetrope](https://github.com/zoetrope) https://github.com/VictoriaMetrics/operator/pull/588

### Features

- vmalertmanagerconfig: adds location, active_time_intervals https://github.com/VictoriaMetrics/operator/commit/66ee8e544f480be386a4a126a6163599ed338705

[Changes][v0.30.4]


<a name="v0.30.3"></a>
## [v0.30.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.30.3) - 16 Jan 2023

### Fixes

- controllers: pass correct selector labels for pvc resize function https://github.com/VictoriaMetrics/operator/commit/e7b57dd73b4fd8dc37b42b7ad7bf5a4d3483caae
- controllers: kubernetes 1.26+ deprecates v2 autoscaling, add api check for it https://github.com/VictoriaMetrics/operator/issues/583

[Changes][v0.30.3]


<a name="v0.30.2"></a>
## [v0.30.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.30.2) - 12 Jan 2023

### Upgrade notes

* It's recommend to upgrade for this release when `vmagent.spec.statefulMode` is used.

### Fixes

- controllers/vmagent: fixes degradation for vmagent statefulMode https://github.com/VictoriaMetrics/operator/commit/6c26786db2ba0b2e85277418e588eac79e886b6e

[Changes][v0.30.2]


<a name="v0.30.1"></a>
## [v0.30.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.30.1) - 09 Jan 2023

### Fixes

- controllers/vmalert: correctly filter notifiers for namespace selector https://github.com/VictoriaMetrics/operator/commit/2290729fcc1b3775141b54ff71a295bd29457fbd
- dependency: upgrade deps for fs-notify  https://github.com/VictoriaMetrics/operator/pull/576 Thanks [@yanggangtony](https://github.com/yanggangtony)
- controllers/options: fixes incorrectly used flags at options https://github.com/VictoriaMetrics/operator/commit/eac040c947ab4821bf6eb0eeae22b9b2d02b938c
- controllers/self-serviceScrape: prevents matching for auto-created serviceScrapes https://github.com/VictoriaMetrics/operator/issues/578
- controllers/vmauth: fixes missing ows for serviceScrape https://github.com/VictoriaMetrics/operator/issues/579

### Features

- adds `/ready` and `/health` api endpoints for probes https://github.com/VictoriaMetrics/operator/commit/b74d103998547fae5e69966bb68eddd08ae1ac00
- controllers/concurrency: introduce new setting for reconcilation concurrency `controller.maxConcurrentReconciles` https://github.com/VictoriaMetrics/operator/commit/e8bbf9159cd61257d11e515fa77510ab2444a557 https://github.com/VictoriaMetrics/operator/issues/575
- api/relabelConfig: adds missing `if`, `labels` and `match` actions https://github.com/VictoriaMetrics/operator/commit/93c9e780981ceb6869ee2953056a9bd3b6e6eae7

[Changes][v0.30.1]


<a name="v0.30.0"></a>
## [v0.30.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.30.0) - 29 Dec 2022

### Fixes

- vmalertmanagerconfig: fixes duplicates at configuration https://github.com/VictoriaMetrics/operator/issues/554
- controllers: correctly set current and update revisions for statefulset  https://github.com/VictoriaMetrics/operator/issues/547
- controller/factory: fix typo in urlRelabelingName Thanks [@dmitryk-dk](https://github.com/dmitryk-dk) in https://github.com/VictoriaMetrics/operator/pull/572
- controllers/vmalert: fixes notifier selector incorrect matching https://github.com/VictoriaMetrics/operator/issues/569
- controllers/cluster: fixes HPA labels for vminsert https://github.com/VictoriaMetrics/operator/issues/562

### Features

- adds Scaling subresource for `VMAgent`.  https://github.com/VictoriaMetrics/operator/issues/570
- add optional namespace label matcher to inhibit rule thanks [@okzheng](https://github.com/okzheng) in https://github.com/VictoriaMetrics/operator/pull/559
- provide crds yaml as release asset Thanks [@avthart](https://github.com/avthart) in https://github.com/VictoriaMetrics/operator/pull/566
- child labels filtering https://github.com/VictoriaMetrics/operator/pull/571
- controllers/vmalert: adds oauth2 and bearer auth for remote dbs in https://github.com/VictoriaMetrics/operator/pull/573

[Changes][v0.30.0]


<a name="v0.29.2"></a>
## [v0.29.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.29.2) - 17 Nov 2022

### Fixes

- vmalertmanagerconfig: fixes duplicates at configuration https://github.com/VictoriaMetrics/operator/issues/554
- controllers: correctly set current and update revisions for statefulset  https://github.com/VictoriaMetrics/operator/issues/547

[Changes][v0.29.2]


<a name="v0.29.1"></a>
## [v0.29.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.29.1) - 14 Nov 2022

### Fixes

- some typos https://github.com/VictoriaMetrics/operator/pull/548 Thanks [@fatsheep9146](https://github.com/fatsheep9146)
- update description for parameter to match behaviour  https://github.com/VictoriaMetrics/operator/pull/549 thanks [@zekker6](https://github.com/zekker6)
- controllers/factory: fix resizing of PVC for vmsingle   https://github.com/VictoriaMetrics/operator/pull/551 thanks [@zekker6](https://github.com/zekker6)

### Features

- Expose no_stale_markers through vm_scrape_params  in https://github.com/VictoriaMetrics/operator/pull/546 Thanks [@tamcore](https://github.com/tamcore)
- {api/vmsingle,api/vmcluster}: add support of `vmbackupmanager` restore on pod start  https://github.com/VictoriaMetrics/operator/pull/544 thanks [@zekker6](https://github.com/zekker6)
- api: changes errors handling for objects unmarshal https://github.com/VictoriaMetrics/operator/pull/550

[Changes][v0.29.1]


<a name="v0.29.0"></a>
## [v0.29.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.29.0) - 24 Oct 2022

### Fixes

* vmcluster: reconcile VMStorage in VMCluster even if PodDisruptionBudget does not exist by [@miketth](https://github.com/miketth) in https://github.com/VictoriaMetrics/operator/pull/535
* crash on Kubernetes 1.25 by [@miketth](https://github.com/miketth) in https://github.com/VictoriaMetrics/operator/pull/536
* throttling for vmagent and vmalert https://github.com/VictoriaMetrics/operator/commit/63ca52bf140b033ecbc3c40f9efc8579b936ea29
* vmalertmanagerconfig:  parsing for nested routes https://github.com/VictoriaMetrics/operator/commit/f2bc0c09069c0cec9bec8757fc3bc339231ccfdd https://github.com/VictoriaMetrics/operator/commit/9472f1fe6e69fd4bfc63d5fb3da14c02b6fb4788
* vmalertmanagerconfig: ownerreference set correctly https://github.com/VictoriaMetrics/operator/commit/2bb5d0234c7b32f27c3f82b007fea409887b54b9
* vmagent: allows to set maxDiskUsage more then 1GB https://github.com/VictoriaMetrics/operator/commit/47f2b508ee503d03111ec03215466a123e2d3978
* vmagent: properly merge ports for additional service https://github.com/VictoriaMetrics/operator/commit/05d332d704fd9cf9c490de22a554badc61e86f51
* vmprobe: correctly set labels for ingress targets https://github.com/VictoriaMetrics/operator/commit/976315cd3dbf57d576414340b1d444d63f8d460d

### Features

* podDistruptionBudget: adds configurable selectors https://github.com/VictoriaMetrics/operator/commit/4f3f5eaf29ad85c6e9b142be5b05ef57b962fcb6

### New Contributors

* [@miketth](https://github.com/miketth) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/535

[Changes][v0.29.0]


<a name="v0.28.5"></a>
## [v0.28.5](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.5) - 13 Sep 2022

### Fixes

* authorization cache usage https://github.com/VictoriaMetrics/operator/commit/e43bdb6c975b712bf5f169b8fa74c8f7760c82f5 Thanks [@AndrewChubatiuk](https://github.com/AndrewChubatiuk)
* claimTemplates: fixes CRD for it https://github.com/VictoriaMetrics/operator/commit/a5d2f9f61ecfc37a776d8f8c1b0f1385536e773c
* vmrules: supress notFound errors https://github.com/VictoriaMetrics/operator/issues/524
* vmagent: fixes regression at default values for tmpDataPath and maxDiskUsage flags https://github.com/VictoriaMetrics/operator/issues/523

### Features

* vmalertmanager: ignore broken receivers https://github.com/VictoriaMetrics/operator/commit/68bbce1f7809d35b42a39925c09a4ddd61f64a9c
* service accounts: do not set labels and annotations for external service accounts https://github.com/VictoriaMetrics/operator/commit/2ea1e640c362271484d0627c4ca571fd0afd74b2

[Changes][v0.28.5]


<a name="v0.28.4"></a>
## [v0.28.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.4) - 12 Sep 2022

### Fixes

* authorization cache usage https://github.com/VictoriaMetrics/operator/commit/e43bdb6c975b712bf5f169b8fa74c8f7760c82f5 Thanks [@AndrewChubatiuk](https://github.com/AndrewChubatiuk)
* claimTemplates: fixes CRD for it https://github.com/VictoriaMetrics/operator/commit/a5d2f9f61ecfc37a776d8f8c1b0f1385536e773c
* vmrules: supress notFound errors https://github.com/VictoriaMetrics/operator/issues/524
* vmagent: fixes regression at default values for tmpDataPath and maxDiskUsage flags https://github.com/VictoriaMetrics/operator/issues/523

### Features

* vmalertmanager: ignore broken receivers https://github.com/VictoriaMetrics/operator/commit/68bbce1f7809d35b42a39925c09a4ddd61f64a9c
* service accounts: do not set labels and annotations for external service accounts https://github.com/VictoriaMetrics/operator/commit/2ea1e640c362271484d0627c4ca571fd0afd74b2

[Changes][v0.28.4]


<a name="v0.28.3"></a>
## [v0.28.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.3) - 02 Sep 2022

### Fixes

* vmalertmanagerConfig: regression at nested routes parsing https://github.com/VictoriaMetrics/operator/commit/07ce4ca80d3ba09506fc41baaecd7087f799a8aa
* vmagent: password_file option was ignored https://github.com/VictoriaMetrics/operator/commit/5ef9710976534be651687aaa71b2110b0a1a348f

[Changes][v0.28.3]


<a name="v0.28.2"></a>
## [v0.28.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.2) - 01 Sep 2022

### Fixes

* vmalert: regression at basicAuth  https://github.com/VictoriaMetrics/operator/commit/f92463949c9fd8be961c52d98ac7f1f956f7eba3
* converter/alertmanager: changes parsing for nested routes - added more context and validation webhook https://github.com/VictoriaMetrics/operator/commit/6af6071db733bbccfe066b45c73d0377a082b822

[Changes][v0.28.2]


<a name="v0.28.1"></a>
## [v0.28.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.1) - 31 Aug 2022

### Fixes

* vmalert: fixes generated crd https://github.com/VictoriaMetrics/operator/commit/7b5b5b27c00e6ef42edb906ff00912157d21acea


[Changes][v0.28.1]


<a name="v0.28.0"></a>
## [v0.28.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.28.0) - 30 Aug 2022

### Fixes

* security: changes base docker image https://github.com/VictoriaMetrics/operator/commit/cda21275517f84b66786e25c5f6b76977ee27a49
* vmagent: fixes incorrect usage of remoteWriteSettings  https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2946
* vmalert: password_file usage https://github.com/VictoriaMetrics/operator/commit/45163164662934587eafd6afed7709efa31ddbe8

### Features

* converter: adds support for prometheus `AlertmanagerConfig`. It converts into `VMAlertmanagerConfig`. https://github.com/VictoriaMetrics/operator/commit/0b99bc09b2bb1fede612bc509237f6ee6c7617a5
* vmalert: tokenFilePath support for any remote endpoint https://github.com/VictoriaMetrics/operator/commit/5b010f4abcd778d35dca7c826bfb84af0e46e08d

[Changes][v0.28.0]


<a name="v0.27.2"></a>
## [v0.27.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.27.2) - 22 Aug 2022

### Fixes

* controllers: fixes `password_file` usage at basicAuth https://github.com/VictoriaMetrics/operator/commit/979f6375d43e33c35137c1006dc3b4be4dba8528
* config-reloader: properly call gzip.Close method https://github.com/VictoriaMetrics/operator/commit/0d3aac72caf3710172c404fbf89f9a4b125dd97c thanks [@Cosrider](https://github.com/Cosrider)

[Changes][v0.27.2]


<a name="v0.27.1"></a>
## [v0.27.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.27.1) - 17 Aug 2022

### Fixes

* controllers: fixes policy/v1 api detection https://github.com/VictoriaMetrics/operator/pull/513

### Features

* vmalert: added `headers` setting for `remoteRead`, `remoteWrite` and `dataSource` https://github.com/VictoriaMetrics/operator/issues/492

[Changes][v0.27.1]


<a name="v0.27.0"></a>
## [v0.27.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.27.0) - 16 Aug 2022

### Fixes

* Adding support tls endpoint for vmauth config reloader by [@mayurvaid-redvest](https://github.com/mayurvaid-redvest) in https://github.com/VictoriaMetrics/operator/pull/511
* Custom config-reloader incorrectly watch for directory at `VMAgent` https://github.com/VictoriaMetrics/operator/issues/510
* Removes validation for `telegram_configs` `parse_mode` validation https://github.com/VictoriaMetrics/operator/issues/506
* Deletion of `VMAgent` in `StatefulMode` https://github.com/VictoriaMetrics/operator/issues/505

### Features

* Allows ignoring objects at argo-cd converted from prometheus CRD with env var: `VM_PROMETHEUSCONVERTERADDARGOCDIGNOREANNOTATIONS=true` https://github.com/VictoriaMetrics/operator/issues/509
* `claimTemplates` now supported at `VMCluster`, `VMAlertmanager`, `VMAgent` https://github.com/VictoriaMetrics/operator/issues/507
* `readinessGates` now supported by CRD objects https://github.com/VictoriaMetrics/operator/commit/29807e65ec817f8a4f095ba5804d0644a4855e46
* HealthChecks now respects `tls` configured at CRD objects https://github.com/VictoriaMetrics/operator/commit/e43a4d5b22d9a507b2a65839a4ca2ce56f08dff8

### New Contributors

* [@mayurvaid-redvest](https://github.com/mayurvaid-redvest) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/511

[Changes][v0.27.0]


<a name="v0.26.3"></a>
## [v0.26.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.26.3) - 26 Jul 2022

### Fixes

* removes breaking changes introduced at v0.26.0. Operator added `docker.io` as container registry prefix and it may break applications, if private repository was configured at spec.repository.image. Now container registry is not set by default.
* alertmanager: removes breaking changes introduced at 0.26.0 release with extraArgs https://github.com/VictoriaMetrics/operator/commit/918595389e62e144c8f5ebae7472bcff62ccef44

[Changes][v0.26.3]


<a name="v0.26.0"></a>
## [v0.26.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.26.0) - 25 Jul 2022

### Breaking changes

**This release contains breaking changes that was fixed at v0.26.2 release. It's recommended to use it instead of upgrading to v0.26.0**

### Fixes

* security: new alpine image with security fixes https://github.com/VictoriaMetrics/operator/commit/c991b5f315ebb3176b98f5cb00c64430efa0d9c1
* alertmanager: metrics endpoint when routePrefix is configured  https://github.com/VictoriaMetrics/operator/pull/488 Thanks [@blesswinsamuel](https://github.com/blesswinsamuel)
* alertmanager: Automaticly disable high availability mode for 1 replica  in https://github.com/VictoriaMetrics/operator/pull/495. Thanks [@hadesy](https://github.com/hadesy)
* vmalertmanager: fix extraArgs, add two dashes  https://github.com/VictoriaMetrics/operator/pull/503 Thanks [@flokli](https://github.com/flokli)
* vmcluster: disables selectNode arg passing to vmselect with enabled `HPA`. It should prevent vmselect cascade restarts https://github.com/VictoriaMetrics/operator/issues/499
* controllers: changes default rate limiter max delay from 16minutes to 2 minutes. https://github.com/VictoriaMetrics/operator/issues/500
* vmagent: now properly changes size for volumes at persistentMode https://github.com/VictoriaMetrics/operator/commit/81f09af5fd3b96c975cdd7b797d02e442e2d96d0
* prometheus converter: adds some missing fields, bumps version dependecy https://github.com/VictoriaMetrics/operator/commit/35f1c26d98e10db06f561e51ee5ff02b9ad72f9d

### Features

* api/v1beta1/VMUser: adds tokenRef  https://github.com/VictoriaMetrics/operator/pull/489
* api/vmauth: adds host param for ingress https://github.com/VictoriaMetrics/operator/pull/490
* api/vmcluster: reworks expanding for cluster https://github.com/VictoriaMetrics/operator/pull/494
* global setting to override container registry by  in https://github.com/VictoriaMetrics/operator/pull/501 Thanks [@tamcore](https://github.com/tamcore)
* api: new versioned kubernetes client https://github.com/VictoriaMetrics/operator/issues/481
* api: adds `authorization` configuration for scrape targets
* api: adds `headers` fields for custom headers passing to targets https://github.com/VictoriaMetrics/operator/commit/0553b60090e51ec800bdbc3698b16752c6551944
* vmagent: adds `headers` configuration per remote storage urls https://github.com/VictoriaMetrics/operator/commit/e0567210098ad53f9c17cc3e260eaab5f754b2f9
* vmagent: allow configuring multitenant mode for remote storage urls https://github.com/VictoriaMetrics/operator/commit/e0567210098ad53f9c17cc3e260eaab5f754b2f9

### New Contributors

* [@blesswinsamuel](https://github.com/blesswinsamuel) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/488
* [@hadesy](https://github.com/hadesy) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/495
* [@tamcore](https://github.com/tamcore) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/501

[Changes][v0.26.0]


<a name="v0.25.1"></a>
## [v0.25.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.25.1) - 20 May 2022

### Fixes

- PersistentVolumeClaim creation for StatefulSet https://github.com/VictoriaMetrics/operator/pull/483 Thanks [@cnych](https://github.com/cnych)

[Changes][v0.25.1]


<a name="v0.25.0"></a>
## [v0.25.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.25.0) - 19 May 2022

### Breaking changes
- **Changes `VMRule` API, now `expr` field can be only `string`, `integer` values are not supported anymore. https://github.com/VictoriaMetrics/operator/commit/f468ae02690e79ed72638f845535d19418b042af**

### Fixes

- PagerDuty config generation https://github.com/VictoriaMetrics/operator/commit/eef8e2eece269d1c64094b2f7cdf69beabaa3739 thanks [@okzheng](https://github.com/okzheng)
- missing `honorTimestamps` for `ServiceMonitor` to `VMServiceScrape` conversion https://github.com/VictoriaMetrics/operator/commit/6728391cc76576fd97571b2efc3bd24c94a4f083 thanks [@gotosre](https://github.com/gotosre)
- PVC volume automatic expansion for `VMCluster` and `VMAlertmanager` https://github.com/VictoriaMetrics/operator/commit/1eac5826b07e7255309b1b9971730e2b79610f85

### Features

- Added `name` field for `VMUser` https://github.com/VictoriaMetrics/operator/issues/472 thanks [@pavan541cs](https://github.com/pavan541cs)
- Added `StatefulMode` for `VMAgent` it allows to use `Statefulset` instead of `Deployment` https://github.com/VictoriaMetrics/operator/issues/219
- Added `Validation Webhook` for `VMRule`, it allows check errors at rules https://github.com/VictoriaMetrics/operator/issues/471
- Added additional metrics for operator `operator_log_messages_total`, `operator_controller_objects_count`, `operator_reconcile_throttled_events_total`, `vm_app_version`, `vm_app_uptime_seconds`, `vm_app_start_timestamp` https://github.com/VictoriaMetrics/operator/commit/b941a42fb6fdfd8ea99ff190e822cb9314efb9d0 https://github.com/VictoriaMetrics/operator/commit/b3c7286e7dc737c46c4d33aa203c0b598a5ef187
- Adds rate limiting for `VMAgent` and `VMAlert` reconcilation https://github.com/VictoriaMetrics/operator/commit/dfb6a14e1193089ba5ab112e0acf4e459aba68b4

### New Contributors
* [@pavan541cs](https://github.com/pavan541cs) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/473
* [@gotosre](https://github.com/gotosre) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/475

[Changes][v0.25.0]


<a name="v0.24.0"></a>
## [v0.24.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.24.0) - 11 Apr 2022

### Fixes

- Finalizers at UrlRelabelConfig and additionalScrapeConfigs https://github.com/VictoriaMetrics/operator/issues/442
- vmagent config update after scrape objects secret data changes https://github.com/VictoriaMetrics/operator/issues/443
- Log typos https://github.com/VictoriaMetrics/operator/issues/459
- Correctly renders `opsgenia_config` for `VMAlertmanagerConfig` https://github.com/VictoriaMetrics/operator/commit/9128b7f24d5d6d98dcf7abc6f212d57cd39b0e7d thanks [@iyuroch](https://github.com/iyuroch)
- Updates basic image with CVE fix https://github.com/VictoriaMetrics/operator/commit/f4a9e530be6d5ebd6e450085ec807117b05e80a8
- Adds missing finalizer for `VMSingle` deployment https://github.com/VictoriaMetrics/operator/commit/06dada488d629d4d321985e80d14ee04e099bdfd thanks [@lujiajing1126](https://github.com/lujiajing1126)
- `pager_duty` generation for `VMAlertmanagerConfig` https://github.com/VictoriaMetrics/operator/pull/439/files thanks [@okzheng](https://github.com/okzheng)
- `VMServiceScrape` generation for `vminsert`, previously opentsdb-http port could be included into it https://github.com/VictoriaMetrics/operator/issues/420

### Features

- Allows filtering for Converted Prometheus CRD objects https://github.com/VictoriaMetrics/operator/issues/444
- Allows overwriting for default arg params https://github.com/VictoriaMetrics/operator/issues/448
- Allows customization for VMServiceScrape objects generated by operator for it's resources https://github.com/VictoriaMetrics/operator/issues/454 https://github.com/VictoriaMetrics/operator/commit/130e54781e1b193e9e65573df0b76440560db57e  Thanks [@artifactori](https://github.com/artifactori)
- Allows configure `terminationGracePeriodSeconds` for CRD objects  https://github.com/VictoriaMetrics/operator/issues/460
- Allows configure `dnsConfig` for CRD objects https://github.com/VictoriaMetrics/operator/commit/dca0b48a175635cecdaf2fe04ea714eb74eecc79 thanks [@fatsheep9146](https://github.com/fatsheep9146)
- Adds `telegram_configs` for `VMAlertmanagerConfig` https://github.com/VictoriaMetrics/operator/commit/076b7d9665e6ac2979421bd8445083dc08cc32ee
- Allows set retentionPeriod less then 1 month https://github.com/VictoriaMetrics/operator/issues/430

### New Contributors

* [@okzheng](https://github.com/okzheng) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/439
* [@iyuroch](https://github.com/iyuroch) made their first contribution in https://github.com/VictoriaMetrics/operator/pull/464

[Changes][v0.24.0]


<a name="v0.23.3"></a>
## [v0.23.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.23.3) - 21 Feb 2022

### Fixes

- fixes retention period for VMSingle and VMCluster, allows to set retentionPeriod lower than 1 month https://github.com/VictoriaMetrics/operator/issues/430

### Features

- allows to control max and min scrape interval for `VMAgent`'s targets with `minScrapeInterval` and `maxScrapeInterval` https://github.com/VictoriaMetrics/operator/commit/3d8183205bef78e877b4f54d7892c4bad47b3971

[Changes][v0.23.3]


<a name="v0.23.2"></a>
## [v0.23.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.23.2) - 14 Feb 2022

### Fixes

- fixed issue with parsing of kubernetes server version https://github.com/VictoriaMetrics/operator/issues/428

[Changes][v0.23.2]


<a name="v0.23.1"></a>
## [v0.23.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.23.1) - 10 Feb 2022

### Fixes

- issue with incorrect vmservicescrape created for vminsert https://github.com/VictoriaMetrics/operator/issues/420

[Changes][v0.23.1]


<a name="v0.23.0"></a>
## [v0.23.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.23.0) - 09 Feb 2022

### Breaking changes

- **job name label was changed, new prefix added with CRD type - probe, podScrape,serviceScrape, nodeScrape and staticScrape**

### Fixes

- fixes job name label with CRD type prefix, it must prevent possible job names collision https://github.com/VictoriaMetrics/operator/commit/3efe28b2de32485aa889118c63093adb291a82ff thanks [@tommy351](https://github.com/tommy351)
- fixes bearerToken usage for VMAgent remoteWriteSpec https://github.com/VictoriaMetrics/operator/issues/422 thanks [@artifactori](https://github.com/artifactori)

### Features

- check kubernetes api server version for deprecated objects and use proper API for it. First of all it's related with `PodSecurityPolicy`  and `PodDisruptionBudget` https://github.com/VictoriaMetrics/operator/commit/5a64f6c01d535f5500a9d9a81ac851f9f12d547a

[Changes][v0.23.0]


<a name="v0.22.1"></a>
## [v0.22.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.22.1) - 21 Jan 2022

### Fixes

- fixes CSV configuration for operator-hub. It allows to launch operator in single-namespace mode https://github.com/VictoriaMetrics/operator/commit/94c7466224bff664552bae4424a54a036d72886b
- fixes annotations merge for deployments, it should fix endless reconcile loop https://github.com/VictoriaMetrics/operator/commit/7d26398ac3303f6684dd01ae12e376b05dd16ac8

### Features

- bumps VictoriaMetrics appllications versions to the v1.72.0 https://github.com/VictoriaMetrics/operator/commit/de289af8af8472e5299fc6ff6e99749b58012edd

[Changes][v0.22.1]


<a name="v0.22.0"></a>
## [v0.22.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.22.0) - 26 Dec 2021

### Fixes

- fixes regression for VMAlert rules selector https://github.com/VictoriaMetrics/operator/issues/394
- fixes build for go 1.17. Removed unneeded deps, upgraded lib versions https://github.com/VictoriaMetrics/operator/issues/392
- fixes docs example https://github.com/VictoriaMetrics/operator/issues/391

### Features

- moves operator API objects into separate go package. It allows to use operator API without import whole operator package. https://github.com/VictoriaMetrics/operator/commit/9fec1898617ba9f73c6c6c78cdebc1535514e263
- allows to set `rollingUpdateStrategy` for statefullsets. With optional `rollingUpdateStrategy: rollingUpdate` operator uses kubernetes controller-manager updates for statefulsets, instead of own implementation. Allows kubectl rollout restart command for deployments and statefulsets https://github.com/VictoriaMetrics/operator/issues/389
- allows to disable namespace label matcher for VMAlertmanager with global option `disableNamespaceMatcher` https://github.com/VictoriaMetrics/operator/issues/390

[Changes][v0.22.0]


<a name="v0.21.0"></a>
## [v0.21.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.21.0) - 30 Nov 2021

### Breaking changes

- **Rollback changes for default behavior for CR selectors, such as serviceScrapeSelector at vmagent.spec. With new option `spec.selectAllByDefault: true` default behavior changes for select all on nil (as was at 0.20 version). https://github.com/VictoriaMetrics/operator/issues/383**
- **moves `ingress` api to `networking/v1` for `VMAuth`, minimal kubernetes supported version for `VMAuth` 1.19 https://github.com/VictoriaMetrics/operator/commit/2c6f81eb91452a7672907aa25acd392ef0777941**

### Fixes

- removes HPA from cache watch, it must remove errors at cluster without such api https://github.com/VictoriaMetrics/operator/commit/04bab9c486babed100522ec12fce3967e4dd5a13
- labels and annotations update for auto-generated serviceScrape components.
- typos at quick-start https://github.com/VictoriaMetrics/operator/commit/e411cfe75b4ff3d57fd532e12c901eda5934645c thanks [@marcbachmann](https://github.com/marcbachmann)

### Features

- Adds alertmanager service scrape auto generation https://github.com/VictoriaMetrics/operator/issues/385 thanks [@FRosner](https://github.com/FRosner)
- Auto-add routing for vminsert and vmselect CRD components for `VMUser` https://github.com/VictoriaMetrics/operator/issues/379
- Updates docs for `VMAuth`https://github.com/VictoriaMetrics/operator/blob/master/docs/auth.MD
- Allows changing default disk space usage for `VMAgent` https://github.com/VictoriaMetrics/operator/pull/381 thanks [@arctan90](https://github.com/arctan90)
- Adds Arch labels for clusterversion template https://github.com/VictoriaMetrics/operator/commit/9e89c3b2459fb85faa8e973fa1f1558d924000f3 thanks [@yselkowitz](https://github.com/yselkowitz)
- improves docs and fixes typos https://github.com/VictoriaMetrics/operator/commit/ae248dcb352a092d9f9caee87454b1ad25650a4c thanks [@flokli](https://github.com/flokli)

[Changes][v0.21.0]


<a name="v0.20.3"></a>
## [v0.20.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.20.3) - 10 Nov 2021

#### Fixes

- changes v1.SecretKeySelector value for pointer, it should help mitigate null error for v1.SecretKeySelector.Key https://github.com/VictoriaMetrics/operator/issues/365
- Fixes `VMAlertmanagerConfig` - some configurations didn't add `send_resolved` option properly to the configration. https://github.com/VictoriaMetrics/operator/commit/6ee75053a4af2a163619908cd10ba4ec051755ab

[Changes][v0.20.3]


<a name="v0.20.2"></a>
## [v0.20.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.20.2) - 07 Nov 2021

#### Fixes

- regression at statefulset update process https://github.com/VictoriaMetrics/operator/issues/366
- adds nullable option for v1.SecretKeySelector https://github.com/VictoriaMetrics/operator/issues/365

[Changes][v0.20.2]


<a name="v0.20.1"></a>
## [v0.20.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.20.1) - 28 Oct 2021

#### Fixes

- regression at alertmanager config generation https://github.com/VictoriaMetrics/operator/commit/0f4368be57b2ccb2fbaebe9ce5fb4394299d89b3

[Changes][v0.20.1]


<a name="v0.20.0"></a>
## [v0.20.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.20.0) - 28 Oct 2021

### Breaking changes

- **changes default behavior for CR selectors, such serviceScrapeSelector at vmagent.spec. Now it select all targets if is missing https://github.com/VictoriaMetrics/operator/commit/519e89b457576099288af2ea135878f6da25b567 See more at docs https://github.com/VictoriaMetrics/operator/blob/master/docs/quick-start.MD#object-selectors**
- **operator doesn't add cluster domain name for in-cluster communication, now its empty value. It should resolve issue with using operator at clusters with custom k8s domain https://github.com/VictoriaMetrics/operator/issues/354 thanks [@flokli](https://github.com/flokli)**

### Features

- adds ability to set custom headers to the `VMUser` target ref https://github.com/VictoriaMetrics/operator/issues/360

### Fixes

- bearer token at staticScrape https://github.com/VictoriaMetrics/operator/issues/357 thanks [@addreas](https://github.com/addreas)
- path for the backups at vmcluster https://github.com/VictoriaMetrics/operator/issues/349
- possible race condition for the cluster backups, now operator adds storage node name into backup path https://github.com/VictoriaMetrics/operator/issues/349
- secret finalizer deletion for vmagent https://github.com/VictoriaMetrics/operator/issues/343
- probes for vmagent https://github.com/VictoriaMetrics/operator/commit/f6de9c5774be0a5cd797c145553579e2e76a8df7
- alertmanagerConfiguration build for slack https://github.com/VictoriaMetrics/operator/issues/339

[Changes][v0.20.0]


<a name="v0.19.1"></a>
## [v0.19.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.19.1) - 28 Sep 2021

### Fixes

- Regression at `VMStaticScrape` - basic auth was incorrectly handled https://github.com/VictoriaMetrics/operator/issues/337
- Convesion from `PodMonitor` to `VMPodScrape` https://github.com/VictoriaMetrics/operator/issues/335

[Changes][v0.19.1]


<a name="v0.19.0"></a>
## [v0.19.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.19.0) - 24 Sep 2021

### Features

- Adds single-namespace mode for operator https://github.com/VictoriaMetrics/operator/issues/239 Thanks [@g7r](https://github.com/g7r)
- improves e2e tests thanks [@g7r](https://github.com/g7r)
- Adds `VMAlert` `Notifier` service discovery  https://github.com/VictoriaMetrics/operator/pull/334
- Updates `VMRule` - now it can use `vmalert` specific features https://github.com/VictoriaMetrics/operator/pull/331
- Disables client caching for `Pod`, `Deployment` and `Statefulset`, it should reduce memory consumption https://github.com/VictoriaMetrics/operator/commit/9cfea5d091f072d1a0c6f8115a5e7652b94c6536

### Fixes

- fixes psp rolebinding for operator https://github.com/VictoriaMetrics/operator/issues/323
- fixes `VMAgent` reconciliation loop https://github.com/VictoriaMetrics/operator/issues/325 Thanks [@silverlyra](https://github.com/silverlyra)

[Changes][v0.19.0]


<a name="v0.18.2"></a>
## [v0.18.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.18.2) - 03 Sep 2021

### Fixes

- Fixes regression at CRD generation https://github.com/VictoriaMetrics/operator/issues/321 https://github.com/VictoriaMetrics/helm-charts/issues/199

[Changes][v0.18.2]


<a name="v0.18.1"></a>
## [v0.18.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.18.1) - 30 Aug 2021

### Fixes

- Fixes regression at CRD generation https://github.com/VictoriaMetrics/operator/issues/316 Thanks [@Cosrider](https://github.com/Cosrider)

[Changes][v0.18.1]


<a name="v0.18.0"></a>
## [v0.18.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.18.0) - 24 Aug 2021

### Deprecations

- **Deprecates `apiextensions.k8s.io/v1beta1` API for CRD. Its still available at legacy mode.**

### Features

- Adds OAuth2 configuration for `VMagent`s remoteWrites and scrape endpoints
- Adds `TLSConfig` for `VMProbes`
- Major API update for `VMServiceScrape`, `VMPodScrape`, `VMProbe`, `VMStaticScrape` and `VMNodeScrape`:
- adds missing config params (sampleLimit and etc)
- Adds new config options `vm_scrape_params` https://github.com/VictoriaMetrics/operator/issues/303
- Adds proxyAuth, that allows to authenticate proxy requests https://docs.victoriametrics.com/vmagent.html#scraping-targets-via-a-proxy
- Adds OAuth2 support.
- Adds `apiextensions.k8s.io/v1` `CRD` generation, `v1beta1` is now legacy https://github.com/VictoriaMetrics/operator/issues/291
- Adds new `CRD` `VMAlertmanagerConfig`, it supports only v0.22 `alertmanager` version or above https://github.com/VictoriaMetrics/operator/issues/188
- Makes `spec.selector` optional for `VMPodScrape` and `VMServiceScrape` https://github.com/VictoriaMetrics/operator/issues/307
- Bumps alpine image for `3.14.1` - it should fixes security issues.
- Adds more unit tests and fixes some bugs

### Fixes

- Fixes bug for incorrect finalizer remove https://github.com/VictoriaMetrics/operator/issues/302

[Changes][v0.18.0]


<a name="v0.17.2"></a>
## [v0.17.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.17.2) - 31 Jul 2021

### Features

- Updated docs.

### Fixes

- fixes vmauth default version
- fixes HPA deletion https://github.com/VictoriaMetrics/operator/issues/296
- fixes VMAlert datasource TlsConfig https://github.com/VictoriaMetrics/operator/issues/298
- fixes VMUser target_path_suffix typo at tags.

[Changes][v0.17.2]


<a name="v0.17.1"></a>
## [v0.17.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.17.1) - 28 Jul 2021

### Features

- Updated default versions for vm apps to v1.63.0 version
- Updated docs.

[Changes][v0.17.1]


<a name="v0.17.0"></a>
## [v0.17.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.17.0) - 27 Jul 2021

### Features

- Changes `VMAuth` config generation, now its possible to add `target_path_suffix` with optional query params https://github.com/VictoriaMetrics/operator/issues/245
- Changes `VMAuth` config generation - in case of `/` it can generate simple config without url_map and regexp https://github.com/VictoriaMetrics/operator/commit/5dcd998b1814b26f75e3f6b5a38f8c3ee20552ec
- Reworks `annotations` merge  https://github.com/VictoriaMetrics/operator/commit/90ae15e300bff68b9140e65819b2a5e1e972b9a0

### Fixes

- Reduces memory usage - coz of improper label selectors and cache usage operator consumed a lot of memory https://github.com/VictoriaMetrics/operator/issues/285
- Fixes VMAlert default image tag typo https://github.com/VictoriaMetrics/operator/issues/287
- Fixes logging configuration https://github.com/VictoriaMetrics/operator/issues/281
- Fixes new config reloader watch logic: https://github.com/VictoriaMetrics/operator/commit/35cadb04b828238ffdec67b3fd1ae7430543055d
- Fixes `VMServiceScrape` for `VMAgent` https://github.com/VictoriaMetrics/operator/commit/7bbbf2cd0557260b419e188b72a001572f848e35

[Changes][v0.17.0]


<a name="v0.16.0"></a>
## [v0.16.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.16.0) - 11 Jul 2021

### Breaking Changes

- Changes `VMAgent` `RemoteWriteSpec` - some options were moved to `RemoteWriteSettings` https://github.com/VictoriaMetrics/operator/pull/273

### Features
- Adds experimental config-reloader implementation, it should help mitigate long configuration sync. It can be enabled with envvar `VM_USECUSTOMCONFIGRELOADER=true`  https://github.com/VictoriaMetrics/operator/issues/124
- Reduces load on kubernetes apiserver for `VMPodScrape` resources https://github.com/VictoriaMetrics/operator/pull/267 thanks [@fatsheep9146](https://github.com/fatsheep9146)
- Adds `/debug/pprof` handler at `0.0.0.0:8435` http server.

### Fixes

- Fixes Tls ingress for `VMAuth` https://github.com/VictoriaMetrics/operator/pull/270
- Fixes endless loop for service account reconciliation https://github.com/VictoriaMetrics/operator/issues/277
- Fixes `VMAlertmanager` update process https://github.com/VictoriaMetrics/operator/issues/271
- Fixes ownership for `ArgoCD` based deployments - https://github.com/VictoriaMetrics/operator/issues/255
- Fixes doc typos https://github.com/VictoriaMetrics/operator/pull/269 thanks [@zasdaym](https://github.com/zasdaym)

[Changes][v0.16.0]


<a name="v0.15.2"></a>
## [v0.15.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.15.2) - 17 Jun 2021

### Features

- reduced CRD size, it should fix operator-hub deployment
- updated lib versions.
- updated docs.

[Changes][v0.15.2]


<a name="v0.15.1"></a>
## [v0.15.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.15.1) - 16 Jun 2021

### Fixes

- Fixed panic at `VMCluster` https://github.com/VictoriaMetrics/operator/issues/264

[Changes][v0.15.1]


<a name="v0.15.0"></a>
## [v0.15.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.15.0) - 14 Jun 2021

### Features

- Adds nodeSelector to all CRD Objects https://github.com/VictoriaMetrics/operator/issues/254
- Adds HPA for `vminsert` and `vmselect` https://github.com/VictoriaMetrics/operator/issues/247
- Adds new CRD resources - `VMAuth` and `VMUser` https://github.com/VictoriaMetrics/operator/issues/245
- Adds hostPath support with ability to override `storageDataPath` setting https://github.com/VictoriaMetrics/operator/issues/240

### Fixes

- Adds prometheus-config-reloader version check and updates its version https://github.com/VictoriaMetrics/operator/issues/259
- Adds ownerReference to ServiceAccounts, it should mitigate ArgoCD issue https://github.com/VictoriaMetrics/operator/issues/255
- Fixes cluster status update process https://github.com/VictoriaMetrics/operator/issues/253
- Fixes `VMAlertmanager` config generation https://github.com/VictoriaMetrics/operator/issues/244

[Changes][v0.15.0]


<a name="v0.14.2"></a>
## [v0.14.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.14.2) - 26 Apr 2021

### Fixes

- fixes insertPorts type for `VMCluster`

[Changes][v0.14.2]


<a name="v0.14.1"></a>
## [v0.14.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.14.1) - 22 Apr 2021

### Fixes

- fixes missing args for inline relabel configs.

[Changes][v0.14.1]


<a name="v0.14.0"></a>
## [v0.14.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.14.0) - 22 Apr 2021

### Fixes

- fixes incorrect tlsConfig handling for vmalert https://github.com/VictoriaMetrics/operator/issues/224
- fixes config sync for relabeling https://github.com/VictoriaMetrics/operator/issues/222

### Features

- improves statefulset rolling update https://github.com/VictoriaMetrics/operator/issues/217
- adds ability to remove vmstorage from cluster routing https://github.com/VictoriaMetrics/operator/issues/218
- adds `inlineRelabelConfig` and `inlineUrlRelabelConfig` for vmagent, it allows to define relabeling rules directly at vmagent CR https://github.com/VictoriaMetrics/operator/issues/154
- adds `inlineScrapeConfig` https://github.com/VictoriaMetrics/operator/pull/230/files
- adds new RBAC permissions for `vmagent`, it should help to monitor `openshift` cluster correctly https://github.com/VictoriaMetrics/operator/issues/229

[Changes][v0.14.0]


<a name="v0.13.1"></a>
## [v0.13.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.13.1) - 13 Apr 2021

### Fixes

- fixes operator role - added missing permission.
- fixes operator crash and improper tlsConfig build https://github.com/VictoriaMetrics/operator/issues/215

[Changes][v0.13.1]


<a name="v0.13.0"></a>
## [v0.13.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.13.0) - 09 Apr 2021

### Fixes

- storage resize detection https://github.com/VictoriaMetrics/operator/pull/211 thanks [@lujiajing1126](https://github.com/lujiajing1126)
- vmagent rbac role  https://github.com/VictoriaMetrics/operator/pull/213 thanks [@viperstars](https://github.com/viperstars)
- fixes CRD for kubernetes version less then 1.16 https://github.com/VictoriaMetrics/operator/pull/210

### Features

- adds probes customization via CRD https://github.com/VictoriaMetrics/operator/pull/204 thanks [@preved911](https://github.com/preved911)

[Changes][v0.13.0]


<a name="v0.12.2"></a>
## [v0.12.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.12.2) - 31 Mar 2021

### Fixes

- fixes serviceAccount update https://github.com/VictoriaMetrics/operator/issues/207

[Changes][v0.12.2]


<a name="v0.12.1"></a>
## [v0.12.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.12.1) - 30 Mar 2021

### Fixes

- removes liveness probe from vmstorage and `VMSingle` https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1158
- fixes update process for `VMCluster` and `VMAlertmanager`

[Changes][v0.12.1]


<a name="v0.12.0"></a>
## [v0.12.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.12.0) - 29 Mar 2021

### Breaking changes

- operator automatically resizes `PVC` and recreates `StatefulSet` for `VMCluster` components if needed, be careful with upgrade, if you are manually edited  `PVC` size. In common cases, it must be safe.

### Features

- Adds scraping sharding for `VMAgent`  https://github.com/VictoriaMetrics/operator/issues/177
- Adds pvc resizing for `VMCluster` and `VMAletermanager`, it also allows to change storage params https://github.com/VictoriaMetrics/operator/issues/161
- Adds `PodDisruptionBudget` for `VMAgent`, `VMCluster`, `VMAlert` and `VMAlertmanager` https://github.com/VictoriaMetrics/operator/issues/191 Thanks [@umezawatakeshi](https://github.com/umezawatakeshi)
- Simplifies `topologySpreadConstraints` configuration https://github.com/VictoriaMetrics/operator/issues/191, thanks [@umezawatakeshi](https://github.com/umezawatakeshi)

### Fixes

- Fixes `VMAlert` `rule` arg - it was unproperly escaped https://github.com/VictoriaMetrics/operator/commit/870f258b324dbaec1e3d0d8739ff2feffc27bf0a
- Fixes `VMProbes`, now it supports relabeling for static targets https://github.com/VictoriaMetrics/operator/commit/b4db7d5128a22d4979d7284e15576322acbc9b4c
- Fixes `VMStaticScrape` - adds `honorLabels` and `honorTimestamps` setting to CRD

[Changes][v0.12.0]


<a name="v0.11.0"></a>
## [v0.11.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.11.0) - 22 Mar 2021

### Breaking changes

- Adds acceptEULA setting to `VMBackuper`, without it backuper cannot be used. https://github.com/VictoriaMetrics/operator/commit/dc7f9e0f830d1e5f1010e7e96ae99f1932fe549f

### Features

- Adds additional service for all components, its useful for service exposition  https://github.com/VictoriaMetrics/operator/issues/163 thanks [@TinySong](https://github.com/TinySong)

### Fixes

- fixes bug with insert ports.
- minor fixes to examples.

[Changes][v0.11.0]


<a name="v0.10.0"></a>
## [v0.10.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.10.0) - 14 Mar 2021

### Features

- Added finalizers to objects created by operator. It must fix an issue with resource deletion by controller manager. Note, it requires additional rbac access. https://github.com/VictoriaMetrics/operator/issues/159 https://github.com/VictoriaMetrics/operator/pull/189
- Added new resouce for static targets scrapping - `VMStaticScrape` https://github.com/VictoriaMetrics/operator/issues/155
- Added `unlimited` param for default resources - https://github.com/VictoriaMetrics/operator/issues/181
- Added clusterVersion spec to `VMCluster` it should simplify management https://github.com/VictoriaMetrics/operator/issues/176

### Fixes

- fixes bug with incorrect object reconciliation - labelMatch heuristic was broken.
- fixes race condition on vmagent reconciliation.
- fixes `VMAlertmanager` version parse https://github.com/VictoriaMetrics/operator/pull/179 thanks [@morimoto-cybozu](https://github.com/morimoto-cybozu)
- other little improvements.

[Changes][v0.10.0]


<a name="v0.9.1"></a>
## [v0.9.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.9.1) - 22 Feb 2021

### Features

- adds externalLabels for vmalert https://github.com/VictoriaMetrics/operator/issues/160

### Fixes

- rbac role namespace.

[Changes][v0.9.1]


<a name="v0.9.0"></a>
## [v0.9.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.9.0) - 21 Feb 2021

### Features

- adds finalizers to the CRDs, it must prevent deletion by controller manager and clean-up created resources properly. https://github.com/VictoriaMetrics/operator/issues/159

### Fixes

- rbac role https://github.com/VictoriaMetrics/operator/issues/166
- fixes incorrect converter start and race condition.

[Changes][v0.9.0]


<a name="v0.8.0"></a>
## [v0.8.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.8.0) - 09 Feb 2021

### Features

- adds VMPodScrape basic auth, token and tls connection support https://github.com/VictoriaMetrics/operator/issues/151
- adds `insertPorts` for `VMSingle` and `VMCluster`, it allows to configure ingestion ports for OpenTSDB,Graphite and Influx servers https://github.com/VictoriaMetrics/operator/pull/157

### Fixes

- fixes operator-hub docs broken links.
- fixes panic at vmcluster.

[Changes][v0.8.0]


<a name="v0.7.4"></a>
## [v0.7.4](https://github.com/VictoriaMetrics/operator/releases/tag/v0.7.4) - 25 Jan 2021

### Fixes

- fixed ExtraArgs typo https://github.com/VictoriaMetrics/operator/pull/150 thanks [@jansyk13](https://github.com/jansyk13)

[Changes][v0.7.4]


<a name="v0.7.3"></a>
## [v0.7.3](https://github.com/VictoriaMetrics/operator/releases/tag/v0.7.3) - 20 Jan 2021

### Fixes

- fixed panic at vmcluster https://github.com/VictoriaMetrics/operator/issues/147 thanks [@gideshrp1JL](https://github.com/gideshrp1JL)

[Changes][v0.7.3]


<a name="v0.7.2"></a>
## [v0.7.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.7.2) - 17 Jan 2021

### Fixes

- serverName for tlsConfig https://github.com/VictoriaMetrics/operator/issues/144
- minScrapeInterval for vmstorage https://github.com/VictoriaMetrics/operator/pull/143 Thansk [@umezawatakeshi](https://github.com/umezawatakeshi)

[Changes][v0.7.2]


<a name="v0.7.1"></a>
## [v0.7.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.7.1) - 01 Jan 2021

### Fixes

-  `VMAlert` deploy inconsistent update https://github.com/VictoriaMetrics/operator/issues/140

### Features

- adds heuristic for selector match between `VMRule`, `VMNodeScrape`, `VMProbe`, `VMServiceScrape` and `VMPodScrape` and corresponding object - `VMAlert` or `VMAgent. It must speed up reconciliation in case of multi-tenancy.

[Changes][v0.7.1]


<a name="v0.7.0"></a>
## [v0.7.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.7.0) - 30 Dec 2020

### Fixes

- https://github.com/VictoriaMetrics/operator/pull/133 VMNodeScrape - fixes nodeScrapeNamespaceSelector. Thanks [@umezawatakeshi](https://github.com/umezawatakeshi)
- VMAlert notifiers support per notifier tlsInSecure. Note, you have to upgrade `vmalert` to v1.51 release.
- Removes null Status and creationTimestamp fields for CRDs.
- https://github.com/VictoriaMetrics/operator/issues/132 - fixes behavior if object was deleted.
- minor fixes to samples for operator-hub.

### Features

- https://github.com/VictoriaMetrics/operator/issues/131 adds support for classic relabelConfigs `target_label` and `source_labels`.
- https://github.com/VictoriaMetrics/operator/issues/127 adds `discoveryRole` with `endpoints`, `endpointslices` and `service` options.

[Changes][v0.7.0]


<a name="v0.6.1"></a>
## [v0.6.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.6.1) - 16 Dec 2020

### Fixes

- VMAlert TLSConfig build was fixed.
- Fixes docs for operator-hub.

[Changes][v0.6.1]


<a name="v0.6.0"></a>
## [v0.6.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.6.0) - 15 Dec 2020

### Breaking changes:

- `VMAgent` RemoteWriteSpec was changed, now it doesnt support `flushInterval,maxBlockSize,maxDiskUsagePerURL and queues`. Because its global flags at `vmagent`.  Added `remoteWriteSettings` instead with corresponding settings.

### Features

- New CRD type `VMNodeScrape`, it's useful for kubernetes nodes exporters scraping. See details at https://github.com/VictoriaMetrics/operator/issues/125.
- `VMAlert` support multiple notifiers with `notifiers` spec.  See details at https://github.com/VictoriaMetrics/operator/issues/117.
- `VMRule` support `concurrency` for group execution, see detail at vmalert docs  https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/app/vmalert#groups.

### Fixes

- Updated docs, thanks [@umezawatakeshi](https://github.com/umezawatakeshi)
- Fixes `VMProbe` spec https://github.com/VictoriaMetrics/operator/issues/125
- Fixes remoteWrite.labels

[Changes][v0.6.0]


<a name="v0.5.0"></a>
## [v0.5.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.5.0) - 04 Dec 2020

### Breaking changes:

- `VMCluster`'s `serviceAccountName` moved from `VMCluster.spec.vm....serviceAccountName` to the root of spec, and now its located at `VMCluster.spec.serviceAccountName`.
- Operator requires additional rbac permissions.

### Features

- PodSecurityPolicy automatically created for each object, with own ServiceAccount, ClusterRole and ClusterRoleBinding. Its possible to use custom PSP. https://github.com/VictoriaMetrics/operator/issues/109
- Adds `VMAgent` rbac auto-creation.
- Adds ServiceAccount auto-creation. Its possible to use custome ServiceAccount instead of default.
- Adds `ownerReferences` for converted resources from `Prometheus-operator` CRDs, https://github.com/VictoriaMetrics/operator/pull/105 thanks [@teqwve](https://github.com/teqwve) .
- Adds `runtimeClassName`, `schedulerName` for all VictoriaMetrics applications.
- Adds `topologySpreadConstraints` for all VictoriaMetrics applications. https://github.com/VictoriaMetrics/operator/issues/107.
- Adds `hostAliases` for `VMAgent` and `VMSingle` applications.

### Fixes

- Fixes rbac for openshift deployment, adds emptyDir for `VMAgent`s persistent queue with 1gb size limit. https://github.com/VictoriaMetrics/operator/issues/106
- Fixes `VMAlert` deployment serviceAccountName.
- Fixes logger levels for operator.
- Fixes labels, now is forbidden to change Selector labels for for all VictoriaMetrics applications. This changes will be ignored.
- Reduces size of CRDs.

[Changes][v0.5.0]


<a name="v0.4.0"></a>
## [v0.4.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.4.0) - 15 Nov 2020

* Adds `VMRules` de-duplication with annotation https://github.com/VictoriaMetrics/operator/issues/99
* Adds Operator-Hub integration https://github.com/VictoriaMetrics/operator/issues/33
* Fixes deployment `Resource` definition (omit limits/requests if provided only one specification).
* Fixes Volumes mounts https://github.com/VictoriaMetrics/operator/issues/97
* Fixes deployments update loop with extra-args https://github.com/VictoriaMetrics/operator/pull/100 . Thanks [@zhiyin009](https://github.com/zhiyin009)
* Fixes securityContext field https://github.com/VictoriaMetrics/operator/pull/101 . Thanks [@zhiyin009](https://github.com/zhiyin009)
* Fixes `VMAgent` start-up error https://github.com/VictoriaMetrics/VictoriaMetrics/issues/879

[Changes][v0.4.0]


<a name="v0.3.0"></a>
## [v0.3.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.3.0) - 29 Oct 2020

* adds fast config update for `VMAlert` https://github.com/VictoriaMetrics/operator/issues/86
* adds docker multiarch support
* updates docs and examples https://github.com/VictoriaMetrics/operator/issues/85 thanks [@elmariofredo](https://github.com/elmariofredo)
* fixes env variables usage with applications https://github.com/VictoriaMetrics/operator/issues/89
* fixes prometheus relabel config inconsistency https://github.com/VictoriaMetrics/operator/issues/92
* fixes vmselect args https://github.com/VictoriaMetrics/operator/pull/95 thanks [@zhiyin009](https://github.com/zhiyin009)

[Changes][v0.3.0]


<a name="v0.2.1"></a>
## [v0.2.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.2.1) - 28 Aug 2020

- [#78](https://github.com/VictoriaMetrics/operator/issues/78) fixed bug with rbac - without access to vmsingles api resource, operator wasn't able to start reconciliation loop.
- [#76](https://github.com/VictoriaMetrics/operator/issues/76) added path prefix support if extraArgs was specified.
- [#71](https://github.com/VictoriaMetrics/operator/issues/71) arm support with cross compilation.

[Changes][v0.2.1]


<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.2.0) - 23 Aug 2020

- Added VMProbe [#59](https://github.com/VictoriaMetrics/operator/issues/59)
- Fixed various bug with prometheus api objects conversion.
- added annotations for control conversion flow [#68](https://github.com/VictoriaMetrics/operator/issues/68)

[Changes][v0.2.0]


<a name="v0.1.2"></a>
## [v0.1.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.1.2) - 21 Aug 2020

- [#66](https://github.com/VictoriaMetrics/operator/issues/66) added path replacement for `CAfile`, `Certfile`, `KeyFile`, `BearerTokenFile` at prometheus api converter.
- [#65](https://github.com/VictoriaMetrics/operator/issues/65) fixed tlsConfig logic, now configuration file renders correctly, if empty value for Cert, Ca or KeySecret defined at tlsConf
- minor documentation update

[Changes][v0.1.2]


<a name="v0.1.1"></a>
## [v0.1.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.1.1) - 18 Aug 2020

- fixed issues with crd patching for 1.18 kubernetes version
- fixed issue with rbac roles
- upgraded go version to 1.15
- upgraded operator-sdk version to 1.0.0

[Changes][v0.1.1]


<a name="v0.1.0"></a>
## [v0.1.0](https://github.com/VictoriaMetrics/operator/releases/tag/v0.1.0) - 12 Aug 2020

Starting point of operator releases
- Documentation update

[Changes][v0.1.0]


<a name="v0.0.6"></a>
## [v0.0.6](https://github.com/VictoriaMetrics/operator/releases/tag/v0.0.6) - 26 Jul 2020

- breaking changes to api (changed group name to operator.victoriametrics.com)
- changed build and release process
- migrated to operator sdk 0.19

[Changes][v0.0.6]


<a name="v0.0.2"></a>
## [v0.0.2](https://github.com/VictoriaMetrics/operator/releases/tag/v0.0.2) - 12 Jun 2020

- fixed panic at vmSingle update
- added support for scraping tls targets with ServiceMonitor TLSConfig

[Changes][v0.0.2]


<a name="v0.0.1"></a>
## [v0.0.1](https://github.com/VictoriaMetrics/operator/releases/tag/v0.0.1) - 06 Jun 2020

it contains basic api objects support:
1) vmAgent
2) vmAlert
3) vmSingle
4) vmAlertmanager

+ prometheus-operator objects:
1) prometheusRule
2) serviceMonitor
3) podMonitor

[Changes][v0.0.1]


[v0.35.1]: https://github.com/VictoriaMetrics/operator/compare/v0.35.0...v0.35.1
[v0.35.0]: https://github.com/VictoriaMetrics/operator/compare/v0.34.1...v0.35.0
[v0.34.1]: https://github.com/VictoriaMetrics/operator/compare/v0.34.0...v0.34.1
[v0.34.0]: https://github.com/VictoriaMetrics/operator/compare/v0.33.0...v0.34.0
[v0.33.0]: https://github.com/VictoriaMetrics/operator/compare/v0.32.1...v0.33.0
[v0.32.1]: https://github.com/VictoriaMetrics/operator/compare/v0.32.0...v0.32.1
[v0.32.0]: https://github.com/VictoriaMetrics/operator/compare/v0.31.0...v0.32.0
[v0.31.0]: https://github.com/VictoriaMetrics/operator/compare/v0.30.4...v0.31.0
[v0.30.4]: https://github.com/VictoriaMetrics/operator/compare/v0.30.3...v0.30.4
[v0.30.3]: https://github.com/VictoriaMetrics/operator/compare/v0.30.2...v0.30.3
[v0.30.2]: https://github.com/VictoriaMetrics/operator/compare/v0.30.1...v0.30.2
[v0.30.1]: https://github.com/VictoriaMetrics/operator/compare/v0.30.0...v0.30.1
[v0.30.0]: https://github.com/VictoriaMetrics/operator/compare/v0.29.2...v0.30.0
[v0.29.2]: https://github.com/VictoriaMetrics/operator/compare/v0.29.1...v0.29.2
[v0.29.1]: https://github.com/VictoriaMetrics/operator/compare/v0.29.0...v0.29.1
[v0.29.0]: https://github.com/VictoriaMetrics/operator/compare/v0.28.5...v0.29.0
[v0.28.5]: https://github.com/VictoriaMetrics/operator/compare/v0.28.4...v0.28.5
[v0.28.4]: https://github.com/VictoriaMetrics/operator/compare/v0.28.3...v0.28.4
[v0.28.3]: https://github.com/VictoriaMetrics/operator/compare/v0.28.2...v0.28.3
[v0.28.2]: https://github.com/VictoriaMetrics/operator/compare/v0.28.1...v0.28.2
[v0.28.1]: https://github.com/VictoriaMetrics/operator/compare/v0.28.0...v0.28.1
[v0.28.0]: https://github.com/VictoriaMetrics/operator/compare/v0.27.2...v0.28.0
[v0.27.2]: https://github.com/VictoriaMetrics/operator/compare/v0.27.1...v0.27.2
[v0.27.1]: https://github.com/VictoriaMetrics/operator/compare/v0.27.0...v0.27.1
[v0.27.0]: https://github.com/VictoriaMetrics/operator/compare/v0.26.3...v0.27.0
[v0.26.3]: https://github.com/VictoriaMetrics/operator/compare/v0.26.0...v0.26.3
[v0.26.0]: https://github.com/VictoriaMetrics/operator/compare/v0.25.1...v0.26.0
[v0.25.1]: https://github.com/VictoriaMetrics/operator/compare/v0.25.0...v0.25.1
[v0.25.0]: https://github.com/VictoriaMetrics/operator/compare/v0.24.0...v0.25.0
[v0.24.0]: https://github.com/VictoriaMetrics/operator/compare/v0.23.3...v0.24.0
[v0.23.3]: https://github.com/VictoriaMetrics/operator/compare/v0.23.2...v0.23.3
[v0.23.2]: https://github.com/VictoriaMetrics/operator/compare/v0.23.1...v0.23.2
[v0.23.1]: https://github.com/VictoriaMetrics/operator/compare/v0.23.0...v0.23.1
[v0.23.0]: https://github.com/VictoriaMetrics/operator/compare/v0.22.1...v0.23.0
[v0.22.1]: https://github.com/VictoriaMetrics/operator/compare/v0.22.0...v0.22.1
[v0.22.0]: https://github.com/VictoriaMetrics/operator/compare/v0.21.0...v0.22.0
[v0.21.0]: https://github.com/VictoriaMetrics/operator/compare/v0.20.3...v0.21.0
[v0.20.3]: https://github.com/VictoriaMetrics/operator/compare/v0.20.2...v0.20.3
[v0.20.2]: https://github.com/VictoriaMetrics/operator/compare/v0.20.1...v0.20.2
[v0.20.1]: https://github.com/VictoriaMetrics/operator/compare/v0.20.0...v0.20.1
[v0.20.0]: https://github.com/VictoriaMetrics/operator/compare/v0.19.1...v0.20.0
[v0.19.1]: https://github.com/VictoriaMetrics/operator/compare/v0.19.0...v0.19.1
[v0.19.0]: https://github.com/VictoriaMetrics/operator/compare/v0.18.2...v0.19.0
[v0.18.2]: https://github.com/VictoriaMetrics/operator/compare/v0.18.1...v0.18.2
[v0.18.1]: https://github.com/VictoriaMetrics/operator/compare/v0.18.0...v0.18.1
[v0.18.0]: https://github.com/VictoriaMetrics/operator/compare/v0.17.2...v0.18.0
[v0.17.2]: https://github.com/VictoriaMetrics/operator/compare/v0.17.1...v0.17.2
[v0.17.1]: https://github.com/VictoriaMetrics/operator/compare/v0.17.0...v0.17.1
[v0.17.0]: https://github.com/VictoriaMetrics/operator/compare/v0.16.0...v0.17.0
[v0.16.0]: https://github.com/VictoriaMetrics/operator/compare/v0.15.2...v0.16.0
[v0.15.2]: https://github.com/VictoriaMetrics/operator/compare/v0.15.1...v0.15.2
[v0.15.1]: https://github.com/VictoriaMetrics/operator/compare/v0.15.0...v0.15.1
[v0.15.0]: https://github.com/VictoriaMetrics/operator/compare/v0.14.2...v0.15.0
[v0.14.2]: https://github.com/VictoriaMetrics/operator/compare/v0.14.1...v0.14.2
[v0.14.1]: https://github.com/VictoriaMetrics/operator/compare/v0.14.0...v0.14.1
[v0.14.0]: https://github.com/VictoriaMetrics/operator/compare/v0.13.1...v0.14.0
[v0.13.1]: https://github.com/VictoriaMetrics/operator/compare/v0.13.0...v0.13.1
[v0.13.0]: https://github.com/VictoriaMetrics/operator/compare/v0.12.2...v0.13.0
[v0.12.2]: https://github.com/VictoriaMetrics/operator/compare/v0.12.1...v0.12.2
[v0.12.1]: https://github.com/VictoriaMetrics/operator/compare/v0.12.0...v0.12.1
[v0.12.0]: https://github.com/VictoriaMetrics/operator/compare/v0.11.0...v0.12.0
[v0.11.0]: https://github.com/VictoriaMetrics/operator/compare/v0.10.0...v0.11.0
[v0.10.0]: https://github.com/VictoriaMetrics/operator/compare/v0.9.1...v0.10.0
[v0.9.1]: https://github.com/VictoriaMetrics/operator/compare/v0.9.0...v0.9.1
[v0.9.0]: https://github.com/VictoriaMetrics/operator/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/VictoriaMetrics/operator/compare/v0.7.4...v0.8.0
[v0.7.4]: https://github.com/VictoriaMetrics/operator/compare/v0.7.3...v0.7.4
[v0.7.3]: https://github.com/VictoriaMetrics/operator/compare/v0.7.2...v0.7.3
[v0.7.2]: https://github.com/VictoriaMetrics/operator/compare/v0.7.1...v0.7.2
[v0.7.1]: https://github.com/VictoriaMetrics/operator/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/VictoriaMetrics/operator/compare/v0.6.1...v0.7.0
[v0.6.1]: https://github.com/VictoriaMetrics/operator/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/VictoriaMetrics/operator/compare/v0.5.0...v0.6.0
[v0.5.0]: https://github.com/VictoriaMetrics/operator/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/VictoriaMetrics/operator/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/VictoriaMetrics/operator/compare/v0.2.1...v0.3.0
[v0.2.1]: https://github.com/VictoriaMetrics/operator/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/VictoriaMetrics/operator/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/VictoriaMetrics/operator/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/VictoriaMetrics/operator/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/VictoriaMetrics/operator/compare/v0.0.6...v0.1.0
[v0.0.6]: https://github.com/VictoriaMetrics/operator/compare/v0.0.2...v0.0.6
[v0.0.2]: https://github.com/VictoriaMetrics/operator/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/VictoriaMetrics/operator/tree/v0.0.1
