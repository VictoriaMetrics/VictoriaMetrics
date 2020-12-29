### Snap integration

https://snapcraft.io/

snap link: https://snapcraft.io/victoriametrics


#### develop

Install snapcraft and multipass:
 ```text
sudo snap install snapcraft --classic
```

build victoria-metrics prod binary and run snapcraft ```snapcraft --debug```. 
It produces snap package with current git version - `victoriametrics_v1.46.0+git1.1bebd021a-dirty_all.snap`.
You can install it with command: `snap install victoriametrics_v1.46.0+git1.1bebd021a-dirty_all.snap --dangerous`


#### usage 

installation and configuration:

```text
# install
snap install victoriametrics
# logs
snap logs victoriametrics
# restart
 snap restart victoriametrics
```

Configuration management:

 Prometheus scrape config can be edited with your favorite editor, its located at
```text
vi /var/snap/victoriametrics/current/etc/victoriametrics-scrape-config.yaml
```
after changes, you can trigger config reread with `curl localhost:8248/-/reload`.

Configuration tuning is possible with editing extra_flags:
```text
echo 'FLAGS="-selfScrapeInterval=10s -search.logSlowQueryDuration=20s"' > /var/snap/victoriametrics/current/extra_flags
snap restart victoriametrics
```

Data folder located at `/var/snap/victoriametrics/current/var/lib/victoriametrics/`