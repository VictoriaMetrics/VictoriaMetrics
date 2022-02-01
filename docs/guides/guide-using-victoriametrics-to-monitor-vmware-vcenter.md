# Using VictoriaMetrics to monitor VMware vCenter

**This guide covers:**

* How to monitor VMware vCenter
* Tools which can be used to monitor VMware vCenter
* How to scrape metrics from VMware vCenter/VMware ESXi with vmagent

**Precondition:**

We will use:

* [VMware vCenter](https://www.vmware.com/products/vcenter-server.html)
* [VMware ESXi](https://www.vmware.com/products/esxi-and-esx.html)
* [Telegraf](https://www.influxdata.com/time-series-platform/telegraf/)
* [vmvare_exporter](https://github.com/pryorda/vmware_exporter)
* [govc_exporter](https://github.com/Intrinsec/govc_exporter)
* [Ubuntu 20.04](https://ubuntu.com/download/desktop)
* [VictoriaMetrics Single](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html)
* [vmagent](https://docs.victoriametrics.com/vmagent.html)

## Choosing tsdb

In VMware vCenter, you can see the real-time metrics of virtual machines, which are stored since the virtual machine has been turned on or migrated for one hour. There is also historical data in a separate database. This is where they are written according to the Data Collection Levels settings. Changing the logging levels and intervals will lead to an increase in the database, degradation of performance. Fortunately, there is an API using which you can get the values of the required performance counters and save them in a separate database for further use.

**What tools can be used to monitor vCenter:**

* telegraf with [vsphere input plugin](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/vsphere)
* vmvare_exporter
* govc_exporter

> Tthe latest one is not in active development.

**Where the data can be written:**

* to the InfluxDB by telegraf;
* to the Prometheus by polling exporters;
* to the VictoriaMetrics by telegraf or by polling exporters because `vmagent` supported [both formats](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#prominent-features): influx and [prometheus](https://docs.victoriametrics.com/vmagent.html#how-to-collect-metrics-in-prometheus-format).

We will use both - telegraf and vmvare_exporter. As a result, all data will be saved in VictoriaMetrics. It is necessary not to forget about this feature:

> InfluxDB line protocol expects timestamps in nanoseconds by default, while VictoriaMetrics stores them with milliseconds precision.

A brief comparison of the exporter vmware_exporter and the Telegraph plugin below:
<table>
  <tbody>
    <tr>
      <th align="center" width="50%"><strong style="font-size: 20px;">Telegraf</strong></th>
      <th align="center" width="50%"><strong style="font-size: 20px;">vmware_exporter</strong></th>
    </tr>
    <tr>
      <td colspan="2" align="center"><strong>Labels and Tags</strong></td>
    </tr>
     <tr>
      <td width="50%">Metrics don’t contain any information about hypervisor or datastore but contain information only about the cluster. That is correct in terms of storing data in tsdb because the time series should not be interrupted by migrating a virtual machine to another hypervisor or datastore. Perfect but unexpected.
      </td>
      <td width="50%">It gives a lot of unnecessary information, but it can be useful. For example you can see when and where the virtual machine migrated.</td>
    </tr>
    <tr>
      <td colspan="2" align="center"><strong>Counters</strong></td>
    </tr>
    <tr>
      <td width="50%">All those specified in the configuration file are polled. </td>
      <td width="50%">
        <ol>
          <li>Embedded in the code.</li>
          <li>Only total values are written and there is no data on the specific core and network interface.</li>
          <li>IOPS counters saving is not configured. It polls but not the result.</li>
          <li>Out of the box save additional counters by number of vcpu, snapshots, data from guest OS by virtual machine.</li>
          <li>When the cloning process is running in VCenter, the counters will be polled but the exporter cannot convert them into metrics because of an error. The same happens when one of the hypervisors is unavailable.</li>
        </ol>
      </td>
    </tr>
    <tr>
      <td colspan="2" align="center"><strong>Custom attributes</strong></td>
    </tr>
    <tr>
      <td>If there is an attribute and it is empty, there will be no tag with it. </td>
      <td>If there is an attribute and it is empty, there will be a tag with the value n/a.</td>
    </tr>
    <tr>
      <td colspan="2" align="center"><strong>Installation</strong></td>
    </tr>
    <tr>
      <td>Binary file with understandable configuration files.</td>
      <td>Installation by python pip package manager  or docker with configuration example from readme.</td>
    </tr>
  </tbody>
</table>

## Performance counters

Through the UI in vCenter you can see different graphs for different counters. Sometimes it is not clear which one you need to poll. There is [govmomi](https://github.com/vmware/govmomi) for that. To see them

```bash
cd /tmp
wget https://github.com/vmware/govmomi/releases/download/v0.27.2/govc_Linux_x86_64.tar.gz
tar zxvf govc_Linux_x86_64.tar.gz
```

We need to set this variables to connect to vCenter by running them in terminal:

```bash
export GOVC_USERNAME=vmuser
export GOVC_PASSWORD=pswd
export GOVC_URL=vcenter-host
export GOVC_INSECURE=true
```

To configure telegraf we will need to set the correct interval. By default this value is 5 minutes.

Run this command to test it:

```bash
./govc metric.interval.info
```

The expected result will look like:

```bash
ID:                   300
  Enabled:            true
  Interval:           5m
  Available Samples:  288
  Name:               Past day
  Level:              1
ID:                   1800
  Enabled:            true
  Interval:           30m
  Available Samples:  336
  Name:               Past week
  Level:              1
ID:                   7200
  Enabled:            true
  Interval:           2h
  Available Samples:  360
  Name:               Past month
  Level:              1
ID:                   86400
  Enabled:            true
  Interval:           24h
  Available Samples:  365
  Name:               Past year
  Level:              1
```

Run this command to export list of available metrics to file `vm.out`:

```bash
./govc metric.ls /*/vm/* > vm.out
```

This command will show information about a specific meter `cpu.usage.average`:

```bash
./govc metric.info - cpu.usage.average
```

The result is similar to the example given in the documentation:

```bash
Name:                cpu.usage.average
  Label:             Usage
  Summary:           CPU usage as a percentage during the interval
  Group:             CPU
  Unit:              %
  Rollup type:       average
  Stats type:        rate
  Level:             1
    Intervals:       Past day,Past week,Past month,Past year
  Per-device level:  3
    Intervals:
```

## VictoriaMetrics

It is assumed that VictoriaMetrics(Cluster or Single) and vmagent are installed and running. The agent should be installed on hosts with vmware_exporter, telegraf, so that if the base is unavailable, the received metrics can be stored in the cache on the local disk. Here we need to decide how often metrics will be written to database - for example, every minute. Then minimal config for vmagent is the following:

```bash
global:
  scrape_interval: 60s
  scrape_timeout: 30s

scrape_configs:
  - job_name: vmagent
    static_configs:
    - targets:
      - exporters-host:8429
  - job_name: victoria-metrics
    static_configs:
    - targets:
      - victoriametrics-host:8428
```

> If you don't have pre-installed VictoriaMetrics and vmagen` you can install it with the next commands:

* this script will download and install the latest version of VictoriaMetrics Single on Ubuntu 20.04:

```bash
#!/bin/bash

set -e

apt update && apt upgrade -y && apt install -y curl wget net-tools jq
# Generate files
mkdir -p /etc/victoriametrics/single
mkdir -p /var/lib/victoria-metrics-data

# Create victoriametrics user
groupadd -r victoriametrics
useradd -g victoriametrics -d /var/lib/victoria-metrics-data -s /sbin/nologin --system victoriametrics
chown -R victoriametrics:victoriametrics /var/lib/victoria-metrics-data
chown -R victoriametrics:victoriametrics /etc/victoriametrics/single

# Install VictoriaMetrics Single
VM_VERSION=`curl -sg "https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/tags" | jq -r '.[0].name'`
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/${VM_VERSION}/victoria-metrics-amd64-${VM_VERSION}.tar.gz  -O /tmp/victoria-metrics.tar.gz
tar xvf /tmp/victoria-metrics.tar.gz -C /usr/bin
chmod +x /usr/bin/victoria-metrics-prod
chown root:root /usr/bin/victoria-metrics-prod

cat <<END >/etc/systemd/system/vmsingle.service
[Unit]
Description=VictoriaMetrics is a fast, cost-effective and scalable monitoring solution and time series database.
# https://docs.victoriametrics.com
After=network.target

[Install]
WantedBy=multi-user.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
WorkingDirectory=/var/lib/victoria-metrics-data
ReadWritePaths=/var/lib/victoria-metrics-data
StartLimitBurst=5
StartLimitInterval=0
Restart=on-failure
RestartSec=5
EnvironmentFile=-/etc/victoriametrics/single/victoriametrics.conf
ExecStart=/usr/bin/victoria-metrics-prod $ARGS
ExecStop=/bin/kill -s SIGTERM $MAINPID
ExecReload=/bin/kill -HUP $MAINPID
# See docs https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#tuning
ProtectSystem=full
LimitNOFILE=1048576
LimitNPROC=1048576
LimitCORE=infinity
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=vmsingle
END

cat <<END >/etc/victoriametrics/single/victoriametrics.conf
ARGS="-promscrape.config=/etc/victoriametrics/single/scrape.yml -storageDataPath=/var/lib/victoria-metrics-data -retentionPeriod=12 -httpListenAddr=:8428 -graphiteListenAddr=:2003 -opentsdbListenAddr=:4242 -influxListenAddr=:8089 -enableTCP6"
END

# Start VictoriaMetrics
systemctl enable vmsingle.service
systemctl restart vmsingle.service
```

* this script will download and install the latest version of vmagent on Ubuntu 20.04:

```bash
#!/bin/bash

set -e

apt update && apt upgrade -y && apt install -y curl wget net-tools jq

# Generate files
mkdir -p /etc/victoriametrics/vmagent/
mkdir -p /var/lib/vmagent-remotewrite-data

chown -R victoriametrics:victoriametrics /etc/victoriametrics/vmagent

# Install VictoriaMetrics Single
VM_VERSION=`curl -sg "https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/tags" | jq -r '.[0].name'`
wget https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/${VM_VERSION}/vmutils-amd64-${VM_VERSION}.tar.gz -O /tmp/vmutils.tar.gz
cd /tmp && tar -xzvf /tmp/vmutils.tar.gz vmagent-prod
mv /tmp/vmagent-prod /usr/bin
chmod +x /usr/bin/vmagent-prod
chown root:root /usr/bin/vmagent-prod

cat <<END >/etc/systemd/system/vmagent.service
[Unit]
Description=vmagent is a tiny agent which helps you collect metrics from various sources and store them in VictoriaMetrics.
# https://docs.victoriametrics.com/vmagent.html
After=network.target

[Install]
WantedBy=multi-user.target

[Service]
Type=simple
User=victoriametrics
Group=victoriametrics
StartLimitBurst=5
StartLimitInterval=0
Restart=on-failure
RestartSec=1
EnvironmentFile=-/etc/victoriametrics/vmagent/vmagent.conf
ExecStart=/usr/bin/vmagent-prod $ARGS
ExecStop=/bin/kill -s SIGTERM $MAINPID
ExecReload=/bin/kill -HUP $MAINPID
# See docs https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#tuning
LimitNOFILE=1048576
LimitNPROC=1048576
LimitCORE=infinity
WorkingDirectory=/var/lib/vmagent-remotewrite-data
ReadWritePaths=/var/lib/vmagent-remotewrite-data
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=vmagent
PrivateTmp=yes
ProtectHome=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectControlGroups=true
ProtectKernelModules=true
ProtectKernelTunables=yes
END

cat <<END >/etc/victoriametrics/vmagent/vmagent.conf
# https://docs.victoriametrics.com/vmagent.html
#
# Example command line:
# /path/to/vmagent -promscrape.config=/path/to/prometheus.yml -remoteWrite.url=https://victoria-metrics-host:8428/api/v1/write
#
# Please note that to write scraped data from vmagent to VictoriaMetrics Cluster you should use url like in this example -remoteWrite.url=http://vminsert-ip:8480/insert/0/prometheus/ .
# See more information here https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format.
# 
# If you only need to collect Influx data, then the following command is sufficient:
#
# /path/to/vmagent -remoteWrite.url=https://victoria-metrics-host:8428/api/v1/write
#
# Then send Influx data to http://vmagent-host:8429. See these https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#how-to-send-data-from-influxdb-compatible-agents-such-as-telegraf for more details.
ARGS="-promscrape.config=/etc/victoriametrics/vmagent/scrape.yml -remoteWrite.url=http://127.0.0.1:8428/api/v1/write -remoteWrite.tmpDataPath=/var/lib/vmagent-remotewrite-data -promscrape.suppressScrapeErrors -influxTrimTimestamp=1s -remoteWrite.urlRelabelConfig=/etc/victoriametrics/vmagent/telegraf.yml"
END

cat <<END >/etc/victoriametrics/vmagent/scrape.yml
# Scrape config example
#
global:
  scrape_interval: 60s
  scrape_timeout: 30s

scrape_configs:
  - job_name: vmagent
    static_configs:
    - targets: ['127.0.0.1:8429']
  - job_name: victoria-metrics
    static_configs:
    - targets: ['127.0.0.1:8428']
END

# Start VictoriaMetrics
systemctl enable vmagent.service
systemctl restart vmagent.service
```

vmagent can be run with flags or specify to use parameters from variables. For the second option `/etc/victoriametrics/vmagent/vmagent.conf` will look this:

telegraf.yml

Running from /etc/systemd/system/vmagent.service with [systemd daemon](https://ru.wikipedia.org/wiki/Systemd):

## vmware_exporter

To install vmware_exporter we will use pip.

To install in a user environment run the following command:

```bash
pip install vmware_exporter
```

Create `config.yml` file with the next configuration:

```bash
cat <<END >config.yml
default:
    vsphere_host: "vcenter-host"
    vsphere_user: "vmuser"
    vsphere_password: "pswd"
    ignore_ssl: True
    specs_size: 5000
    fetch_custom_attributes: True
    fetch_tags: False
    fetch_alarms: False
    collect_only:
        vms: True
        vmguests: True
        datastores: False
        hosts: False
        snapshots: True
END
```

Run `vmware_exporter`:

```bash
vmware_exporter -c config.yml
```

Check that exporter show metrics by running curl query:

```bash
curl http://localhost:9272/metrics
```

Add a job to poll the exporter in vmagent with the next configuration:

```bash
 - job_name: vcenter
    metric_relabel_configs:
    - regex: ^([A-Z]):.*
      replacement: $1
      source_labels:
      - partition
      target_label: partition
    - action: drop
      regex: /target|/storage/archive
      source_labels:
      - partition
    - action: labeldrop
      regex: host_name|ds_name
    static_configs:
    - targets:
      - exporters-host:9272
```

When polling the exporter the relabel rules will be applied to all received metrics namely:
the partition names of windows OS disks will have colons removed otherwise they will have to be escaped in queries;
all metrics with partitions in linux `/target` or `/storage/archive` will be removed because there is no sense to collect them;
remove `host_name` and `ds_name` labels to reduce the number of time series a.k.a. [churn rate](https://docs.victoriametrics.com/FAQ.html#what-is-high-churn-rate).

## telegraf

> The polling interval is specified in each configuration file or globally. It makes sense to make a lot of files because if the plugin does not have time to complete the work in the specified interval the data on all devices will be lost. When a problem occurs, the loss of monitoring data makes monitoring pointless. 

We need to [install telegraf](https://docs.influxdata.com/telegraf/v1.21/introduction/installation/) and create configuration file without unnecessary lines.

Run this command to create it:

```bash
telegraf --input-filter vsphere --output-filter influxdb config | grep -v '#' | grep -v '^$' > influx
```

Then apply changes to configuration but keep in mind that timestamp is rounded to seconds in precision, the interval is 1m and put the inputs section in a separate file. The final configuration in `/etc/telegraf/telegra.conf` will look like this:

```bash
[global_tags]
[agent]
  interval = "1m"
  round_interval = false
  metric_batch_size = 1000
  metric_buffer_limit = 100000
  collection_jitter = "0s"
  flush_interval = "10s"
  flush_jitter = "0s"
  precision = "s"
  hostname = ""
  omit_hostname = true
[[outputs.influxdb]]
  urls = ["http://127.0.0.1:8429"]
  exclude_database_tag = true
  skip_database_creation = true
```

and it will write the data to `vmagent` on port `8429`.

`hostname` tag not needed but `db` tag will be dropped by `vmagent`’s [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling) before writing to the database.

The vsphere plugin with default configuration polls a lot of data. It is necessary to make one configuration file for quick polling of virtual machine counters and if you need counters for cluster and hosts then make separate configuration files for them - this will allow to execute requests in parallel.

File `/etc/telegraf/telegraf.d/vcenter_vm.conf`:

```bash
[[inputs.vsphere]]
  vcenters = [ "https://vcenter-host/sdk" ]
  username = "vmuser"
  password = "pswd"
  tagexclude = [ "dcname", "source", "vcenter", "uuid", "moid" ]
  vm_metric_include = [
    "cpu.costop.summation",
    "cpu.ready.summation",
    "cpu.wait.summation",
    "cpu.run.summation",
    "cpu.idle.summation",
    "cpu.used.summation",
    "cpu.demand.average",
    "cpu.usagemhz.average",
    "cpu.usage.average",
    "mem.active.average",
    "mem.granted.average",
    "mem.consumed.average",
    "mem.usage.average",
    "mem.vmmemctl.average",
    "net.bytesRx.average",
    "net.bytesTx.average",
    "net.droppedRx.summation",
    "net.droppedTx.summation",
    "net.usage.average",
    "power.power.average",
    "virtualDisk.numberReadAveraged.average",
    "virtualDisk.numberWriteAveraged.average",
    "virtualDisk.read.average",
    "virtualDisk.readOIO.latest",
    "virtualDisk.throughput.usage.average",
    "virtualDisk.totalReadLatency.average",
    "virtualDisk.totalWriteLatency.average",
    "virtualDisk.write.average",
    "virtualDisk.writeOIO.latest",
    "sys.uptime.latest",
  ]
  host_include = []
  host_exclude = ["*"]
  cluster_include = []
  cluster_exclude = ["*"]
  datastore_include = []
  datastore_exclude = ["*"]
  datacenter_include = []
  datacenter_exclude = ["*"]
  separator = "_"
  collect_concurrency = 10
  discover_concurrency = 10
  custom_attribute_include = ["*"]
  custom_attribute_exclude = []
  insecure_skip_verify = true
  historical_interval = "5m"

[inputs.vsphere.tags]
 job = "vmware_vm"
```

There will be a separate job tag for each plugin/file similar to other exporters. Directly here `job="vmware_vm"` will be added to each metric. The tags specified in `tagexclude` will be removed from each metric. The `historical_interval` defaults to 5 minutes and matches the interval from `metric.interval.info`. All custom attributes, if any - then they will be used as tags.

To check that config file works run this command:

```bash
 telegraf --config /etc/telegraf/telegraf.d/vcenter_vm.conf --test > /tmp/vcenter_vm.out
```

This command will create a file with output information from vCenter.
After installation telegraf is up and running with the default config file. The relabel config `/etc/victoriametrics/vmagent/telegraf.yml`:

```yaml
---
# rename label
- action: labelmap_all
  regex: "vmname"
  replacement: "vm_name"
# remove labels
- action: labeldrop
  regex:
    - "db"
```

must be configured before restarting.

## VMUI

vmagent polls vmware_exporter once a minute and receives metrics from telegraf via influx protocol. You can go to `http://victoriametrics-host:8428/vmui/` in the instant query table and make queries to see what telegraf has collected. For example run the following query:

```bash
max({job="vmware_vm"}) by  (__name__)
```

To see what data was collected by `vmware_exporter` run the following query:

```bash
max({job="vcenter"}) by  (__name__)
```

## Grafana

To see the graphs we need to:
[install grafana](https://grafana.com/docs/grafana/latest/installation/);
[add Prometheus datasource](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#grafana-setup) with url `http://victoriametrics-host:8428`;
create or add ready-made dashboards. Note that dashboards for influx will have to be redone completely.

## Kapacitor

Not needed.

## [vmalert](https://docs.victoriametrics.com/vmalert.html)

It is recommend to use `vmalert` for alerting from [VictoriaMetrics](https://docs.victoriametrics.com/Single-server-VictoriaMetrics.html#alerting)

Here is an example of alert for our setup:

```yaml
---
groups:
  - name: vmware
    rules:
      - alert: VMMemoryBalooning
        expr: vmware_vm_mem_vmmemctl_average > 0
        for: 1h
        labels:
          severity: info
        annotations:
          summary: "Balooning {{ $labels.vm_name }}"
      - alert: VMDiskLowSpace
        expr: vmware_vm_guest_disk_free{partition="C"} < 10000000000
        for: 5m
        labels:
          severity: warning
          partition: C
        annotations:
          summary: 'Low disk space {{ $labels.partition }} {{ $labels.vm_name }}'
```

See [vmware.rules.yml](https://github.com/pryorda/vmware_exporter/blob/main/alerts/vmware.rules.yml) to get more examples of alerts. Similar rules can be created for Telegraf metrics.