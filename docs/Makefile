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
<!-- The file should not be updated manually. Run make docs-update-flags while preparing a new release to sync flags in docs from actual binaries. -->
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

docs-update-vmsingle-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG) && $(MAKE) victoria-metrics && \
	./bin/victoria-metrics -help > /tmp/victoria_metrics_common_flags_tmp.md

	git checkout $(TAG)-enterprise && $(MAKE) victoria-metrics && \
	./bin/victoria-metrics -help > /tmp/victoria_metrics_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/victoria_metrics_common_flags.md && \
	cat /tmp/victoria_metrics_common_flags_tmp.md >> docs/victoriametrics/victoria_metrics_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/victoria_metrics_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/victoria_metrics_enterprise_flags.md && \
	diff /tmp/victoria_metrics_enterprise_flags_tmp.md /tmp/victoria_metrics_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/victoria_metrics_enterprise_flags.md && \
	printf '```\n' >> docs/victoriametrics/victoria_metrics_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/victoria_metrics_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/victoria_metrics_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/victoria_metrics_common_flags.md
	sed -i '/The maximum number of concurrent search requests\./ s/(default [0-9]\+)/(default vmselect.getDefaultMaxConcurrentRequests())/' docs/victoriametrics/victoria_metrics_common_flags.md
	sed -i '/The maximum number of CPU cores a single query can use\./ s/(default [0-9]\+)/(default netstorage.defaultMaxWorkersPerQuery())/' docs/victoriametrics/victoria_metrics_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/victoria_metrics_common_flags.md

docs-update-vmauth-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG) && $(MAKE) vmauth && \
	./bin/vmauth -help > /tmp/vmauth_common_flags_tmp.md
	git checkout $(TAG)-enterprise && $(MAKE) vmauth && \
	./bin/vmauth -help > /tmp/vmauth_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmauth_common_flags.md
	cat /tmp/vmauth_common_flags_tmp.md >> docs/victoriametrics/vmauth_common_flags.md
	printf '```\n' >> docs/victoriametrics/vmauth_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmauth_enterprise_flags.md
	diff /tmp/vmauth_enterprise_flags_tmp.md /tmp/vmauth_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmauth_enterprise_flags.md
	printf '```' >> docs/victoriametrics/vmauth_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmauth_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmauth_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmauth_common_flags.md

docs-update-vmagent-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG) && $(MAKE) vmagent && \
	./bin/vmagent -help > /tmp/vmagent_common_flags_tmp.md

	git checkout $(TAG)-enterprise && $(MAKE) vmagent && \
	./bin/vmagent -help > /tmp/vmagent_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmagent_common_flags.md && \
	cat /tmp/vmagent_common_flags_tmp.md >> docs/victoriametrics/vmagent_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmagent_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmagent_enterprise_flags.md && \
	diff /tmp/vmagent_enterprise_flags_tmp.md /tmp/vmagent_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmagent_enterprise_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmagent_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmagent_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmagent_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_common_flags.md
	sed -i '/The number of concurrent queues to each -remoteWrite.url./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmagent_common_flags.md

docs-update-vmalert-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG) && $(MAKE) vmalert && \
	./bin/vmalert -help > /tmp/vmalert_common_flags_tmp.md

	git checkout $(TAG)-enterprise && $(MAKE) vmalert && \
	./bin/vmalert -help > /tmp/vmalert_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmalert_common_flags.md && \
	cat /tmp/vmalert_common_flags_tmp.md >> docs/victoriametrics/vmalert_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmalert_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmalert_enterprise_flags.md && \
	diff /tmp/vmalert_enterprise_flags_tmp.md /tmp/vmalert_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmalert_enterprise_flags.md && \
	printf '```' >> docs/victoriametrics/vmalert_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmalert_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmalert_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/Defines number of writers for concurrent writing into remote write endpoint./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmalert_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmalert_common_flags.md

docs-update-vmselect-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG)-cluster && $(MAKE) vmselect && \
	./bin/vmselect -help > /tmp/vmselect_common_flags_tmp.md

	git checkout $(TAG)-enterprise-cluster && $(MAKE) vmselect && \
	./bin/vmselect -help > /tmp/vmselect_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmselect_common_flags.md && \
	cat /tmp/vmselect_common_flags_tmp.md >> docs/victoriametrics/vmselect_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmselect_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmselect_enterprise_flags.md && \
	diff /tmp/vmselect_enterprise_flags_tmp.md /tmp/vmselect_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmselect_enterprise_flags.md && \
	printf '```' >> docs/victoriametrics/vmselect_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmselect_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmselect_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent search requests\./ s/(default [0-9]\+)/(default vmselect.getDefaultMaxConcurrentRequests())/' docs/victoriametrics/vmselect_common_flags.md
	sed -i '/The maximum number of CPU cores a single query can use\./ s/(default [0-9]\+)/(default netstorage.defaultMaxWorkersPerQuery())/' docs/victoriametrics/vmselect_common_flags.md
	sed -i '/The maximum number of concurrent vmselect requests the server can process at -clusternativeListenAddr/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmselect_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmselect_common_flags.md

