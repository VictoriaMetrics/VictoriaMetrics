# All these commands must run from repository root.
copy-metricsql-docs:
	cp docs/victoriametrics/MetricsQL.md app/vmui/packages/vmui/src/assets/MetricsQL.md

vmui-run-npm-command: vmui-package-base-image
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)/app/vmui",dst=/build \
		-w /build/packages/vmui \
		--entrypoint=/bin/bash \
		vmui-builder-image -c "[ \"$$VMUI_SKIP_INSTALL\" = \"true\" ] || npm ci; $(NPM_COMMAND)"

vmui-install:
	NPM_COMMAND="true" $(MAKE) vmui-run-npm-command

vmui-package-base-image:
	docker build -t vmui-builder-image -f app/vmui/Dockerfile-build ./app/vmui

vmui-build: copy-metricsql-docs
	NPM_COMMAND="npm run build" $(MAKE) vmui-run-npm-command

vmui-release: vmui-build
	docker build -t ${DOCKER_NAMESPACE}/vmui:latest -f app/vmui/Dockerfile-web ./app/vmui/packages/vmui
	docker tag ${DOCKER_NAMESPACE}/vmui:latest ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-publish-latest: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui

vmui-publish-release: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-update: vmui-build
	rm -rf app/vmselect/vmui/* && mv app/vmui/packages/vmui/build/* app/vmselect/vmui

vmui-install-dependencies:
	cd app/vmui/packages/vmui && npm ci

vmui-lint:
	NPM_COMMAND="npm run lint" $(MAKE) vmui-run-npm-command

vmui-typecheck:
	NPM_COMMAND="npm run typecheck" $(MAKE) vmui-run-npm-command

vmui-test:
	NPM_COMMAND="npm run test" $(MAKE) vmui-run-npm-command
