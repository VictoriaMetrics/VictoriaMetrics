---
sort: 20
---

# VM best practices 

VictoriaMetrics is a fast, cost-effective and scalable monitoring solution and time series database. It can be used as a long-term, remote storage for Prometheus which allows it to gather metrics from different systems and store them in a single location or separate them for different purposes (short-, long-term, responsibility zones etc). 

## Install Recommendation
  There is no need to tune VictoriaMetrics because it uses reasonable defaults for command-line flags.  These flags are automatically adjusted for the available CPU and RAM resources. There is no need for Operating System tuning because VictoriaMetrics is optimized for default OS settings. The only option is to increase the limit on the [number of open files in the OS](https://medium.com/@muhammadtriwibowo/set-permanently-ulimit-n-open-files-in-ubuntu-4d61064429a), so Prometheus instances could establish more connections to VictoriaMetrics (65535 standard production value).
## Filesystem Considerations 

  The recommended filesystem is ext4. If you plan to store more than 1TB of data on ext4 partition or plan to extend it to more than 16TB, then the following options are recommended to pass to mkfs.ext4:
mkfs.ext4 ... -O 64bit,huge_file,extent -T huge

## Operation System 
  When configuring VictoriaMetrics, the best practice is to use the latest Ubuntu OS version. 

## VictoriaMetrics Versions 
  Always update VictoriaMetrics instances in the environment to avoid version and build mismatch that will result in differences in performance and operational features. It is strongly recommended that you keep VictoriaMetrics in the environment up-to-date and install all VictoriaMetrics updates as soon as they are available. The best place to find the most recent updates as soon as they are available is to follow [this link](https://github.com/VictoriaMetrics/VictoriaMetrics/releases).

## Upgrade
  It is safe to upgrade VictoriaMetrics to new versions unless the [release notes](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) say otherwise. It is safe to skip multiple versions during the upgrade unless release notes say otherwise. It is recommended to perform regular upgrades to the latest version, since it may contain important bug fixes, performance optimizations or new features.
It is also safe to downgrade to the previous version unless release notes say otherwise.
The following steps must be performed during the upgrade / downgrade process:

  * Send SIGINT signal to VictoriaMetrics process so that it is stopped gracefully.
  * Wait until the process stops. This can take a few seconds. 
  * Start the upgraded VictoriaMetrics.
		
Prometheus doesn't drop data during the VictoriaMetrics restart. See [this article](https://grafana.com/blog/2019/03/25/whats-new-in-prometheus-2.8-wal-based-remote-write/) for details.
 
## Security
  Do not forget to protect sensitive endpoints in VictoriaMetrics when exposing them to untrusted networks such as the internet. Please consider setting the following command-line flags:
  
  * tls, -tlsCertFile and -tlsKeyFile for switching from HTTP to HTTPS.
  * httpAuth.username and -httpAuth.password for protecting all the HTTP endpoints with [HTTP Basic Authentication](https://en.wikipedia.org/wiki/Basic_access_authentication).
  * deleteAuthKey for protecting /api/v1/admin/tsdb/delete_series endpoint. See how to [delete time series](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-delete-time-series).
  * snapshotAuthKey for protecting /snapshot* endpoints. See [how to work with snapshots](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#how-to-work-with-snapshots).
  * forceMergeAuthKey for protecting /internal/force_merge endpoint. See [force merge docs](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#forced-merge).
  * search.resetCacheAuthKey for protecting /internal/resetRollupResultCache endpoint. See [backfilling](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#backfilling) for more details.
		
  Explicitly set internal network interface to TCP and UDP ports for data ingestion with Graphite and OpenTSDB formats. For example, substitute -graphiteListenAddr=:2003 with -graphiteListenAddr=<internal_iface_ip>:2003.
It is preferable to authorize all  incoming requests from untrusted networks with [vmauth](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmauth/README.md) or a similar auth proxy.

## Backup Recommendations 
  VictoriaMetrics supports backups via [vmbackup](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmbackup/README.md) and [vmrestore](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/app/vmrestore/README.md) tools. We also provide  the vmbackuper tool for our paid, enterprise subscribers - see [this issue](https://github.com/VictoriaMetrics/VictoriaMetrics/issues/466) for additional details. 

## Networking 
  Network usage: outbound traffic is negligible. Ingress traffic is ~100 bytes per ingested data point via [Prometheus remote_write API](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#remote_write). The actual ingress bandwidth usage depends on the average number of labels per ingested metric and the average size of label values. A higher number of per-metric labels and longer label values result inhigher ingress bandwidth.

## Storage Considerations 
  Storage space: VictoriaMetrics needs less than a byte per data point on average. So, ~260GB is required to store a month-long insert stream of 100K data points per second. The actual storage size depends largely on data randomness (entropy). Higher randomness means higher storage size requirements. Read [this article](https://medium.com/faun/victoriametrics-achieving-better-compression-for-time-series-data-than-gorilla-317bc1f95932) for details.

## RAM  
  RAM size: VictoriaMetrics needs less than 1KB per active time series. Therefore, ~1GB of RAM is required for 1M active time series. Time series are considered active if new data points have been added recently or if they have been recently queried. The number of active time series may be obtained from vm_cache_entries{type="storage/hour_metric_ids"} metric exported on the /metrics page. VictoriaMetrics stores various caches in RAM. Memory size for these caches may be limited with -memory.allowedPercent or -memory.allowedBytes flags.

## CPU
  CPU cores: VictoriaMetrics needs one CPU core per 300K inserted data points per second. So, ~4 CPU cores are required for processing the insert stream of 1M data points per second. The ingestion rate may be lower for high cardinality data or for time series with a high number of labels. See [this article](https://valyala.medium.com/insert-benchmarks-with-inch-influxdb-vs-victoriametrics-e31a41ae2893) for details. If you see lower numbers per CPU core, it is likely that the active time series info doesn't fit in your caches and you will need more RAM to lower CPU usage.
 
## Technical Support and Services
  If you have questions about installing or using this software pleasecheck this and other documents first. Answers to the most frequently askedquestions can be found on the Technical Papers webpage or in VictoriaMetrics community channels. If you need further assistance with VictoriaMetrics, please contact us at info@victoriametrics.com - we’ll be happy to help.

Following VictoriaMetrics best practices allows for the optimal configuration of our fast and scalable monitoring solution and time series database while minimizing or avoiding downtime or performance issues during installation and software usage. Our best practices also allow you to quickly troubleshoot any issues that might arise. 


