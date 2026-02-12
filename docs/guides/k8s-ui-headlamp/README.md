---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---

Headlamp is a user-friendly Kubernetes web UI with a built‑in Prometheus plugin that can read metrics from VictoriaMetrics.

This guide shows how to point Headlamp’s Prometheus integration at VictoriaMetrics Single or VictoriaMetrics Cluster, so you can get CPU, memory, network, and filesystem graphs for your Kubernetes resources directly inside the UI.

## 1. Install VictoriaMetrics

The VictoriaMetrics time-series database must be running in your Kubernetes cluster.

Install either of these versions:

- VictoriaMetrics single-node: [Kubernetes monitoring via VictoriaMetrics Single](https://docs.victoriametrics.com/guides/k8s-monitoring-via-vm-single/)
- VictoriaMetrics cluster: [Kubernetes monitoring with VictoriaMetrics Cluster](https://docs.victoriametrics.com/guides/k8s-monitoring-via-vm-cluster/)

Once installed and running, take note of the `NAME`, `PORT`, and namespace where the service is running.

- For the single-node version:

    ```sh
    $ kubectl get svc -l app.kubernetes.io/instance=vmsingle

    NAME                                      TYPE        CLUSTER-IP   EXTERNAL-IP   PORT(S)    AGE
    vmsingle-victoria-metrics-single-server   ClusterIP   None         <none>        8428/TCP   15m
    ```

- For the cluster version:

    ```sh
    $ kubectl get svc -l app=vmselect

    NAME                                          TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)    AGE
    vmcluster-victoria-metrics-cluster-vmselect   ClusterIP   10.43.41.195   <none>        8481/TCP   2m2s
    ```


## 2. Install Headlamp

You can run Headlamp as a [desktop application](https://headlamp.dev/docs/latest/installation/desktop/) or [run it as an in-cluster service](https://headlamp.dev/docs/latest/installation/in-cluster/).

Next, go to **Settings** > **Plugins** and select Prometheus.

![Screenshot of Headlamp UI](headlamp-plugins.webp)

Ensure **Enable metrics** is activated and **Auto-detect** is disabled. 

Fill in the Prometheus Service Address in the following format:

```text
namespace/service-name:port
```

For example, in the single-node version running in the default namespace, the address looks like:

    ```text
    default/vmsingle-victoria-metrics-single-server:8428
    ```

    ![Screenshot of Prometheus Plugin](promtheus-config-vmsingle.webp)

For the cluster version, the address looks like:

    ```text
    default/vmcluster-victoria-metrics-cluster-vmselect:8481
    ```

In addition, only for the cluster version, you must fill in the following path in **Prometheus service subpath**. Where `0` is the default [Tenant ID](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#multitenancy):

    ```text
    /select/0/prometheus
    ```

    ![Screenshot of Prometheus Plugin](promtheus-config-vmcluster.webp)

> [!TIP]
> The **Test Connection** button does not work with VictoriaMetrics. You can ignore the error since the plugin works even when the test connection check fails.

Press **Save** to confirm your changes.

![Screenshot of Headlamp UI](prometheus-save.webp)

Now you should find a Show Prometheus metrics option in several pages.

![Screenshot of Headlamp UI](pod-metrics.webp)

