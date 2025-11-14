
CURR_TAG = v1.128.0
NEXT_TAG = v1.129.0
DIFF_CMD ?= diff

.PHONY: init
init:
	mkdir -p tmp && rm -rf tmp/*

	git clone git@github.com:VictoriaMetrics/VictoriaMetrics.git tmp/VictoriaMetrics

	(cd tmp/VictoriaMetrics && git remote rename origin opensource)
	(cd tmp/VictoriaMetrics && git remote add enterprise git@github.com:VictoriaMetrics/VictoriaMetrics-enterprise.git && git fetch enterprise)

	(cd tmp/VictoriaMetrics && git checkout master && git pull opensource master)
	(cd tmp/VictoriaMetrics && git checkout -b cluster opensource/cluster && git pull opensource cluster)
	(cd tmp/VictoriaMetrics && git checkout -b enterprise-single-node enterprise/enterprise-single-node && git pull enterprise enterprise-single-node)
	(cd tmp/VictoriaMetrics && git checkout -b enterprise-cluster enterprise/enterprise-cluster && git pull enterprise enterprise-cluster)
	@echo "SUCCESS: Release from tag $(CURR_TAG) to $(NEXT_TAG) initialized"

.PHONY: compare-git-history
compare-git-history:
	@echo "Collecting git logs from all branches..."
	(cd tmp/VictoriaMetrics && git checkout master && git log --format=%s "${CURR_TAG}..HEAD" > ../master_changelog.txt)
	(cd tmp/VictoriaMetrics && git checkout cluster && git log --format=%s "$(CURR_TAG)-cluster..HEAD" > ../cluster_changelog.txt)
	(cd tmp/VictoriaMetrics && git checkout enterprise-single-node && git log --format=%s "$(CURR_TAG)-enterprise..HEAD" > ../enterprise_changelog.txt)
	(cd tmp/VictoriaMetrics && git checkout enterprise-cluster && git log --format=%s "$(CURR_TAG)-enterprise-cluster..HEAD" > ../enterprise_cluster_changelog.txt)

	@echo "=== Comparing master vs cluster ==="
	@$(DIFF_CMD) tmp/master_changelog.txt tmp/cluster_changelog.txt || true
	@echo ""
	@read -p "Are master and cluster branches equal? (y/n): " answer; \
	if [ "$$answer" != "y" ]; then \
		echo "ERROR: Branches are not equal. Aborting."; \
		exit 1; \
	fi

	@echo ""
	@echo "=== Comparing master vs enterprise-single-node ==="
	@$(DIFF_CMD) tmp/master_changelog.txt tmp/enterprise_changelog.txt || true
	@echo ""
	@read -p "Are master and enterprise-single-node branches equal? (y/n): " answer; \
	if [ "$$answer" != "y" ]; then \
		echo "ERROR: Branches are not equal. Aborting."; \
		exit 1; \
	fi

	@echo ""
	@echo "=== Comparing cluster vs enterprise-cluster ==="
	@$(DIFF_CMD) tmp/cluster_changelog.txt tmp/enterprise_cluster_changelog.txt || true
	@echo ""
	@read -p "Are cluster and enterprise-cluster branches equal? (y/n): " answer; \
	if [ "$$answer" != "y" ]; then \
		echo "ERROR: Branches are not equal. Aborting."; \
		exit 1; \
	fi

	@echo ""
	@echo "SUCCESS: All branches confirmed equal"


.PHONY: tests
tests:
	(cd tmp/VictoriaMetrics && git checkout master && make test check-all integration-test)
	(cd tmp/VictoriaMetrics && git checkout cluster && make test check-all integration-test)
	(cd tmp/VictoriaMetrics && git checkout enterprise-single-node && make test check-all integration-test)
	(cd tmp/VictoriaMetrics && git checkout enterprise-cluster && make test check-all integration-test)
	@echo "SUCCESS: all tests passed"


