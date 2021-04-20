---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.
It would be a great [upgrading](https://docs.victoriametrics.com/#how-to-upgrade) to [the latest avaialble release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
and verifying whether the bug is reproducible there.
It is also recommended reading [troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting).

**To Reproduce**
Steps to reproduce the behavior.

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Version**
The line returned when passing `--version` command line flag to binary. For example:
```
$ ./victoria-metrics-prod --version
victoria-metrics-20190730-121249-heads-single-node-0-g671d9e55
```

**Used command-line flags**
Command-line flags are listed as `flag{name="httpListenAddr", value=":443"} 1` lines at the `/metrics` page.
See the following docs for details:

* [monitoring for single-node VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
* [montioring for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#monitoring)

**Additional context**
Add any other context about the problem here such as error logs from VictoriaMetrics and Prometheus,
`/metrics` output, screenshots from the official Grafana dashboards for VictoriaMetrics:

* [Grafana dashboard for single-node VictoriaMetrics](https://grafana.com/dashboards/10229)
* [Grafana dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11176)
