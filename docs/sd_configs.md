---
sort: 24
---

# Prometheus service discovery

[vmagent](https://docs.victoriametrics.com/vmagent.html) and [single-node VictoriaMetrics](https://docs.victoriametrics.com/#how-to-scrape-prometheus-exporters-such-as-node-exporter) supports the following Prometheus-compatible service discovery options for Prometheus-compatible scrape targets in the file pointed by `-promscrape.config` command-line flag.

* `azure_sd_configs` is for scraping the targets registered in [Azure Cloud](https://azure.microsoft.com/en-us/). See [these docs](#azure_sd_configs).
* `consul_sd_configs` is for discovering and scraping targets registered in [Consul](https://www.consul.io/). See [these docs](#consul_sd_configs).
* `digitalocean_sd_configs` is for discovering and scraping targerts registered in [DigitalOcean](https://www.digitalocean.com/). See [these docs](#digitalocean_sd_configs).
* `dns_sd_configs` is for discovering and scraping targets from [DNS](https://it.wikipedia.org/wiki/Domain_Name_System) records (SRV, A and AAAA). See [these docs](#dns_sd_configs).
* `docker_sd_configs` is for discovering and scraping [Docker](https://www.docker.com/) targets. See [these docs](#docker_sd_configs).
* `dockerswarm_sd_configs` is for discovering and scraping [Docker Swarm](https://docs.docker.com/engine/swarm/) targets. See [these docs](#dockerswarm_sd_configs).
* `ec2_sd_configs` is for discovering and scraping [Amazon EC2](https://aws.amazon.com/ec2/) targets. See [these docs](#ec2_sd_configs).
* `eureka_sd_configs` is for discovering and scraping targets registered in [Netflix Eureka](https://github.com/Netflix/eureka). See [these docs](#eureka_sd_configs).
* `file_sd_configs` is for scraping targets defined in external files (aka file-based service discovery). See [these docs](#file_sd_configs).
* `gce_sd_configs` is for discovering and scraping [Google Compute Engine](https://cloud.google.com/compute) targets. See [these docs](#gce_sd_configs).
* `http_sd_configs` is for discovering and scraping targerts provided by external http-based service discovery. See [these docs](#http_sd_configs).
* `kubernetes_sd_configs` is for discovering and scraping Kubernetes (K8S) targets. See [kubernetes_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config).
* `openstack_sd_configs` is for discovering and scraping OpenStack targets. See [openstack_sd_config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#openstack_sd_config). [OpenStack identity API v3](https://docs.openstack.org/api-ref/identity/v3/) is supported only.
* `static_configs` is for scraping statically defined targets. See [these docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config).
* `yandexcloud_sd_configs` is for discoverying and scraping [Yandex Cloud](https://cloud.yandex.com/en/) targets. See [these docs](#yandexcloud_sd_configs).

Note that the `refresh_interval` option isn't supported for these scrape configs. Use the corresponding `-promscrape.*CheckInterval`
command-line flag instead. For example, `-promscrape.consulSDCheckInterval=60s` sets `refresh_interval` for all the `consul_sd_configs`
entries to 60s. Run `vmagent -help` or `victoria-metrics -help` in order to see default values for the `-promscrape.*CheckInterval` flags.

Please file feature requests to [our issue tracker](https://github.com/VictoriaMetrics/VictoriaMetrics/issues) if you need other service discovery mechanisms to be supported by VictoriaMetrics and `vmagent`.

## azure_sd_configs

Azure SD configurations allow retrieving scrape targets from [Microsoft Azure](https://azure.microsoft.com/en-us/) VMs.

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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
  - subscription_id: "..."

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

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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
  - server: "localhost:8500"

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
    # See https://www.consul.io/api-docs/catalog#list-nodes-for-service .
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

## digitalocean_sd_configs

DigitalOcean SD configurations allow retrieving scrape targets from [DigitalOcean's Droplets API](https://docs.digitalocean.com/reference/api/api-reference/#tag/Droplets).

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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


Configuration example:

```yaml
scrape_configs:
- job_name: digitalocean
  digitalocean_sd_configs:
    # server is an optional DigitalOcean API server to query.
    # By default https://api.digitalocean.com is used.
  - server: "https://api.digitalocean.com"

    # port is an optional port to scrape metrics from. By default port 80 is used.
    # port: ...

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## dns_sd_configs

DNS-based service discovery configuration allows specifying a set of DNS domain names which are periodically queried to discover a list of targets.

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

* `__meta_dns_name`: the record name that produced the discovered target.
* `__meta_dns_srv_record_target`: the target field of the SRV record
* `__meta_dns_srv_record_port`: the port field of the SRV record

Configuration example:

```yaml
scrape_configs:
- job_name: dns
  dns_sd_configs:
    # names must contain a list of DNS names to query.
  - names: ["...", "..."]

    # type is an optional type of DNS query to perform.
    # Supported values are: SRV, A, or AAAA.
    # By default SRV is used.
    # type: ...

    # port is a port number to use if the query type is not SRV.
    # port: ...
```

## docker_sd_configs

Docker SD configurations allow retrieving scrape targets from [Docker Engine](https://docs.docker.com/engine/) hosts.

Available meta labels during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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

Configuration example:

```yaml
scrape_configs:
- job_name: docker
  docker_sd_configs:

    # host must contain the address of the Docker daemon.
  - host: "..."

    # port is an optional port to scrape metrics from.
    # By default port 80 is used.
    # port: ...

    # host_networking_host is an optional host to use if the container is in host networking mode.
    # By default localhost is used.
    # host_networking_host: "..."

    # filters is an optional filters to limit the discovery process to a subset of available resources.
    # See https://docs.docker.com/engine/api/v1.40/#operation/ContainerList
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## dockerswarm_sd_configs

Docker Swarm SD configurations allow retrieving scrape targets from [Docker Swarm engine](https://docs.docker.com/engine/swarm/).

One of the following roles can be configured to discover targets:

* `role: services`

  The `services` role discovers all Swarm services and exposes their ports as targets.
  For each published port of a service, a single target is generated. If a service has no published ports,
  a target per service is created using the port parameter defined in the SD configuration.

  Available meta labels for `role: services` during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

  * `__meta_dockerswarm_service_id`: the id of the service
  * `__meta_dockerswarm_service_name`: the name of the service
  * `__meta_dockerswarm_service_mode`: the mode of the service
  * `__meta_dockerswarm_service_endpoint_port_name`: the name of the endpoint port, if available
  * `__meta_dockerswarm_service_endpoint_port_publish_mode`: the publish mode of the endpoint port
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

  The `tasks` role discovers all Swarm tasks and exposes their ports as targets.
  For each published port of a task, a single target is generated. If a task has no published ports,
  a target per task is created using the port parameter defined in the SD configuration.

  Available meta labels for `role: tasks` during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

  * `__meta_dockerswarm_container_label_<labelname>`: each label of the container
  * `__meta_dockerswarm_task_id`: the id of the task
  * `__meta_dockerswarm_task_container_id`: the container id of the task
  * `__meta_dockerswarm_task_desired_state`: the desired state of the task
  * `__meta_dockerswarm_task_slot`: the slot of the task
  * `__meta_dockerswarm_task_state`: the state of the task
  * `__meta_dockerswarm_task_port_publish_mode`: the publish mode of the task port
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

  Available meta labels for `role: nodes` during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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


Configuration example:

```yaml
scrape_configs:
- job_name: dockerswarm
  dockerswarm_sd_configs:

    # host must contain the address of the Docker daemon.
  - host: "..."

    # role must contain `services`, `tasks` or `nodes` as described above.
    role: ...

    # port is an optional port to scrape metrics from, when `role` is nodes, and for discovered
    # tasks and services that don't have published ports.
    # By default port 80 is used.
    # port: ...

    # filters is an optional filters to limit the discovery process to a subset of available resources.
    # The available filters are listed in the upstream documentation:
    # Services: https://docs.docker.com/engine/api/v1.40/#operation/ServiceList
    # Tasks: https://docs.docker.com/engine/api/v1.40/#operation/TaskList
    # Nodes: https://docs.docker.com/engine/api/v1.40/#operation/NodeList
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## ec2_sd_configs

EC2 SD configuration allows retrieving scrape targets from [AWS EC2 instances](https://aws.amazon.com/ec2/).

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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
* `__meta_ec2_subnet_id`: comma separated list of subnets IDs in which the instance is running, if available
* `__meta_ec2_tag_<tagkey>`: each tag value of the instance
* `__meta_ec2_vpc_id`: the ID of the VPC in which the instance is running, if available

Configuration example:

```yaml
scrape_configs:
- job_name: ec2
  ec2_sd_configs:
    # region is an optional config for AWS region.
    # By default the region from the instance metadata is used.
  - region: "..."

    # endpoint is an optional custom AWS API endpoint to use.
    # By default the standard endpoint for the given region is used.
    # endpoint: "..."

    # sts_endpoint is an optional custom STS API endpoint to use.
    # By default the standard endpoint for the given region is used.
    # sts_endpoint: "..."

    # access_key is an optional AWS API access key.
    # By default the access key is loaded from AWS_ACCESS_KEY_ID environment var.
    # access_key: "..."

    # secret_key is an optional AWS API secret key.
    # By default the secret key is loaded from AWS_SECRET_ACCESS_KEY environment var.
    # secret_key: "..."

    # role_arn is an optional AWS Role ARN, an alternative to using AWS API keys.
    # role_arn: "..."

    # port is an optional port to scrape metrics from.
    # By default port 80 is used.
    # port: ...

    # filters is an optional filters for the instance list.
    # Available filter criteria can be found here:
    # https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
    # Filter API documentation: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
    # filters:
    # - name: "..."
    #   values: ["...", "..."]

    # az_filters is an optional filters for the availability zones list.
    # Available filter criteria can be found here:
    # https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
    # Filter API documentation: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
    # az_filters:
    # - name: "..."
    #   values: ["...", "..."]
```

## eureka_sd_configs

Eureka SD configuration allows retrieving scrape targets using the [Eureka REST API](https://github.com/Netflix/eureka/wiki/Eureka-REST-operations).

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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

Configuration example:

```yaml
scrape_configs:
- job_name: eureka
  eureka_sd_configs:
    # server is an optional URL to connect to the Eureka server.
    # By default The http://localhost:8080/eureka/v2 is used.
  - server: "..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## file_sd_configs

File-based service discovery reads a set of files containing a list of targets to scrape.
Scrape targets are automatically updated when the underlying files are changed.
Files may be provided in YAML or JSON format. Files must contain a list of static configs in the following format:

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

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

* `__meta_filepath`: the filepath from which the target was extracted

Configuration example:

```yaml
scrape_configs:
- job_name: file
  file_sd_configs:
    # files must contain a list of file patterns for files with scrape targets.
    # The last path segment can contain `*`, which matches any number of chars in file name.
  - files:
    - "my/path/*.yaml"
    - "another/path.json"
```

See the [list of integrations](https://prometheus.io/docs/operating/integrations/#file-service-discovery) with `file_sd_configs`.


## gce_sd_configs

GCE SD configurations allow retrieving scrape targets from [GCP GCE instances](https://cloud.google.com/compute).

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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
* `__meta_gce_tags`: comma separated list of instance tags
* `__meta_gce_zone`: the GCE zone URL in which the instance is running

Configuration example:

```yaml
scrape_configs:
- job_name: gce
  gce_sd_configs:
    # project is an optional GCE project where targets must be discovered.
    # By default the local project is used.
  - project: "..."

    # zone is an optional zone where targets must be discovered.
    # By default the local zone is used.
    # If zone equals to '*', then targets in all the zones for the given project are discovered.
    # The zone may contain a list of zones: zone["us-east1-a", "us-east1-b"]
    # zone: "..."

    # filter is an optional filter for the instance list.
    # See https://cloud.google.com/compute/docs/reference/latest/instances/list
    # filter: "..."

    # port is an optional port to scrape metrics from.
    # By default port 80 is used.
    # port: ...

    # tag_separator is an optional separator for tags in `__meta_gce_tags` label.
    # By default "," is used.
    # tag_separator: "..."
```

Credentials are discovered by looking in the following places, preferring the first location found:

1. a JSON file specified by the `GOOGLE_APPLICATION_CREDENTIALS` environment variable
2. a JSON file in the well-known path `$HOME/.config/gcloud/application_default_credentials.json`
3. fetched from the GCE metadata server

When running within GCE, the service account associated with the instance it is running on should have
at least read-only permissions to the compute resources. If running outside of GCE make sure to create
an appropriate service account and place the credential file in one of the expected locations.

## http_sd_configs

HTTP-based service discovery fetches targets from the specified `url`. The service at `url` must return JSON response in the following format:

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

The endpoint is queried periodically with the refresh interval specified in `-promscrape.httpSDCheckInterval` command-line flag.
Discovery errors are tracked in `promscrape_discovery_http_errors_total` metric.

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

* `__meta_url`: the URL from which the target was extracted

Configuration example:

```yaml
scrape_configs:
- job_name: http
  http_sd_configs:
    # url must contain the URL from which the targets are fetched.
  - url: "http://..."

    # Additional HTTP API client options can be specified here.
    # See https://docs.victoriametrics.com/sd_configs.html#http-api-client-options
```

## yandexcloud_sd_configs

[Yandex Cloud](https://cloud.yandex.com/en/) SD configurations allow retrieving scrape targets from accessible folders.

The following meta labels are available on targets during [relabeling](https://docs.victoriametrics.com/vmagent.html#relabeling):

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
