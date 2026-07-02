---
weight: 22
title: Best practices
menu:
  docs:
    identifier: vm-best-practices
    parent: 'victoriametrics'
    weight: 22
tags:
  - metrics
  - guide
aliases:
- /BestPractices.html
- /bestpractices/index.html
- /bestpractices/
---
## Install Recommendation

It is recommended to run the latest available release of VictoriaMetrics from [this page](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest), as it includes all bug fixes and enhancements.

There is no need to tune VictoriaMetrics, as it uses reasonable defaults for its command-line flags. These flags are automatically adjusted for the available CPU and RAM resources. There is no need for operating system tuning because VictoriaMetrics is optimized for default OS settings. The only option is to increase the limit on the [number of open files in the OS](https://medium.com/@muhammadtriwibowo/set-permanently-ulimit-n-open-files-in-ubuntu-4d61064429a), so VictoriaMetrics could accept more incoming connections and could keep open more data files. VictoriaMetrics is tested and developed to run efficiently on these defaults, which fit the majority of workloads. Change a setting only when the docs explicitly instruct you to, including when and why.

## Memory

VictoriaMetrics components detect the available memory at startup as the smaller of the host RAM and the cgroup memory limit.
To keep them stable:

1. Do not set `GOMEMLIMIT`. Set the container/cgroup memory limit, and VictoriaMetrics automatically
   sizes its memory-aware limits from it. All VictoriaMetrics components have their own GC settings,
   which are recommended.

1. Do not hand-tune cache sizes with `-storage.cacheSize*` flags; rely on the defaults.
   If a component needs larger caches, move it to a host with more memory.
   See [Cache tuning](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cache-tuning).

1. Do not autoscale `vmstorage` with the Vertical Pod Autoscaler (VPA) or the Horizontal Pod Autoscaler (HPA).
   VPA: cache sizes are derived from the memory limit, read only once at startup.
   Modes that recreate the pod (`Recreate`, `Auto`) reset the caches and force a cold start,
   causing slow inserts and query latency spikes. In-place resizing is not picked up at runtime,
   so `vmstorage` keeps the budget and `vm_available_memory_bytes` initialized at startup, which also skews the dashboards.
   Set fixed memory requests and limits for `vmstorage` rather than autoscaling.
   HPA: `vmstorage` is stateful. Adding nodes sends new series to them while existing data stays where it is.
   Removing nodes makes the data on them unavailable to queries and can cause data loss without replication.
   Frequent scaling keeps changing the routing and can degrade the cluster.

1. Leave headroom for the OS page cache and workload spikes -
   see [capacity planning](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#capacity-planning).

## Swap

It is recommended to disable swap for machines running [vmstorage](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#storage) or [Single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/).
If swap is enabled, the operating system may move actively used data from fast RAM to much slower disk storage when memory usage approaches system limits or configured thresholds.
This leads to performance degradation and latency spikes. On systemd-based Linux distributions run:

```sh
sed -i '/\sswap\s/s/^/#/' /etc/fstab
systemctl mask swap.target
```

Reboot the host after applying the commands.

If you're unsure whether swap-related issues are occurring, check the `Troubleshooting – Major page faults` 
and `Resource usage – Memory pressure` panels on [official Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards) for VictoriaMetrics.
See how to [monitor VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/#monitoring).

## Filesystem

The recommended filesystem for VictoriaMetrics is [ext4](https://en.wikipedia.org/wiki/Ext4). If you plan to store more than 1TB of data on ext4 partition,
then the following options are recommended to pass to `mkfs.ext4`:

```sh
mkfs.ext4 ... -O 64bit,huge_file,extent -T huge
```

VictoriaMetrics should work OK with other filesystems too.

## Operating System

VictoriaMetrics is production-ready for the following operating systems:

* Linux (Alpine, Ubuntu, Debian, RedHat, etc.)
* FreeBSD
* OpenBSD
* Solaris/SmartOS

There is an experimental support of VictoriaMetrics components for Windows.

VictoriaMetrics can also run on macOS for testing and development purposes.

## Supported Architectures

* **Linux**: i386, amd64, arm, arm64, ppc64le, s390x
* **FreeBSD**: i386, amd64, arm
* **OpenBSD**: i386, amd64, arm
* **Solaris/SmartOS**: i386, amd64
* **macOS**: amd64, arm64 (for testing and development purposes)
* **Windows**: amd64

## Kubernetes

VictoriaMetrics natively supports deployment in Kubernetes via [helm charts](https://docs.victoriametrics.com/helm/)
and [kubernetes operator](https://docs.victoriametrics.com/operator/). See how to [start using k8s operator](https://docs.victoriametrics.com/guides/getting-started-with-vm-operator/).

Common recommendations:

1. Prefer setting [requests equal to limits](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)
for stateful components like [vmstorage](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview) to avoid unnecessary
component restarts.

1. Avoid using [fractional CPU units](https://kubernetes.io/docs/tasks/configure-pod-container/assign-cpu-resource/#cpu-units)
when configuring resources for optimal performance. VictoriaMetrics is written in Go and its runtime requires specifying
[integer number](https://pkg.go.dev/runtime#GOMAXPROCS) of concurrently running threads.
When fractional CPU unit is specified, VictoriaMetrics will automatically round it down.

## Upgrade procedure

It is safe to upgrade VictoriaMetrics to new versions unless the [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest) say otherwise.
It is safe to skip multiple versions during the upgrade unless release notes say otherwise. It is recommended to perform regular upgrades to the latest version,
since it may contain important bug fixes, performance optimizations or new features.

It is also safe to downgrade to the previous version unless release notes say otherwise.

The following steps must be performed during the upgrade / downgrade procedure:

* Send SIGINT signal to VictoriaMetrics process to stop it gracefully.
* Wait until the process stops. This can take a few seconds.
* Start the upgraded VictoriaMetrics.

> If you'd prefer not to manage upgrades yourself, [VictoriaMetrics Cloud](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_vm_bestpractices_upgrade)
> performs version upgrades automatically during scheduled maintenance windows with no action required on your part.
> See the [VictoriaMetrics Cloud documentation](https://docs.victoriametrics.com/victoriametrics-cloud/) to get started.

## Backup Recommendations

VictoriaMetrics supports backups via [vmbackup](https://docs.victoriametrics.com/victoriametrics/vmbackup/) and [vmrestore](https://docs.victoriametrics.com/victoriametrics/vmrestore/) tools. There is also [vmbackupmanager](https://docs.victoriametrics.com/victoriametrics/vmbackupmanager/), which simplifies backup automation.

## Technical Support and Services

Technical support for VictoriaMetrics is available using the following channels:

* [GitHub issues](https://github.com/VictoriaMetrics/VictoriaMetrics/issues)
* [Slack Inviter](https://slack.victoriametrics.com/) and [Slack channel](https://victoriametrics.slack.com/)
* [Telegram channel](https://t.me/VictoriaMetrics_en)

We also provide [Enterprise support](https://docs.victoriametrics.com/victoriametrics/enterprise/).
