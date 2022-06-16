---
sort: 18
---

# Release process guidance

## Release version and Docker images

0. Make sure that the release commits have no security issues.
1. Document all the changes for new release in [CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md).
2. Create the following release tags:
   * `git tag -s v1.xx.y` in `master` branch
   * `git tag -s v1.xx.y-cluster` in `cluster` branch
   * `git tag -s v1.xx.y-enterprise` in `enterprise` branch
   * `git tag -s v1.xx.y-enterprise-cluster` in `enterprise-cluster` branch
3. Run `TAG=v1.xx.y make publish-release`. It will create `*.tar.gz` release archives with the corresponding `_checksums.txt` files inside `bin` directory and publish Docker images for the given `TAG`, `TAG-cluster`, `TAG-enterprise` and `TAG-enterprise-cluster`.
4. Push release tags to <https://github.com/VictoriaMetrics/VictoriaMetrics> : `git push origin v1.xx.y` and `git push origin v1.xx.y-cluster`. Do not push `-enterprise` tags to public repository.
5. Go to <https://github.com/VictoriaMetrics/VictoriaMetrics/releases> , create new release from the pushed tag on step 4 and upload `*.tar.gz` archive with the corresponding `_checksums.txt` from step 3.
6. Copy the [CHANGELOG](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md) for this release to [releases](https://github.com/VictoriaMetrics/VictoriaMetrics/releases) page.
7. Bump version of the VictoriaMetrics cluster setup in for [sandbox environment](https://github.com/VictoriaMetrics/ops/blob/main/sandbox/manifests/benchmark-vm/vmcluster.yaml)
by [opening and merging PR](https://github.com/VictoriaMetrics/ops/pull/58).

## Building snap package

 pre-requirements:

* snapcraft binary, can be installed with commands:
   for MacOS `brew install snapcraft` and [install mutipass](https://discourse.ubuntu.com/t/installing-multipass-on-macos/8329),
   for Ubuntu - `sudo snap install snapcraft --classic`
* exported snapcraft login to `~/.snap/login.json` with `snapcraft export-login login.json && mkdir -p ~/.snap && mv login.json ~/.snap/`
* already created release at github (it operates `git describe` version, so git tag must be annotated).

1. checkout to the latest git tag for single-node version.
2. execute `make release-snap` - it must build and upload snap package.
3. promote release to current, if needed manually at release page [snapcraft-releases](https://snapcraft.io/victoriametrics/releases)

### Public Announcement

* Publish message in Slack  at <https://victoriametrics.slack.com>
* Post at Twitter at <https://twitter.com/MetricsVictoria>
* Post in Reddit at <https://www.reddit.com/r/VictoriaMetrics/>
* Post in Linkedin at <https://www.linkedin.com/company/victoriametrics/>
* Publish message in Telegram at <https://t.me/VictoriaMetrics_en> and <https://t.me/VictoriaMetrics_ru1>
* Publish message in google groups at <https://groups.google.com/forum/#!forum/victorametrics-users>

## Helm Charts

The helm chart repository [https://github.com/VictoriaMetrics/helm-charts/](https://github.com/VictoriaMetrics/helm-charts/)

### Bump the version of images

1. Update `vmagent` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-agent/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-agent/Chart.yaml) 
2. Update `vmalert` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-alert/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-alert/Chart.yaml)
3. Update `vmauth` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-auth/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-auth/Chart.yaml)
4. Update `cluster` chart versions in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/values.yaml), bump version for `vmselect`, `vminsert` and `vmstorage` and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/Chart.yaml)
5. Update `k8s-stack` chart versions in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/values.yaml), bump version for `vmselect`, `vminsert`, `vmstorage`, `vmsingle`, `vmalert`, `vmagent` and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/Chart.yaml)
6. Update `single-node` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-single/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-single/Chart.yaml)
8. Run `make gen-doc`
9. Run `make package` that creates or updates zip file with the packed chart
10. Run `make merge`. It creates or updates metadata for charts in index.yaml
11. Push changes to master. `master` is a source of truth
12. Push the changes to `gh-pages` branch

## Ansible Roles 

### Bump the version of images

Repository [https://github.com/VictoriaMetrics/ansible-playbooks](https://github.com/VictoriaMetrics/ansible-playbooks)

1. Update `vmagent` version in [`main.yml`](https://github.com/VictoriaMetrics/ansible-playbooks/blob/master/roles/vmagent/defaults/main.yml#L4)
2. Update `vmalert` version in [`main.yml`](https://github.com/VictoriaMetrics/ansible-playbooks/blob/master/roles/vmalert/defaults/main.yml#L4)
3. Update `cluster` version in [`main.yml`](https://github.com/VictoriaMetrics/ansible-playbooks/blob/master/roles/cluster/defaults/main.yml#L3)
4. Update `single` version in [`main.yml`](https://github.com/VictoriaMetrics/ansible-playbooks/blob/master/roles/single/defaults/main.yml#L6)
5. Commit changes
6. Create a new tag
7. Create a new release. This automatically publishes the new versions to galaxy.ansible.com 

## Github pages

All changes in `README.md`, `docs` folder and `.md` extension automatically push to Wiki
