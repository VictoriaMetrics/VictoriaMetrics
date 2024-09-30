---
weight: 36
title: Prometheus service discovery
menu:
  docs:
    parent: 'victoriametrics'
    weight: 36
aliases:
- /sd_configs.html
---
# Supported service discovery configs

[vmagent](https://docs.victoriametrics.com/vmagent/) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter)
supports the following Prometheus-compatible service discovery options for Prometheus-compatible scrape targets in the file pointed by `-promscrape.config` command-line flag:

* `azure_sd_configs` is for scraping the targets registered in [Azure Cloud](https://azure.microsoft.com/en-us/). See [these docs](#azure_sd_configs).
* `consul_sd_configs` is for discovering and scraping targets registered in [Consul](https://www.consul.io/). See [these docs](#consul_sd_configs).
* `consulagent_sd_configs` is for discovering and scraping targets registered in [Consul Agent](https://developer.hashicorp.com/consul/api-docs/agent/service). See [these docs](#consulagent_sd_configs).
* `digitalocean_sd_configs` is for discovering and scraping targets registered in [DigitalOcean](https://www.digitalocean.com/). See [these docs](#digitalocean_sd_configs).
* `dns_sd_configs` is for discovering and scraping targets from [DNS](https://it.wikipedia.org/wiki/Domain_Name_System) records (SRV, A and AAAA). See [these docs](#dns_sd_configs).
* `docker_sd_configs` is for discovering and scraping [Docker](https://www.docker.com/) targets. See [these docs](#docker_sd_configs).
* `dockerswarm_sd_configs` is for discovering and scraping [Docker Swarm](https://docs.docker.com/engine/swarm/) targets. See [these docs](#dockerswarm_sd_configs).
* `ec2_sd_configs` is for discovering and scraping [Amazon EC2](https://aws.amazon.com/ec2/) targets. See [these docs](#ec2_sd_configs).
* `eureka_sd_configs` is for discovering and scraping targets registered in [Netflix Eureka](https://github.com/Netflix/eureka). See [these docs](#eureka_sd_configs).
* `file_sd_configs` is for scraping targets defined in external files (aka file-based service discovery). See [these docs](#file_sd_configs).
* `gce_sd_configs` is for discovering and scraping [Google Compute Engine](https://cloud.google.com/compute) targets. See [these docs](#gce_sd_configs).
* `hetzner_sd_configs` is for discovering and scraping [Hetzner Cloud](https://www.hetzner.com/cloud) and [Hetzner Robot](https://docs.hetzner.com/robot) targets. See [these docs](#hetzner_sd_configs).
* `http_sd_configs` is for discovering and scraping targets provided by external http-based service discovery. See [these docs](#http_sd_configs).
* `kubernetes_sd_configs` is for discovering and scraping [Kubernetes](https://kubernetes.io/) targets. See [these docs](#kubernetes_sd_configs).
* `kuma_sd_configs` is for discovering and scraping [Kuma](https://kuma.io) targets. See [these docs](#kuma_sd_configs).
* `nomad_sd_configs` is for discovering and scraping targets registered in [HashiCorp Nomad](https://www.nomadproject.io/). See [these docs](#nomad_sd_configs).
* `openstack_sd_configs` is for discovering and scraping OpenStack targets. See [these docs](#openstack_sd_configs).
* `ovhcloud_sd_configs` is for discovering and scraping OVH Cloud VPS and dedicated server targets. See [these docs](#ovhcloud_sd_configs).
* `static_configs` is for scraping statically defined targets. See [these docs](#static_configs).
* `vultr_sd_configs` is for discovering and scraping [Vultr](https://www.vultr.com/) targets. See [these docs](#vultr_sd_configs).
* `yandexcloud_sd_configs` is for discovering and scraping [Yandex Cloud](https://cloud.yandex.com/en/) targets. See [these docs](#yandexcloud_sd_configs).

Note that the `refresh_interval` option isn't supported for these scrape configs. Use the corresponding `-promscrape.*CheckInterval`
command-line flag instead. For example, `-promscrape.consulSDCheckInterval=60s` sets `refresh_interval` for all the `consul_sd_configs`
entries to 60s. Run `vmagent -help` or `victoria-metrics -help` in order to see default values for the `-promscrape.*CheckInterval` flags.

Please file feature requests to [our issue tracker](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you need other service discovery mechanisms
to be supported by VictoriaMetrics and `vmagent`.

## azure_sd_configs

Azure SD configuration discovers scrape targets from [Microsoft Azure](https://azure.microsoft.com/en-us/) VMs.

Configuration example:

```yaml
scrape_configs:
- job_name: azure
  azure_sd_configs:

    # subscription_id is a mandatory subscription ID.
    #
  - subscription_id: "..."

    # environment is an optional Azure environment. By default "AzurePublicCloud" is used.
    #
    # environment: "..."

    # authentication_method is an optional authentication method, either OAuth or ManagedIdentity.
    # See https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/overview
    # By default OAuth is used.
    #
    # authentication_method: "..."

    # tenant_id is an optional tenant ID. Only required with authentication_method OAuth.
    #
    # tenant_id: "..."

    # client_id is an optional client ID. Only required with authentication_method OAuth.
    #
    # client_id: "..."

    # client_secret is an optional client secret. Only required with authentication_method OAuth.
    #
    # client_secret: "..."

    # resource_group is an optional resource group name. Limits discovery to this resource group. 
    #
    # resource_group: "..."

    # port is an optional port to scrape metrics from.
    # Port 80 is used by default.
    #
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<private_ip>:<port>`, where `<private_ip>` is the machine's private IP and the `<port>` is the `port`
option specified in the `azure_sd_configs`.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_azure_machine_id`: the machine ID
* `__meta_azure_machine_location`: the location the machine runs in
* `__meta_azure_machine_name`: the machine name
* `__meta_azure_machine_computer_name`: the machine computer name
* `__meta_azure_machine_os_type`: the machine operating system
* `__meta_azure_machine_private_ip`: the machine's private IP
* `__meta_azure_machine_public_ip`: the machine's public IP if it exists
* `__meta_azure_machine_resource_group`: the machine's resource group
* `__meta_azure_machine_scale_set`: the name of the scale set which the vm is part of (this value is only set if you are using a scale set)
* `__meta_azure_machine_size`: the machine size
* `__meta_azure_machine_tag_<tagname>`: each tag value of the machine
* `__meta_azure_subscription_id`: the subscription ID
* `__meta_azure_tenant_id`: the tenant ID

The list of discovered Azure targets is refreshed at the interval, which can be configured via `-promscrape.azureSDCheckInterval` command-line flag.

## consul_sd_configs

Consul SD configuration allows retrieving scrape targets from [Consul's Catalog API](https://www.consul.io/api-docs/catalog).

Configuration example:

```yaml
scrape_configs:
- job_name: consul
  consul_sd_configs:

    # server is an optional Consul server to connect to. By default, localhost:8500 is used
    #
  - server: "localhost:8500"

    # token is an optional Consul API token.
    # If the token isn't specified, then it is read from a file pointed by CONSUL_HTTP_TOKEN_FILE
    # environment var or from the CONSUL_HTTP_TOKEN environment var.
    #
    # token: "..."

    # datacenter is an optional Consul API datacenter.
    # If the datacenter isn't specified, then it is read from Consul server.
    # See https://www.consul.io/api-docs/agent#read-configuration
    #
    # datacenter: "..."

    # namespace is an optional Consul namespace.
    # See https://developer.hashicorp.com/consul/docs/enterprise/namespaces
    # If the namespace isn't specified, then it is read from CONSUL_NAMESPACE environment var.
    #
    # namespace: "..."

    # partition is an optional Consul partition.
    # See https://developer.hashicorp.com/consul/docs/enterprise/admin-partitions
    # If partition isn't specified, then the default partition is used.
    #
    # partition: "..."

    # scheme is an optional scheme (http or https) to use for connecting to Consul server.
    # By default, http scheme is used.
    #
    # scheme: "..."

    # services is an optional list of services for which targets are retrieved.
    # If omitted, all services are scraped.
    # See https://www.consul.io/api-docs/catalog#list-nodes-for-service .
    #
    # services: ["...", "..."]

    # tags is an optional list of tags used to filter nodes for a given service.
    # Services must contain all tags in the list.
    # Deprecated: use filter instead with ServiceTags selector.
    #
    # tags: ["...", "..."]

    # node_meta is an optional node metadata key/value pairs to filter nodes for a given service.
    # Deprecated: use filter instead with NodeMeta selector.
    #
    # node_meta:
    #   "...": "..."

    # tag_separator is an optional string by which Consul tags are joined into the __meta_consul_tags label.
    # By default, "," is used as a tag separator.
    # Individual tags are also available via __meta_consul_tag_<tagname> labels - see below.
    #
    # tag_separator: "..."

    # filter is an optional filter for service discovery.
    # Replaces tags and node_meta options.
    # Consul supports it since 1.14 version.
    # See the list of supported filters at https://developer.hashicorp.com/consul/api-docs/catalog#filtering-1
    # See filter examples at https://developer.hashicorp.com/consul/api-docs/features/filtering
    #
    # filter: "..."

    # allow_stale is an optional config, which allows stale Consul results.
    # See https://developer.hashicorp.com/consul/api-docs/features/consistency
    # Reduce load on Consul if set to true. By default, it is set to true.
    #
    # allow_stale: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<service_or_node_addr>:<service_port>`, where `<service_or_node_addr>` is the service address. If the service address is empty,
then the node address is used instead. The `<service_port>` is the service port.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_consul_address`: the address of the target
* `__meta_consul_dc`: the datacenter name for the target
* `__meta_consul_health`: the health status of the service
* `__meta_consul_metadata_<key>`: each node metadata key value of the target
* `__meta_consul_namespace`: namespace of the service - see [namespace docs](https://developer.hashicorp.com/consul/docs/enterprise/namespaces)
* `__meta_consul_node`: the node name defined for the target
* `__meta_consul_partition`: partition of the service - see [partition docs](https://developer.hashicorp.com/consul/docs/enterprise/admin-partitions)
* `__meta_consul_service_address`: the service address of the target
* `__meta_consul_service_id`: the service ID of the target
* `__meta_consul_service_metadata_<key>`: each service metadata key value of the target
* `__meta_consul_service_port`: the service port of the target
* `__meta_consul_service`: the name of the service the target belongs to
* `__meta_consul_tagged_address_<key>`: each node tagged address key value of the target
* `__meta_consul_tag_<tagname>`: the value for the given <tagname> tag of the target
* `__meta_consul_tagpresent_<tagname>`: "true" for every <tagname> tag of the target
* `__meta_consul_tags`: the list of tags of the target joined by the `tag_separator`

The list of discovered Consul targets is refreshed at the interval, which can be configured via `-promscrape.consulSDCheckInterval` command-line flag.

If you have performance issues with `consul_sd_configs` on a large cluster, then consider using [consulagent_sd_configs](#consulagent_sd_configs) instead.

## consulagent_sd_configs

Consul Agent SD configuration allows retrieving scrape targets from [Consul Agent API](https://developer.hashicorp.com/consul/api-docs/agent/service).
When using the Agent API, only services registered in the locally running Consul Agent are discovered.
It is suitable for huge clusters for which using the [Catalog API](https://developer.hashicorp.com/consul/api-docs/catalog#list-services) would be too slow or resource intensive,
in other cases it is recommended to use [consul_sd_configs](#consul_sd_configs).

Configuration example:

```yaml
scrape_configs:
- job_name: consulagent
  consulagent_sd_configs:

    # server is an optional Consul Agent to connect to. By default, localhost:8500 is used
    #
  - server: "localhost:8500"

    # token is an optional Consul API token.
    # If the token isn't specified, then it is read from a file pointed by CONSUL_HTTP_TOKEN_FILE
    # environment var or from the CONSUL_HTTP_TOKEN environment var.
    #
    # token: "..."

    # datacenter is an optional Consul API datacenter.
    # If the datacenter isn't specified, then it is read from Consul server.
    # See https://www.consul.io/api-docs/agent#read-configuration
    #
    # datacenter: "..."

    # namespace is an optional Consul namespace.
    # See https://developer.hashicorp.com/consul/docs/enterprise/namespaces
    # If the namespace isn't specified, then it is read from CONSUL_NAMESPACE environment var.
    #
    # namespace: "..."

    # scheme is an optional scheme (http or https) to use for connecting to Consul server.
    # By default, http scheme is used.
    #
    # scheme: "..."

    # services is an optional list of services for which targets are retrieved.
    # If omitted, all services are scraped.
    # See https://www.consul.io/api-docs/catalog#list-nodes-for-service .
    #
    # services: ["...", "..."]

    # tag_separator is an optional string by which Consul tags are joined into the __meta_consul_tags label.
    # By default, "," is used as a tag separator.
    # Individual tags are also available via __meta_consul_tag_<tagname> labels - see below.
    #
    # tag_separator: "..."

    # filter is optional filter for service nodes discovery request.
    # Replaces tags and node_metadata options.
    # consul supports it since 1.14 version
    # list of supported filters https://developer.hashicorp.com/consul/api-docs/catalog#filtering-1
    # syntax examples https://developer.hashicorp.com/consul/api-docs/features/filtering
    #
    # filter: "..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<service_or_node_addr>:<service_port>`, where `<service_or_node_addr>` is the service address. If the service address is empty,
then the node address is used instead. The `<service_port>` is the service port.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_consulagent_address`: the address of the target
* `__meta_consulagent_dc`: the datacenter name for the target
* `__meta_consulagent_health`: the health status of the service
* `__meta_consulagent_metadata_<key>`: each node metadata key value of the target
* `__meta_consulagent_namespace`: namespace of the service - see [namespace docs](https://developer.hashicorp.com/consul/docs/enterprise/namespaces)
* `__meta_consulagent_node`: the node name defined for the target
* `__meta_consulagent_service_address`: the service address of the target
* `__meta_consulagent_service_id`: the service ID of the target
* `__meta_consulagent_service_metadata_<key>`: each service metadata key value of the target
* `__meta_consulagent_service_port`: the service port of the target
* `__meta_consulagent_service`: the name of the service the target belongs to
* `__meta_consulagent_tagged_address_<key>`: each node tagged address key value of the target
* `__meta_consulagent_tag_<tagname>`: the value for the given <tagname> tag of the target
* `__meta_consulagent_tagpresent_<tagname>`: "true" for every <tagname> tag of the target
* `__meta_consulagent_tags`: the list of tags of the target joined by the `tag_separator`

The list of discovered Consul Agent targets is refreshed at the interval, which can be configured via `-promscrape.consulagentSDCheckInterval` command-line flag.

## digitalocean_sd_configs

DigitalOcean SD configuration allows retrieving scrape targets from [DigitalOcean's Droplets API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Droplets).

Configuration example:

```yaml
scrape_configs:
- job_name: digitalocean
  digitalocean_sd_configs:

    # server is an optional DigitalOcean API server to query.
    # By default, https://api.digitalocean.com is used.
    #
  - server: "https://api.digitalocean.com"

    # port is an optional port to scrape metrics from. By default, port 80 is used.
    #
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<public_ip>:<port>`, where `<public_ip>` is a public ipv4 address of the droplet, while `<port>` is the port specified in the `digitalocean_sd_configs`.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_digitalocean_droplet_id`: the id of the droplet
* `__meta_digitalocean_droplet_name`: the name of the droplet
* `__meta_digitalocean_image`: the slug of the droplet's image
* `__meta_digitalocean_image_name`: the display name of the droplet's image
* `__meta_digitalocean_private_ipv4`: the private IPv4 of the droplet
* `__meta_digitalocean_public_ipv4`: the public IPv4 of the droplet
* `__meta_digitalocean_public_ipv6`: the public IPv6 of the droplet
* `__meta_digitalocean_region`: the region of the droplet
* `__meta_digitalocean_size`: the size of the droplet
* `__meta_digitalocean_status`: the status of the droplet
* `__meta_digitalocean_features`: the comma-separated list of features of the droplet
* `__meta_digitalocean_tags`: the comma-separated list of tags of the droplet
* `__meta_digitalocean_vpc`: the id of the droplet's VPC

The list of discovered DigitalOcean targets is refreshed at the interval, which can be configured via `-promscrape.digitaloceanSDCheckInterval` command-line flag.

## dns_sd_configs

DNS-based service discovery allows retrieving scrape targets from the specified DNS domain names.
These specified names are periodically queried to discover a list of targets with the interval
configured via `-promscrape.dnsSDCheckInterval` command-line flag.

Configuration example:

```yaml
scrape_configs:
- job_name: dns
  dns_sd_configs:

    # names must contain a list of DNS names to query.
    #
  - names: ["...", "..."]

    # type is an optional type of DNS query to perform.
    # Supported values are: SRV, A, AAAA or MX.
    # By default, SRV is used.
    #
    # type: ...

    # port is a port number to use if the query type is not SRV.
    #
    # port: ...
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to the `<addr>:<port>`, where `<addr>` is the discovered DNS address, while `<port>` is either the discovered port for SRV records or the port
specified in the `dns_sd_config`.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_dns_name`: the record name that produced the discovered target.
* `__meta_dns_srv_record_target`: the target field of the SRV record
* `__meta_dns_srv_record_port`: the port field of the SRV record
* `__meta_dns_mx_record_target`: the target field of the MX record.

The list of discovered DNS targets is refreshed at the interval, which can be configured via `-promscrape.dnsSDCheckInterval` command-line flag.

## docker_sd_configs

Docker SD configuration allows retrieving scrape targets from [Docker Engine](https://docs.docker.com/engine/) hosts.

Configuration example:

```yaml
scrape_configs:
- job_name: docker
  docker_sd_configs:

    # host must contain the address of the Docker daemon.
    #
  - host: "..."

    # port is an optional port to scrape metrics from.
    # By default, port 80 is used.
    #
    # port: ...

    # host_networking_host is an optional host to use if the container is in host networking mode.
    # By default, localhost is used.
    #
    # host_networking_host: "..."

    # filters is an optional filters to limit the discovery process to a subset of available resources.
    # See https://docs.docker.com/engine/api/v1.40/#operation/ContainerList
    #
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<ip_address>:<port>`, where `<ip_address>` is the exposed ip address of the docker container, while the `<port>` is either the exposed port
of the docker container or the port specified in the `docker_sd_configs` if the docker container has no exposed ports.
If a container exposes multiple ip addresses, then multiple targets will be discovered - one per each exposed ip address.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_docker_container_id`: the id of the container
* `__meta_docker_container_name`: the name of the container
* `__meta_docker_container_network_mode`: the network mode of the container
* `__meta_docker_container_label_<labelname>`: each label of the container
* `__meta_docker_network_id`: the ID of the network
* `__meta_docker_network_name`: the name of the network
* `__meta_docker_network_ingress`: whether the network is ingress
* `__meta_docker_network_internal`: whether the network is internal
* `__meta_docker_network_label_<labelname>`: each label of the network
* `__meta_docker_network_scope`: the scope of the network
* `__meta_docker_network_ip`: the IP of the container in this network
* `__meta_docker_port_private`: the port on the container
* `__meta_docker_port_public`: the external port if a port-mapping exists
* `__meta_docker_port_public_ip`: the public IP if a port-mapping exists

The list of discovered Docker targets is refreshed at the interval, which can be configured via `-promscrape.dockerSDCheckInterval` command-line flag.

## dockerswarm_sd_configs

Docker Swarm SD configuration allows retrieving scrape targets from [Docker Swarm engine](https://docs.docker.com/engine/swarm/).

Configuration example:

```yaml
scrape_configs:
- job_name: dockerswarm
  dockerswarm_sd_configs:

    # host must contain the address of the Docker daemon.
    #
  - host: "..."

    # role must contain `services`, `tasks` or `nodes` as described below.
    #
    role: ...

    # port is an optional port to scrape metrics from, when `role` is nodes, and for discovered
    # tasks and services that don't have published ports.
    # By default, port 80 is used.
    #
    # port: ...

    # filters is an optional filters to limit the discovery process to a subset of available resources.
    # The available filters are listed in the upstream documentation:
    # Services: https://docs.docker.com/engine/api/v1.40/#operation/ServiceList
    # Tasks: https://docs.docker.com/engine/api/v1.40/#operation/TaskList
    # Nodes: https://docs.docker.com/engine/api/v1.40/#operation/NodeList
    #
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

One of the following roles can be configured to discover targets:

* `role: services`

  The `services` role discovers all Swarm services.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<ip>:<port>`, where `<ip>` is the endpoint's virtual IP, while the `<port>` is the published port of the service.
  If the service has multiple published ports, then multiple targets are generated - one per each port.
  If the service has no published ports, then the `<port>` is set to the `port` value obtained from `dockerswarm_sd_configs`.

  Available meta labels for `role: services` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_dockerswarm_service_id`: the id of the service
  * `__meta_dockerswarm_service_name`: the name of the service
  * `__meta_dockerswarm_service_mode`: the mode of the service
  * `__meta_dockerswarm_service_endpoint_port_name`: the name of the endpoint port, if available
  * `__meta_dockerswarm_service_endpoint_port_publish_mode`: the publishing mode of the endpoint port
  * `__meta_dockerswarm_service_label_<labelname>`: each label of the service
  * `__meta_dockerswarm_service_task_container_hostname`: the container hostname of the target, if available
  * `__meta_dockerswarm_service_task_container_image`: the container image of the target
  * `__meta_dockerswarm_service_updating_status`: the status of the service, if available
  * `__meta_dockerswarm_network_id`: the ID of the network
  * `__meta_dockerswarm_network_name`: the name of the network
  * `__meta_dockerswarm_network_ingress`: whether the network is ingress
  * `__meta_dockerswarm_network_internal`: whether the network is internal
  * `__meta_dockerswarm_network_label_<labelname>`: each label of the network
  * `__meta_dockerswarm_network_scope`: the scope of the network

* `role: tasks`

  The `tasks` role discovers all Swarm tasks.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<ip>:<port>`, where the `<ip>` is the node IP, while the `<port>` is the published port of the task.
  If the task has multiple published ports, then multiple targets are generated - one per each port.
  If the task has no published ports, then the `<port>` is set to the `port` value obtained from `dockerswarm_sd_configs`.

  Available meta labels for `role: tasks` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_dockerswarm_container_label_<labelname>`: each label of the container
  * `__meta_dockerswarm_task_id`: the id of the task
  * `__meta_dockerswarm_task_container_id`: the container id of the task
  * `__meta_dockerswarm_task_desired_state`: the desired state of the task
  * `__meta_dockerswarm_task_slot`: the slot of the task
  * `__meta_dockerswarm_task_state`: the state of the task
  * `__meta_dockerswarm_task_port_publish_mode`: the publishing mode of the task port
  * `__meta_dockerswarm_service_id`: the id of the service
  * `__meta_dockerswarm_service_name`: the name of the service
  * `__meta_dockerswarm_service_mode`: the mode of the service
  * `__meta_dockerswarm_service_label_<labelname>`: each label of the service
  * `__meta_dockerswarm_network_id`: the ID of the network
  * `__meta_dockerswarm_network_name`: the name of the network
  * `__meta_dockerswarm_network_ingress`: whether the network is ingress
  * `__meta_dockerswarm_network_internal`: whether the network is internal
  * `__meta_dockerswarm_network_label_<labelname>`: each label of the network
  * `__meta_dockerswarm_network_label`: each label of the network
  * `__meta_dockerswarm_network_scope`: the scope of the network
  * `__meta_dockerswarm_node_id`: the ID of the node
  * `__meta_dockerswarm_node_hostname`: the hostname of the node
  * `__meta_dockerswarm_node_address`: the address of the node
  * `__meta_dockerswarm_node_availability`: the availability of the node
  * `__meta_dockerswarm_node_label_<labelname>`: each label of the node
  * `__meta_dockerswarm_node_platform_architecture`: the architecture of the node
  * `__meta_dockerswarm_node_platform_os`: the operating system of the node
  * `__meta_dockerswarm_node_role`: the role of the node
  * `__meta_dockerswarm_node_status`: the status of the node

  The `__meta_dockerswarm_network_*` meta labels are not populated for ports which are published with `mode=host`.

* `role: nodes`

  The `nodes` role is used to discover Swarm nodes.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<ip>:<port>`, where `<ip>` is the node IP, while the `<port>` is the `port` value obtained from the `dockerswarm_sd_configs`.

  Available meta labels for `role: nodes` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_dockerswarm_node_address`: the address of the node
  * `__meta_dockerswarm_node_availability`: the availability of the node
  * `__meta_dockerswarm_node_engine_version`: the version of the node engine
  * `__meta_dockerswarm_node_hostname`: the hostname of the node
  * `__meta_dockerswarm_node_id`: the ID of the node
  * `__meta_dockerswarm_node_label_<labelname>`: each label of the node
  * `__meta_dockerswarm_node_manager_address`: the address of the manager component of the node
  * `__meta_dockerswarm_node_manager_leader`: the leadership status of the manager component of the node (true or false)
  * `__meta_dockerswarm_node_manager_reachability`: the reachability of the manager component of the node
  * `__meta_dockerswarm_node_platform_architecture`: the architecture of the node
  * `__meta_dockerswarm_node_platform_os`: the operating system of the node
  * `__meta_dockerswarm_node_role`: the role of the node
  * `__meta_dockerswarm_node_status`: the status of the node

The list of discovered Docker Swarm targets is refreshed at the interval, which can be configured via `-promscrape.dockerswarmSDCheckInterval` command-line flag.

## ec2_sd_configs

EC2 SD configuration allows retrieving scrape targets from [AWS EC2 instances](https://aws.amazon.com/ec2/).

Configuration example:

```yaml
scrape_configs:
- job_name: ec2
  ec2_sd_configs:

    # region is an optional config for AWS region.
    # By default, the region from the instance metadata is used.
    #
  - region: "..."

    # endpoint is an optional custom AWS API endpoint to use.
    # By default, the standard endpoint for the given region is used.
    #
    # endpoint: "..."

    # sts_endpoint is an optional custom STS API endpoint to use.
    # By default, the standard endpoint for the given region is used.
    #
    # sts_endpoint: "..."

    # access_key is an optional AWS API access key.
    # By default, the access key is loaded from AWS_ACCESS_KEY_ID environment var.
    #
    # access_key: "..."

    # secret_key is an optional AWS API secret key.
    # By default, the secret key is loaded from AWS_SECRET_ACCESS_KEY environment var.
    #
    # secret_key: "..."

    # role_arn is an optional AWS Role ARN, an alternative to using AWS API keys.
    #
    # role_arn: "..."

    # port is an optional port to scrape metrics from.
    # By default, port 80 is used.
    #
    # port: ...

    # filters is an optional filters for the instance list.
    # Available filter criteria can be found here:
    # https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
    # Filter API documentation: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
    #
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # az_filters is an optional filters for the availability zones list.
    # Available filter criteria can be found here:
    # https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
    # Filter API documentation: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
    #
    # az_filters:
    # - name: "..."
    #   values: ["...", "..."]
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<instance_ip>:<port>`, where `<instance_ip>` is the private IP of the instance, while the `<port>` is set to the `port` value
obtain from `ec2_sd_configs`.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_ec2_ami`: the EC2 Amazon Machine Image
* `__meta_ec2_architecture`: the architecture of the instance
* `__meta_ec2_availability_zone`: the availability zone in which the instance is running
* `__meta_ec2_availability_zone_id`: the availability zone ID in which the instance is running (requires ec2:DescribeAvailabilityZones)
* `__meta_ec2_instance_id`: the EC2 instance ID
* `__meta_ec2_instance_lifecycle`: the lifecycle of the EC2 instance, set only for 'spot' or 'scheduled' instances, absent otherwise
* `__meta_ec2_instance_state`: the state of the EC2 instance
* `__meta_ec2_instance_type`: the type of the EC2 instance
* `__meta_ec2_ipv6_addresses`: comma separated list of IPv6 addresses assigned to the instance's network interfaces, if present
* `__meta_ec2_owner_id`: the ID of the AWS account that owns the EC2 instance
* `__meta_ec2_platform`: the Operating System platform, set to 'windows' on Windows servers, absent otherwise
* `__meta_ec2_primary_subnet_id`: the subnet ID of the primary network interface, if available
* `__meta_ec2_private_dns_name`: the private DNS name of the instance, if available
* `__meta_ec2_private_ip`: the private IP address of the instance, if present
* `__meta_ec2_public_dns_name`: the public DNS name of the instance, if available
* `__meta_ec2_public_ip`: the public IP address of the instance, if available
* `__meta_ec2_region`: EC2 region for the discovered instance
* `__meta_ec2_subnet_id`: comma separated list of subnets IDs in which the instance is running, if available
* `__meta_ec2_tag_<tagkey>`: each tag value of the instance
* `__meta_ec2_vpc_id`: the ID of the VPC in which the instance is running, if available

The list of discovered EC2 targets is refreshed at the interval, which can be configured via `-promscrape.ec2SDCheckInterval` command-line flag.

## eureka_sd_configs

Eureka SD configuration allows retrieving scrape targets using the [Eureka REST API](https://github.com/Netflix/eureka/wiki/Eureka-REST-operations).

Configuration example:

```yaml
scrape_configs:
- job_name: eureka
  eureka_sd_configs:

    # server is an optional URL to connect to the Eureka server.
    # By default, the http://localhost:8080/eureka/v2 is used.
    #
  - server: "..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<instance_host>:<instance_port>`, where `<instance_host>` is the discovered instance hostname, while the `<instance_port>`
is the discovered instance port. If the instance has no port, then port 80 is used.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_eureka_app_name`: the name of the app
* `__meta_eureka_app_instance_id`: the ID of the app instance
* `__meta_eureka_app_instance_hostname`: the hostname of the instance
* `__meta_eureka_app_instance_homepage_url`: the homepage url of the app instance
* `__meta_eureka_app_instance_statuspage_url`: the status page url of the app instance
* `__meta_eureka_app_instance_healthcheck_url`: the health check url of the app instance
* `__meta_eureka_app_instance_ip_addr`: the IP address of the app instance
* `__meta_eureka_app_instance_vip_address`: the VIP address of the app instance
* `__meta_eureka_app_instance_secure_vip_address`: the secure VIP address of the app instance
* `__meta_eureka_app_instance_status`: the status of the app instance
* `__meta_eureka_app_instance_port`: the port of the app instance
* `__meta_eureka_app_instance_port_enabled`: the port enabled of the app instance
* `__meta_eureka_app_instance_secure_port`: the secure port address of the app instance
* `__meta_eureka_app_instance_secure_port_enabled`: the secure port of the app instance
* `__meta_eureka_app_instance_country_id`: the country ID of the app instance
* `__meta_eureka_app_instance_metadata_<metadataname>`: app instance metadata
* `__meta_eureka_app_instance_datacenterinfo_name`: the datacenter name of the app instance
* `__meta_eureka_app_instance_datacenterinfo_metadata_<metadataname>`: the datacenter metadata

The list of discovered Eureka targets is refreshed at the interval, which can be configured via `-promscrape.eurekaSDCheckInterval` command-line flag.

## file_sd_configs

File-based service discovery reads a set of files with lists of targets to scrape.

Configuration example:

```yaml
scrape_configs:
- job_name: file
  file_sd_configs:

    # files must contain a list of file patterns for files with scrape targets.
    # The last path segment can contain `*`, which matches any number of chars in file name.
    #
    # files may contain http/https urls additionally to local files. These urls cannot contain `*`.
    #
  - files:
    - "my/path/*.yaml"
    - "another/path.json"
    - "http://central-config-server/targets?type=foobar"
```

See [these examples](https://docs.victoriametrics.com/scrape_config_examples/#file-based-target-discovery) on how to configure file-based target discovery.

The referred files and urls must contain a list of static configs in one of the following formats:

* JSON:

  ```json
  [
    {
      "targets": ["<host>", ... ],
      "labels": {
        "<labelname>": "<labelvalue>",
        ...,
      }
    },
    ...
  ]
  ```

* YAML:

  ```yaml
  - targets: ["<host>", ... ]
    labels:
      <labelname>: <labelvalue>
      ...
    ...
  ```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to one of the `target` value specified in the target files.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_filepath`: the filepath from which the target was extracted

See the [list of integrations](https://prometheus.io/docs/operating/integrations/#file-service-discovery) with `file_sd_configs`.

The list of discovered file-based targets is refreshed at the interval, which can be configured via `-promscrape.fileSDCheckInterval` command-line flag.

## gce_sd_configs

GCE SD configuration allows retrieving scrape targets from [GCP GCE instances](https://cloud.google.com/compute).

Configuration example:

```yaml
scrape_configs:
- job_name: gce
  gce_sd_configs:

    # project is an optional GCE project where targets must be discovered.
    # By default, the local project is used.
    #
  - project: "..."

    # zone is an optional zone where targets must be discovered.
    # By default, the local zone is used.
    # If zone equals to '*', then targets in all the zones for the given project are discovered.
    # The zone may contain a list of zones: zone["us-east1-a", "us-east1-b"]
    #
    # zone: "..."

    # filter is an optional filter for the instance list.
    # See https://cloud.google.com/compute/docs/reference/latest/instances/list
    #
    # filter: "..."

    # port is an optional port to scrape metrics from.
    # By default, port 80 is used.
    #
    # port: ...

    # tag_separator is an optional separator for tags in `__meta_gce_tags` label.
    # By default, "," is used.
    #
    # tag_separator: "..."
```

Credentials are discovered by looking in the following places, preferring the first location found:

1. a JSON file specified by the `GOOGLE_APPLICATION_CREDENTIALS` environment variable
1. a JSON file in the well-known path `$HOME/.config/gcloud/application_default_credentials.json`
1. fetched from the GCE metadata server

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<iface_ip>:<port>`, where `<iface_ip>` is private IP of the discovered instance, while `<port>` is the `port` value
specified in the `gce_sd_configs`.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_gce_instance_id`: the numeric id of the instance
* `__meta_gce_instance_name`: the name of the instance
* `__meta_gce_label_<labelname>`: each GCE label of the instance
* `__meta_gce_machine_type`: full or partial URL of the machine type of the instance
* `__meta_gce_metadata_<name>`: each metadata item of the instance
* `__meta_gce_network`: the network URL of the instance
* `__meta_gce_private_ip`: the private IP address of the instance
* `__meta_gce_interface_ipv4_<name>`: IPv4 address of each named interface
* `__meta_gce_project`: the GCP project in which the instance is running
* `__meta_gce_public_ip`: the public IP address of the instance, if present
* `__meta_gce_subnetwork`: the subnetwork URL of the instance
* `__meta_gce_tags`: list of instance tags separated by tag_separator
* `__meta_gce_zone`: the GCE zone URL in which the instance is running

The list of discovered GCE targets is refreshed at the interval, which can be configured via `-promscrape.gceSDCheckInterval` command-line flag.

## hetzner_sd_configs

Hetzner SD configuration allows retrieving scrape targets from [Hetzner Cloud](https://www.hetzner.com/cloud) and [Hetzner Robot](https://docs.hetzner.com/robot).

Configuration example:

```yaml
scrape_configs:
- job_name: hetzner
  hetzner_sd_configs:

    # The mandatory Hetzner role for entity discovery.
    # Must be either 'robot' or 'hcloud'.
    #
    role: "hcloud"

    # Required credentials for API server authentication for 'hcloud' role.
    #
    authorization:
      credentials: "..."
      # type: "..."  # default: Bearer
      # credentials_file: "..."  # is mutually-exclusive with credentials

    # Required credentials for API server authentication for 'robot' role.
    #
    # basic_auth:
    #  username: "..."
    #  username_file: "..."  # is mutually-exclusive with username
    #  password: "..."
    #  password_file: "..."  # is mutually-exclusive with password

    # port is an optional port to scrape metrics from.
    # By default, port 80 is used.
    #
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<FQDN>:<port>`, where FQDN is discovered instance address and `<port>` is the port from the `hetzner_sd_configs` (default port is `80`).

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

Common labels for both `hcloud` and `robot` roles:

* `__meta_hetzner_datacenter`: the datacenter of the server
* `__meta_hetzner_public_ipv4`: the public IPv4 address of the server
* `__meta_hetzner_public_ipv6_network`: the public IPv6 network (/64) of the server
* `__meta_hetzner_role`: the current role `hcloud` or `robot`
* `__meta_hetzner_server_id`: the ID of the server
* `__meta_hetzner_server_name`: the name of the server
* `__meta_hetzner_server_status`: the status of the server

Additional labels for `role: hcloud`:

* `__meta_hetzner_hcloud_datacenter_location`: the location of the server
* `__meta_hetzner_hcloud_datacenter_location_network_zone`: the network zone of the server
* `__meta_hetzner_hcloud_cpu_cores`: the CPU cores count of the server
* `__meta_hetzner_hcloud_cpu_type`: the CPU type of the server (shared or dedicated)
* `__meta_hetzner_hcloud_disk_size_gb`: the disk size of the server (in GB)
* `__meta_hetzner_hcloud_image_description`: the description of the server image
* `__meta_hetzner_hcloud_image_name`: the image name of the server
* `__meta_hetzner_hcloud_image_os_flavor`: the OS flavor of the server image
* `__meta_hetzner_hcloud_image_os_version`: the OS version of the server image
* `__meta_hetzner_hcloud_label_<labelname>`: each label of the server
* `__meta_hetzner_hcloud_labelpresent_<labelname>`: true for each label of the server
* `__meta_hetzner_hcloud_memory_size_gb`: the amount of memory of the server (in GB)
* `__meta_hetzner_hcloud_private_ipv4_<networkname>`: the private IPv4 address of the server within a given network
* `__meta_hetzner_hcloud_server_type`: the type of the server

Additional labels for `role: robot`:

* `__meta_hetzner_robot_cancelled`: the server cancellation status
* `__meta_hetzner_robot_product`: the product of the server

The list of discovered Hetzner targets is refreshed at the interval, which can be configured via `-promscrape.hetznerSDCheckInterval` command-line flag.

## http_sd_configs

HTTP-based service discovery fetches targets from the specified `url`.

Configuration example:

```yaml
scrape_configs:
- job_name: http
  http_sd_configs:

    # url must contain the URL from which the targets are fetched.
    #
  - url: "http://..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

See [these examples](https://docs.victoriametrics.com/scrape_config_examples/#http-based-target-discovery) on how to configure http-based target discovery.

The service at `url` must return JSON response in the following format:

```json
[
  {
    "targets": [ "<host>", ... ],
    "labels": {
      "<labelname>": "<labelvalue>",
      ...
    }
  },
  ...
]
```

The `url` is queried periodically with the interval specified in `-promscrape.httpSDCheckInterval` command-line flag.
Discovery errors are tracked in `promscrape_discovery_http_errors_total` metric.

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to one of the targets returned by the http service.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_url`: the URL from which the target was extracted

The list of discovered HTTP-based targets is refreshed at the interval, which can be configured via `-promscrape.httpSDCheckInterval` command-line flag.

## kubernetes_sd_configs

Kubernetes SD configuration allows retrieving scrape targets from [Kubernetes REST API](https://kubernetes.io/docs/reference/using-api/).

Configuration example:

```yaml
scrape_configs:
- job_name: kubernetes
  kubernetes_sd_configs:

    # role must contain the Kubernetes role of entities that should be discovered.
    # It must have one of the following values:
    # endpoints, endpointslice, service, pod, node or ingress.
    # See docs below about each particular role.
    #
  - role: "..."

    # api_server is an optional url for Kubernetes API server.
    # By default, it is read from /var/run/secrets/kubernetes.io/serviceaccount/
    #
    # api_server: "..."

    # kubeconfig_file is an optional path to a kubeconfig file.
    # Note that api_server and kubeconfig_file are mutually exclusive.
    #
    # kubeconfig_file: "..."

    # namespaces is an optional namespace for service discovery.
    # By default, all namespaces are used.
    # If own_namespace is set to true, then the current namespace is used for service discovery.
    #
    # namespaces:
    #   own_namespace: <boolean>
    #   names: ["...", "..."]

    # selects is an optional label and field selectors to limit the discovery process to a subset of available resources.
    # See https://kubernetes.io/docs/concepts/overview/working-with-objects/field-selectors/
    # and https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # The `role: endpoints` supports pod, service and endpoints selectors.
    # The `role: pod` supports node selectors when configured with `attach_metadata: {node: true}`.
    # Other roles only support selectors matching the role itself (e.g. node role can only contain node selectors).
    #
    # selectors:
    # - role: "..."
    #   label: "..."
    #   field: "..."

    # attach_metadata is an optional metadata to attach to discovered targets.
    # When `node` is set to true, then node metadata is attached to discovered targets.
    # Valid for roles: pod, endpoints, endpointslice.
    #
    # Set `-promscrape.kubernetes.attachNodeMetadataAll` command-line flag
    # for attaching `node` metadata for all the discovered targets.
    #
    # attach_metadata:
    #   node: <boolean>

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

See [these examples](https://docs.victoriametrics.com/scrape_config_examples/#kubernetes-target-discovery) on how to discover and scrape Kubernetes targets.

One of the following `role` types can be configured to discover targets:

* `role: node`

  The `role: node` discovers one target per cluster node.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<ip>:<port>`, where `<ip>` is to the first existing address of the Kubernetes node object in the address type order
  of `NodeInternalIP`, `NodeExternalIP`, `NodeLegacyHostIP` and `NodeHostName`,
  while `<port>` is the kubelet port on the given node.

  Available meta labels for `role: node` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_node_name`: The name of the node object.
  * `__meta_kubernetes_node_provider_id`: The cloud provider's name for the node object.
  * `__meta_kubernetes_node_label_<labelname>`: Each label from the node object.
  * `__meta_kubernetes_node_labelpresent_<labelname>`: "true" for each label from the node object.
  * `__meta_kubernetes_node_annotation_<annotationname>`: Each annotation from the node object.
  * `__meta_kubernetes_node_annotationpresent_<annotationname>`: "true" for each annotation from the node object.
  * `__meta_kubernetes_node_address_<address_type>`: The first address for each node address type, if it exists.

  In addition, the `instance` label for the node will be set to the node name as retrieved from the API server.

* `role: service`

  The `role: service` discovers Kubernetes services.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<service_name>.<namespace>:<port>`, where `<service_name>` is the service name, `<namespace>` is the service namespace
  and `<port>` is the service port.
  If the service has multiple ports, then multiple targets are discovered for the service - one per each port.

  This is generally useful for blackbox monitoring of a service. The target address will be set to the Kubernetes DNS name
  of the service and respective service port.

  Available meta labels for `role: service` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_namespace`: The namespace of the service object.
  * `__meta_kubernetes_service_annotation_<annotationname>`: Each annotation from the service object.
  * `__meta_kubernetes_service_annotationpresent_<annotationname>`: "true" for each annotation of the service object.
  * `__meta_kubernetes_service_cluster_ip`: The cluster IP address of the service. (Does not apply to services of type ExternalName)
  * `__meta_kubernetes_service_external_name`: The DNS name of the service. (Applies to services of type ExternalName)
  * `__meta_kubernetes_service_label_<labelname>`: Each label from the service object.
  * `__meta_kubernetes_service_labelpresent_<labelname>`: "true" for each label of the service object.
  * `__meta_kubernetes_service_name`: The name of the service object.
  * `__meta_kubernetes_service_port_name`: Name of the service port for the target.
  * `__meta_kubernetes_service_port_number`: Service port number for the target.
  * `__meta_kubernetes_service_port_protocol`: Protocol of the service port for the target.
  * `__meta_kubernetes_service_type`: The type of the service.

* `role: pod`

  The `role: pod` discovers all pods and exposes their containers as targets.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<ip>:<port>`, where `<ip>` is pod IP, while `<port>` is the exposed container port.
  If the pod has multiple container ports, then multiple targets are generated for the pod - one per each exposed container port.
  If the pod has no exposed container ports, then the `__address__` for pod target is set to the pod IP.

  Available meta labels for `role: pod` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_namespace`: The namespace of the pod object.
  * `__meta_kubernetes_pod_name`: The name of the pod object.
  * `__meta_kubernetes_pod_ip`: The pod IP of the pod object.
  * `__meta_kubernetes_pod_label_<labelname>`: Each label from the pod object.
  * `__meta_kubernetes_pod_labelpresent_<labelname>`: "true" for each label from the pod object.
  * `__meta_kubernetes_pod_annotation_<annotationname>`: Each annotation from the pod object.
  * `__meta_kubernetes_pod_annotationpresent_<annotationname>`: "true" for each annotation from the pod object.
  * `__meta_kubernetes_pod_container_id`: ID of the container in the form `<type>://<container_id>`.
  * `__meta_kubernetes_pod_container_image`: Container image the target address points to.
  * `__meta_kubernetes_pod_container_init`: "true" if the container is an InitContainer.
  * `__meta_kubernetes_pod_container_name`: Name of the container the target address points to.
  * `__meta_kubernetes_pod_container_port_name`: Name of the container port.
  * `__meta_kubernetes_pod_container_port_number`: Number of the container port.
  * `__meta_kubernetes_pod_container_port_protocol`: Protocol of the container port.
  * `__meta_kubernetes_pod_ready`: Set to true or false for the pod's ready state.
  * `__meta_kubernetes_pod_phase`: Set to Pending, Running, Succeeded, Failed or Unknown in the lifecycle.
  * `__meta_kubernetes_pod_node_name`: The name of the node the pod is scheduled onto.
  * `__meta_kubernetes_pod_host_ip`: The current host IP of the pod object.
  * `__meta_kubernetes_pod_uid`: The UID of the pod object.
  * `__meta_kubernetes_pod_controller_kind`: Object kind of the pod controller.
  * `__meta_kubernetes_pod_controller_name`: Name of the pod controller.


* `role: endpoints`

  The `role: endpoints` discovers targets from listed endpoints of a service.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<addr>:<port>`, where `<addr>` is the endpoint address, while `<port>` is the endpoint port.
  If the endpoint has multiple ports, then a single target per each port is generated.
  If the endpoint is backed by a pod, all additional container ports of the pod, not bound to an endpoint port, are discovered as targets as well.

  Available meta labels for `role: endpoints` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_namespace`: The namespace of the endpoints object.
  * `__meta_kubernetes_endpoints_name`: The names of the endpoints object.
  * `__meta_kubernetes_endpoints_label_<labelname>`: Each label from the endpoints object.
  * `__meta_kubernetes_endpoints_labelpresent_<labelname>`: "true" for each label from the endpoints object.

  For all targets discovered directly from the endpoints list (those not additionally inferred from underlying pods), the following labels are attached:

  * `__meta_kubernetes_endpoint_hostname`: Hostname of the endpoint.
  * `__meta_kubernetes_endpoint_node_name`: Name of the node hosting the endpoint.
  * `__meta_kubernetes_endpoint_ready`: Set to true or false for the endpoint's ready state.
  * `__meta_kubernetes_endpoint_port_name`: Name of the endpoint port.
  * `__meta_kubernetes_endpoint_port_protocol`: Protocol of the endpoint port.
  * `__meta_kubernetes_endpoint_address_target_kind`: Kind of the endpoint address target.
  * `__meta_kubernetes_endpoint_address_target_name`: Name of the endpoint address target.

  If the endpoints belong to a service, all labels of the `role: service` are attached.
  For all targets backed by a pod, all labels of the `role: pod` are attached.

* `role: endpointslice`

  The `role: endpointslice` discovers targets from existing endpointslices.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<addr>:<port>`, where `<addr>` is the endpoint address, while `<port>` is the endpoint port.
  If the endpoint has multiple ports, then a single target per each port is generated.
  If the endpoint is backed by a pod, all additional container ports of the pod, not bound to an endpoint port, are discovered as targets as well.

  Available meta labels for `role: endpointslice` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_namespace`: The namespace of the endpointslice object.
  * `__meta_kubernetes_endpointslice_name`: The name of endpointslice object.

  For all targets discovered directly from the endpointslice list (those not additionally inferred from underlying pods), the following labels are attached:

  * `__meta_kubernetes_endpointslice_address_target_kind`: Kind of the referenced object.
  * `__meta_kubernetes_endpointslice_address_target_name`: Name of referenced object.
  * `__meta_kubernetes_endpointslice_address_type`: The ip protocol family of the address of the target.
  * `__meta_kubernetes_endpointslice_endpoint_conditions_ready`: Set to true or false for the referenced endpoint's ready state.
  * `__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname`: Name of the node hosting the referenced endpoint.
  * `__meta_kubernetes_endpointslice_endpoint_topology_present_kubernetes_io_hostname`: Flag that shows if the referenced object has a kubernetes.io/hostname annotation.
  * `__meta_kubernetes_endpointslice_port`: Port of the referenced endpoint.
  * `__meta_kubernetes_endpointslice_port_name`: Named port of the referenced endpoint.
  * `__meta_kubernetes_endpointslice_port_protocol`: Protocol of the referenced endpoint.

  If the endpoints belong to a service, all labels of the `role: service` are attached.
  For all targets backed by a pod, all labels of the `role: pod` are attached.

* `role: ingress`

  The `role: ingress` discovers a target for each path of each ingress.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to the host obtained from ingress spec.
  If the ingress has multiple specs with multiple hosts, then a target per each host is created.

  This is generally useful for blackbox monitoring of an ingress.

  Available meta labels for `role: ingress` during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_kubernetes_namespace`: The namespace of the ingress object.
  * `__meta_kubernetes_ingress_name`: The name of the ingress object.
  * `__meta_kubernetes_ingress_label_<labelname>`: Each label from the ingress object.
  * `__meta_kubernetes_ingress_labelpresent_<labelname>`: "true" for each label from the ingress object.
  * `__meta_kubernetes_ingress_annotation_<annotationname>`: Each annotation from the ingress object.
  * `__meta_kubernetes_ingress_annotationpresent_<annotationname>`: "true" for each annotation from the ingress object.
  * `__meta_kubernetes_ingress_class_name`: Class name from ingress spec, if present.
  * `__meta_kubernetes_ingress_scheme`: Protocol scheme of ingress, https if TLS config is set. Defaults to http.
  * `__meta_kubernetes_ingress_path`: Path from ingress spec. Defaults to `/`.

The list of discovered Kubernetes targets is refreshed at the interval, which can be configured via `-promscrape.kubernetesSDCheckInterval` command-line flag.

## kuma_sd_configs

Kuma service discovery config allows to fetch targets from the specified control plane `server` of [Kuma Service Mesh](https://kuma.io).

It discovers "monitoring assignments" based on Kuma Dataplane Proxies, 
via the [MADS (Monitoring Assignment Discovery Service)](https://kuma.io/docs/2.1.x/policies/traffic-metrics/#traffic-metrics) 
[xDS RESP API](http://envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol).

Configuration example:

```yaml
scrape_configs:
- job_name: kuma
  kuma_sd_configs:

    # server must contain the URL of Kuma Control Plane's MADS xDS server.
    #
  - server: "http://localhost:5676"

    # client_id is an optional client ID to send to Kuma Control Plane.
    # The hostname of the server where vmagent runs is used if it isn't set.
    # If the hostname is empty, then "vmagent" string is used as client_id.
    #
    # client_id: "..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

The `server` is queried periodically with the interval specified in `-promscrape.kumaSDCheckInterval` command-line flag.
Discovery errors are tracked in `promscrape_discovery_kuma_errors_total` metric.

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to one of the targets returned by the http service.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_kuma_mesh`: the name of the mesh
* `__meta_kuma_dataplane`: the name of the proxy
* `__meta_kuma_service`: the name of the service associated with the proxy
* `__meta_kuma_label_<label_name>`: each label of target given from Kuma Control Plane

The list of discovered Kuma targets is refreshed at the interval, which can be configured via `-promscrape.kumaSDCheckInterval` command-line flag.

## nomad_sd_configs

Nomad SD configuration allows retrieving scrape targets from [HashiCorp Nomad Services](https://www.hashicorp.com/blog/nomad-service-discovery).

Configuration example:

```yaml
scrape_configs:
- job_name: nomad
  nomad_sd_configs:

    # server is an optional Nomad server to connect to.
    # If the server isn't specified, then it is read from NOMAD_ADDR environment var.
    # If the NOMAD_ADDR environment var isn't set, then localhost:4646 is used.
    #
  - server: "localhost:4646"

    # namespace is an optional Nomad namespace.
    # If the namespace isn't specified, then it is read from NOMAD_NAMESPACE environment var.
    #
    # namespace: "..."

    # region is an optional Nomad region.
    # If the region isn't specified, then it is read from NOMAD_REGION environment var.
    # If NOMAD_REGION environment var isn't set, then "global" region is used
    #
    # region: "..."

    # tag_separator is an optional string by which Nomad tags are joined into the __meta_nomad_tags label.
    # By default, "," is used as a tag separator.
    # Individual tags are also available via __meta_nomad_tag_<tagname> labels - see below.
    #
    # tag_separator: "..."

    # allow_stale is an optional config, which allows stale Nomad results.
    # See https://developer.hashicorp.com/consul/api-docs/features/consistency
    # Reduces load on Nomad if set to true. By default, it is set to true.
    #
    # allow_stale: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<addr>:<port>`, where `<addr>` is the service address, while `<port>` is the service port.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_nomad_address`: the address of the target
* `__meta_nomad_dc`: the datacenter name for the target
* `__meta_nomad_namespace`: namespace of the service
* `__meta_nomad_node_id`: the node ID defined for the target
* `__meta_nomad_service`: the name of the service the target belongs to
* `__meta_nomad_service_address`: the service address of the target
* `__meta_nomad_service_alloc_id`: the AllocID of the target service
* `__meta_nomad_service_id`: the ID of the target service
* `__meta_nomad_service_job_id`: the JobID of the target service
* `__meta_nomad_service_port`: the service port of the target
* `__meta_nomad_tag_<tagname>`: the value for the given <tagname> tag of the target
* `__meta_nomad_tagpresent_<tagname>`: "true" for every <tagname> tag of the target
* `__meta_nomad_tags`: the list of tags of the target joined by the `tag_separator`

The list of discovered Nomad targets is refreshed at the interval, which can be configured via `-promscrape.nomadSDCheckInterval` command-line flag.

## openstack_sd_configs

OpenStack SD configuration allows retrieving scrape targets from [OpenStack Nova](https://docs.openstack.org/nova/latest/) instances.

[OpenStack identity API v3](https://docs.openstack.org/api-ref/identity/v3/) is supported only.

Configuration example:

```yaml
scrape_configs:
- job_name: openstack
  openstack_sd_configs:

    # role must contain either `hypervisor` or `instance`.
    # See docs below for details.
    #
  - role: "..."

    # region must contain OpenStack region for targets' discovery.
    #
    region: "..."

    # identity_endpoint is an optional HTTP Identity API endpoint.
    # By default, it is read from OS_AUTH_URL environment variable.
    #
    # identity_endpoint: "..."

    # username is an optional username to query Identity API.
    # By default, it is read from OS_USERNAME environment variable.
    #
    # username: "..."

    # userid is an optional userid to query Identity API.
    # By default, it is read from OS_USERID environment variable.
    #
    # userid: "..."

    # password is an optional password to query Identity API.
    # By default, it is read from OS_PASSWORD environment variable.
    #
    # password: "..."

    # At most one of domain_id and domain_name must be provided.
    # By default, they are read from OS_DOMAIN_NAME and OS_DOMAIN_ID environment variables.
    #
    # domain_name: "..."
    # domain_id: "..."

    # project_name and project_id are optional project name and project id.
    # By default, it is read from OS_PROJECT_NAME and OS_PROJECT_ID environment variables.
    # If these vars are empty, then the options are read
    # from OS_TENANT_NAME and OS_TENANT_ID environment variables.
    #
    # project_name: "..."
    # project_id: "..."

    # By default, these fields are read from OS_APPLICATION_CREDENTIAL_NAME
    # and OS_APPLICATION_CREDENTIAL_ID environment variables
    #
    # application_credential_name: "..."
    # application_credential_id: "..."

    # By default, this field is read from OS_APPLICATION_CREDENTIAL_SECRET
    #
    # application_credential_secret: "..."

    # all_tenants can be set to true if all instances in all projects must be discovered.
    # It is only relevant for the 'role: instance' and usually requires admin permissions.
    #
    # all_tenants: ...

    # port is an optional port to scrape metrics from.
    # Port 80 is used by default.
    #
    # port: ...

    # availability is the availability of the endpoint to connect to.
    # Must be one of public, admin or internal.
    # By default, it is set to public
    #
    # availability: "..."

    # tls_config is an optional tls config.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    #
    # tls_config:
    #   ...
```

One of the following `role` types can be configured to discover targets:

* `role: hypervisor`

  The `role: hypervisor` discovers one target per Nova hypervisor node.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<host>:<port>`, where `<host>` is the discovered node IP, while `<port>` is the port specified in the `openstack_sd_configs`.

  The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_openstack_hypervisor_host_ip`: the hypervisor node's IP address.
  * `__meta_openstack_hypervisor_hostname`: the hypervisor node's name.
  * `__meta_openstack_hypervisor_id`: the hypervisor node's ID.
  * `__meta_openstack_hypervisor_state`: the hypervisor node's state.
  * `__meta_openstack_hypervisor_status`: the hypervisor node's status.
  * `__meta_openstack_hypervisor_type`: the hypervisor node's type.

* `role: instance`

  The `role: instance` discovers one target per network interface of Nova instance.

  Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
  to `<host>:<port>`, where `<host>` is the private IP address of the discovered instance, while `<port>` is the port specified in the `openstack_sd_configs`.

  The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

  * `__meta_openstack_address_pool`: the pool of the private IP.
  * `__meta_openstack_instance_flavor`: the flavor of the OpenStack instance.
  * `__meta_openstack_instance_id`: the OpenStack instance ID.
  * `__meta_openstack_instance_name`: the OpenStack instance name.
  * `__meta_openstack_instance_status`: the status of the OpenStack instance.
  * `__meta_openstack_private_ip`: the private IP of the OpenStack instance.
  * `__meta_openstack_project_id`: the project (tenant) owning this instance.
  * `__meta_openstack_public_ip`: the public IP of the OpenStack instance.
  * `__meta_openstack_tag_<tagkey>`: each tag value of the instance.
  * `__meta_openstack_user_id`: the user account owning the tenant.

The list of discovered OpenStack targets is refreshed at the interval, which can be configured via `-promscrape.openstackSDCheckInterval` command-line flag.

## ovhcloud_sd_configs

_Available from [v1.104](https://docs.victoriametrics.com/changelog/#v11040) version._

OVH Cloud SD configuration allows retrieving scrape targets from [OVH Cloud VPS](https://www.ovhcloud.com/en/vps/) 
and [OVH Cloud dedicated server](https://ovhcloud.com/en/bare-metal/).

Configuration example:

```yaml
scrape_configs:
- job_name: ovh_job
  ovhcloud_sd_configs:

  # (optional) depending on the API you want to use, you may set the endpoint to:
  # `ovh-eu` for OVH Europe API (default).
  # `ovh-us` for OVH US API.
  # `ovh-ca` for OVH North-America API.
  # `soyoustart-eu` for "So you Start Europe API".
  # `soyoustart-ca` for "So you Start North America API".
  # `kimsufi-eu` for Kimsufi Europe API.
  # `kimsufi-ca` for Kimsufi North America API.
  # See: https://github.com/ovh/go-ovh?tab=readme-ov-file#supported-apis
  - endpoint: "..."

    # (mandatory) application_key is a self generated tokens. 
    # create one by visiting: https://eu.api.ovh.com/createApp/
    application_key: "..."

    # (mandatory) application_secret holds the application secret key.
    application_secret: "..."
    
    # (mandatory) consumer_key holds the user/app specific token. It must have been validated before use.
    consumer_key: "..."

    # (mandatory) service could be either `vps` or `dedicated_server`.
    service: "..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set to either `<ipv4>` address or `<ipv6>` address.

In addition, the `instance` label for the VPS/dedicated server will be set to the VPS/dedicated server name as retrieved from OVH Cloud API.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling).

VPS:
* `__meta_ovhcloud_vps_cluster`: the cluster of the server.
* `__meta_ovhcloud_vps_datacenter`: the datacenter of the server.
* `__meta_ovhcloud_vps_disk`: the disk of the server.
* `__meta_ovhcloud_vps_display_name`: the display name of the server.
* `__meta_ovhcloud_vps_ipv4`: the IPv4 of the server.
* `__meta_ovhcloud_vps_ipv6`: the IPv6 of the server.
* `__meta_ovhcloud_vps_keymap`: the KVM keyboard layout of the server.
* `__meta_ovhcloud_vps_maximum_additional_ip`: the maximum additional IPs of the server.
* `__meta_ovhcloud_vps_memory_limit`: the memory limit of the server.
* `__meta_ovhcloud_vps_memory`: the memory of the server.
* `__meta_ovhcloud_vps_monitoring_ip_blocks`: the monitoring IP blocks of the server.
* `__meta_ovhcloud_vps_name`: the name of the server.
* `__meta_ovhcloud_vps_netboot_mode`: the netboot mode of the server.
* `__meta_ovhcloud_vps_offer_type`: the offer type of the server.
* `__meta_ovhcloud_vps_offer`: the offer of the server.
* `__meta_ovhcloud_vps_state`: the state of the server.
* `__meta_ovhcloud_vps_vcore`: the number of virtual cores of the server.
* `__meta_ovhcloud_vps_version`: the version of the server.
* `__meta_ovhcloud_vps_zone`: the zone of the server.

Dedicated servers:
* `__meta_ovhcloud_dedicated_server_commercial_range`: the commercial range of the server.    
* `__meta_ovhcloud_dedicated_server_datacenter`: the datacenter of the server.                
* `__meta_ovhcloud_dedicated_server_ipv4`: the IPv4 of the server.                            
* `__meta_ovhcloud_dedicated_server_ipv6`: the IPv6 of the server.                            
* `__meta_ovhcloud_dedicated_server_link_speed`: the link speed of the server.                
* `__meta_ovhcloud_dedicated_server_name`: the name of the server.                            
* `__meta_ovhcloud_dedicated_server_no_intervention`: the [intervention](https://support.us.ovhcloud.com/hc/en-us/articles/27991435200147-FAQ-Interventions-and-Hardware-Replacement) of the server.
* `__meta_ovhcloud_dedicated_server_os`: the operating system of the server.
* `__meta_ovhcloud_dedicated_server_rack`: the rack of the server.
* `__meta_ovhcloud_dedicated_server_reverse`: the reverse DNS name of the server.
* `__meta_ovhcloud_dedicated_server_server_id`: the ID of the server.
* `__meta_ovhcloud_dedicated_server_state`: the state of the server.
* `__meta_ovhcloud_dedicated_server_support_level`: the support level of the server.

The list of discovered OVH Cloud targets is refreshed at the interval, which can be configured via `-promscrape.ovhcloudSDCheckInterval` command-line flag.

## static_configs

A static config allows specifying a list of targets and a common label set for them.

Configuration example:

```yaml
scrape_configs:
- job_name: static
  static_configs:

    # targets must contain a list of `host:port` targets to scrape.
    # The `http://host:port/metrics` endpoint is scraped per each configured target then.
    # The `http` scheme can be changed to `https` by setting it via `scheme` field at `scrape_config` level.
    # The `/metrics` path can be changed to arbitrary path via `metrics_path` field at `scrape_config` level.
    # See https://docs.victoriametrics.com/sd_configs/#scrape_configs .
    #
    # Alternatively the scheme and path can be changed via `relabel_configs` section at `scrape_config` level.
    # See https://docs.victoriametrics.com/vmagent/#relabeling .
    #
    # It is also possible specifying full target urls here, e.g. "http://host:port/metrics/path?query_args"
    #
  - targets:
    - "vmsingle1:8428"
    - "vmsingleN:8428"

    # labels is an optional labels to add to all the targets.
    #
    # labels:
    #   <labelname1>: "<labelvalue1>"
    #   ...
    #   <labelnameN>: "<labelvalueN>"
```

See [these examples](https://docs.victoriametrics.com/scrape_config_examples/#static-configs) on how to configure scraping for static targets.

## vultr_sd_configs
Vultr SD configuration discovers scrape targets from [Vultr](https://www.vultr.com/) Instances.

Configuration example:

```yaml
scrape_configs:
- job_name: vultr
  vultr_sd_configs:

    # bearer_token is a Bearer token to send in every HTTP API request during service discovery (mandatory).
    # See: https://my.vultr.com/settings/#settingsapi
  - bearer_token: "..."

    # Vultr provides query arguments to filter instances.
    # See: https://www.vultr.com/api/#tag/instances

    # label is an optional query arguments to filter instances by label.
    #
    # label: "..."

    # main_ip is an optional query arguments to filter instances by main ip address.
    #
    # main_ip: "..."

    # region is an optional query arguments to filter instances by region id.
    #
    # region: "..."

    # firewall_group_id is an optional query arguments to filter instances by firewall group id.
    #
    # firewall_group_id: "..."

    # hostname is an optional query arguments to filter instances by hostname.
    #
    # hostname: "..."

    # port is an optional port to scrape metrics from.
    # By default, port 80 is used.
    #
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options


```

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to `<FQDN>:<port>`, where FQDN is discovered instance address and `<port>` is the port from the `vultr_sd_configs` (default port is `80`).

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

* `__meta_vultr_instance_allowed_bandwidth_gb`: monthly bandwidth quota in GB.
* `__meta_vultr_instance_disk_gb`: the size of the disk in GB.
* `__meta_vultr_instance_features`: comma-separated list of features available to instance, such as "auto_backups", "ipv6", "ddos_protection".
* `__meta_vultr_instance_hostname`: hostname for this instance.
* `__meta_vultr_instance_id`: unique ID for the VPS Instance.
* `__meta_vultr_instance_internal_ip`: internal IP used by this instance, if set. Only relevant when a VPC is attached.
* `__meta_vultr_instance_label`: user-supplied label for this instance.
* `__meta_vultr_instance_main_ip`: main IPv4 address.
* `__meta_vultr_instance_main_ipv6`: main IPv6 network address.
* `__meta_vultr_instance_os`: [operating System name](https://www.vultr.com/api/#operation/list-os).
* `__meta_vultr_instance_os_id`: [operating System id](https://www.vultr.com/api/#operation/list-os) used by this instance.
* `__meta_vultr_instance_plan`: unique ID for the Plan.
* `__meta_vultr_instance_ram_mb`: the amount of RAM in MB.
* `__meta_vultr_instance_region`: [region id](https://www.vultr.com/api/#operation/list-regions) where the Instance is located.
* `__meta_vultr_instance_server_status`: server health status, which could be `none`, `locked`, `installingbooting`, `ok`.
* `__meta_vultr_instance_tags`: comma-separated list of tags applied to the instance.
* `__meta_vultr_instance_vcpu_count`: the number of vCPUs.

The list of discovered Vultr targets is refreshed at the interval, which can be configured via `-promscrape.vultrSDCheckInterval` command-line flag, default: 30s.

## yandexcloud_sd_configs

[Yandex Cloud](https://cloud.yandex.com/en/) SD configurations allow retrieving scrape targets from accessible folders.

Configuration example:

```yaml
scrape_configs:
- job_name: yandexcloud
  yandexcloud_sd_configs:

    # service is a mandatory option for yandexcloud service discovery
    # currently only "compute" service is supported
    #
  - service: compute

    # api_endpoint is an optional API endpoint for service discovery
    # The https://api.cloud.yandex.net endpoint is used by default.
    #
    # api_endpoint: "https://api.cloud.yandex.net"

    # yandex_passport_oauth_token is an optional OAuth token
    # for querying yandexcloud API. See https://cloud.yandex.com/en-ru/docs/iam/concepts/authorization/oauth-token
    #
    # yandex_passport_oauth_token: "..."

    # tls_config is an optional tls config.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    #
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

Each discovered target has an [`__address__`](https://docs.victoriametrics.com/relabeling/#how-to-modify-scrape-urls-in-targets) label set
to the FQDN of the discovered instance.

The following meta labels are available on discovered targets during [relabeling](https://docs.victoriametrics.com/vmagent/#relabeling):

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

The list of discovered Yandex Cloud targets is refreshed at the interval, which can be configured via `-promscrape.yandexcloudSDCheckInterval` command-line flag.

## scrape_configs

The `scrape_configs` section at file pointed by `-promscrape.config` command-line flag can contain [supported service discovery options](#supported-service-discovery-configs).
Additionally, it can contain the following options:

```yaml
scrape_configs:
  # job_name must contain value for `job` label, which is added
  # to all the metrics collected from the configured and discovered scrape targets.
  # See https://prometheus.io/docs/concepts/jobs_instances/ .
  #
- job_name: "..."

  # scrape_interval is an optional interval to scrape targets.
  # By default, the scrape_interval specified in `global` section is used.
  # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
  # If `global` section doesn't contain the `scrape_interval` option,
  # then one minute interval is used.
  # Example values:
  # - "30s" - 30 seconds
  # - "2m" - 2 minutes
  # The scrape_interval can be set on a per-target basis by specifying `__scrape_interval__`
  # label during target relabeling phase.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # scrape_interval: <duration>

  # scrape_timeout is an optional timeout when scraping the targets.
  # By default, the scrape_timeout specified in `global` section is used.
  # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
  # If `global` section doesn't contain the `scrape_timeout` option,
  # then 10 seconds interval is used.
  # Example values:
  # - "30s" - 30 seconds
  # - "2m" - 2 minutes
  # The `scrape_timeout` cannot exceed the `scrape_interval`.
  # The scrape_timeout can be set on a per-target basis by specifying `__scrape_timeout__`
  # label during target relabeling phase.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # scrape_timeout: <duration>

  # max_scrape_size is an optional parameter for limiting the response size in bytes from scraped targets.
  # If max_scrape_size isn't set, then the limit from -promscrape.maxScrapeSize command-line flag is used instead.
  # Example values:
  # - "10MiB" - 10 * 1024 * 1024 bytes
  # - "100MB" - 100 * 1000 * 1000 bytes
  #
  # max_scrape_size: <size>

  # metrics_path is the path to fetch metrics from targets.
  # By default, metrics are fetched from "/metrics" path.
  #
  # metrics_path: "..."

  # honor_labels controls how to handle conflicts between labels that are
  # already present in scraped data and labels that would be attached
  # server-side "job" and "instance" labels, manually configured target
  # labels, labels generated by service discovery, etc.
  #
  # If honor_labels is set to "true", label conflicts are resolved by keeping label
  # values from the scraped data and ignoring the conflicting server-side labels.
  #
  # If honor_labels is set to "false", label conflicts are resolved by renaming
  # conflicting labels in the scraped data to "exported_<original-label>" (for
  # example "exported_instance", "exported_job") and then attaching server-side
  # labels.
  #
  # Setting honor_labels to "true" is useful for use cases such as federation and
  # scraping the Pushgateway, where all labels specified in the target should be
  # preserved.
  #
  # By default, honor_labels is set to false for security and consistency reasons.
  #
  # honor_labels: <boolean>

  # honor_timestamps controls whether to respect the timestamps present in scraped data.
  #
  # If honor_timestamps is set to "true", the timestamps of the metrics exposed
  # by the target will be used.
  #
  # If honor_timestamps is set to "false", the timestamps of the metrics exposed
  # by the target will be ignored.
  #
  # By default, honor_timestamps is set to false.
  # See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4697#issuecomment-1656540535 for details.
  #
  # honor_timestamps: <boolean>

  # scheme configures the protocol scheme used for requests.
  # Supported values: http and https.
  # By default, http is used.
  #
  # scheme: "..."

  # Optional query arg parameters to add to scrape url.
  #
  # params:
  #   "param_name1": ["value1", ..., "valueN"]
  #   ...
  #   "param_nameM": ["valueM1", ..., "valueMN"]

  # relabel_configs is an optional relabeling configurations
  # for the specified and discovered scrape targets.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # relabel_configs:
  # - <relabel_config> ...

  # metric_relabel_configs is an optional relabeling configs
  # for the collected metrics from active scrape targets.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # metric_relabel_configs:
  # - <relabel_config> ...

  # sample_limit is an optional per-scrape limit on number
  # of scraped samples that will be accepted.
  # If more than this number of samples are present after metric relabeling
  # the entire scrape will be treated as failed.
  # By default, the limit is disabled.
  # The sample_limit can be set on a per-target basis by specifying `__sample_limit__`
  # label during target relabeling phase. Available starting from v1.103.0.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # sample_limit: <int>

  # disable_compression allows disabling HTTP compression for responses received from scrape targets.
  # By default, scrape targets are queried with `Accept-Encoding: gzip` http request header,
  # so targets could send compressed responses in order to save network bandwidth.
  # See https://docs.victoriametrics.com/vmagent/#scrape_config-enhancements
  #
  # disable_compression: <boolean>

  # disable_keepalive allows disabling HTTP keep-alive when scraping targets.
  # By default, HTTP keep-alive is enabled, so TCP connections to scrape targets
  # could be re-used.
  # See https://docs.victoriametrics.com/vmagent/#scrape_config-enhancements
  #
  # disable_keepalive: <boolean>

  # stream_parse allows enabling stream parsing mode when scraping targets.
  # By default, stream parsing mode is disabled for targets which return up to a few thousands samples.
  # See https://docs.victoriametrics.com/vmagent/#stream-parsing-mode .
  # The stream_parse can be set on a per-target basis by specifying `__stream_parse__`
  # label during target relabeling phase.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # stream_parse: <boolean>

  # scrape_align_interval allows aligning scrapes to the given interval.
  # Example values:
  # - "5m" - align scrapes to every 5 minutes.
  # - "1h" - align scrapes to every hour.
  # See https://docs.victoriametrics.com/vmagent/#scrape_config-enhancements
  #
  # scrape_align_interval: <duration>

  # scrape_offset allows specifying the exact offset for scrapes.
  # Example values:
  # - "5m" - align scrapes to every 5 minutes.
  # - "1h" - align scrapes to every hour.
  # See https://docs.victoriametrics.com/vmagent/#scrape_config-enhancements
  #
  # scrape_offset: <duration>

  # series_limit is an optional limit on the number of unique time series
  # a single target can expose during all the scrapes on the time window of 24h.
  # By default, there is no limit on the number of exposed series.
  # See https://docs.victoriametrics.com/vmagent/#cardinality-limiter .
  # The series_limit can be set on a per-target basis by specifying `__series_limit__`
  # label during target relabeling phase.
  # See https://docs.victoriametrics.com/vmagent/#relabeling
  #
  # series_limit: ...

  # no_stale_markers allows disabling staleness tracking.
  # By default, staleness tracking is enabled for all the discovered scrape targets.
  # See https://docs.victoriametrics.com/vmagent/#prometheus-staleness-markers
  #
  # no_stale_markers: <boolean>

  # Additional HTTP client options for target scraping can be specified here.
  # See https://docs.victoriametrics.com/sd_configs/#http-api-client-options
```

## HTTP API client options

The following additional options can be specified in the [scrape_configs](#scrape_configs)
and in the majority of [supported service discovery configs](#supported-service-discovery-configs):

```yaml
    # authorization is an optional `Authorization` header configuration.
    #
    # authorization:
    #   type: "..."  # default: Bearer
    #   credentials: "..."
    #   credentials_file: "..."

    # basic_auth is an optional HTTP basic authentication configuration.
    #
    # basic_auth:
    #   username: "..."
    #   username_file: "..."  # is mutually-exclusive with username
    #   password: "..."
    #   password_file: "..."  # is mutually-exclusive with password

    # bearer_token is an optional Bearer token to send in every HTTP API request during service discovery.
    #
    # bearer_token: "..."

    # bearer_token_file is an optional path to file with Bearer token to send
    # in every HTTP API request during service discovery.
    # The file is re-read every second, so its contents can be updated without the need to restart the service discovery.
    #
    # bearer_token_file: "..."

    # oauth2 is an optional OAuth 2.0 configuration.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2
    #
    # oauth2:
    #   ...

    # tls_config is an optional TLS configuration.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    #
    # tls_config:
    #   ...

    # headers is an optional HTTP headers to pass with each request.
    #
    # headers:
    # - "HeaderName1: HeaderValue"
    # - "HeaderNameN: HeaderValueN"

    # proxy_url is an optional URL for the proxy to use for HTTP API queries during service discovery.
    #
    # proxy_url: "..."

    # proxy_authorization is an optional `Authorization` header config for the proxy_url.
    #
    # proxy_authorization:
    #   type: "..."  # default: Bearer
    #   credentials: "..."
    #   credentials_file: "..."

    # proxy_basic_auth is an optional HTTP basic authentication configuration for the proxy_url.
    #
    # proxy_basic_auth:
    #   username: "..."
    #   username_file: "..."  # is mutually-exclusive with username
    #   password: "..."
    #   password_file: "..."  # is mutually-exclusive with password

    # proxy_bearer_token is an optional Bearer token to send to proxy_url.
    #
    # proxy_bearer_token: "..."

    # proxy_bearer_token_file is an optional path to file with Bearer token to send to proxy_url.
    # The file is re-read every second, so its contents can be updated without the need to restart the service discovery.
    #
    # proxy_bearer_token_file: "..."

    # proxy_oauth2 is an optional OAuth 2.0 configuration for the proxy_url.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#oauth2
    #
    # proxy_oauth2:
    #   ...

    # proxy_tls_config is an optional TLS configuration for the proxy_url.
    # See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#tls_config
    #
    # proxy_tls_config:
    #   ...

    # proxy_headers is an optional HTTP headers to pass to the proxy_url.
    #
    # proxy_headers:
    # - "HeaderName1: HeaderValue"
    # - "HeaderNameN: HeaderValueN"

    # follow_redirects can be used for disallowing HTTP redirects.
    # By default HTTP redirects are followed.
    #
    # follow_redirects: false
```
