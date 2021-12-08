---
sort: 17
---

# Release process guidance

## Release version and Docker images

0. Document all the changes for new release in [CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md).
1. Create the following release tags:
   * `git tag v1.xx.y` in `master` branch
   * `git tag v1.xx.y-cluster` in `cluster` branch
   * `git tag v1.xx.y-enterprise` in `enterprise` branch
   * `git tag v1.xx.y-enterprise-cluster` in `enterprise-cluster` branch
2. Run `TAG=v1.xx.y make publish-release`. It will create `*.tar.gz` release archives with the corresponding `_checksums.txt` files inside `bin` directory and publish Docker images for the given `TAG`, `TAG-cluster`, `TAG-enterprise` and `TAG-enterprise-cluster`.
5. Push release tag to https://github.com/VictoriaMetrics/VictoriaMetrics : `git push origin v1.xx.y`.
6. Go to https://github.com/VictoriaMetrics/VictoriaMetrics/releases , create new release from the pushed tag on step 5 and upload `*.tar.gz` archive with the corresponding `_checksums.txt` from step 2.

## Building snap package.

 pre-requirements: 
- snapcraft binary, can be installed with commands:
   for MacOS `brew install snapcraft` and [install mutipass](https://discourse.ubuntu.com/t/installing-multipass-on-macos/8329),
   for Ubuntu - `sudo snap install snapcraft --classic`
- login with `snapcraft login`
- already created release at github (it operates `git describe` version, so git tag must be annotated).

0. checkout to the latest git tag for single-node version.
1. execute `make release-snap` - it must build and upload snap package.
2. promote release to current, if needed manually at release page [snapcraft-releases](https://snapcraft.io/victoriametrics/releases)

### Public Announcement 

- Publish message in Slack  at https://victoriametrics.slack.com
- Post at Twitter at https://twitter.com/MetricsVictoria
- Post in Reddit at https://www.reddit.com/r/VictoriaMetrics/
- Post in Linkedin at https://www.linkedin.com/company/victoriametrics/
- Publish message in Telegram at https://t.me/VictoriaMetrics_en and https://t.me/VictoriaMetrics_ru1
- Publish message in google groups at https://groups.google.com/forum/#!forum/victorametrics-users

## Helm Charts

The helm chart repository [https://github.com/VictoriaMetrics/helm-charts/](https://github.com/VictoriaMetrics/helm-charts/)


### Bump the version of images. 
In that case, don't need to bump the helm chart version

1. Need to update [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/values.yaml), bump version for `vmselect`, `vminsert` and `vmstorage`
2. Specify the correct version in [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/Chart.yaml)
3. Update version [README.md](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/README.md), specify the new version in the documentation
4. Push changes to master. `master` is a source of truth
5. Rebase `master` into `gh-pages` branch
6. Run `make package` which creates or updates zip file with the packed chart
7. Run `make merge`. It creates or updates metadata for charts in index.yaml 
8. Push the changes to `gh-pages` branch 

### Updating the chart.
1. Update chart version in [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/Chart.yaml)
2. Update [README.md](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/README.md) file, reflect changes in the documentation.
3. Repeat the procedure from step _4_ previous section.


## Wiki pages

All changes from `docs` folder and `.md` extension automatically push to Wiki

**_Note_**: no vice versa, direct changes on Wiki will be overitten after any changes in `docs/*.md` 

## Github pages

All changes in `README.md`, `docs` folder and `.md` extension automatically push to Wiki
