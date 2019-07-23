# All these commands must run from repository root.

run-vmstorage:
	mkdir -p vmstorage-data
	DOCKER_OPTS='-v $(shell pwd)/vmstorage-data:/vmstorage-data' \
	APP_NAME=vmstorage \
	ARGS='-retentionPeriod=12' \
	$(MAKE) run-via-docker

vmstorage:
	APP_NAME=vmstorage $(MAKE) app-local

vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) app-local

vmstorage-prod:
	APP_NAME=vmstorage $(MAKE) app-via-docker

vmstorage-prod-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) app-via-docker

vmstorage-pure:
	APP_NAME=vmstorage $(MAKE) app-local-pure

vmstorage-pure-prod:
	APP_NAME=vmstorage APP_SUFFIX='-pure' DOCKER_OPTS='--env CGO_ENABLED=0' $(MAKE) app-via-docker

package-vmstorage:
	APP_NAME=vmstorage $(MAKE) package-via-docker

package-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) package-via-docker

publish-vmstorage:
	APP_NAME=vmstorage $(MAKE) publish-via-docker

publish-vmstorage-race:
	APP_NAME=vmstorage RACE=-race $(MAKE) publish-via-docker
