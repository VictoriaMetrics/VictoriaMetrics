## Yandex Cloud Service Discovery Configs

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