---
sort: 2
---

# Helm charts release

## Bump the version of images.

1. Need to update [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-operator/values.yaml), 
2. Specify the correct version in [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-operator/Chart.yaml)
3. Update version [README.md](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-operator/README.md), specify the new version in the documentation
4. Push changes to master. `master` is a source of truth
5. Rebase `master` into `gh-pages` branch
6. Run `make package` which creates or updates zip file with the packed chart
7. Run `make merge`. It creates or updates metadata for charts in index.yaml
8. Push the changes to `gh-pages` branch


## Operator Hub release

checkout to the latest release:
1) `git checkout tags/v0.7.2`
2) build package manifest: `TAG=v0.7.2 make packagemanifests`
3) add replacement for a previous version to generated cluster csv:
`vi packagemanifests/0.7.2/victoriametrics-operator.0.7.2.clusterserviceversion.yaml`
```yaml
spec:
  replaces: victoriametrics-operator.v0.6.1
```
4) publish changes to the quay, login first with your login, password:
`bash hack/get_quay_token.sh`, copy token content to var `export AUTH_TOKEN="basic afsASF"`, 
   then push change to quay: `TAG=v0.7.2 make packagemanifests-push`
   
5) now you have to copy content of packagemanifests to the community-operators repo,
  first for upstream, next for community, sign-off commits and create PRs.
   ```bash
   cp -R packagemanifests/* ~/community-operators/upstream-community-operators/victoriametrics-operator
   cd ~/community-operators
   git add upstream-community-operators/victoriametrics-operator/0.7.2/
   git add upstream-community-operators/victoriametrics-operator/victoriametrics-operator.package.yaml
   git commit -m "updates victoriametrics operator version 0.7.2
   Signed-off-by: Nikolay Khramchikhin <nik@victoriametrics.com>" --signoff
   ```
   checkout to the new branch and create separate commit for openshift operator-hub
   ```bash
   cp -R packagemanifests/* ~/community-operators/community-operators/victoriametrics-operator
   cd ~/community-operators
   git add community-operators/victoriametrics-operator/0.7.2/
   git add community-operators/victoriametrics-operator/victoriametrics-operator.package.yaml
   git commit -m "updates victoriametrics operator version 0.7.2
   Signed-off-by: Nikolay Khramchikhin <nik@victoriametrics.com>" --signoff
   ```
   
6) create pull requests at community-operator repo:
   https://github.com/operator-framework/community-operators
