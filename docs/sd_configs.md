# Prometheus service discovery

[vmagent](https://docs.victoriametrics.com/vmagent.html) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter) supports the following Prometheus-compatible service discovery options for Prometheus-compatible scrape targets in the file pointed by `-promscrape.config` command-line flag.

* `azure_sd_configs` - is for scraping the targets registered in Azure Cloud.
  See [azure_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#azure_sd_config) for details.
* `consul_sd_configs` is for discovering and scraping targets registered in Consul. See [consul_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#consul_sd_config) for details.
* `digitalocean_sd_configs` is for discovering and scraping targerts registered in [DigitalOcean](https://www.digitalocean.com/). See [digitalocean_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config) for details.
* `dns_sd_configs` is for discovering and scraping targets from DNS records (SRV, A and AAAA). See [dns_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config) for details.
* `docker_sd_configs` is for discovering and scraping Docker targets. See [docker_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#docker_sd_config) for details.
* `dockerswarm_sd_configs` is for discovering and scraping Docker Swarm targets. See [dockerswarm_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dockerswarm_sd_config) for details.
* `ec2_sd_configs` is for discovering and scraping Amazon EC2 targets. See [ec2_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#ec2_sd_config) for details. `vmagent` doesn't support the `profile` config param yet.
* `eureka_sd_configs` is for discovering and scraping targets registered in [Netflix Eureka](https://github.com/Netflix/eureka). See [eureka_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#eureka_sd_config) for details.
* `file_sd_configs` is for scraping targets defined in external files (aka file-based service discovery). See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config) for details.
* `gce_sd_configs` is for discovering and scraping Google Compute Engine (GCE) targets. See [gce_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#gce_sd_config) for details. `vmagent` provides the following additional functionality for `gce_sd_config`:
  * if `project` arg is missing then `vmagent` uses the project for the instance where it runs;
  * if `zone` arg is missing then `vmagent` uses the zone for the instance where it runs;
  * if `zone` arg equals to `"*"`, then `vmagent` discovers all the zones for the given project;
  * `zone` may contain a list of zones, i.e. `zone: [us-east1-a, us-east1-b]`.
* `http_sd_configs` is for discovering and scraping targerts provided by external http-based service discovery. See [http_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#http_sd_config) for details.
* `kubernetes_sd_configs` is for discovering and scraping Kubernetes (K8S) targets. See [kubernetes_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) for details.
* `openstack_sd_configs` is for discovering and scraping OpenStack targets. See [openstack_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config) for details. [OpenStack identity API v3](https://docs.openstack.org/api-ref/identity/v3/) is supported only.
* `static_configs` is for scraping statically defined targets. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config) for details.
* `yandexcloud_sd_configs` is for discoverying and scraping [Yandex Cloud](https://cloud.yandex.com/en/) targets. See [these docs](#yandexcloud-sd-configs) for details.

Note that the `refresh_interval` option isn't supported for these scrape configs. Use the corresponding `-promscrape.*CheckInterval`
command-line flag instead. For example, `-promscrape.consulSDCheckInterval=60s` sets `refresh_interval` for all the `consul_sd_configs`
entries to 60s. Run `vmagent -help` or `victoria-metrics -help` in order to see default values for the `-promscrape.*CheckInterval` flags.

Please file feature requests to [our issue tracker](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you need other service discovery mechanisms to be supported by VictoriaMetrics and `vmagent`.


## yandexcloud_sd_configs

Yandex Cloud SD configurations allow retrieving scrape targets from accessible folders.

Only compute instances currently supported and the following meta labels are available on targets during relabeling:

* `__meta_yandexcloud_instance_name`: the name of instance
* `__meta_yandexcloud_instance_id`: the id of instance
* `__meta_yandexcloud_instance_fqdn`: generated FQDN for instance
* `__meta_yandexcloud_instance_status`: the status of instance
* `__meta_yandexcloud_instance_platform_id`: instance platform ID (i.e. "standard-v3")
* `__meta_yandexcloud_instance_resources_cores`: instance vCPU cores
* `__meta_yandexcloud_instance_resources_core_fraction`: instance core fraction
* `__meta_yandexcloud_instance_resources_memory`: instance memory
* `__meta_yandexcloud_folder_id`: instance folder ID
* `__meta_yandexcloud_instance_label_<label name>`: each label from instance
* `__meta_yandexcloud_instance_private_ip_<interface index>`: private IP of <interface index> network interface
* `__meta_yandexcloud_instance_public_ip_<interface index>`: public (NAT) IP of <interface index> network interface
* `__meta_yandexcloud_instance_private_dns_<record number>`: if configured DNS records for private IP
* `__meta_yandexcloud_instance_public_dns_<record number>`: if configured DNS records for public IP

Yandex Cloud SD support both user [OAuth token](https://cloud.yandex.com/en-ru/docs/iam/concepts/authorization/oauth-token) and [instance service account](https://cloud.yandex.com/en-ru/docs/compute/operations/vm-connect/auth-inside-vm) if OAuth is omitted.

```yaml
---
global:
  scrape_interval: 10s

scrape_configs:
  - job_name: YC_with_oauth
    yandexcloud_sd_configs:
      - service: "compute"
        yandex_passport_oauth_token: "AQAAAAAsfasah<...>7E10SaotuL0"
    relabel_configs:
      - source_labels: [__meta_yandexcloud_instance_public_ip_0]
        target_label: __address__
        replacement: "$1:9100"

  - job_name: YC_with_Instance_service_account
    yandexcloud_sd_configs:
      - service: "compute"
    relabel_configs:
      - source_labels: [__meta_yandexcloud_instance_private_ip_0]
        target_label: __address__
        replacement: "$1:9100"
```
