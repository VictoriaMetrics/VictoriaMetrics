---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''
---

**Describe the bug**
A clear and concise description of what the bug is.
It would be a great [upgrading](https://docs.victoriametrics.com/#how-to-upgrade) 
to [the latest available release](https://github.com/VictoriaMetrics/VictoriaMetrics/releases)
and verifying whether the bug is reproducible there.
It is also recommended reading [troubleshooting docs](https://docs.victoriametrics.com/#troubleshooting).

**To Reproduce**
Steps to reproduce the behavior.

**Expected behavior**
A clear and concise description of what you expected to happen.

**Logs**
Check if any warnings or errors were logged by VictoriaMetrics components
or components in communication with VictoriaMetrics (e.g. Prometheus, Grafana).

**Screenshots**
If applicable, add screenshots to help explain your problem.

For VictoriaMetrics health-state issues please provide full-length screenshots 
of Grafana dashboards if possible:
* [Grafana dashboard for single-node VictoriaMetrics](https://grafana.com/dashboards/10229)
* [Grafana dashboard for VictoriaMetrics cluster](https://grafana.com/grafana/dashboards/11176)

See how to setup monitoring here:
* [monitoring for single-node VictoriaMetrics](https://docs.victoriametrics.com/#monitoring)
* [montioring for VictoriaMetrics cluster](https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#monitoring)

**Version**
The line returned when passing `--version` command line flag to binary. For example:
```
$ ./victoria-metrics-prod --version
victoria-metrics-20190730-121249-heads-single-node-0-g671d9e55
```

**Used command-line flags**
Please provide applied command-line flags used for running VictoriaMetrics and its components. 