docs-update-vminsert-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG)-cluster && $(MAKE) vminsert && \
	./bin/vminsert -help > /tmp/vminsert_common_flags_tmp.md

	git checkout $(TAG)-enterprise-cluster && $(MAKE) vminsert && \
	./bin/vminsert -help > /tmp/vminsert_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vminsert_common_flags.md && \
	cat /tmp/vminsert_common_flags_tmp.md >> docs/victoriametrics/vminsert_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/vminsert_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vminsert_enterprise_flags.md && \
	diff /tmp/vminsert_enterprise_flags_tmp.md /tmp/vminsert_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vminsert_enterprise_flags.md && \
	printf '```' >> docs/victoriametrics/vminsert_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vminsert_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vminsert_enterprise_flags.md

	# uncomment and adjust if you need to remove some flags from the documentation.
	# should be used as a temporary workaround only.
	#awk -i inplace '\
	#	  /^  -promscrape./ {skip=1; next}\
	#	  skip && /^    / {next}\
	#	  skip {skip=0}\
	#	  {print}\
	#	' docs/victoriametrics/vminsert_common_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vminsert_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vminsert_common_flags.md

docs-update-vmstorage-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG)-cluster && $(MAKE) vmstorage && \
	./bin/vmstorage -help > /tmp/vmstorage_common_flags_tmp.md

	git checkout $(TAG)-enterprise-cluster && $(MAKE) vmstorage && \
	./bin/vmstorage -help > /tmp/vmstorage_enterprise_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmstorage_common_flags.md && \
	cat /tmp/vmstorage_common_flags_tmp.md >> docs/victoriametrics/vmstorage_common_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmstorage_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmstorage_enterprise_flags.md && \
	diff /tmp/vmstorage_enterprise_flags_tmp.md /tmp/vmstorage_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmstorage_enterprise_flags.md && \
	printf '```' >> docs/victoriametrics/vmstorage_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmstorage_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmstorage_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_common_flags.md
	sed -i '/The maximum number of concurrent vmselect requests the vmstorage can process at./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmstorage_common_flags.md

docs-update-vmctl-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	git checkout $(TAG) && $(MAKE) vmctl && \
	./bin/vmctl -help > /tmp/vmctl_flags_tmp.md && \
	./bin/vmctl opentsdb -help > /tmp/vmctl_opentsdb_flags_tmp.md && \
	./bin/vmctl influx -help > /tmp/vmctl_influx_flags_tmp.md && \
	./bin/vmctl remote-read -help > /tmp/vmctl_remote-read_flags_tmp.md && \
	./bin/vmctl prometheus -help > /tmp/vmctl_prometheus_flags_tmp.md && \
	./bin/vmctl vm-native -help > /tmp/vmctl_vm-native_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_flags.md && \
	cat /tmp/vmctl_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md && \
	cat /tmp/vmctl_opentsdb_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_influx_flags.md && \
	cat /tmp/vmctl_influx_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_influx_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_influx_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_remote-read_flags.md && \
	cat /tmp/vmctl_remote-read_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_remote-read_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_remote-read_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_prometheus_flags.md && \
	cat /tmp/vmctl_prometheus_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_prometheus_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_prometheus_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_vm-native_flags.md && \
	cat /tmp/vmctl_vm-native_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_vm-native_flags.md && \
	printf '```\n' >> docs/victoriametrics/vmctl/vmctl_vm-native_flags.md

	# remove Total time line from all vmctl flag files to reduce diffs noise
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_influx_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_remote-read_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_prometheus_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_vm-native_flags.md

	# remove Version line and the actual version line from vmctl_flags.md to reduce diffs noise
	sed -i '/^VERSION:/,+1d' docs/victoriametrics/vmctl/vmctl_flags.md

# docs-update-flags updates flags in the documentation
# using the actual binaries compiled from the provided $TAG.
# The command also normalizes the output a bit.
#
# There is no need to update flags manually while working on change in the code.
# The flags will be synced when a new release tag is cut.
#
# The command can be run from any branch.
# The script checks out the required $TAG, builds the binaries, and updates the documentation.
docs-update-flags:
ifndef TAG
	$(error TAG must be provided to update flags in docs)
endif
	# Note for MacOS users:
	# You need to install gnu versions of sed and awk to enable in-place editing
	# Install using: brew install gnu-sed gawk
	# Add tools to PATH see how in `brew info gnu-sed` and `brew info gawk

	orig_branch=$$(git rev-parse --abbrev-ref HEAD); \
	$(MAKE) docs-update-vmctl-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmsingle-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmalert-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmauth-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmagent-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmselect-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vminsert-flags && git checkout "$$orig_branch" && \
	$(MAKE) docs-update-vmstorage-flags && git checkout "$$orig_branch"