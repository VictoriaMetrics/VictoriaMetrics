## Case studies and talks

Below are approved public case studies and talks from VictoriaMetrics users. Join our [community Slack channel](http://slack.victoriametrics.com/)
and feel free asking for references, reviews and additional case studies from real VictoriaMetrics users there.

### Adidas

See [slides](https://promcon.io/2019-munich/slides/remote-write-storage-wars.pdf) and [video](https://youtu.be/OsH6gPdxR4s)
from [Remote Write Storage Wars](https://promcon.io/2019-munich/talks/remote-write-storage-wars/) talk at [PromCon 2019](https://promcon.io/2019-munich/).
VictoriaMetrics is compared to Thanos, Corex and M3DB in the talk.


### COLOPL

[COLOPL](http://www.colopl.co.jp/en/) is Japaneese Game Development company. It started using VictoriaMetrics
after evaulating the following remote storage solutions for Prometheus:

* Cortex
* Thanos
* M3DB
* VictoriaMetrics

See [slides](https://speakerdeck.com/inletorder/monitoring-platform-with-victoria-metrics) and [video](https://www.youtube.com/watch?v=hUpHIluxw80)
from `Large-scale, super-load system monitoring platform built with VictoriaMetrics` talk at [Prometheus Meetup Tokyo #3](https://prometheus.connpass.com/event/157721/).


### Wix.com

[Wix.com](https://en.wikipedia.org/wiki/Wix.com) is the leading web development platform.

> We needed to redesign metric infrastructure from the ground up after the move to Kubernethes. A few approaches/designs have been tried before the one that works great has been chosen: Prometheus instance in every datacenter with 2 hours retention for local storage and remote write into [HA pair of single-node VictoriaMetrics instances](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/README.md#high-availability).

Numbers:

* The number of active time series per VictoriaMetrics instance is 20M.
* The total number of time series per VictoriaMetrics instance is 400M+.
* Ingestion rate per VictoriaMetrics instance is 800K data points per second.
* The average time series churn rate is ~3M per day.
* The average query rate is ~1K per minute (mostly alert queries).
* Query duration: median is ~70ms, 99th percentile is ~2sec.
* Retention: 6 months.

> Alternatives that we’ve played with before choosing VictoriaMetrics are: federated Prometheus, Cortex, IronDB and Thanos.
> Points that were critical to us when we were choosing a central tsdb, in order of importance:

* At least 3 month worth of history.
* Raw data, no aggregation, no sampling.
* High query speed.
* Clean fail state for HA (multi-node clusters may return partial data resulting in false alerts).
* Enough head room/scaling capacity for future growth, up to 100M active time series.
* Ability to split DB replicas per workload. Alert queries go to one replica, user queries go to another (speed for users, effective cache).

> Optimizing for those points and our specific workload VictoriaMetrics proved to be the best option. As an icing on a cake we’ve got [PromQL extensions](https://github.com/VictoriaMetrics/VictoriaMetrics/wiki/MetricsQL) - `default 0` and `histogram` are my favorite ones, for example. What we specially like is having a lot of tsdb params easily available via config options, that makes tsdb easy to tune for specific use case. Also worth noting is a great community in [Slack channel](http://slack.victoriametrics.com/) and of course maintainer support.

Alex Ulstein, Head of Monitoring, Wix.com


### Wedos.com

> [Wedos](https://www.wedos.com/) is the Biggest Czech Hosting. We have our own private data center, that holds only our servers and technologies. The second data center, where the servers will be cooled in an oil bath, is being built. We started using [cluster VictoriaMetrics](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/cluster/README.md) to store Prometheus metrics from all our infrastructure after receiving positive references from our friends who successfully use VictoriaMetrics.

Numbers:

* The number of acitve time series: 5M.
* Ingestion rate: 170K data points per second.
* Query duration: median is ~2ms, 99th percentile is ~50ms.

> We like configuration simplicity and zero maintenance for VictoriaMetrics - once installed and forgot about it. It works out of the box without any issues.


### Synthesio

[Synthesio](https://www.synthesio.com/) is the leading social intelligence tool for social media monitoring & social analytics.

> We fully migrated from [Metrictank](https://grafana.com/oss/metrictank/)  to Victoria Metrics

Numbers:
- Single node
- Active time series - 5 Million
- Datapoints: 1.25 Trillion
- Ingestion rate - 550k datapoints per second
- Disk usage - 150gb
- Index size - 3gb
- Query duration 99th percentile - 147ms
- Churn rate - 100 new time series per hour


### MHI Vestas Offshore Wind

The mission of [MHI Vestas Offshore Wind](http://www.mhivestasoffshore.com) is to co-develop offshore wind as an economically viable and sustainable energy resource to benefit future generations.

MHI Vestas Offshore Wind is using VictoriaMetrics to ingest and visualize sensor data from offshore wind turbines. The very efficient storage and ability to backfill was key in chosing VictoriaMetrics. MHI Vestas Offshore Wind is running the cluster version of VictoriaMetrics on Kubernetes using the Helm charts for deployment to be able to scale up capacity as the solution will be rolled out. Current roll-out is

Numbers with current limited roll out:

- Active time series: 270K
- Ingestion rate: 70K/sec
- Total number of datapoints: 850 billions
- Data size on disk: 800 GiB
- Retention time: 3 years


### Dreamteam

[Dreamteam](https://dreamteam.gg/) successfully uses single-node VictoriaMetrics in multiple environments.

Numbers:

* Active time series: from 350K to 725K.
* Total number of time series: from 100M to 320M.
* Total number of datapoints: from 120 billions to 155 billions.
* Retention: 3 months.

VictoriaMetrics in production environment runs on 2 M5 EC2 instances in "HA" mode, managed by Terraform and Ansible TF module.
2 Prometheus instances are writing to both VMs, with 2 [Promxy](https://github.com/jacksontj/promxy) replicas
as load balancer for reads.
