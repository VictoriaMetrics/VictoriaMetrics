---
weight: 1
title: Quick start
menu:
  docs:
    identifier: vm-quick-start
    parent: victoriametrics
    weight: 1
tags:
  - metrics
  - guide
aliases:
- /Quick-Start.html
- /quick-start/index.html
- /quick-start/
---
## How to install

VictoriaMetrics is available in the following distributions:

* [Single-server-VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/) - all-in-one
  binary that is easy to run and maintain. Single-server-VictoriaMetrics perfectly scales vertically and easily handles
  millions of metrics;
* [VictoriaMetrics Cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/) - set of components
  for building horizontally scalable clusters;
* [VictoriaMetrics Cloud](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_vm_quickstart_guide) - VictoriaMetrics installation in the cloud.
  Users can pick a suitable installation size and don't think of typical DevOps tasks such as configuration tuning,
  monitoring, logs collection, access protection, software updates, backups, etc.

VictoriaMetrics is available as:

* docker images at [Docker Hub](https://hub.docker.com/r/victoriametrics/victoria-metrics/) and [Quay](https://quay.io/repository/victoriametrics/victoria-metrics?tab=tags)
* [Helm Charts](https://docs.victoriametrics.com/helm/)
* [Kubernetes operator](https://docs.victoriametrics.com/operator/)
* [Binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
* [Ansible Roles](https://github.com/VictoriaMetrics/ansible-playbooks)
* [Source code](https://github.com/VictoriaMetrics/VictoriaMetrics).
  See [How to build from sources](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-build-from-sources)
* [VictoriaMetrics on Linode](https://www.linode.com/marketplace/apps/victoriametrics/victoriametrics/)
* [VictoriaMetrics on DigitalOcean](https://marketplace.digitalocean.com/apps/victoriametrics-single)

Just download VictoriaMetrics and follow [these instructions](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-start-victoriametrics).
See [available integrations](https://docs.victoriametrics.com/victoriametrics/integrations/) with other systems like
[Prometheus](https://docs.victoriametrics.com/victoriametrics/integrations/prometheus/) or [Grafana](https://docs.victoriametrics.com/victoriametrics/integrations/grafana/).

VictoriaMetrics is developed at a fast pace, so it is recommended periodically checking the [CHANGELOG](https://docs.victoriametrics.com/victoriametrics/changelog/)
and performing [regular upgrades](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-upgrade-victoriametrics).

### Starting VictoriaMetrics Single Node or Cluster on VictoriaMetrics Cloud {id="starting-vm-on-cloud"}

1. Go to [VictoriaMetrics Cloud](https://console.victoriametrics.cloud/signUp?utm_source=website&utm_campaign=docs_vm_quickstart_guide) and sign up (it's free).
1. After signing up, you will be immediately granted $200 of trial credits you can spend on running Single node or Cluster.
1. Navigate to the VictoriaMetrics Cloud [quick start](https://docs.victoriametrics.com/victoriametrics-cloud/get-started/quickstart/#creating-deployments) guide for detailed instructions.

### Starting VictoriaMetrics Single Node via Docker {id="starting-vm-single-via-docker"}

Download the newest available [VictoriaMetrics release](https://docs.victoriametrics.com/victoriametrics/changelog/)
from [DockerHub](https://hub.docker.com/r/victoriametrics/victoria-metrics) or [Quay](https://quay.io/repository/victoriametrics/victoria-metrics?tab=tags):

```sh
docker pull victoriametrics/victoria-metrics:v1.134.0
docker run -it --rm -v `pwd`/victoria-metrics-data:/victoria-metrics-data -p 8428:8428 \
 victoriametrics/victoria-metrics:v1.134.0 --selfScrapeInterval=5s -storageDataPath=victoria-metrics-data
```

_For Enterprise images see [this link](https://docs.victoriametrics.com/victoriametrics/enterprise/#docker-images)._

You should see:

```sh
 started server at http://0.0.0.0:8428/
 partition "2025_03" has been created
```

Open `http://localhost:8428/vmui` in WEB browser to see graphical interface [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui).
With `--selfScrapeInterval=5s` VictoriaMetrics scrapes its own metrics, and they should become queryable 30s after start.
Visit `http://localhost:8428/vmui/#/metrics` to explore available metrics or run an arbitrary query at
`http://localhost:8428/vmui` (i.e. `process_cpu_cores_available`).
Other available HTTP endpoints are listed on `http://localhost:8428` page.

See how to [write](https://docs.victoriametrics.com/victoriametrics/quick-start/#write-data) or [read](https://docs.victoriametrics.com/victoriametrics/quick-start/#query-data)
from VictoriaMetrics.

### Starting VictoriaMetrics Cluster via Docker {id="starting-vm-cluster-via-docker"}

Clone [VictoriaMetrics repository](https://github.com/VictoriaMetrics/VictoriaMetrics) and start the docker environment
via `make docker-vm-cluster-up` command:

```sh
git clone https://github.com/VictoriaMetrics/VictoriaMetrics && cd VictoriaMetrics
make docker-vm-cluster-up
```

You should see:

```sh
 ✔ Container vmstorage-1        Started                                                                                                                                                0.4s
 ✔ Container vmselect-1         Started                                                                                                                                                0.4s
 ✔ Container vminsert           Started                                                                                                                                                0.4s
 ✔ Container vmagent            Started
```

The command starts a set of VictoriaMetrics components for metrics collection, storing, alerting and Grafana for user
interface. See the [description of deployed topology](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#victoriametrics-cluster).

Visit Grafana `http://localhost:3000/` (admin:admin) or vmui `http://localhost:8427/select/0/vmui` to start exploring metrics.

_Further customization is possible by editing the [compose-vm-cluster.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/compose-vm-cluster.yml)
file._

See more details about [cluster architecture](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#cluster-setup).

### Starting VictoriaMetrics Single Node from a Binary {id="starting-vm-single-from-a-binary"}

1. Download the correct binary for your OS and architecture from [GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
For Enterprise binaries see [this link](https://docs.victoriametrics.com/victoriametrics/enterprise/#binary-releases).

2. Extract the archive to /usr/local/bin by running:

```sh
sudo tar -xvf <victoriametrics-archive> -C /usr/local/bin
```

Replace `<victoriametrics-archive>` with the path to the archive you downloaded in step 1.

3. Create a VictoriaMetrics user on the system:

```sh
sudo useradd -s /usr/sbin/nologin victoriametrics
```

4. Create a folder for storing VictoriaMetrics data:

```sh
sudo mkdir -p /var/lib/victoria-metrics && sudo chown -R victoriametrics:victoriametrics /var/lib/victoria-metrics
```

5. Create a Linux Service by running the following:

```sh
sudo bash -c 'cat <<END >/etc/systemd/system/victoriametrics.service
[Unit]
Description=VictoriaMetrics service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
ExecStart=/usr/local/bin/victoria-metrics-prod -storageDataPath=/var/lib/victoria-metrics -retentionPeriod=90d -selfScrapeInterval=10s
SyslogIdentifier=victoriametrics
Restart=always

PrivateTmp=yes
ProtectHome=yes
NoNewPrivileges=yes

ProtectSystem=full

[Install]
WantedBy=multi-user.target
END'
```

Extra [command-line flags](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#list-of-command-line-flags) can be added to `ExecStart` line.

If you want to deploy VictoriaMetrics Single Node as a Windows Service review the [running as a Windows service docs](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#running-as-windows-service).

> Please note, `victoriametrics` service is listening on `:8428` for HTTP connections (see `-httpListenAddr` flag).

6. Start and enable the service by running the following command:

```sh
sudo systemctl daemon-reload && sudo systemctl enable --now victoriametrics.service
```

7. Check that service started successfully:

```sh
sudo systemctl status victoriametrics.service
```

8. After VictoriaMetrics is in `Running` state, verify [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) is working
by going to `http://<ip_or_hostname>:8428/vmui`.

### Starting VictoriaMetrics Cluster from Binaries {id="starting-vm-cluster-from-binaries"}

VictoriaMetrics cluster consists of [3 components](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview).
It is recommended to run these components in the same private network (for [security reasons](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#security)),
but on the separate physical nodes for the best performance.

On all nodes you will need to do the following:

1. Download the correct binary for your OS and architecture with `-cluster` suffix from [GitHub](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).
For Enterprise binaries see [this link](https://docs.victoriametrics.com/victoriametrics/enterprise/#binary-releases).

2. Extract the archive to /usr/local/bin by running:

```sh
sudo tar -xvf <victoriametrics-archive> -C /usr/local/bin
```

Replace `<victoriametrics-archive>` with the path to the archive you downloaded in step 1

3. Create a user account for VictoriaMetrics:

```sh
sudo useradd -s /usr/sbin/nologin victoriametrics
```

See recommendations for installing each type of [cluster component](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#architecture-overview) below.

#### Installing vmstorage

1. Create a folder for storing `vmstorage` data:

```sh
sudo mkdir -p /var/lib/vmstorage && sudo chown -R victoriametrics:victoriametrics /var/lib/vmstorage
```

2. Create a Linux Service for `vmstorage` service by running the following command:

```sh
sudo bash -c 'cat <<END >/etc/systemd/system/vmstorage.service
[Unit]
Description=VictoriaMetrics vmstorage service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
Restart=always
ExecStart=/usr/local/bin/vmstorage-prod -retentionPeriod=90d -storageDataPath=/var/lib/vmstorage

PrivateTmp=yes
NoNewPrivileges=yes
ProtectSystem=full

[Install]
WantedBy=multi-user.target
END'
```

Extra [command-line flags](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vmstorage)
for vmstorage can be added to `ExecStart` line.

> Please note, `vmstorage` service is listening on `:8400` for vminsert connections (see `-vminsertAddr` flag),
> on `:8401` for vmselect connections (see `--vmselectAddr` flag) and on `:8482` for HTTP connections (see `-httpListenAddr` flag).

3. Start and Enable `vmstorage`:

```sh
sudo systemctl daemon-reload && sudo systemctl enable --now vmstorage
```

4. Check that service started successfully:

```sh
sudo systemctl status vmstorage
```

5. After `vmstorage` is in `Running` state, confirm the service is healthy by visiting `http://<ip_or_hostname>:8482/-/healthy` link.
It should say "VictoriaMetrics is Healthy".

#### Installing vminsert

1. Create a Linux Service for `vminsert` by running the following command:

```sh
sudo bash -c 'cat <<END >/etc/systemd/system/vminsert.service
[Unit]
Description=VictoriaMetrics vminsert service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
Restart=always
ExecStart=/usr/local/bin/vminsert-prod -storageNode=<list of vmstorages>

PrivateTmp=yes
NoNewPrivileges=yes
ProtectSystem=full

[Install]
WantedBy=multi-user.target
END'
```

Replace `<list of vmstorages>` with addresses of previously configured `vmstorage` services.
To specify multiple addresses you can repeat the flag multiple times, or separate addresses with commas
in one flag. See more details in `-storageNode` flag description in [vminsert flags](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vminsert).

> Please note, `vminsert` service is listening on `:8480` for HTTP connections (see `-httpListenAddr` flag).

2. Start and Enable `vminsert`:

```sh
sudo systemctl daemon-reload && sudo systemctl enable --now vminsert.service
```

3. Check that service started successfully:

```sh
sudo systemctl status vminsert.service
```

4. After `vminsert` is in `Running` state, confirm the service is healthy by visiting `http://<ip_or_hostname>:8480/-/healthy` link.
It should say "VictoriaMetrics is Healthy"

#### Installing vmselect

1. Create a folder to store temporary cache:

```sh
sudo mkdir -p /var/lib/vmselect-cache && sudo chown -R victoriametrics:victoriametrics /var/lib/vmselect-cache
```

2. Add a Linux Service for `vmselect` by running

```sh
sudo bash -c 'cat <<END >/etc/systemd/system/vmselect.service
[Unit]
Description=VictoriaMetrics vmselect service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
Restart=always
ExecStart=/usr/local/bin/vmselect-prod -storageNode=<list of vmstorages> -cacheDataPath=/var/lib/vmselect-cache

PrivateTmp=yes
NoNewPrivileges=yes

ProtectSystem=full

[Install]
WantedBy=multi-user.target
END'
```

Replace `<list of vmstorages>` with addresses of previously configured `vmstorage` services.
To specify multiple addresses you can repeat the flag multiple times, or separate addresses with commas
in one flag. See more details in `-storageNode` flag description [vminsert flags](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#list-of-command-line-flags-for-vminsert).

> Please note, `vmselect` service is listening on `:8481` for HTTP connections (see `-httpListenAddr` flag).

3. Start and Enable `vmselect`:

```sh
sudo systemctl daemon-reload && sudo systemctl enable --now vmselect.service
```

4. Make sure the `vmselect` service is running:

```sh
sudo systemctl status vmselect.service
```

5. After `vmselect` is in `Running` state, confirm the service is healthy by visiting `http://<ip_or_hostname>:8481/select/0/vmui` link.
It should open [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) page.

## Write data

There are two main models in monitoring for data collection: [push](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model)
and [pull](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#pull-model). Both are used in modern monitoring and both are
supported by VictoriaMetrics.

See more details on [key concepts of writing data here](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#write-data).
See documentation for configuring [metrics collectors](https://docs.victoriametrics.com/victoriametrics/data-ingestion/)
and [other integrations](https://docs.victoriametrics.com/victoriametrics/integrations/).

## Query data

VictoriaMetrics has built-in [vmui](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#vmui) - graphical
User Interface for querying and visualizing metrics. [MetricsQL](https://docs.victoriametrics.com/victoriametrics/metricsql/) - is the
query language for executing read queries in VictoriaMetrics. See examples of MetricsQL queries in [MetricsQL concepts](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#metricsql).

VictoriaMetrics provides an [HTTP API](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#prometheus-querying-api-usage)
for serving read queries. The API is used in various integrations such as [Grafana](https://docs.victoriametrics.com/victoriametrics/integrations/grafana/).

See more details on [key concepts of querying data](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#query-data)
and [other integrations](https://docs.victoriametrics.com/victoriametrics/integrations/).

## Alerting

To run periodic conditions checks use [vmalert](https://docs.victoriametrics.com/victoriametrics/vmalert/).
It allows creating set of conditions using MetricsQL expressions and send notifications to [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/)
when such conditions are met.

See [vmalert quick start](https://docs.victoriametrics.com/victoriametrics/vmalert/#quickstart).

## Data migration

Migrating data from other TSDBs to VictoriaMetrics is as simple as importing data via any of
[supported formats](https://docs.victoriametrics.com/victoriametrics/keyconcepts/#push-model).

The migration might get easier when using [vmctl](https://docs.victoriametrics.com/victoriametrics/vmctl/) - VictoriaMetrics
command line tool. It supports the following databases for migration to VictoriaMetrics:

* [Prometheus using snapshot API](https://docs.victoriametrics.com/victoriametrics/vmctl/prometheus/);
* [Thanos](https://docs.victoriametrics.com/victoriametrics/vmctl/thanos/);
* [Mimir](https://docs.victoriametrics.com/victoriametrics/vmctl/mimir/);
* [Promscale](https://docs.victoriametrics.com/victoriametrics/vmctl/promscale/);
* [InfluxDB](https://docs.victoriametrics.com/victoriametrics/vmctl/influxdb/);
* [OpenTSDB](https://docs.victoriametrics.com/victoriametrics/vmctl/opentsdb/);
* [Migrate data between VictoriaMetrics single and cluster versions](https://docs.victoriametrics.com/victoriametrics/vmctl/victoriametrics/).
* [Migrate data via Prometheus Remote Read protocol](https://docs.victoriametrics.com/victoriametrics/vmctl/remoteread/).

## Productionization

When going to production with VictoriaMetrics we recommend following the recommendations below.

### Monitoring

Each VictoriaMetrics component emits its own metrics with various details regarding performance
and health state. Docs for the components also contain a `Monitoring` section with an explanation
of what and how should be monitored. For example,
[Single-server-VictoriaMetrics Monitoring](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#monitoring).

VictoriaMetrics has a list of [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).
Each dashboard contains a lot of useful information and tips. It is recommended to have these dashboards installed and up to date.

Using the [recommended alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
will help to identify unwanted issues.

The rule of thumb is to have a separate installation of VictoriaMetrics or any other monitoring system to monitor the
production installation of VictoriaMetrics. This would make monitoring independent and will help identify problems with
the main monitoring installation.

See more details in the article [VictoriaMetrics Monitoring](https://victoriametrics.com/blog/victoriametrics-monitoring/).

### Capacity planning

See capacity planning sections in [docs](https://docs.victoriametrics.com) for
[Single-server-VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#capacity-planning)
and [VictoriaMetrics Cluster](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#capacity-planning).

Capacity planning isn't possible without [monitoring](#monitoring), so consider configuring it first.
Understanding resource usage and performance of VictoriaMetrics also requires knowing the tech terms
[active series](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-an-active-time-series),
[churn rate](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-churn-rate),
[cardinality](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-high-cardinality),
[slow inserts](https://docs.victoriametrics.com/victoriametrics/faq/#what-is-a-slow-insert).
All of them are present in [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).

### Data safety

It is recommended to read [Replication and data safety](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#replication-and-data-safety),
[Why replication doesn’t save from disaster?](https://valyala.medium.com/speeding-up-backups-for-big-time-series-databases-533c1a927883)

For backup configuration, please refer to [vmbackup documentation](https://docs.victoriametrics.com/victoriametrics/vmbackup/).

### Configuring limits

To avoid excessive resource usage or performance degradation limits must be in place:

* [Resource usage limits](https://docs.victoriametrics.com/victoriametrics/faq/#how-to-set-a-memory-limit-for-victoriametrics-components);
* [Cardinality limiter](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#cardinality-limiter).

### Security recommendations

* [Security recommendations for single-node VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#security)
* [Security recommendations for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#security)
