---
sort: 15
---

# CONTRIBUTING

## Required programs

for developing you need: 
- golang 1.15+
- operator-sdk v1.0.0
- docker
- minikube or kind for e2e tests
- golangci-lint


## installing local env

- make install-develop-tools
- kind create cluster

## local build and run

Use `make build` - it will generate new crds and build binary


for running locally you need minikube and run two commands:
```bash
make install
make run
```
or you can run it from IDE with ```main.go```

## publish changes

before creating merge request, ensure that tests passed locally:
```bash
make build # it will update crds
make lint # linting project
make test #unit tests
make e2e-local #e2e tests with minikube
```

## adding new api

For adding new kind - KIND_NAME, you have to execute command:

```bash
operator-sdk create api --group operator --version v1beta1 --kind KIND_NAME
```

This will scaffold api and controller. Then you have to edit code at `api` and `controllers` folder.

## create olm package

Choose version (release tag at github) and generate or update corresponding csv file
```bash
TAG=v0.2.1 make packagemanifest
TAG=v0.2.1 make bundle
```

it will generate files at directories: `packagemanifest/0.2.1/` and `bundle/`


commit changes

publish olm package to quay.io with (you have to define AUTH_TOKEN for quay firsh)

```bash
export AUTH_TOKEN="basic ..."
TAG=v0.2.1 make packagemanifest-publish
TAG=v0.2.1 make bundle-publish
```

### integration with operator-hub

 Clone repo locally: git clone https://github.com/operator-framework/community-operators.git
 
 copy content to operator-hub repo and run tests
 you can specify version (OP_VER) and channel OP_CHANNEL
 ```bash
cp -R packagemanifests/* $PATH_TO_OPERATOR_REPO/upstream-community-operators/victoriametrics/
cd $PATH_TO_OPERATOR_REPO
#run tests
make operator.verify OP_PATH=upstream-community-operators/victoria-metrics-operator VERBOSE=1
make operator.test OP_PATH=upstream-community-operators/victoria-metrics-operator/ VERBOSE=1

```

 Now you can submit merge request with changes to operator-hub repo


troubleshooting: [url](https://github.com/operator-framework/community-operators/blob/master/docs/using-scripts.md#troubleshooting)
