# victoriametrics-rpm
RPM for VictoriaMetrics - the best long-term remote storage for Prometheus

*Get and started*

```
yum -y install yum-plugin-copr

yum copr enable antonpatsev/VictoriaMetrics

yum makecache

yum -y install victoriametrics

systemctl start victoriametrics
```
