# All these commands must run from repository root.

run-vmselect:
	mkdir -p vmselect-cache
	DOCKER_OPTS='-v $(shell pwd)/vmselect-cache:/cache' \
	APP_NAME=vmselect \
	ARGS='-storageNode=localhost:8401 -selectNode=localhost:8481 -cacheDataPath=/cache' \
	$(MAKE) run-via-docker

vmselect:
	APP_NAME=vmselect $(MAKE) app-local

vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) app-local

vmselect-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker

vmselect-pure-prod:
	APP_NAME=vmselect $(MAKE) app-via-docker-pure

vmselect-prod-race:
	APP_NAME=vmselect RACE=-race $(MAKE) app-via-docker

vmselect-pure:
	APP_NAME=vmselect $(MAKE) app-local-pure

package-vmselect:
	APP_NAME=vmselect $(MAKE) package-via-docker

package-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) package-via-docker

publish-vmselect:
	APP_NAME=vmselect $(MAKE) publish-via-docker

publish-vmselect-race:
	APP_NAME=vmselect RACE=-race $(MAKE) publish-via-docker
