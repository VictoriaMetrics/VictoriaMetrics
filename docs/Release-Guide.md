---
sort: 18
weight: 18
title: Release process guidance
menu:
  docs:
    parent: "victoriametrics"
    weight: 18
aliases:
- /Release-Guide.html
---

# Release process guidance

## Pre-reqs

1. Make sure you have enterprise remote configured
```
git remote add enterprise <url>
```
2. Make sure you have singing key configured
3. Make sure you have github token with at least `read:org, repo, write:packages` permissions exported under `GITHUB_TOKEN` env variable.
   You can create token [here](https://github.com/settings/tokens)
4. Make sure you're [authorized](https://hub.docker.com/orgs/victoriametrics/settings/enforce-sign-in/windows) for pushing docker images 

### For MacOS users

Make sure you have GNU version of utilities `zip`, `tar`, `sha256sum`. To install them run the following commands:
```bash
brew install coreutils
brew install gnu-tar
export PATH="/usr/local/opt/coreutils/libexec/gnubin:$PATH"
```

Docker may need additional configuration changes:
```bash 
docker buildx create --use --name=qemu
docker buildx inspect --bootstrap  
```

For ARM arch (M1/M2 processors) additionally configure docker with preferred platform:
```
export DOCKER_DEFAULT_PLATFORM=linux/amd64
```

By default, docker on MacOS has limited amount of resources (CPU, mem) to use. 
Bumping the limits may significantly improve build speed.

## Release version and Docker images

0. Make sure that the release commits have no security issues.
1a. Document all the changes for new release in [CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md) and update version if needed in [SECURITY.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/SECURITY.md)
1b. Add `(available starting from v1.xx.y)` line to feature docs introduced in the upcoming release.
1c. Cut new version in [CHANGELOG.md](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md)
and make it merged. See example in this [commit](https://github.com/VictoriaMetrics/VictoriaMetrics/commit/b771152039d23b5ccd637a23ea748bc44a9511a7).
1d. Cherry-pick all changes from `master` to `cluster` branch, and to ENT branches `enterprise-single-node`, `enterprise-cluster`.
1e. Cherry-pick bug fixes relevant for LTS releases.
2. Make sure you get all changes fetched `git fetch --all`.
3. Create the following release tags:
   * `git tag -s v1.xx.y` in `master` branch
   * `git tag -s v1.xx.y-cluster` in `cluster` branch
   * `git tag -s v1.xx.y-enterprise` in `enterprise` branch
   * `git tag -s v1.xx.y-enterprise-cluster` in `enterprise-cluster` branch
3. Run `TAG=v1.xx.y make publish-release`. This command performs the following tasks:
   a) Build and package binaries in `*.tar.gz` release archives with the corresponding `_checksums.txt` files inside `bin` directory.
      This step can be run manually with the command `make release` from the needed git tag.
   b) Build and publish [multi-platform Docker images](https://docs.docker.com/build/buildx/multiplatform-images/)
      for the given `TAG`, `TAG-cluster`, `TAG-enterprise` and `TAG-enterprise-cluster`.
      The multi-platform Docker image is built for the following platforms:
      * linux/amd64
      * linux/arm64
      * linux/arm
      * linux/ppc64le
      * linux/386
      This step can be run manually with the command `make publish` from the needed git tag.
4. Push the tags `v1.xx.y` and `v1.xx.y-cluster` created at step 2 to public GitHub repository at https://github.com/VictoriaMetrics/VictoriaMetrics.
   Push the tags `v1.xx.y`, `v1.xx.y-cluster`, `v1.xx.y-enterprise` and `v1.xx.y-enterprise-cluster` to the corresponding
   branches in private repository.
   **Important note:** do not push enterprise tags to public GitHub repository - they must be pushed only to private repository.
5. Run `TAG=v1.xx.y make github-create-release github-upload-assets`. This command performs the following tasks:
   a) Create draft GitHub release with the name `TAG`. This step can be run manually
      with the command `TAG=v1.xx.y make github-create-release`.
      The release id is stored at `/tmp/vm-github-release` file.
   b) Upload all the binaries and checksums created at step `3a` to that release.
      This step can be run manually with the command `make github-upload-assets`.
      It is expected that the needed release id is stored at `/tmp/vm-github-release` file,
      which must be created at the step `a`.
      If the upload process is interrupted by any reason, then the following recovery steps must be performed:
      - To delete the created draft release by running the command `make github-delete-release`.
        This command expects that the id of the release to delete is located at `/tmp/vm-github-release`
        file created at the step `a`.
      - To run the command `TAG=v1.xx.y make github-create-release github-upload-assets`, so new release is created
        and all the needed assets are re-uploaded to it.
