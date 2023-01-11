---
sort: 14
---

# Configuration synchronization

## Basic concepts
VictoriaMetrics applications, like many other applications with configuration file deployed at Kubernetes, uses `ConfigMaps` and `Secrets` for configuration files.
Usually, it's out of application scope to watch for configuration on-disk changes.
Applications reload their configuration by a signal from a user or some other tool, that knows how to watch for updates.
At Kubernetes, the most popular design for this case is a sidecar container, that watches for configuration file changes and sends an HTTP request to the application.

`Configmap` or `Secret` that mounted at `Pod` holds a copy of its content.
Kubernetes component `kubelet` is responsible for content synchronization between an object at Kubernetes API and a file served on disk.
It's not efficient to sync its content immediately, and `kubelet` eventually synchronizes it. There is a configuration option, that controls this period.

That's why, `VMAgent`, `VMAlert` and other applications managed by operator don't receive changes immediately. It usually takes 1-2 min, before content will be updated.

It may trigger errors when an application was deleted, but `VMAgent` still tries to scrape it.

## Possible mitigations

The naive solution for this case decrease the synchronization period. But it configures globally and may be hard for operator users.

That's why operator uses a few hacks.

For `ConfigMap` updates, operator changes annotation with a time of `Configmap` content update. It triggers `ConfigMap`'s content synchronization by kubelet immediately.
It's the case for `VMAlert`, it uses `ConfigMap` as a configuration source.

For `Secret` it doesn't work. And operator offers its implementation for side-car container. It can be configured with env variable for operator:
 ```
 - name: VM_USECUSTOMCONFIGRELOADER
   value: "true"
 ```
If it's defined, operator uses own [config-reloader](https://github.com/VictoriaMetrics/operator/tree/master/internal/config-reloader) instead of [prometheus-config-reload](https://github.com/prometheus-operator/prometheus-operator/tree/main/cmd/prometheus-config-reloader).

It watches corresponding `Secret` for changes with Kubernetes API watch call and writes content into emptyDir.
This emptyDir shared with the application.
In case of content changes, `config-reloader` sends HTTP requests to the application.
It greatly reduces the time for configuration synchronization.