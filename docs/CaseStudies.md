---
sort: 21
weight: 21
title: Case studies and talks
menu:
  docs:
    parent: 'victoriametrics'
    weight: 21
aliases:
- /CaseStudies.html
---

# Case studies and talks

Below please find public case studies and talks from VictoriaMetrics users. You can also join our [community Slack channel](https://slack.victoriametrics.com/)
where you can chat with VictoriaMetrics users to get additional references, reviews and case studies.

- [Case studies and talks](#case-studies-and-talks)
  - [AbiosGaming](#abiosgaming)
  - [adidas](#adidas)
  - [Adsterra](#adsterra)
  - [ARNES](#arnes)
  - [Brandwatch](#brandwatch)
  - [CERN](#cern)
  - [COLOPL](#colopl)
  - [Criteo](#criteo)
  - [Dig Security](#dig-security)
  - [Fly.io](#flyio)
  - [German Research Center for Artificial Intelligence](#german-research-center-for-artificial-intelligence)
  - [Grammarly](#grammarly)
  - [Groove X](#groove-x)
  - [Idealo.de](#idealode)
  - [MHI Vestas Offshore Wind](#mhi-vestas-offshore-wind)
  - [Naver](#naver)
  - [NetEase Cloud Music](#netease-cloud-music)
  - [Percona](#percona)
  - [Razorpay](#razorpay)
  - [RELEX Solutions](#relex-solutions)
  - [Roblox](#roblox)
  - [Sensedia](#sensedia)
  - [Smarkets](#smarkets)
  - [Synthesio](#synthesio)
  - [Wedos.com](#wedoscom)
  - [Wix.com](#wixcom)
  - [Xiaohongshu](#xiaohongshu)
  - [Zerodha](#zerodha)
  - [zhihu](#zhihu)

You can also read [articles about VictoriaMetrics from our users](https://docs.victoriametrics.com/articles/#third-party-articles-and-slides-about-victoriametrics).

## AbiosGaming

[AbiosGaming](https://abiosgaming.com/) provides industry leading esports data and technology across the globe.

> At Abios, we are running Grafana and Prometheus for our operational insights. We are collecting all sorts of operational metrics such as request latency, active WebSocket connections, and cache statistics to determine if things are working as we expect them to.

> Prometheus explicitly recommends their users not to use high cardinality labels for their time-series data, which is exactly what we want to do. Prometheus is thus a poor solution to keep using. However, since we were already using Prometheus, we needed an alternative solution to be fully compatible with the Prometheus query language.

> The options we decided to try were TimescaleDB together with Promscale to act as a remote write intermediary and VictoriaMetrics. In both cases we still used Prometheus Operator to launch Prometheus instances to scrape metrics and send them to the respective storage layers.

> The biggest difference for our day-to-day operation is perhaps that VictoriaMetrics does not have a Write-Ahead log. The WAL has caused us trouble when Prometheus has experienced issues and starts to run out of RAM when replaying the WAL, thus entering a crash-loop.

> All in all, we are quite impressed with VictoriaMetrics. Not only is the core time-series database well designed, easy to deploy and operate, and performant but the entire ecosystem around it seems to have been given an equal amount of love. There are utilities for things such as taking snapshots (backups) and storing to S3 (and reloading from S3), a Kubernetes Operator, and authentication proxies. It also provides a cluster deployment option if we were to scale up to those numbers.

> From a usability point of view, VictoriaMetrics is the clear winner. Neither Prometheus nor TimescaleDB managed to do any kind of aggregations on our high cardinality metrics, whereas VictoriaMetrics does.

See [the full article](https://abiosgaming.com/press/high-cardinality-aggregations/).

## adidas

See our [slides](https://promcon.io/2019-munich/slides/remote-write-storage-wars.pdf) and [video](https://youtu.be/OsH6gPdxR4s)
from [Remote Write Storage Wars](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) talk at [PromCon 2019](https://promcon.io/2019-munich/).
VictoriaMetrics is compared to Thanos, Cortex and M3DB in the talk.

## Adsterra

[Adsterra Network](https://adsterra.com) is a leading digital advertising agency that offers
performance-based solutions for advertisers and media partners worldwide.

We used to collect and store our metrics with Prometheus. Over time, the data volume on our servers
and metrics increased to the point that we were forced to gradually reduce what we were retaining. When our retention got as low as 7 days
we looked for alternative solutions. We chose between Thanos, VictoriaMetrics and Prometheus federation.

We ended up with the following configuration:

- Local instances of Prometheus with VictoriaMetrics as the remote storage on our backend servers.
- A single Prometheus on our monitoring server scrapes metrics from other servers and writes to VictoriaMetrics.
- A separate Prometheus that federates from other instances of Prometheus and processes alerts.

We learned that remote write protocol generated too much traffic and connections so after 8 months we started looking for alternatives.

Around the same time, VictoriaMetrics released [vmagent](https://docs.victoriametrics.com/vmagent/).
We tried to scrape all the metrics via a single instance of vmagent but it that didn't work because vmagent wasn't able to catch up with writes
into VictoriaMetrics. We tested different options and end up with the following scheme:

- We removed Prometheus from our setup.
- VictoriaMetrics [can scrape targets](https://docs.victoriametrics.com/single-server-victoriametrics/#how-to-scrape-prometheus-exporters-such-as-node-exporter) as well
so we removed vmagent. Now, VictoriaMetrics scrapes all the metrics from 110 jobs and 5531 targets.
- We use [Promxy](https://github.com/jacksontj/promxy) for alerting.

Such a scheme has generated the following benefits compared with Prometheus:

- We can store more metrics.
- We need less RAM and CPU for the same workload.

Cons are the following:

- VictoriaMetrics didn't support replication (it [supports replication now](https://docs.victoriametrics.com/cluster-victoriametrics/#replication-and-data-safety)) - we run an extra instance of VictoriaMetrics and Promxy in front of a VictoriaMetrics pair for high availability.
- VictoriaMetrics stores 1 extra month for defined retention (if retention is set to N months, then VM stores N+1 months of data), but this is still better than other solutions.

Here are some numbers from our single-node VictoriaMetrics setup:

- active time series: 10M
- ingestion rate: 800K samples/sec
- total number of datapoints: more than 2 trillion
- total number of entries in inverted index: more than 1 billion
- daily time series churn rate: 2.6M
- data size on disk: 1.5 TB
- index size on disk: 27 GB
- average datapoint size on disk: 0.75 bytes
- range query rate: 16 rps
- instant query rate: 25 rps
- range query duration: max: 0.5s; median: 0.05s; 97th percentile: 0.29s
- instant query duration: max: 2.1s; median: 0.04s; 97th percentile: 0.15s

VictoriaMetrics consumes about 50GB of RAM.

Setup:

We have 2 single-node instances of VictoriaMetrics. The first instance collects and stores high-resolution metrics (10s scrape interval) for a month.
The second instance collects and stores low-resolution metrics (300s scrape interval) for a month.
We use Promxy + Alertmanager for global view and alerts evaluation.

## ARNES

[The Academic and Research Network of Slovenia](https://www.arnes.si/en/) (ARNES) is a public institute that provides network services to research,
educational and cultural organizations enabling connections and cooperation with each other and with related organizations worldwide.

After using Cacti, Graphite and StatsD for years, we wanted to upgrade our monitoring stack to something that:

- has native alerting support
- can be run on-prem
- has multi-dimensional metrics
- has lower hardware requirements
- is scalable
- has a simple client that allows for provisioning and discovery with Puppet

We had been running Prometheus for about a year in a test environment and it was working well but there was a need/wish for a few more years of retention than the old system provided. We tested Thanos which was a bit resource hungry but worked great for about half a year.
Then we discovered VictoriaMetrics. Our scale isn't that big so we don't have on-prem S3 and no Kubernetes. VM's single node instance provided
the same result with far less maintenance overhead and lower hardware requirements.

After testing it a few months and with great support from the maintainers on [Slack](https://slack.victoriametrics.com/),
we decided to go with it. VM's support for the ingestion of InfluxDB metrics was an additional bonus as our hardware team uses
SNMPCollector to collect metrics from network devices and switching from InfluxDB to VictoriaMetrics required just a simple change in the config file.

Numbers:

- 2 single node instances per DC (one for Prometheus and one for InfluxDB metrics)
- Active time series per VictoriaMetrics instance: ~500k (Prometheus) + ~320k (InfluxDB)
- Ingestion rate per VictoriaMetrics instance: 45k/s (Prometheus) / 30k/s (InfluxDB)
- Query duration: median ~5ms, 99th percentile ~45ms
- Total number of datapoints per instance: 390B (Prometheus), 110B (InfluxDB)
- Average datapoint size on drive: 0.4 bytes
- Disk usage per VictoriaMetrics instance: 125GB (Prometheus), 185GB (InfluxDB)
- Index size per VictoriaMetrics instance: 1.6GB (Prometheus), 1.2GB (InfluxDB)

We are running 1 Prometheus, 1 VictoriaMetrics and 1 Grafana server in each datacenter on baremetal servers, scraping 350+ targets
(and 3k+ devices collected via SNMPCollector sending metrics directly to VM). Each Prometheus is scraping all targets
so we have all metrics in both VictoriaMetrics instances. We are using [Promxy](https://github.com/jacksontj/promxy) to deduplicate metrics from both instances.
Grafana has an LB in front so if one DC has problems we can still view all metrics from both DCs on the other Grafana instance.

We are still in the process of migration, but we are really happy with the whole stack. It has proven to be an essential tool
for gathering insights into our services during COVID-19 and has enabled us to provide better service and identify problems faster.

## Brandwatch

[Brandwatch](https://www.brandwatch.com/) is the world's pioneering digital consumer intelligence suite,
helping over 2,000 of the world's most admired brands and agencies to make insightful, data-driven business decisions.

The engineering department at Brandwatch has been using InfluxDB to store application metrics for many years
but when End-of-Life of InfluxDB version 1.x was announced we decided to re-evaluate our entire metrics collection and storage stack.

The main goals for the new metrics stack were:

- improved performance
- lower maintenance
- support for native clustering in open source version
- the less metrics shipment had to change, the better
- longer data retention time period would be great but not critical

We initially tested CrateDB and TimescaleDB wand found that both had limitations or requirements in their open source versions
that made them unfit for our use case. Prometheus was also considered but it's push vs. pull metrics was a big change we did not want
to include in the already significant change.

Once we found VictoriaMetrics it solved the following problems:

- it is very lightweight and we can now run virtual machines instead of dedicated hardware machines for metrics storage
- very short startup time and any possible gaps in data can easily be filled in using Promxy
- we could continue using Telegraf as our metrics agent and ship identical metrics to both InfluxDB and VictoriaMetrics during the migration period (migration just about to start)
- compression im VM is really good. We can store more metrics and we can easily spin up new VictoriaMetrics instances
for new data and keep read-only nodes with older data if we need to extend our retention period further
than single virtual machine disks allow and we can aggregate all the data from VictoriaMetrics with Promxy

High availability is done the same way we did with InfluxDB by running parallel single nodes of VictoriaMetrics.

Numbers:

- active time series: up to 25 million
- ingestion rate: ~300 000
- total number of datapoints: 380 billion and growing
- total number of entries in inverted index: 575 million and growing
- daily time series churn rate: ~550 000
- data size on disk: ~660GB and growing
- index size on disk: ~9,3GB and growing
- average datapoint size on disk: ~1.75 bytes

Query rates are insignificant as we have concentrated on data ingestion so far.

Anders Bomberg, Monitoring and Infrastructure Team Lead, brandwatch.com

## CERN

The European Organization for Nuclear Research better known as [CERN](https://home.cern/) uses VictoriaMetrics for real-time monitoring
of the [CMS](https://home.cern/science/experiments/cms) detector system.
According to [published talk](https://indico.cern.ch/event/877333/contributions/3696707/attachments/1972189/3281133/CMS_mon_RD_for_opInt.pdf)
VictoriaMetrics is used for the following purposes as a part of the "CMS Monitoring cluster":

- As a long-term storage for messages ingested from the [NATS messaging system](https://nats.io/). Ingested messages are pushed directly to VictoriaMetrics via HTTP protocol
- As a long-term storage for Prometheus monitoring system (30 days retention policy. There are plans to increase it up to ½ year)
- As a data source for visualizing metrics in Grafana.

R&D topic: Evaluate VictoriaMetrics vs InfluxDB for large cardinality data.

Please also see [The CMS monitoring infrastructure and applications](https://arxiv.org/pdf/2007.03630.pdf) publication from CERN with details about their VictoriaMetrics usage.

## COLOPL

[COLOPL](http://www.colopl.co.jp/en/) is Japanese game development company. It started using VictoriaMetrics
after evaluating the following remote storage solutions for Prometheus:

- Cortex
- Thanos
- M3DB
- VictoriaMetrics

See [slides](https://speakerdeck.com/inletorder/monitoring-platform-with-victoria-metrics) and [video](https://www.youtube.com/watch?v=hUpHIluxw80)
from `Large-scale, super-load system monitoring platform built with VictoriaMetrics` talk at [Prometheus Meetup Tokyo #3](https://prometheus.connpass.com/event/157721/).

## Criteo

[Criteo](https://www.criteo.com/) is a global technology company that helps marketers and media owners reach their goals through the world’s leading Commerce Media Platform.

See [this blog post](https://medium.com/criteo-engineering/victoriametrics-a-prometheus-remote-storage-solution-57081a3d8e61) on how Criteo started using VictoriaMetrics
and why they prefer VictoriaMetrics over competing solutions.

## Dig Security

[Dig Security](https://www.dig.security) is a cloud data security startup with 50+ employees that provides real-time visibility, control, and protection of data assets.

### How we came across VictoriaMetrics

We started with a Prometheus server on EKS. That worked until it didn't. We then spent time scaling it, maintaining it, throwing more $ at it, until we stumbled across VictoriaMetrics.

### What VictoriaMetrics means for us

* Easy to use and maintain
* Cost effective
* The ability to handle billions of time series events at any point of time
* Multiple K8s clusters to monitor
* Consistent monitoring infra for each cluster across multiple Regions and clouds
* Secure communication and data storage
* Easy Retention

### Some of our initial challenges prior to moving to VictoriaMetrics

* Reducing cost by not using a managed solution of one of the clouds
* Support HA and recover fast
* No downtimes
* Having our main prometheus using too much Ram and restarts.

### Some of the reasons we chose VictoriaMetrics

* The API is compatible with Prometheus and all standard PromQL queries work well out of the box
* Handles storage well
* Available to use in Grafana easily
* Single and small executable
* Easy and fast backups
* Better benchmarks than all the competitors
* Open Source and maintained with good community

### Some of the benefits we experienced since working with VictoriaMetrics

* We saved around &#x24;5K USD per month
* It’s seamless and doesn’t cause any override complications on the Infrastructure team
* It doesn’t use lots of storage
* It can serve us in the future in even bigger scales
* It has support with a great community.

## Fly.io

[Fly.io](https://fly.io/about/) is a platform for running full stack apps and databases close to your users.

> Victoria Metrics (“Vicky”), in a clustered configuration, is our metrics database. We run a cluster of fairly big Vicky hosts.

> Like everyone else, we started with a simple Prometheus server. That worked until it didn’t. We spent some time scaling it with Thanos, and Thanos was a lot, as far as ops hassle goes. We’d dabbled with Vicky just as a long-term storage engine for vanilla Prometheus, with promxy set up to deduplicate metrics.

> Vicky grew into a more ambitious offering, and added its own Prometheus scraper; we adopted it and scaled it as far as we reasonably could in a single-node configuration. Scaling requirements ultimately pushed us into a clustered deployment; we run an HA cluster (fronted by haproxy). Current Vicky has a really straightforward multi-tenant API — it’s easy to namespace metrics for customers — and it chugs along for us without too much minding.

See [the full post](https://fly.io/blog/measuring-fly/).

## German Research Center for Artificial Intelligence

[German Research Center for Artificial Intelligence](https://en.wikipedia.org/wiki/German_Research_Centre_for_Artificial_Intelligence) (DFKI) is one of the world's largest nonprofit contract research institutes for software technology based on artificial intelligence (AI) methods. DFKI was founded in 1988, and has facilities in the German cities of Kaiserslautern, Saarbrücken, Bremen and Berlin.

> Traditionally research groups in DFKI each used their own hardware. In mid 2020 we started an initiative to consolidate existing (and future) hardware into a central Slurm cluster to enable our researchers and students to run more and larger experiments. Based on the Nvidia deepops stack this included Prometheus for short-term metric storage. Our users liked the level of detail they got from our custom dashboards compared to our previous Zabbix-based solution, so we decided to extend the retention period to several years. Ideally we wanted PhD students to be able to recall even their earliest experiments by the time they finished their thesis. Since we do everything on-premise we needed a solution that is primarily space-efficient.

> We initially considered simply extending the retention period of the Prometheus instances included with deepops, since this would be the “batteries included” solution and appeared to be what everyone else was doing. We naively also liked the concept behind TimescaleDB, since it relies on Postgres for storage that has had decades of development. Turns out relational databases are not good at storing time-series and integration with existing exporters and Grafana would have been more difficult.

> VictoriaMetrics kept showing up in searches and benchmarks on time-series DB performance and consistently came out on top when it came to required storage. Quite frankly, the presented numbers looked like magic, so we decided to put this to the test. First impressions upon trial were excellent. Download the binary and point it at a storage location. Almost no configuration required. Apart from minor tweaks to the command line (turning on deduplication) and running it as a systemd unit we still use the same instance from the first tests today. It was further superior to Prometheus in every measurable way. It used considerably less CPU time and RAM than Prometheus and a third of the storage.

> While initially storage efficiency was the primary driver, the simplicity of setting up a testbed definitely helped. Seeing how effortlessly the single-node instance deals with our current setup gives us confidence that it will keep up with our growth for quite a while. And when the time comes that we outgrow it there is always the robust cluster variant of VictoriaMetrics that we can turn to.

> We like hassle-free experience with VictoriaMetrics. And at least for our use case a straight upgrade compared to Prometheus, while fully compatible with that ecosystem. While it can use cloud storage, there appears to be no downsides to using the filesystem instead, so it fits very well into our on-premise culture. It even comes with an excellent official Grafana dashboard to monitor performance.

Joachim Folz, Researcher, German Research Center for Artificial Intelligence (DFKI)

Numbers:

- Single-node mode
- Active time series: 130K
- Ingestion rate: 24K new samples per second
- Total number of datapoints: 160 billions
- Churn rate: 20K new time series per day
- Data size on disk: 82 GB
- Index size on disk: 300 MB
- Query rate:
  - `/api/v1/query_range`: 2 queries per second
  - `/api/v1/query`: 1.2 queries per second
- Query duration:
  - 99th percentile: 6.5 milliseconds
  - 90th percentile: 4 milliseconds
  - median: 1 millisecond
- CPU usage: 0.1 CPU cores
- RAM usage: 2.8 GB

## Grammarly

[Grammarly](https://www.grammarly.com/) provides digital writing assistant that helps 30 million people and 30 thousand teams write more clearly and effectively every day. In building a product that scales across multiple platforms and devices, Grammarly works to empower users whenever and wherever they communicate.

See [this blogpost on how Grammarly reduces costs and maintenance burden for their observability solution by 10x after switching to VictoriaMetrics](https://www.grammarly.com/blog/engineering/monitoring-with-victoriametrics/).

## Groove X

[Groove X](https://groove-x.com/en/) designs and produces robotics solutions. Its mission is to bring out humanity’s full potential through robotics.

> We need monitoring solution for Device (Robot and Charge Station) health monitoring. At first, we used the Prometheus server, and then migrated to Thanos. But it was difficult to manage Thanos cluster and also we had a performance issue (long latency on request). Colopl, Inc. used VictoriaMetrics and we got interested in it. We built another k8s cluster besides our original Thanos cluster, and tried VictoriaMetrics in parallel for a while. It worked better and finally we decided to switch to VictoriaMetrics, because it provides low latency, it is in active development and it is easy to maintain.

> We like performance and scalability provided by VictoriaMetrics. We use metrics in our daily work, and long latency would be a big problem. Also, metrics correctness is important. We reported some inconsistencies with Prometheus during the evaluation period and received quick feedback from VictoriaMetrics developers.

Junya Hayashi, Senior Software Engineer, Groove X

Numbers:

- Active time series: 14 millions
- Ingestion rate: 235K samples per second
- Total number of datapoints: 3.2 trillions
- Churn rate: 420K new time series per day
- Data size on disk: 2 TB
- Index size on disk: 52 GB
- Query duration:
  - 99th percentile: 2.6 seconds
  - 90th percentile: 0.4 seconds
  - median: 0.006 seconds

## Idealo.de

[idealo.de](https://www.idealo.de/) is the leading price comparison website in Germany. We use Prometheus for metrics on our container platform.
When we introduced Prometheus at idealo we started with m3db as our longterm storage. In our setup, m3db was quite unstable and consumed a lot of resources.

VictoriaMetrics in production is very stable for us and uses only a fraction of the resources even though we also increased our retention period from 1 month to 13 months.

Numbers:

- The number of active time series per VictoriaMetrics instance is 21M
- Total ingestion rate 120k metrics per second
- The total number of datapoints 3.1 trillion
- The average time series churn rate is ~9M per day
- The average query rate is ~20 per second. Response time for 99th quantile is 120ms
- Retention: 13 months
- Size of all datapoints: 3.5 TB

## MHI Vestas Offshore Wind

The mission of [MHI Vestas Offshore Wind](http://www.mhivestasoffshore.com) is to co-develop offshore wind as an economically viable and sustainable energy resource to benefit future generations.

MHI Vestas Offshore Wind is using VictoriaMetrics to ingest and visualize sensor data from offshore wind turbines. The very efficient storage and ability to backfill was key in choosing VictoriaMetrics. MHI Vestas Offshore Wind is running the cluster version of VictoriaMetrics on Kubernetes using the Helm charts for deployment to be able to scale up capacity as the solution is rolled out.

Numbers with current, limited roll out:

- Active time series: 270K
- Ingestion rate: 70K samples per second
- Total number of datapoints: 850 billions
- Data size on disk: 800 GiB
- Retention period: 3 years

## Naver

[Naver](https://www.navercorp.com/en/) is a global tech platform that enables wide access to advanced technologies for SMEs, creators and partners,
fueling their greater growth around the world.

See [this video](https://www.youtube.com/watch?v=OUyXPgVcdw4) and [these slides](https://deview.kr/data/deview/session/attach/%5B2B4%5DVictoriaMetrics_%E1%84%89%E1%85%B5%E1%84%80%E1%85%A8%E1%84%8B%E1%85%A7%E1%86%AF_%E1%84%83%E1%85%A6%E1%84%8B%E1%85%B5%E1%84%90%E1%85%A5_%E1%84%83%E1%85%A2%E1%84%92%E1%85%A9%E1%86%AB%E1%84%83%E1%85%A9%E1%86%AB%E1%84%8B%E1%85%B4_%E1%84%86%E1%85%A5%E1%86%AF%E1%84%90%E1%85%B5%E1%84%87%E1%85%A5%E1%84%89%E1%85%B3_Kor+Eng.pdf) on why and how Naver uses VictoriaMetrics.

## NetEase Cloud Music

[NetEase Cloud Music](https://music.163.com/) is a Chinese freemium music streaming service developed and owned by [NetEase, Inc](https://en.wikipedia.org/wiki/NetEase). It is one of the biggest competitors in the Chinese music streaming business, primarily competing with [Tencent](https://en.wikipedia.org/wiki/Tencent)'s QQ Music.

The huge scale of services and the diverse monitoring requirements bring great challenges to timeseries database’s reliability, availability, and performance. With year’s evolution, we finally build a metrics system around VictoriaMetrics, aiming to solve following problems:
* Weak observability on application layer: in the past, internal monitoring of the product mainly focused on machine level. Although it also provided monitoring plugins for common frameworks, there was still room for improvement in both performance and visualization effects.
* Linking metrics to trace: metrics are the most intuitive way to discover problems, such as "getting 10 failed http requests in the past 30s", but sometimes traces are also needed to locate the root cause of the errors.
* Performance and cost: storage cost of the old metric system is relatively high, since prometheus as a standalone application cannot support large scale of data.
* aggregate queries: aggregate queries are often needed and could take several seconds or even tens of seconds, slowing down troubleshooting process seriously.
* Weak visualization capabilities: monitoring data are often used in YoY comparison and multi-instance comparison to help locate problems. Neither Prometheus UI nor Grafana supports this feature.
 
See [this article](https://juejin.cn/post/7322268449409744931) for details on how NetEase Cloud Music build a metrics system base on VictoriaMetrics and give solutions to above problems.

## Percona

[Percona](https://www.percona.com/) is a leader in providing best-of-breed enterprise-class support, consulting, managed services, training and software for MySQL®, MariaDB®, MongoDB®, PostgreSQL® and other open source databases in on-premises and cloud environments.

Percona migrated from Prometheus to VictoriaMetrics in the [Percona Monitoring and Management](https://www.percona.com/software/database-tools/percona-monitoring-and-management) product. This allowed [reducing resource usage](https://www.percona.com/blog/2020/12/23/observations-on-better-resource-usage-with-percona-monitoring-and-management-v2-12-0/) and [getting rid of complex firewall setup](https://www.percona.com/blog/2020/12/01/foiled-by-the-firewall-a-tale-of-transition-from-prometheus-to-victoriametrics/), while [improving user experience](https://www.percona.com/blog/2020/02/28/better-prometheus-rate-function-with-victoriametrics/).

## Razorpay

[Razorpay](https://razorpay.com/) aims to revolutionize money management for online businesses by providing clean, developer-friendly APIs and hassle-free integration.

> As a fintech organization, we move billions of dollars every month. Our customers and merchants have entrusted us with a paramount responsibility. To handle our ever-growing business, building a robust observability stack is not just “nice to have”, but absolutely essential. And all of this starts with better monitoring and metrics.

> We executed a variety of POCs on various solutions and finally arrived at the following technologies: M3DB, Thanos, Cortex and VictoriaMetrics. The clear winner was VictoriaMetrics.

> The following are some of the basic observations we derived from Victoria Metrics:
>
> - Simple components, each horizontally scalable.
> - Clear separation between writes and reads.
> - Runs from default configurations, with no extra frills.
> - Default retention starts with 1 month
> - Storage, ingestion, and reads can be easily scaled.
> - High Compression store ~ 70% more compression.
> - Currently running in production with commodity hardware with a good mix of spot instances.
> - Successfully ran some of the worst Grafana dashboards/queries that have historically failed to run.

See [the full article](https://engineering.razorpay.com/scaling-to-trillions-of-metric-data-points-f569a5b654f2).

## RELEX Solutions

[RELEX Solutions](https://www.relexsolutions.com/), a global software company from Finland, is the market-leading supply chain and retail planning platform.

VictoriaMetrics is used as the central metrics storage for timeseries about applications and machines hosted both in the public cloud and in the private cloud. Metrics are remote-written by Prometheus, the OpenTelemetry collector and sometimes directly by custom Prometheus exporters. 

Alerts are evaluated on vmalert when necessary, either because metrics from multiple sources are needed or because the source is Prometheus in agent mode (mostly for kubernetes clusters). Prometheus Alertmanager and Grafana+Promxy combine all sources together so that the end users of dashboards (and ideally the recipients of alert notifications) don't have to worry where to look for some information.

VictoriaMetrics has allowed us to extend data retention for our metrics effortlessly, while Prometheus tsdb would have required us to manage ever bigger disks or to plan aggregation and downsampling within a Prometheus hierarchical federation. It also allows for a separation of concerns: whereas alerting requires immediacy and precision in the metrics, visualizations of historical data can bear with delay and downsampling.

Across our production VictoriaMetrics clusters, in a 12 months period we go beyond the following figures.

- Active time series: 10M
- Ingestion rate: 300k samples per second
- Total number of datapoints: 4400G
- Data size on disk: 3600 GiB
- Available memory: 320 GiB as seen by kubernetes (160 GiB physical memory for the hosts)
- CPU: 20 cores (AMD EPYC 7763), about 70 % idle
- Retention period: ~1 year
- Churn rate: 6M new time series per day (monthly average)
- Query rate:
  - `/api/v1/query_range`: 10 queries per second
  - `/api/v1/query`: 10 queries per second
- Query duration for `/api/v1/query_range` (weekly mean):
  - 99th percentile: 700 ms
  - median: 10 ms

## Roblox

[Roblox](https://roblox.com/) builds the tools and platform that empower people to create their own immersive experiences,
so that any world they can imagine can be brought to life.

With more than 200 million active monthly users, Roblox is one of the most popular gaming platforms on the Internet.
The company started with Prometheus and InfluxDB for observability, but as volume continued to build,
Roblox eventually adopted a time-series database called VictoriaMetrics.

That sudden growth coincided with a substantial rebuild of Roblox’s observability system, which informed internal users about
everything from the condition of Roblox application servers and storage infrastructure to the number of players and player experience.

The original observability system was largely homegrown, consisting of some open source software and internally developed tools.
It was based on a decentralized model and leveraged individual Prometheus and InfluxDB instances to store data streaming
in from the infrastructure and the platform, which internal Roblox users analyzed using Grafana and an internally developed tool called RCity.

From a usability perspective, there was no central repository for data or analysis, and engineers had to jump around to different tools
to get what they needed. It also hurt Roblox’s flexibility to know how the platform was working in a new region or a new country.

And while the simplicity of storing data in key-value pairs aided visualization, it made doing in-depth analysis a real pain.

Lastly, the homegrown observability system hurt reliability for Roblox.

By all accounts, the move to VictoriaMetrics and Grafana has been a success. With a centralized telemetry database and visualization system,
Roblox engineers don’t have to hunt around for different tools when problems arise, which was source of delay before.
The new setup has also helped Roblox deliver 100% availability for three straight quarters, despite moving a massive amount of data.

Numbers:

- 200 storage nodes in VictoriaMetrics cluster
- Active time series: 5 billion
- Data ingestion rate: 120 million data points per second

See more details [in this article](https://www.datanami.com/2023/05/30/why-roblox-picked-victoriametrics-for-observability-data-overhaul/).

## Sensedia

[Sensedia](https://www.sensedia.com) is a leading integration solutions provider with more than 120 enterprise clients across a range of sectors. Its world-class portfolio includes: an API Management Platform, Adaptive Governance, Events Hub, Service Mesh, Cloud Connectors and Strategic Professional Services' teams.

> Our initial requirements for monitoring solution: the metrics must be stored for 15 days, the solution must be scalable and must offer high availability of the metrics. It must being integrated into Grafana and allowing the use of PromQL when creating/editing dashboards in Grafana to obtain metrics from the Prometheus datasource. The solution also needs to receive data from Prometheus using HTTPS and needs to request a login and password to write/read the metrics. Details are available [in this article](https://nordicapis.com/api-monitoring-with-prometheus-grafana-alertmanager-and-victoriametrics/).

> We evaluated VictoriaMetrics, InfluxDB OpenSource and Enterprise, Elasticsearch, Thanos, Cortex, TimescaleDB/PostgreSQL and M3DB. We selected VictoriaMetrics because it has [good community support](https://slack.victoriametrics.com/), [good documentation](https://docs.victoriametrics.com/) and it just works.

> We started using VictoriaMetrics in the production environment days before the start of BlackFriday in 2020, the period of greatest use of the Sensedia API-Platform by customers. There was a record in the generation of metrics and there was no instability with the monitoring stack.

> We use VictoriaMetrics in cluster mode for centralized storage of metrics collected by several Prometheus servers installed in Kubernetes clusters from two different cloud providers. VictoriaMetrics has also been integrated with Grafana to view metrics.

[Aecio dos Santos Pires](http://aeciopires.com), Cloud Architect, Sensedia.

Numbers:

- Cluster mode
- Active time series: 700K
- Ingestion rate: 70K datapoints per second
- Datapoints: 112 billions
- Data size on disk: 82 GB
- Index size on disk: 30 GB
- Churn rate: 3 million of new time series per day
- Query response time (99th percentile): 500ms

## Smarkets

[Smarkets](https://smarkets.com/) simplifies peer-to-peer trading on sporting and political events.

> We always wanted our developers to have out-of-the-box monitoring available for any application or service. Before we adopted Kubernetes this was achieved either with Prometheus metrics, or with statsd being sent over to the underlying host and then converted into Prometheus metrics. As we expanded our Kubernetes adoption and started to split clusters, we also wanted developers to be able to expose metrics directly to Prometheus by annotating services. Those metrics were then only available inside the cluster so they couldn’t be scraped globally.

> We considered three different solutions to improve our architecture:
>
> - Prometheus + Cortex
> - Prometheus + Thanos Receive
> - Prometheus + Victoria Metrics

> We selected Victoria Metrics. Our new architecture has been very stable since it was put into production. With the previous setup we would have had two or three cardinality explosions in a two-week period, with this new one we have none.

See [the full article](https://smarketshq.com/monitoring-kubernetes-clusters-41a4b24c19e3).

## Synthesio

[Synthesio](https://www.synthesio.com/) is the leading social intelligence tool for social media monitoring and analytics.

> We fully migrated from [Metrictank](https://github.com/grafana/metrictank)  to VictoriaMetrics

Numbers:

- Single node
- Active time series: 5 millions
- Datapoints: 1.25 trillions
- Ingestion rate: 550K datapoints per second
- Disk usage: 150 GB
- Index size: 3 GB
- Query duration 99th percentile: 147ms
- Churn rate: 2400 new time series per day

## Wedos.com

> [Wedos](https://www.wedos.com/) is the biggest hosting provider in the Czech Republic. We have two our own private data centers that hold our servers and technologies, such as cooling the servers in bath oils. We started using [cluster VictoriaMetrics](https://docs.victoriametrics.com/cluster-victoriametrics/) to store Prometheus metrics from all our infrastructure after receiving positive references from people who had successfully used VictoriaMetrics. We're using it throughout our services, including the new WEDOS Global Protection.

Numbers:

- The number of active time series: 32M.
- Ingestion rate: 1.6M data points per second.
- Query duration: median is ~2ms, 99th percentile is ~50ms.
- The total number of datapoints per VictoriaMetrics "hot" storage is 5.3 trillion

> We like that VictoriaMetrics is simple to configure and requires zero maintenance. It works right out of the box and once it's set up you can just forget about it.

## Wix.com

[Wix.com](https://en.wikipedia.org/wiki/Wix.com) is the leading web development platform.

> We needed to redesign our metrics infrastructure from the ground up after the move to Kubernetes. We had tried out a few different options before landing on this solution which is working great. We have a Prometheus instance in every datacenter with 2 hours retention for local storage and remote write into [HA pair of single-node VictoriaMetrics instances](https://docs.victoriametrics.com/single-server-victoriametrics/#high-availability).

Numbers:

- The number of active time series per VictoriaMetrics instance is 50 millions.
- The total number of time series per VictoriaMetrics instance is 5000 million.
- Ingestion rate per VictoriaMetrics instance is 1.1 millions data points per second.
- The total number of datapoints per VictoriaMetrics instance is 8.5 trillion.
- The average churn rate is 150 millions new time series per day.
- The average query rate is ~150 per second (mostly alert queries).
- Query duration: median is ~1ms, 99th percentile is ~1sec.
- Retention period: 3 months.

> The alternatives that we tested prior to choosing VictoriaMetrics were: Prometheus federated, Cortex, IronDB and Thanos.
> The items that were critical to us central tsdb, in order of importance were as follows:

- At least 3 month worth of retention.
- Raw data, no aggregation, no sampling.
- High query speed.
- Clean fail state for HA (multi-node clusters may return partial data resulting in false alerts).
- Enough headroom/scaling capacity for future growth which is planned to be up to 100M active time series.
- Ability to split DB replicas per workload. Alert queries go to one replica and user queries go to another (speed for users, effective cache).

> Optimizing for those points and our specific workload, VictoriaMetrics proved to be the best option. As icing on the cake we’ve got [PromQL extensions](https://docs.victoriametrics.com/metricsql/) - `default 0` and `histogram` are my favorite ones. We really like having a lot of tsdb params easily available via config options which makes tsdb easy to tune for each specific use case. We've also found a great community in [Slack channel](https://slack.victoriametrics.com/) and responsive and helpful maintainer support.

Alex Ulstein, Head of Monitoring, Wix.com


## xiaohongshu

With a mission to “inspire lives”, [Xiaohongshu](https://www.xiaohongshu.com) is a lifestyle platform that inspires people to discover and connect with a range of diverse lifestyles from China.
In the past year, we used VictoriaMetrics to replace Prometheus. After migrating to VictoriaMetrics, we had saved more than ten thousand cpu cores, and our metrics system is more stable. 
Now more than thirty VictoriaMetrics storage clusters are running online, including all of our key business areas, such as recommendations, search, community, advertising, infrastructure, etc.
See [this article](https://mp.weixin.qq.com/s/uJ1t0B8WBBryzvbLWDfl5A) on how Xiaohongshu build metrics system base on VictoriaMetrics and the competing solutions.

Across our production VictoriaMetrics clusters, numbers as below:
- Cpu cores in all VictoriaMetrics clusters: almost 50000
- Data size on disk: 2400 TB
- Retention period: 1 month
- Largest VictoriaMetrics cluster: 450 million samples every scrape interval (15 seconds)
- Total scrape sample every scrape interval: 2400 million samples
- Query rate:
  - /api/v1/query_range: 2300 queries per second
  - /api/v1/query: 260 queries per second


## Zerodha

[Zerodha](https://zerodha.com/) is India's largest stock broker. The monitoring team at Zerodha had the following requirements:

- Multiple K8s clusters to monitor
- Consistent monitoring infra for each cluster across the fleet
- The ability to handle billions of timeseries events at any point of time
- Easy to operate and cost effective

Thanos, Cortex and VictoriaMetrics were evaluated as a long-term storage for Prometheus. VictoriaMetrics has been selected for the following reasons:

- Blazingly fast benchmarks for a single node setup.
- Single binary mode. Easy to scale vertically with far fewer operational headaches.
- Considerable [improvements on creating Histograms](https://medium.com/@valyala/improving-histogram-usability-for-prometheus-and-grafana-bc7e5df0e350).
- [MetricsQL](https://docs.victoriametrics.com/metricsql/) gives us the ability to extend PromQL with more aggregation operators.
- The API is compatible with Prometheus and nearly all standard PromQL queries work well out of the box.
- Handles storage well, with periodic compaction which makes it easy to take snapshots.

Please see [Monitoring K8S with VictoriaMetrics](https://docs.google.com/presentation/d/1g7yUyVEaAp4tPuRy-MZbPXKqJ1z78_5VKuV841aQfsg/edit) slides,
[video](https://youtu.be/ZJQYW-cFOms) and [Infrastructure monitoring with Prometheus at Zerodha](https://zerodha.tech/blog/infra-monitoring-at-zerodha/) blog post for more details.

## zhihu

[zhihu](https://www.zhihu.com) is the largest Chinese question-and-answer website. We use VictoriaMetrics to store and use Graphite metrics. We shared the [promate](https://github.com/zhihu/promate) solution in our [单机 20 亿指标，知乎 Graphite 极致优化！](https://qcon.infoq.cn/2020/shenzhen/presentation/2881)([slides](https://static001.geekbang.org/con/76/pdf/828698018/file/%E5%8D%95%E6%9C%BA%2020%20%E4%BA%BF%E6%8C%87%E6%A0%87%EF%BC%8C%E7%9F%A5%E4%B9%8E%20Graphite%20%E6%9E%81%E8%87%B4%E4%BC%98%E5%8C%96%EF%BC%81-%E7%86%8A%E8%B1%B9.pdf)) talk at [QCon 2020](https://qcon.infoq.cn/2020/shenzhen/).

Numbers:

- Active time series: ~25 Million
- Datapoints: ~20 Trillion
- Ingestion rate: ~1800k/s
- Disk usage: ~20 TB
- Index size: ~600 GB
- The average query rate is ~3k per second (mostly alert queries).
- Query duration: median is ~40ms, 99th percentile is ~100ms.
