# All these commands must run from repository root.

copy-metricsql-docs:
	cp docs/MetricsQL.md app/vmui/packages/vmui/src/assets/MetricsQL.md

vmui-package-base-image:
	docker build -t vmui-builder-image -f app/vmui/Dockerfile-build ./app/vmui

vmui-build: copy-metricsql-docs vmui-package-base-image
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)/app/vmui",dst=/build \
		-w /build/packages/vmui \
		--entrypoint=/bin/bash \
		vmui-builder-image -c "npm install && npm run build"

vmui-logs-build: vmui-package-base-image
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)/app/vmui",dst=/build \
		-w /build/packages/vmui \
		--entrypoint=/bin/bash \
		vmui-builder-image -c "npm install && npm run build:logs"

vmui-anomaly-build: vmui-package-base-image
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)/app/vmui",dst=/build \
		-w /build/packages/vmui \
		--entrypoint=/bin/bash \
		vmui-builder-image -c "npm install && npm run build:anomaly"

vmui-release: vmui-build
	docker build -t ${DOCKER_NAMESPACE}/vmui:latest -f app/vmui/Dockerfile-web ./app/vmui/packages/vmui
	docker tag ${DOCKER_NAMESPACE}/vmui:latest ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-publish-latest: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui

vmui-publish-release: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-update: vmui-build
	rm -rf app/vmselect/vmui/* && mv app/vmui/packages/vmui/build/* app/vmselect/vmui

vmui-logs-update: vmui-logs-build
	rm -rf app/vlselect/vmui/* && mv app/vmui/packages/vmui/build/* app/vlselect/vmui && rm -rf app/vlselect/vmui/dashboards
