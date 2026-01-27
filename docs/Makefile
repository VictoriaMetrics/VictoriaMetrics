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
	(cd /tmp/vm-enterprise-single-node && make victoria-metrics)
	(cd /tmp/vm-opensource-single-node && make victoria-metrics)
	(cd /tmp/vm-enterprise-single-node && ./bin/victoria-metrics -help 2>&1) > /tmp/vm-enterprise-single-node/victoria_metrics_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/victoria-metrics -help 2>&1) > /tmp/vm-opensource-single-node/victoria_metrics_common_flags_tmp.md
	echo "$$FLAGS_HEADER" > docs/victoriametrics/victoria_metrics_common_flags.md
	cat /tmp/vm-opensource-single-node/victoria_metrics_common_flags_tmp.md >> docs/victoriametrics/victoria_metrics_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/victoria_metrics_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/victoria_metrics_enterprise_flags.md
	diff /tmp/vm-enterprise-single-node/victoria_metrics_enterprise_flags_tmp.md /tmp/vm-opensource-single-node/victoria_metrics_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/victoria_metrics_enterprise_flags.md
	printf -- '```\n' >> docs/victoriametrics/victoria_metrics_enterprise_flags.md

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
	# ---- vmauth
	(cd /tmp/vm-enterprise-single-node && make vmauth)
	(cd /tmp/vm-opensource-single-node && make vmauth)
	(cd /tmp/vm-enterprise-single-node && ./bin/vmauth -help 2>&1) > /tmp/vm-enterprise-single-node/vmauth_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmauth -help 2>&1) > /tmp/vm-opensource-single-node/vmauth_common_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmauth_common_flags.md
	cat /tmp/vm-opensource-single-node/vmauth_common_flags_tmp.md >> docs/victoriametrics/vmauth_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmauth_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmauth_enterprise_flags.md
	diff /tmp/vm-enterprise-single-node/vmauth_enterprise_flags_tmp.md /tmp/vm-opensource-single-node/vmauth_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmauth_enterprise_flags.md
	printf -- '```' >> docs/victoriametrics/vmauth_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmauth_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmauth_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmauth_common_flags.md

docs-update-vmagent-flags:
	(cd /tmp/vm-enterprise-single-node && make vmagent)
	(cd /tmp/vm-opensource-single-node && make vmagent)
	(cd /tmp/vm-enterprise-single-node && ./bin/vmagent -help 2>&1) > /tmp/vm-enterprise-single-node/vmagent_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmagent -help 2>&1) > /tmp/vm-opensource-single-node/vmagent_common_flags_tmp.md
	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmagent_common_flags.md
	cat /tmp/vm-opensource-single-node/vmagent_common_flags_tmp.md >> docs/victoriametrics/vmagent_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmagent_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmagent_enterprise_flags.md
	diff /tmp/vm-enterprise-single-node/vmagent_enterprise_flags_tmp.md /tmp/vm-opensource-single-node/vmagent_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmagent_enterprise_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmagent_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmagent_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmagent_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_common_flags.md
	sed -i '/The number of concurrent queues to each -remoteWrite.url./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmagent_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmagent_common_flags.md

docs-update-vmalert-flags:
	(cd /tmp/vm-enterprise-single-node && make vmalert)
	(cd /tmp/vm-opensource-single-node && make vmalert)
	(cd /tmp/vm-enterprise-single-node && ./bin/vmalert -help 2>&1) > /tmp/vm-enterprise-single-node/vmalert_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmalert -help 2>&1) > /tmp/vm-opensource-single-node/vmalert_common_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmalert_common_flags.md
	cat /tmp/vm-opensource-single-node/vmalert_common_flags_tmp.md >> docs/victoriametrics/vmalert_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmalert_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmalert_enterprise_flags.md
	diff /tmp/vm-enterprise-single-node/vmalert_enterprise_flags_tmp.md /tmp/vm-opensource-single-node/vmalert_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmalert_enterprise_flags.md
	printf -- '```' >> docs/victoriametrics/vmalert_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmalert_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmalert_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/Defines number of writers for concurrent writing into remote write endpoint./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmalert_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmalert_common_flags.md

