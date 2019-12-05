Release process guidance

## Release version and Docker images

1. Create release tag with `git tag v1.xx.y`.
2. Run `make release` for creating `*.tar.gz` release archive with the corresponding `_checksums.txt` inside `bin` directory.
3. Run `make publish` for creating and publishing Docker images.
4. Push release tag to https://github.com/VictoriaMetrics/VictoriaMetrics : `git push origin v1.xx.y`.
5. Go to https://github.com/VictoriaMetrics/VictoriaMetrics/releases , create new release from the pushed tag on step 4
   and upload `*.tar.gz` archive with the corresponding `_checksums.txt` from step 2.


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
