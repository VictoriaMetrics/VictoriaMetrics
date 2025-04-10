# These commands must be run from the VictoriaMetrics repository root

# Converts images at docs folder to webp format
# See https://docs.victoriametrics.com/single-server-victoriametrics/#images-in-documentation
docs-image:
	if [ ! -d vmdocs ]; then \
		git clone --depth 1 git@github.com:VictoriaMetrics/vmdocs vmdocs; \
	fi; \
	cd vmdocs && \
	git checkout main && \
	git pull origin main && \
	cd .. && \
	docker build \
		-t vmdocs-docker-package \
		vmdocs

docs-debug: docs docs-image
	docker run \
		--rm \
		--name vmdocs-docker-container \
		-p 1313:1313 \
		$(foreach dir,$(wildcard ./docs/$(dir)/*), -v ./docs/$(notdir $(dir)):/opt/docs/content/$(notdir $(dir))) \
			vmdocs-docker-package

docs-update-version: docs-image
	$(if $(filter v%,$(PKG_TAG)), \
		docker run \
			--rm \
			--entrypoint /usr/bin/find \
			--name vmdocs-docker-container \
			-v ./docs:/opt/docs/content/victoriametrics vmdocs-docker-package \
				content \
					-regex ".*\.md" \
					-exec sed -i 's/{{% available_from "#" %}}/{{% available_from "$(PKG_TAG)" %}}/g' {} \;, \
		$(info "Skipping docs version update, invalid $$PKG_TAG: $(PKG_TAG)"))

docs-images-to-webp: docs-image
	docker run \
		--rm \
		--entrypoint /usr/bin/find \
		--name vmdocs-docker-container \
		-v ./docs:/opt/docs/content/victoriametrics vmdocs-docker-package \
			content \
				-regex ".*\.\(png\|jpg\|jpeg\)" \
				-exec sh -c 'cwebp -preset drawing -m 6 -o $$(echo {} | cut -f-1 -d.).webp {} && rm -rf {}' {} \;
