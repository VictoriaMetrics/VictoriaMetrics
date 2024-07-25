ROOTDIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
REPODIR := $(ROOTDIR)/..
WORKDIR := $(REPODIR)/..

# These commands must be run from the VictoriaMetrics repository root

# Converts images at docs folder to webp format
# See https://docs.victoriametrics.com/single-server-victoriametrics/#images-in-documentation
docs-images-to-webp:
	IMAGES_EXTENSION=jpg $(MAKE) docs-images-to-webp-by-extension
	IMAGES_EXTENSION=jpeg $(MAKE) docs-images-to-webp-by-extension
	IMAGES_EXTENSION=png $(MAKE) docs-images-to-webp-by-extension

docs-images-to-webp-by-extension:
	docker run --rm -it \
		-v $(shell pwd)/docs:/docs \
		elswork/cwebp \
		sh -c 'find /docs/ -type f ! -path "/docs/operator/*" ! -path "/docs/_site/*" -name "*.$(IMAGES_EXTENSION)" -print0 | \
			xargs -0 -P $(MAKE_CONCURRENCY) -I {} sh -c '"'"'cwebp -preset drawing -m 6 -o "$${1%.*}.webp" $$1'"'"' _ {}'
	find docs/ -type f ! -path 'docs/operator/*' ! -path 'docs/_site/*' -name '*.$(IMAGES_EXTENSION)' -print0 | xargs -0 rm -f

docs-debug:
	if [ ! -d $(WORKDIR)/vmdocs ]; then \
		git clone git@github.com:VictoriaMetrics/vmdocs $(WORKDIR)/vmdocs; \
	fi; \
	cd $(WORKDIR)/vmdocs && \
	git checkout main && \
	git pull origin main && \
	cd $(REPODIR) && \
	docker build \
		-t vmdocs \
		$(WORKDIR)/vmdocs && \
	docker rm -f vmdocs || true && \
	docker run \
		-d \
		--name vmdocs \
		-p 1313:1313 \
		-v ./docs:/opt/docs/content vmdocs
