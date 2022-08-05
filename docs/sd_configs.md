---
sort: 24
---

# Prometheus service discovery

[vmagent](https://docs.victoriametrics.com/vmagent.html) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter) supports the following Prometheus-compatible service discovery options for Prometheus-compatible scrape targets in the file pointed by `-promscrape.config` command-line flag.

* `azure_sd_configs` is for scraping the targets registered in [Azure Cloud](https://azure.microsoft.com/en-us/). See [these docs](#azure_sd_configs) for details.
* `consul_sd_configs` is for discovering and scraping targets registered in [Consul](https://www.consul.io/). See [these docs](#consul_sd_configs) for details.
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
* `yandexcloud_sd_configs` is for discoverying and scraping [Yandex Cloud](https://cloud.yandex.com/en/) targets. See [these docs](#yandexcloud_sd_configs) for details.

Note that the `refresh_interval` option isn't supported for these scrape configs. Use the corresponding `-promscrape.*CheckInterval`
command-line flag instead. For example, `-promscrape.consulSDCheckInterval=60s` sets `refresh_interval` for all the `consul_sd_configs`
entries to 60s. Run `vmagent -help` or `victoria-metrics -help` in order to see default values for the `-promscrape.*CheckInterval` flags.

Please file feature requests to [our issue tracker](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you need other service discovery mechanisms to be supported by VictoriaMetrics and `vmagent`.

## azure_sd_configs

Azure SD configurations allow retrieving scrape targets from [Microsoft Azure](https://azure.microsoft.com/en-us/) VMs.

The following meta labels are available on targets during relabeling:

* `__meta_azure_machine_id`: the machine ID
* `__meta_azure_machine_location`: the location the machine runs in
* `__meta_azure_machine_name`: the machine name
* `__meta_azure_machine_computer_name`: the machine computer name
* `__meta_azure_machine_os_type`: the machine operating system
* `__meta_azure_machine_private_ip`: the machine's private IP
* `__meta_azure_machine_public_ip`: the machine's public IP if it exists
* `__meta_azure_machine_resource_group`: the machine's resource group
* `__meta_azure_machine_tag_<tagname>`: each tag value of the machine
* `__meta_azure_machine_scale_set`: the name of the scale set which the vm is part of (this value is only set if you are using a scale set)
* `__meta_azure_subscription_id`: the subscription ID
* `__meta_azure_tenant_id`: the tenant ID

Configuration example:

```yaml
scrape_configs:
- job_name: azure
  azure_sd_configs:
    # subscription_id is a mandatory subscription ID.
    subscription_id: "..."

    # environment is an optional Azure environment. By default "AzurePublicCloud" is used.
    # environment: ...

    # authentication_method is an optional authentication method, either OAuth or ManagedIdentity.
    # See https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview
    # By default OAuth is used.
    # authentication_method: ...

    # tenant_id is an optional tenant ID. Only required with authentication_method OAuth.
    # tenant_id: "..."

    # client_id is an optional client ID. Only required with authentication_method OAuth.
    # client_id: "..."

    # client_secret is an optional client secret. Only required with authentication_method OAuth.
    # client_secret: "..."

    # resource_group is an optional resource group name. Limits discovery to this resource group. 
    # resource_group: "..."

    # port is an optional port to scrape metrics from.
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## consul_sd_configs

Consul SD configurations allow retrieving scrape targets from [Consul's Catalog API](https://www.consul.io/api-docs/catalog).

The following meta labels are available on targets during relabeling:

* `__meta_consul_address`: the address of the target
* `__meta_consul_dc`: the datacenter name for the target
* `__meta_consul_health`: the health status of the service
* `__meta_consul_metadata_<key>`: each node metadata key value of the target
* `__meta_consul_node`: the node name defined for the target
* `__meta_consul_service_address`: the service address of the target
* `__meta_consul_service_id`: the service ID of the target
* `__meta_consul_service_metadata_<key>`: each service metadata key value of the target
* `__meta_consul_service_port`: the service port of the target
* `__meta_consul_service`: the name of the service the target belongs to
* `__meta_consul_tagged_address_<key>`: each node tagged address key value of the target
* `__meta_consul_tags`: the list of tags of the target joined by the tag separator

Configuration example:

```yaml
scrape_configs:
- job_name: consul
  consul_sd_configs:
    # server is an optional Consul server to connect to. By default localhost:8500 is used
    # server: "..."

    # token is an optional Consul API token.
    # If the token isn't specified, then it is read from a file pointed by CONSUL_HTTP_TOKEN_FILE
    # environment var or from the CONSUL_HTTP_TOKEN environment var.
    # token: "..."

    # datacenter is an optional Consul API datacenter.
    # If the datacenter isn't specified, then it is read from Consul server.
    # See https://www.consul.io/api-docs/agent#read-configuration
    # datacenter: "..."

    # namespace is an optional Consul namespace.
    # If the namespace isn't specified, then it is read from CONSUL_NAMESPACE environment var.
    # namespace: "..."

    # scheme is an optional scheme (http or https) to use for connecting to Consul server.
    # By default http scheme is used.
    # scheme: "..."

    # services is an optional list of services for which targets are retrieved.
    # If omitted, all services are scraped.
    # See https://www.consul.io/api-docs/catalog#list-nodes-for-service for details.
    # services: ["...", "..."]

    # tags is an optional list of tags used to filter nodes for a given service.
    # Services must contain all tags in the list.
    # tags: ["...", "..."]

    # node_meta is an optional node metadata key/value pairs to filter nodes for a given service.
    # node_meta:
    #   "...": "..."

    # tag_separate is an optional string by which Consul tags are joined into the __meta_consul_tags label.
    # By default "," is used as a tag separator.
    # tag_separator: "..."

    # allow_stale is an optional config, which allows stale Consul results.
    # See https://www.consul.io/api/features/consistency.html
    # Reduce load on Consul if set to true. By default is is set to true.
    # allow_stale: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## yandexcloud_sd_configs

[Yandex Cloud](https://cloud.yandex.com/en/) SD configurations allow retrieving scrape targets from accessible folders.

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

Configuration example:

```yaml
scrape_configs:
- job_name: yandexcloud
  yandexcloud_sd_configs:
    # service is a mandatory option for yandexcloud service discovery
    # currently only "compute" service is supported
  - service: compute

    # api_endpoint is an optional API endpoint for service discovery
    # The https://api.cloud.yandex.net endpoint is used by default.
    # api_endpoint: "https://api.cloud.yandex.net"

    # yandex_passport_oauth_token is an optional OAuth token
    # for querying yandexcloud API. See https://cloud.yandex.com/en-ru/docs/iam/concepts/authorization/oauth-token
    # yandex_passport_oauth_token: "..."

    # tls_config is an optional tls config.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    # tls_config:
    #   ...
```

Yandex Cloud SD support both user [OAuth token](https://cloud.yandex.com/en-ru/docs/iam/concepts/authorization/oauth-token)
and [instance service account](https://cloud.yandex.com/en-ru/docs/compute/operations/vm-connect/auth-inside-vm) if `yandex_passport_oauth_token` is omitted:

```yaml
scrape_configs:
- job_name: YC_with_oauth
  yandexcloud_sd_configs:
  - service: compute
    yandex_passport_oauth_token: "AQAAAAAsfasah<...>7E10SaotuL0"
  relabel_configs:
  - source_labels: [__meta_yandexcloud_instance_public_ip_0]
    target_label: __address__
    replacement: "$1:9100"

- job_name: YC_with_Instance_service_account
  yandexcloud_sd_configs:
  - service: compute
  relabel_configs:
  - source_labels: [__meta_yandexcloud_instance_private_ip_0]
    target_label: __address__
    replacement: "$1:9100"
```


## HTTP API client options

The following additional options can be specified in the majority of supported service discovery types:

```yaml
    # authorization is an optional `Authorization` header configuration.
    # authorization:
    #   type: "..."  # default: Bearer
    #   credentials: "..."
    #   credentials_file: "..."

    # basic_auth is an optional HTTP basic authentication configuration.
    # basic_auth:
    #   username: "..."
    #   password: "..."
    #   password_file: "..."

    # bearer_token is an optional Bearer token to send in every HTTP API request during service discovery.
    # bearer_token: "..."

    # bearer_token_file is an optional path to file with Bearer token to send
    # in every HTTP API request during service discovery.
    # The file is re-read every second, so its contents can be updated without the need to restart the service discovery.
    # bearer_token_file: "..."

    # oauth2 is an optional OAuth 2.0 configuration.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2
    # oauth2:
    #   ...

    # tls_config is an optional TLS configuration.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    # tls_config:
    #   ...

    # proxy_url is an optional URL for the proxy to use for HTTP API queries during service discovery.
    # proxy_url: "..."

    # proxy_authorization is an optional `Authorization` header config for the proxy_url.
    # proxy_authorization:
    #   type: "..."  # default: Bearer
    #   credentials: "..."
    #   credentials_file: "..."

    # proxy_basic_auth is an optional HTTP basic authentication configuration for the proxy_url.
    # proxy_basic_auth:
    #   username: "..."
    #   password: "..."
    #   password_file: "..."

    # proxy_bearer_token is an optional Bearer token to send to proxy_url.
    # proxy_bearer_token: "..."

    # proxy_bearer_token_file is an optional path to file with Bearer token to send to proxy_url.
    # The file is re-read every second, so its contents can be updated without the need to restart the service discovery.
    # proxy_bearer_token_file: "..."

    # proxy_oauth2 is an optional OAuth 2.0 configuration for the proxy_url.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2
    # proxy_oauth2:
    #   ...

    # proxy_tls_config is an optional TLS configuration for the proxy_url.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    # proxy_tls_config:
    #   ...
```
