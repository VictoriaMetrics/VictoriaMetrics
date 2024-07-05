---
sort: 22
weight: 22
title: Quick start
menu:
  docs:
    parent: 'victoriametrics'
    weight: 22
aliases:
- /Quick-Start.html
---

# Quick start

## How to install

VictoriaMetrics is distributed in two forms:
* [Single-server-VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/) - all-in-one
  binary, which is very easy to use and maintain.
  Single-server-VictoriaMetrics perfectly scales vertically and easily handles millions of metrics/s;
* [VictoriaMetrics Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/) - set of components
  for building horizontally scalable clusters.

Single-server-VictoriaMetrics VictoriaMetrics is available as:

* [Managed VictoriaMetrics at AWS](https://aws.amazon.com/marketplace/pp/prodview-4tbfq5icmbmyc)
* [Docker images](https://hub.docker.com/r/victoriametrics/victoria-metrics/)
* [Helm Charts](https://github.com/VictoriaMetrics/helm-charts#list-of-charts)
* [Binary releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases/latest)
* [Ansible Roles](https://github.com/VictoriaMetrics/ansible-playbooks)
* [Source code](https://github.com/VictoriaMetrics/VictoriaMetrics).
  See [How to build from sources](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-build-from-sources)
* [VictoriaMetrics on Linode](https://www.linode.com/marketplace/apps/victoriametrics/victoriametrics/)
* [VictoriaMetrics on DigitalOcean](https://marketplace.digitalocean.com/apps/victoriametrics-single)

Just download VictoriaMetrics and follow
[these instructions](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-start-victoriametrics).
Then read [Prometheus setup](https://docs.victoriametrics.com/single-server-victoriametrics/#prometheus-setup)
and [Grafana setup](https://docs.victoriametrics.com/single-server-victoriametrics/#grafana-setup) docs.

VictoriaMetrics is developed at a fast pace, so it is recommended periodically checking the [CHANGELOG](https://docs.victoriametrics.com/changelog/) and performing [regular upgrades](https://docs.victoriametrics.com/#how-to-upgrade-victoriametrics).


### Starting VictoriaMetrics Single via Docker

The following commands download the latest available
[Docker image of VictoriaMetrics](https://hub.docker.com/r/victoriametrics/victoria-metrics)
and start it at port 8428, while storing the ingested data at `victoria-metrics-data` subdirectory
under the current directory:


```sh
docker pull victoriametrics/victoria-metrics:latest
docker run -it --rm -v `pwd`/victoria-metrics-data:/victoria-metrics-data -p 8428:8428 victoriametrics/victoria-metrics:latest
```


Open <a href="http://localhost:8428">http://localhost:8428</a> in web browser
and read [these docs](https://docs.victoriametrics.com/#operation).

There is also [VictoriaMetrics cluster](https://docs.victoriametrics.com/cluster-victoriametrics/)
- horizontally scalable installation, which scales to multiple nodes.

### Starting VictoriaMetrics Cluster via Docker

The following commands clone the latest available
[VictoriaMetrics repository](https://github.com/VictoriaMetrics/VictoriaMetrics)
and start the docker container via 'make docker-cluster-up'. Further customization is possible by editing
the [docker-compose-cluster.yml](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/deployment/docker/docker-compose-cluster.yml)
file.


```sh
git clone https://github.com/VictoriaMetrics/VictoriaMetrics && cd VictoriaMetrics
make docker-cluster-up
```


See more details [here](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#readme).

* [Cluster setup](https://docs.victoriametrics.com/cluster-victoriametrics/#cluster-setup)


### Starting VictoriaMetrics Single from a Binary

1. Download the correct archive from [github](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

For Open source it will look like

  ```victoria-metrics-<os>-<architecture>-v<version>.tar.gz```

For Enterprise it will look like

  `victoria-metrics-<os>-<architecture>-v<version>-enterprise.tar.gz`

In order for VictoriaMetrics Enterprise to start the, the -license flag must be set equal to a valid VictoriaMetrics key or the -licenseFile flag needs to point to a file containing your VictoriaMetrics license.

2. Extract the archive to /usr/local/bin by running

`sudo tar -xvf <victoriametrics-archive> -C /usr/local/bin`

Replace victoriametrics-archive with the path to the archive you downloaded in step 1

3. Create a VictoriaMetrics user on the system

`sudo useradd -s /usr/sbin/nologin victoriametrics`

4. Create a folder for storing VictoriaMetrics data

`mkdir -p /var/lib/victoria-metrics && chown -R victoriametrics:victoriametrics /var/lib/victoria-metrics`


5. Create a Linux Service by running the following

```bash
cat <<END >/etc/systemd/system/victoriametrics.service
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
END
```

If you want to deploy VictoriaMetrics single as a Windows Service review the [running as a windows service docs]({{< ref "/single-server-victoriametrics/#running-as-windows-service" >}})

6. Adjust the command line flags in the `ExecStart` line to fit your needs.

The list of command line flags for VMSingle can be found [here]({{< ref "single-server-victoriametrics/#list-of-command-line-flags" >}})

7. Start and enable the service by running

`sudo systemctl daemon-reload && sudo systemctl enable --now victoriametrics.service`

8. Check the that service started successfully 

`sudo systemctl status victoriametrics.service`

9. After VictoriaMetrics is Running verify VMUI is working by going to `http://<ip_or_hostname>:8428/vmui`


### Starting VictoriaMetrics Cluster from Binaries

On all nodes you will need to do the following

1. Download the archive that matches your operating system and processor architecture from [github releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)

For Open source it will look like

  `victoria-metrics-<os>-<architecture>-v<version>-cluster.tar.gz`

For Enterprise versions of VictoriaMetrics the ar will look like 

  `victoria-metrics-<os>-<architecture>-v<version>-enterprise-cluster.tar.gz`

In order for VictoriaMetrics Enterprise to start the, the -license flag must be set equal to a valid VictoriaMetrics key or the -licenseFile flag needs to point to a file containing your VictoriaMetrics license.

2. Extract the archive to /usr/local/bin by running

`sudo tar -xvf <victoriametrics-archive> -C /usr/local/bin`

Replace victoriametrics-archive with the path to the archive you downloaded in step 1

3. Create a user account for VictoriaMetrics

`sudo useradd -s /usr/sbin/nologin victoriametrics`

##### VMStorage

1. Create a folder for storing VictoriaMetrics data

`mkdir -p /var/lib/vmstorage && chown -R victoriametrics:victoriametrics /var/lib/vmstorage`

2. Create a Linux Service for VMStorage service by running 

```bash
cat <<END >/etc/systemd/system/vmstorage.service
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
END
```

3. Adjust the command line flags in the ExecStart line to fit your needs.

The list of command line flags for VMStorage can be found [here]({{< ref "cluster-victoriametrics/#list-of-command-line-flags-for-vmstorage" >}})

4. Start and Enable VMStorage

`sudo systemctl daemon-reload && systemctl enable --now vmstorage`

5. Check that the service is running

`sudo systemctl status vmstorage`

6. After VMStorage is running confirm the service healthy by going to


`http://<ip_or_hostname>:8482/-/healthy`

It should say "VictoriaMetrics is Healthy"

##### VMInsert

1. Create a Linux Service for VMInsert by running

```bash
cat << END >/etc/systemd/system/vminsert.service
[Unit]
Description=VictoriaMetrics vminsert service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
Restart=always
ExecStart=/usr/local/bin/vminsert-prod -replicationFactor=1 -storageNode=localhost

PrivateTmp=yes
NoNewPrivileges=yes
ProtectSystem=full

[Install]
WantedBy=multi-user.target
END
```

2. Adjust the command line flags in the ExecStart line to fit your needs.

The list of command line flags for VMInsert can be found [here]({{< ref "cluster-victoriametrics/#list-of-command-line-flags-for-vminsert" >}}).

3. Start and enable VMInsert

`sudo systemctl daemon-reload && sudo systemctl enable --now vminsert.service`

4. Make sure VMInsert is working

`sudo systemctl status vminsert.service`

5. After VMInsert is started you can confirm that is healthy by going to

`http://<ip_or_hostname>:8480/-/healthy`

It should say "VictoriaMetrics is Healthy"

##### VMSelect

1. Create a folder to store query cache data `sudo mkdir -p /var/lib/vmselect-cache && sudo chown -R victoriametrics:victoriametrics /var/lib/vmselect-cache`

2. Add a Linux Service for VMSelect by running

```bash
cat << END >/etc/systemd/system/vmselect.service
[Unit]
Description=VictoriaMetrics vmselect service
After=network.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
Restart=always
ExecStart=/usr/local/bin/vmselect-prod -storageNode localhost -cacheDataPath=/var/lib/vmselect-cache

PrivateTmp=yes
NoNewPrivileges=yes

ProtectSystem=full

[Install]
WantedBy=multi-user.target
END
```

3. Adjust the command line flags in the ExecStart line to fit your needs.

The list of command line flags for VMSelect can be found [here]({{< ref "cluster-victoriametrics/#list-of-command-line-flags-for-vmselect" >}})

4. Start and enable VMSelect

`sudo systemctl daemon-reload && sudo systemctl enable --now vmselect.service`

5. Make sure VMSelect is working

`sudo systemctl status vmselect.service`

6. After VMSelect is running you can verify it is working by going to VMUI located at

`http://<ip_or_hostname>:8481/select/vmui/vmui/`

## Write data

There are two main models in monitoring for data collection: 
[push](https://docs.victoriametrics.com/keyconcepts/#push-model) 
and [pull](https://docs.victoriametrics.com/keyconcepts/#pull-model). 
Both are used in modern monitoring and both are supported by VictoriaMetrics.

See more details on [writing data here](https://docs.victoriametrics.com/keyconcepts/#write-data).


## Query data

VictoriaMetrics provides an 
[HTTP API](https://docs.victoriametrics.com/single-server-victoriametrics/#prometheus-querying-api-usage)
for serving read queries. The API is used in various integrations such as
[Grafana](https://docs.victoriametrics.com/single-server-victoriametrics/#grafana-setup).
The same API is also used by
[VMUI](https://docs.victoriametrics.com/single-server-victoriametrics/#vmui) - graphical User Interface
for querying and visualizing metrics.

[MetricsQL](https://docs.victoriametrics.com/metricsql/) - is the query language for executing read queries
in VictoriaMetrics. MetricsQL is a [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics) 
-like query language with a powerful set of functions and features for working specifically with time series data.

See more details on [querying data here](https://docs.victoriametrics.com/keyconcepts/#query-data)


## Alerting

It is not possible to physically trace all changes on graphs all the time, that is why alerting exists.
In [vmalert](https://docs.victoriametrics.com/vmalert/) it is possible to create a set of conditions
based on PromQL and MetricsQL queries that will send a notification when such conditions are met.

## Data migration

Migrating data from other TSDBs to VictoriaMetrics is as simple as importing data via any of
[supported formats](https://docs.victoriametrics.com/keyconcepts/#push-model).

The migration might get easier when using [vmctl](https://docs.victoriametrics.com/vmctl/) - VictoriaMetrics
command line tool. It supports the following databases for migration to VictoriaMetrics:
* [Prometheus using snapshot API](https://docs.victoriametrics.com/vmctl/#migrating-data-from-prometheus);
* [Thanos](https://docs.victoriametrics.com/vmctl/#migrating-data-from-thanos);
* [InfluxDB](https://docs.victoriametrics.com/vmctl/#migrating-data-from-influxdb-1x);
* [OpenTSDB](https://docs.victoriametrics.com/vmctl/#migrating-data-from-opentsdb);
* [Migrate data between VictoriaMetrics single and cluster versions](https://docs.victoriametrics.com/vmctl/#migrating-data-from-victoriametrics).

## Productionization

When going to production with VictoriaMetrics we recommend following the recommendations.

### Monitoring

Each VictoriaMetrics component emits its own metrics with various details regarding performance
and health state. Docs for the components also contain a `Monitoring` section with an explanation
of what and how should be monitored. For example,
[Single-server-VictoriaMetrics Monitoring](https://docs.victoriametrics.com/cluster-victoriametrics/#monitoring).

VictoriaMetric team prepared a list of [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards)
for the main components. Each dashboard contains a lot of useful information and tips. It is recommended
to have these dashboards installed and up to date.

Using the [recommended alerting rules](https://github.com/VictoriaMetrics/VictoriaMetrics/tree/master/deployment/docker#alerts)
versions would also help to identify and notify about issues with the system.

The rule of thumb is to have a separate installation of VictoriaMetrics or any other monitoring system
to monitor the production installation of VictoriaMetrics. This would make monitoring independent and
will help identify problems with the main monitoring installation.

See more details in the article [VictoriaMetrics Monitoring](https://victoriametrics.com/blog/victoriametrics-monitoring/).

### Capacity planning

See capacity planning sections in [docs](https://docs.victoriametrics.com) for
[Single-server-VictoriaMetrics](https://docs.victoriametrics.com/single-server-victoriametrics/#capacity-planning)
and [VictoriaMetrics Cluster](https://docs.victoriametrics.com/cluster-victoriametrics/#capacity-planning).

Capacity planning isn't possible without [monitoring](#monitoring), so consider configuring it first.
Understanding resource usage and performance of VictoriaMetrics also requires knowing the tech terms
[active series](https://docs.victoriametrics.com/faq/#what-is-an-active-time-series),
[churn rate](https://docs.victoriametrics.com/faq/#what-is-high-churn-rate),
[cardinality](https://docs.victoriametrics.com/faq/#what-is-high-cardinality),
[slow inserts](https://docs.victoriametrics.com/faq/#what-is-a-slow-insert).
All of them are present in [Grafana dashboards](https://grafana.com/orgs/victoriametrics/dashboards).


### Data safety

It is recommended to read [Replication and data safety](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety),
[Why replication doesn’t save from disaster?](https://valyala.medium.com/speeding-up-backups-for-big-time-series-databases-533c1a927883)
and [backups](https://docs.victoriametrics.com/single-server-victoriametrics/#backups).


### Configuring limits

To avoid excessive resource usage or performance degradation limits must be in place:
* [Resource usage limits](https://docs.victoriametrics.com/faq/#how-to-set-a-memory-limit-for-victoriametrics-components);
* [Cardinality limiter](https://docs.victoriametrics.com/single-server-victoriametrics/#cardinality-limiter).

### Security recommendations

* [Security recommendations for single-node VictoriaMetrics](https://docs.victoriametrics.com/#security)
* [Security recommendations for cluster version of VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/#security)
