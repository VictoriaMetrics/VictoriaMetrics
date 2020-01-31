# All these commands must run from repository root.

DOCKER_NAMESPACE := docker.io/victoriametrics
BUILDER_IMAGE := local/builder:go1.13.7
CERTS_IMAGE := local/certs:1.0.3

package-certs:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(CERTS_IMAGE)$$') \
		|| docker build -t $(CERTS_IMAGE) deployment/docker/certs

package-builder:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(BUILDER_IMAGE)$$') \
		|| docker build -t $(BUILDER_IMAGE) deployment/docker/builder

app-via-docker: package-certs package-builder
	mkdir -p gocache-for-docker
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)",dst=/VictoriaMetrics \
		-w /VictoriaMetrics \
		--mount type=bind,src="$(shell pwd)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		--env GO111MODULE=on \
		$(DOCKER_OPTS) \
		$(BUILDER_IMAGE) \
		go build $(RACE) -mod=vendor -trimpath -ldflags "-s -w -extldflags '-static' $(GO_BUILDINFO)" -tags 'netgo osusergo' \
			-o bin/$(APP_NAME)$(APP_SUFFIX)-prod $(PKG_PREFIX)/app/$(APP_NAME)

package-via-docker:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE)$$') || (\
		$(MAKE) app-via-docker && \
		docker build \
			--build-arg src_binary=$(APP_NAME)$(APP_SUFFIX)-prod \
			--build-arg certs_image=$(CERTS_IMAGE) \
			-t $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) \
			-f app/$(APP_NAME)/deployment/Dockerfile bin)

package-manifest: \
		package-via-docker-amd64 \
		package-via-docker-arm \
		package-via-docker-arm64 \
		package-via-docker-ppc64le \
		package-via-docker-386
	$(MAKE) package-manifest-internal

package-manifest-internal:
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-amd64$(RACE)
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm$(RACE)
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm64$(RACE)
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-ppc64le$(RACE)
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-386$(RACE)
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest create --amend $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-amd64$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm64$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-ppc64le$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-386$(RACE)
	GOARCH=amd64 $(MAKE) package-manifest-annotate-goarch
	GOARCH=arm $(MAKE) package-manifest-annotate-goarch
	GOARCH=arm64 $(MAKE) package-manifest-annotate-goarch
	GOARCH=ppc64le $(MAKE) package-manifest-annotate-goarch
	GOARCH=386 $(MAKE) package-manifest-annotate-goarch

package-manifest-annotate-goarch:
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest annotate $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) \
				$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-$(GOARCH)$(RACE) --os linux --arch $(GOARCH)

publish-via-docker: package-manifest
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-amd64$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest-amd64$(RACE)
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest-arm$(RACE)
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-arm64$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest-arm64$(RACE)
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-ppc64le$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest-ppc64le$(RACE)
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)-386$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest-386$(RACE)
	PKG_TAG=latest $(MAKE) package-manifest-internal
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)
	DOCKER_CLI_EXPERIMENTAL=enabled docker manifest push --purge $(DOCKER_NAMESPACE)/$(APP_NAME):latest$(RACE)

run-via-docker: package-via-docker
	docker run -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_OPTS) \
		$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(APP_SUFFIX)$(RACE) $(ARGS)

app-via-docker-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=linux --env GOARCH=$(GOARCH)' \
	$(MAKE) app-via-docker

app-via-docker-goarch-cgo:
	CGO_ENABLED=1 $(MAKE) app-via-docker-goarch

app-via-docker-goarch-nocgo:
	CGO_ENABLED=0 $(MAKE) app-via-docker-goarch

app-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

app-via-docker-amd64:
	GOARCH=amd64 $(MAKE) app-via-docker-goarch-cgo

app-via-docker-arm:
	GOARCH=arm $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-arm64:
	GOARCH=arm64 $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-ppc64le:
	GOARCH=ppc64le $(MAKE) app-via-docker-goarch-nocgo

app-via-docker-386:
	GOARCH=386 $(MAKE) app-via-docker-goarch-nocgo

package-via-docker-goarch:
	APP_SUFFIX='-$(GOARCH)' \
	DOCKER_OPTS='--env CGO_ENABLED=$(CGO_ENABLED) --env GOOS=linux --env GOARCH=$(GOARCH)' \
	$(MAKE) package-via-docker

package-via-docker-goarch-cgo:
	CGO_ENABLED=1 $(MAKE) package-via-docker-goarch

package-via-docker-goarch-nocgo:
	CGO_ENABLED=0 $(MAKE) package-via-docker-goarch

package-via-docker-pure:
	APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) package-via-docker

package-via-docker-amd64:
	GOARCH=amd64 $(MAKE) package-via-docker-goarch-cgo

package-via-docker-arm:
	GOARCH=arm $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-arm64:
	GOARCH=arm64 $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-ppc64le:
	GOARCH=ppc64le $(MAKE) package-via-docker-goarch-nocgo

package-via-docker-386:
	GOARCH=386 $(MAKE) package-via-docker-goarch-nocgo

remove-docker-images:
	docker image ls --format '{{.Repository}}\t{{.ID}}' | grep $(DOCKER_NAMESPACE)/ | grep -v /builder | awk '{print $$2}' | xargs docker image rm -f
