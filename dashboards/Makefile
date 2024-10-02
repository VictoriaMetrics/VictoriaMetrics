dashboard-copy:
	echo "" > dashboards/vm/${SRC}
	cat dashboards/${SRC} >> dashboards/vm/${SRC}
	sed -i='.tmp' 's/prometheus/victoriametrics-datasource/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/Prometheus/VictoriaMetrics/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/${D_UID}/${D_UID}_vm/g' dashboards/vm/${SRC}
	sed -i='.tmp' 's/"title": "${TITLE}"/"title": "${TITLE} (VM)"/g' dashboards/vm/${SRC}
	rm -rf dashboards/vm/*.tmp

# Copies listed dashboards to vm/* but changes the datasource type from Prometheus to VictoriaMetrics.
# The command should be called before commiting changes to dashboards/* files.
dashboards-sync:
	SRC=victoriametrics.json D_UID=wNf0q_kZk TITLE="VictoriaMetrics - single-node" $(MAKE) dashboard-copy
	SRC=victoriametrics-cluster.json D_UID=oS7Bi_0Wz TITLE="VictoriaMetrics - cluster" $(MAKE) dashboard-copy
	SRC=vmagent.json D_UID=G7Z9GzMGz TITLE="VictoriaMetrics - vmagent" $(MAKE) dashboard-copy
	SRC=vmalert.json D_UID=LzldHAVnz TITLE="VictoriaMetrics - vmalert" $(MAKE) dashboard-copy
	SRC=vmauth.json D_UID=nbuo5Mr4k TITLE="VictoriaMetrics - vmauth" $(MAKE) dashboard-copy
	SRC=operator.json D_UID=1H179hunk TITLE="VictoriaMetrics - operator" $(MAKE) dashboard-copy
