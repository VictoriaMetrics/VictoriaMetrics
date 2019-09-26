DOCKER_NAMESPACE := victoriametrics
BUILDER_IMAGE := local/builder:go1.13.1
CERTS_IMAGE := local/certs:1.0.2

package-certs:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(CERTS_IMAGE)') \
		|| docker build -t $(CERTS_IMAGE) deployment/docker/certs

package-builder:
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(BUILDER_IMAGE)') \
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
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)') || (\
		$(MAKE) app-via-docker && \
		docker build -t $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) -f app/$(APP_NAME)/deployment/Dockerfile .)

publish-via-docker: package-via-docker
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE)
	docker tag $(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) $(DOCKER_NAMESPACE)/$(APP_NAME):latest
	docker push $(DOCKER_NAMESPACE)/$(APP_NAME):latest

run-via-docker: package-via-docker
	docker run -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_OPTS) \
		$(DOCKER_NAMESPACE)/$(APP_NAME):$(PKG_TAG)$(RACE) $(ARGS)

remove-docker-images:
	docker image ls --format '{{.Repository}}\t{{.ID}}' | grep $(DOCKER_NAMESPACE)/ | grep -v /builder | awk '{print $$2}' | xargs docker image rm -f
