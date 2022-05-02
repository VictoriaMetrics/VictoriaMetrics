# All these commands must run from repository root.

vmui-package-base-image:
	docker build -t vmui-builder-image -f app/vmui/Dockerfile-build ./app/vmui

vmui-build: vmui-package-base-image
	docker run --rm \
          --user $(shell id -u):$(shell id -g) \
         --mount type=bind,src="$(shell pwd)/app/vmui",dst=/build \
         -w /build/packages/vmui \
         --entrypoint=/bin/bash \
         vmui-builder-image -c "npm install && npm run build"

vmui-release: vmui-build
	docker build -t ${DOCKER_NAMESPACE}/vmui:latest -f app/vmui/Dockerfile-web ./app/vmui/packages/vmui
	docker tag ${DOCKER_NAMESPACE}/vmui:latest ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-publish-latest: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui

vmui-publish-release: vmui-release
	docker push ${DOCKER_NAMESPACE}/vmui:${PKG_TAG}

vmui-update: vmui-build
	rm -rf app/vmselect/vmui/* && mv app/vmui/packages/vmui/build/* app/vmselect/vmui
