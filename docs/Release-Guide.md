Release process guidance

## Release version and Docker images

0. Document all the changes for new release in [CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md).
1. Create release tag with `git tag v1.xx.y` in `master` branch and `git tag v1.xx.y-cluster` in `cluster` branch.
2. Run `make release` for creating `*.tar.gz` release archive with the corresponding `_checksums.txt` inside `bin` directory.
3. Run `make publish` for creating and publishing Docker images.
4. Repeat steps 3-4 for `cluster` branch.
5. Push release tag to https://github.com/VictoriaMetrics/VictoriaMetrics : `git push origin v1.xx.y`.
6. Go to https://github.com/VictoriaMetrics/VictoriaMetrics/releases , create new release from the pushed tag on step 5 and upload `*.tar.gz` archive with the corresponding `_checksums.txt` from step 2.

## Building snap package.

 pre-requirements: 
- snapcraft binary, can be installed with commands:
   for MacOS`brew install snapcraft` and [install mutipass](https://discourse.ubuntu.com/t/installing-multipass-on-macos/8329),
   for Ubuntu - `sudo snap install snapcraft --classic`
- login with `snapcraft login`

0. checkout to the latest git tag for single-node version.
1. execute `make release-snap` - it must build and upload snap package.
2. promote release to current, if needed manually at release page [snapcraft-releases](https://snapcraft.io/victoriametrics/releases)

### Public Announcement 

1. Publish message in slack (victoriametrics.slack.com, general channel)
2. Post twit with release notes URL
3. Post in subreddit https://www.reddit.com/r/VictoriaMetrics/ 
4. Post in linkedin

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
