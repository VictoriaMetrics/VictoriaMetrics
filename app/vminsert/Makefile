# All these commands must run from repository root.

run-vminsert:
	APP_NAME=vminsert \
	ARGS='-storageNode=localhost:8400' \
	$(MAKE) run-via-docker

vminsert:
	APP_NAME=vminsert $(MAKE) app-local

vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) app-local

vminsert-prod:
	APP_NAME=vminsert $(MAKE) app-via-docker

vminsert-prod-race:
	APP_NAME=vminsert RACE=-race $(MAKE) app-via-docker

package-vminsert:
	APP_NAME=vminsert $(MAKE) package-via-docker

package-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) package-via-docker

publish-vminsert:
	APP_NAME=vminsert $(MAKE) publish-via-docker

publish-vminsert-race:
	APP_NAME=vminsert RACE=-race $(MAKE) publish-via-docker
