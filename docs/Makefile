UID := "$(shell id -u)"
PLATFORM := $(shell uname -m)
DOCKER_PLATFORM := "linux/$(if $(findstring $(PLATFORM),arm64),arm64,amd64)"

define FLAGS_HEADER
---
build:
  list: never
  publishResources: false
  render: never
sitemap:
  disable: true
---
<!-- The file has to be manually updated during feature work in PR, make docs-update-flags command could be used peridically to ensure the flags in sync. -->
```shellhelp
endef
export FLAGS_HEADER

# These commands must be run from the VictoriaMetrics repository root

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
		--build-arg UID=$(UID) \
		--platform $(DOCKER_PLATFORM) \
		vmdocs

docs-debug: docs docs-image
	docker run \
		--rm \
		--name vmdocs-docker-container \
		--platform $(DOCKER_PLATFORM) \
		-p 1313:1313 \
		$(foreach dir,$(wildcard ./docs/$(dir)/*), -v ./docs/$(notdir $(dir)):/opt/docs/content/$(notdir $(dir))) \
			vmdocs-docker-package

docs-update-version: docs-image
	$(if $(filter v%,$(PKG_TAG)), \
		docker run \
			--rm \
			--entrypoint /usr/bin/find \
			--platform $(DOCKER_PLATFORM) \
			--name vmdocs-docker-container \
			-v ./docs:/opt/docs/content/victoriametrics vmdocs-docker-package \
				content \
					-regex ".*\.md" \
					-exec sed -i 's/{{% available_from "#" %}}/{{% available_from "$(PKG_TAG)" %}}/g' {} \;, \
		$(info "Skipping docs version update, invalid $$PKG_TAG: $(PKG_TAG)"))

# Converts images at docs folder to webp format
# See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#images-in-documentation
docs-images-to-webp: docs-image
	docker run \
		--rm \
		--platform $(DOCKER_PLATFORM) \
		--entrypoint /usr/bin/find \
		--name vmdocs-docker-container \
		-v ./docs:/opt/docs/content/victoriametrics vmdocs-docker-package \
			content \
				-regex ".*\.\(png\|jpg\|jpeg\)" \
				-exec sh -c 'cwebp -preset drawing -m 6 -o $$(echo {} | cut -f-1 -d.).webp {} && rm -rf {}' {} \;

# docs-update-flags updates flags in the documentation using the actual binaries compiled
# from the latest enterprise-single-node and enterprise-cluster branches (hardcoded for now).
# The command also normalizes the output a bit.
#
# The command does not replace the need to manually update flags in the documentation when
# new flags are added or existing flags are updated. It just helps to keep the documentation
# in sync with the actual binaries.
#
# It can be run from any branch.
# All work happens inside temporary directories under /tmp.
# The script checks out the required branch, builds the binaries, and updates the documentation.
# The current Git repository is not touched.
docs-update-flags:
	# Note for MacOS users:
	# You need to install gnu versions of sed and awk inorder fo inplace editing to work
	# Install using: brew install gnu-sed gawk
	# Add tools to PATH see how in `brew info gnu-sed` and `brew info gawk

	git fetch enterprise

	rm -rf /tmp/vm-enterprise-cluster
	git worktree remove /tmp/vm-enterprise-cluster || true
	git worktree add /tmp/vm-enterprise-cluster enterprise/enterprise-cluster

	rm -rf /tmp/vm-enterprise-single-node
	git worktree remove /tmp/vm-enterprise-single-node || true
	git worktree add /tmp/vm-enterprise-single-node enterprise/enterprise-single-node

	# ---- victoria-metrics
	echo "$$FLAGS_HEADER" > docs/victoriametrics/victoria_metrics_flags.md
	(cd /tmp/vm-enterprise-single-node && make victoria-metrics)
	(cd /tmp/vm-enterprise-single-node && ./bin/victoria-metrics -help 2>&1) >> docs/victoriametrics/victoria_metrics_flags.md
	printf -- '```' >> docs/victoriametrics/victoria_metrics_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/victoria_metrics_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/victoria_metrics_flags.md
	sed -i '/The maximum number of concurrent search requests\./ s/(default [0-9]\+)/(default vmselect.getDefaultMaxConcurrentRequests())/' docs/victoriametrics/victoria_metrics_flags.md
	sed -i '/The maximum number of CPU cores a single query can use\./ s/(default [0-9]\+)/(default netstorage.defaultMaxWorkersPerQuery())/' docs/victoriametrics/victoria_metrics_flags.md

	# ---- vmagent
	(cd /tmp/vm-enterprise-single-node && make vmagent)
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmagent_flags.md
	(cd /tmp/vm-enterprise-single-node && ./bin/vmagent -help 2>&1) >> docs/victoriametrics/vmagent_flags.md
	echo '```' >> docs/victoriametrics/vmagent_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmagent_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_flags.md
	sed -i '/The number of concurrent queues to each -remoteWrite.url./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_flags.md

	# ---- vmalert
	(cd /tmp/vm-enterprise-single-node && make vmalert)
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmalert_flags.md
	(cd /tmp/vm-enterprise-single-node && ./bin/vmalert -help 2>&1) >> docs/victoriametrics/vmalert_flags.md
	echo '```' >> docs/victoriametrics/vmalert_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmalert_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/Defines number of writers for concurrent writing into remote write endpoint./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmalert_flags.md

	# ---- vminsert
	(cd /tmp/vm-enterprise-cluster && make vminsert)
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vminsert_flags.md
	(cd /tmp/vm-enterprise-cluster && ./bin/vminsert -help 2>&1) >> docs/victoriametrics/vminsert_flags.md
	echo '```' >> docs/victoriametrics/vminsert_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vminsert_flags.md

	# uncomment and adjust if you need to remove some flags from the documentation.
	# should be used as a temporary workaround only.
	#awk -i inplace '\
	#	  /^  -promscrape./ {skip=1; next}\
	#	  skip && /^    / {next}\
	#	  skip {skip=0}\
	#	  {print}\
	#	' docs/victoriametrics/vminsert_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vminsert_flags.md

	# ---- vmselect
	(cd /tmp/vm-enterprise-cluster && make vmselect)
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmselect_flags.md
	(cd /tmp/vm-enterprise-cluster && ./bin/vmselect -help 2>&1) >> docs/victoriametrics/vmselect_flags.md
	echo '```' >> docs/victoriametrics/vmselect_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmselect_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent search requests\./ s/(default [0-9]\+)/(default vmselect.getDefaultMaxConcurrentRequests())/' docs/victoriametrics/vmselect_flags.md
	sed -i '/The maximum number of CPU cores a single query can use\./ s/(default [0-9]\+)/(default netstorage.defaultMaxWorkersPerQuery())/' docs/victoriametrics/vmselect_flags.md
	sed -i '/The maximum number of concurrent vmselect requests the server can process at -clusternativeListenAddr/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmselect_flags.md

	# ---- vmstorage
	(cd /tmp/vm-enterprise-cluster && make vmstorage)
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmstorage_flags.md
	(cd /tmp/vm-enterprise-cluster && ./bin/vmstorage -help 2>&1) >> docs/victoriametrics/vmstorage_flags.md
	echo '```' >> docs/victoriametrics/vmstorage_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmstorage_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_flags.md
	sed -i '/The maximum number of concurrent vmselect requests the vmstorage can process at./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_flags.md