6. Go to <https://github.com/VictoriaMetrics/VictoriaMetrics/releases> and verify that draft release with the name `TAG` has been created
   and this release contains all the needed binaries and checksums.
7. Update the release description with the content of [CHANGELOG](https://github.com/VictoriaMetrics/VictoriaMetrics/blob/master/docs/CHANGELOG.md) for this release.
8. Publish release by pressing "Publish release" green button in GitHub's UI.
9. Bump version of the VictoriaMetrics cluster in the [sandbox environment](https://github.com/VictoriaMetrics/ops/blob/main/gcp-test/sandbox/manifests/benchmark-vm/vmcluster.yaml)
   by [opening and merging PR](https://github.com/VictoriaMetrics/ops/pull/58).
10. Bump VictoriaMetrics version at `deployment/docker/docker-compose.yml` and at `deployment/docker/docker-compose-cluster.yml`.

## Building snap package

 pre-requirements:

* snapcraft binary, can be installed with commands:
   for MacOS `brew install snapcraft` and [install multipass](https://discourse.ubuntu.com/t/installing-multipass-on-macos/8329),
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
* Post in LinkedIn at <https://www.linkedin.com/company/victoriametrics/>
* Publish message in Telegram at <https://t.me/VictoriaMetrics_en> and <https://t.me/VictoriaMetrics_ru1>
* Publish message in Google Groups at <https://groups.google.com/forum/#!forum/victorametrics-users>

## Helm Charts

The helm chart repository [https://github.com/VictoriaMetrics/helm-charts/](https://github.com/VictoriaMetrics/helm-charts/)

### Bump the version of images

Bump `tag` field in `values.yaml` with new release version.
Bump `appVersion` field in `Chart.yaml` with new release version.
Bump `version` field in `Chart.yaml` with incremental semver version.
Do these updates to the following charts:

1. Update `vmagent` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-agent/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-agent/Chart.yaml) 
2. Update `vmalert` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-alert/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-alert/Chart.yaml)
3. Update `vmauth` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-auth/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-auth/Chart.yaml)
4. Update `cluster` chart versions in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/values.yaml), bump version for `vmselect`, `vminsert` and `vmstorage` and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-cluster/Chart.yaml)
5. Update `k8s-stack` chart versions in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/values.yaml), bump version for `vmselect`, `vminsert`, `vmstorage`, `vmsingle`, `vmalert`, `vmagent` and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-k8s-stack/Chart.yaml)
6. Update `single-node` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-single/values.yaml) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-single/Chart.yaml)
7. Update `vmgateway` chart version in [`values.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-gateway/values.yamll) and [`Chart.yaml`](https://github.com/VictoriaMetrics/helm-charts/blob/master/charts/victoria-metrics-gateway/Chart.yaml)

Once updated, run the following commands:

1. Run `make gen-docs`
2. Run `make package` that creates or updates zip file with the packed chart
3. Run `make merge`. It creates or updates metadata for charts in index.yaml
4. Push changes to master. `master` is a source of truth
5. Push the same changes to `gh-pages` branch

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

## RPM packages

### Bump the version of components (for LTS releases only)

Repository [https://github.com/VictoriaMetrics/victoriametrics-lts-rpm](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm)

1. Update `vmagent` version in [`vmagent.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmagent.spec#L2)
2. Update `vmalert` version in [`vmalert.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmalert.spec#L2)
3. Update `vmauth` version in [`vmauth.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmauth.spec#L2)
4. Update `vmbackup` version in [`vmbackup.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmbackup.spec#L2)
5. Update `vmctl` version in [`vmctl.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmctl.spec#L2)
6. Update `vmrestore` version in [`vmrestore.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmrestore.spec#L2)
7. Update `vminsert` version in [`vminsert.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vminsert.spec#L2)
8. Update `vmselect` version in [`vmselect.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmselect.spec#L2)
9. Update `vmstorage` version in [`vmstorage.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmstorage.spec#L2)
10. Update `vmsingle` version in [`vmsingle.spec`](https://github.com/VictoriaMetrics/victoriametrics-lts-rpm/blob/master/vmsingle.spec#L2)
11. Commit and push changes to the repository. This will automatically build and publish new versions of RPM packages.
