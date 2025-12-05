dashboard-copy:
	echo "" > dashboards/vm/${SRC}
	cat dashboards/${SRC} >> dashboards/vm/${SRC}
	sed -i='.tmp' 's/prometheus/victoriametrics-metrics-datasource/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/Prometheus/VictoriaMetrics/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/${D_UID}/${D_UID}_vm/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/"title": "${TITLE}"/"title": "${TITLE} (VM)"/g' dashboards/vm/${SRC}
	rm -rf dashboards/vm/*.tmp

# Copies listed dashboards to vm/* but changes the datasource type from Prometheus to VictoriaMetrics.
# Also generates status-page-generated.json from alert rules.
# The command should be called before committing changes to dashboards/* files.
dashboards-sync:
	SRC=victoriametrics.json D_UID=wNf0q_kZk TITLE="VictoriaMetrics - single-node" $(MAKE) dashboard-copy
	SRC=victoriametrics-cluster.json D_UID=oS7Bi_0Wz TITLE="VictoriaMetrics - cluster" $(MAKE) dashboard-copy
	SRC=vmagent.json D_UID=G7Z9GzMGz TITLE="VictoriaMetrics - vmagent" $(MAKE) dashboard-copy
	SRC=vmalert.json D_UID=LzldHAVnz TITLE="VictoriaMetrics - vmalert" $(MAKE) dashboard-copy
	SRC=vmauth.json D_UID=nbuo5Mr4k TITLE="VictoriaMetrics - vmauth" $(MAKE) dashboard-copy
	SRC=operator.json D_UID=1H179hunk TITLE="VictoriaMetrics - operator" $(MAKE) dashboard-copy
	SRC=backupmanager.json D_UID=gF-lxRdVz TITLE="VictoriaMetrics - backupmanager" $(MAKE) dashboard-copy
	SRC=clusterbytenant.json D_UID=IZFqd3lMz TITLE="VictoriaMetrics Cluster Per Tenant Statistic" $(MAKE) dashboard-copy
	$(MAKE) generate-status-page
	SRC=status-page-generated.json D_UID=vm-status-page TITLE="VictoriaMetrics - Status Page" $(MAKE) dashboard-copy


# Build the dashboard generator tool
build-dashgen:
	cd dashboards/dashgen && go build -o dashgen-bin ./main.go

# Generate status page dashboard from alert rules.
# This reads alerts from deployment/docker/rules and produces status-page-generated.json.
# The generated dashboard shows OK/KO status for each alert across all VictoriaMetrics components.
generate-status-page: build-dashgen
	cd dashboards/dashgen && ./dashgen-bin \
		--alerts-dir=../../deployment/docker/rules \
		--output=../status-page-generated.json \
		--title="VictoriaMetrics - Status Page" \
		--uid=vm-status-page

# Clean generated files
clean-dashgen:
	rm -f dashboards/dashgen/dashgen-bin dashboards/status-page-generated.json

# Run dashgen unit tests (isolated, can be disabled by commenting out)
dashgen-test:
	cd dashboards/dashgen && go test -v ./...

# Run dashgen tests with coverage
dashgen-test-cover:
	cd dashboards/dashgen && go test -coverprofile=coverage.txt -covermode=atomic ./...