docs-update-vmselect-flags:
	(cd /tmp/vm-enterprise-cluster && make vmselect)
	(cd /tmp/vm-opensource-cluster && make vmselect)
	(cd /tmp/vm-enterprise-cluster && ./bin/vmselect -help 2>&1) > /tmp/vm-enterprise-cluster/vmselect_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-cluster && ./bin/vmselect -help 2>&1) > /tmp/vm-opensource-cluster/vmselect_common_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmselect_common_flags.md
	cat /tmp/vm-opensource-cluster/vmselect_common_flags_tmp.md >> docs/victoriametrics/vmselect_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmselect_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmselect_enterprise_flags.md
	diff /tmp/vm-enterprise-cluster/vmselect_enterprise_flags_tmp.md /tmp/vm-opensource-cluster/vmselect_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmselect_enterprise_flags.md
	printf -- '```' >> docs/victoriametrics/vmselect_enterprise_flags.md

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
	(cd /tmp/vm-enterprise-cluster && make vminsert)
	(cd /tmp/vm-opensource-cluster && make vminsert)
	(cd /tmp/vm-enterprise-cluster && ./bin/vminsert -help 2>&1) > /tmp/vm-enterprise-cluster/vminsert_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-cluster && ./bin/vminsert -help 2>&1) > /tmp/vm-opensource-cluster/vminsert_common_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vminsert_common_flags.md
	cat /tmp/vm-opensource-cluster/vminsert_common_flags_tmp.md >> docs/victoriametrics/vminsert_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vminsert_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vminsert_enterprise_flags.md
	diff /tmp/vm-enterprise-cluster/vminsert_enterprise_flags_tmp.md /tmp/vm-opensource-cluster/vminsert_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vminsert_enterprise_flags.md
	printf -- '```' >> docs/victoriametrics/vminsert_enterprise_flags.md

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
	(cd /tmp/vm-enterprise-cluster && make vmstorage)
	(cd /tmp/vm-opensource-cluster && make vmstorage)
	(cd /tmp/vm-enterprise-cluster && ./bin/vmstorage -help 2>&1) > /tmp/vm-enterprise-cluster/vmstorage_enterprise_flags_tmp.md
	(cd /tmp/vm-opensource-cluster && ./bin/vmstorage -help 2>&1) > /tmp/vm-opensource-cluster/vmstorage_common_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmstorage_common_flags.md
	cat /tmp/vm-opensource-cluster/vmstorage_common_flags_tmp.md >> docs/victoriametrics/vmstorage_common_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmstorage_common_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmstorage_enterprise_flags.md
	diff /tmp/vm-enterprise-cluster/vmstorage_enterprise_flags_tmp.md /tmp/vm-opensource-cluster/vmstorage_common_flags_tmp.md |grep '^<' | sed 's/^< //' >> docs/victoriametrics/vmstorage_enterprise_flags.md
	printf -- '```' >> docs/victoriametrics/vmstorage_enterprise_flags.md

	# replace tabs in output with one space
	sed -i 's/\t/ /g' docs/victoriametrics/vmstorage_common_flags.md
	sed -i 's/\t/ /g' docs/victoriametrics/vmstorage_enterprise_flags.md

	# adjust flags with dynamic default values
	# remove after https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9680 implemented
	sed -i '/The maximum number of concurrent insert requests/ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_common_flags.md
	sed -i '/The maximum number of concurrent vmselect requests the vmstorage can process at./ s/(default [0-9]\+)/(default 2*cgroup.AvailableCPUs())/' docs/victoriametrics/vmstorage_common_flags.md
	sed -i '/The maximum number of concurrent goroutines to work with files;/ s/(default [0-9]\+)/(default fsutil.getDefaultConcurrency())/' docs/victoriametrics/vmstorage_common_flags.md

docs-update-vmctl-flags:
	(cd /tmp/vm-opensource-single-node && make vmctl)
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl opentsdb -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_opentsdb_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl influx -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_influx_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl remote-read -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_remote-read_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl prometheus -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_prometheus_flags_tmp.md
	(cd /tmp/vm-opensource-single-node && ./bin/vmctl vm-native -help 2>&1) > /tmp/vm-opensource-single-node/vmctl_vm-native_flags_tmp.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_opentsdb_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_influx_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_influx_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_influx_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_influx_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_remote-read_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_remote-read_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_remote-read_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_remote-read_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_prometheus_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_prometheus_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_prometheus_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_prometheus_flags.md

	echo "$$FLAGS_HEADER" > docs/victoriametrics/vmctl/vmctl_vm-native_flags.md
	cat /tmp/vm-opensource-single-node/vmctl_vm-native_flags_tmp.md >> docs/victoriametrics/vmctl/vmctl_vm-native_flags.md
	printf -- '```\n' >> docs/victoriametrics/vmctl/vmctl_vm-native_flags.md

	# remove Total time line from all vmctl flag files to reduce diffs noise
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_opentsdb_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_influx_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_remote-read_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_prometheus_flags.md
	sed -i '/Total time:/d' docs/victoriametrics/vmctl/vmctl_vm-native_flags.md

	# remove Version line and the actual version line from vmctl_flags.md to reduce diffs noise
	sed -i '/^VERSION:/,+1d' docs/victoriametrics/vmctl/vmctl_flags.md

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
	git fetch opensource

	rm -rf /tmp/vm-enterprise-cluster
	git worktree remove /tmp/vm-enterprise-cluster || true
	git worktree add /tmp/vm-enterprise-cluster enterprise/enterprise-cluster

	rm -rf /tmp/vm-enterprise-single-node
	git worktree remove /tmp/vm-enterprise-single-node || true
	git worktree add /tmp/vm-enterprise-single-node enterprise/enterprise-single-node


	rm -rf /tmp/vm-opensource-cluster
	git worktree remove /tmp/vm-opensource-cluster || true
	git worktree add /tmp/vm-opensource-cluster opensource/cluster

	rm -rf /tmp/vm-opensource-single-node
	git worktree remove /tmp/vm-opensource-single-node || true
	git worktree add /tmp/vm-opensource-single-node opensource/master

	make docs-update-vmctl-flags
	make docs-update-vmsingle-flags
	make docs-update-vmalert-flags
	make docs-update-vmauth-flags
	make docs-update-vmagent-flags
	make docs-update-vmselect-flags
	make docs-update-vminsert-flags
	make docs-update-vmstorage-flags