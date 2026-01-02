
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

.PHONY: security
security:
	(cd tmp/VictoriaMetrics && git checkout master)
	(cd tmp/VictoriaMetrics && make govulncheck)

	@echo "Checking image tag: docker.io/victoriametrics/victoria-metrics:$(NEXT_TAG)-security-checks"
	(cd tmp/VictoriaMetrics && BUILDINFO_TAG=$(NEXT_TAG)-security-checks make package-victoria-metrics)
	grype db update && grype --only-fixed docker.io/victoriametrics/victoria-metrics:$(NEXT_TAG)-security-checks
	docker scout cves docker.io/victoriametrics/victoria-metrics:$(NEXT_TAG)-security-checks
	@echo "SUCCESS: all security checks passed"

.PHONY: vmui-opensource-update
vmui-opensource-update:
	@echo "=== Updating VMUI on master branch ==="
	#(cd tmp/VictoriaMetrics && git checkout master)
	(cd tmp/VictoriaMetrics && make vmui-update)
	@echo ""
	@echo "=== Changes in master branch ==="
	(cd tmp/VictoriaMetrics && git diff --name-only)
	@echo ""
	@read -p "Commit changes to master? (y/n): " answer; \
	if [ "$$answer" != "y" ]; then \
		echo "Aborting. Rolling back changes..."; \
		(cd tmp/VictoriaMetrics && git checkout .); \
		exit 1; \
	fi
	(cd tmp/VictoriaMetrics && git add -A && git commit -m "app/vmui: update vmui build for release $(NEXT_TAG)")
	@echo "Changes committed to master"
	@echo ""
	@echo "=== Cherry-picking to cluster branch ==="
	(cd tmp/VictoriaMetrics && git checkout cluster && git cherry-pick master)
	@echo "Cherry-picked to cluster"
	@echo "SUCCESS: VMUI updated on master and cluster branches"

.PHONY: vmui-enterprise-update
vmui-enterprise-update:
	@echo "=== Updating VMUI on enterprise-single-node branch ==="
	(cd tmp/VictoriaMetrics && git checkout enterprise-single-node)
	(cd tmp/VictoriaMetrics && make vmui-update)
	@echo ""
	@echo "=== Changes in enterprise-single-node branch ==="
	(cd tmp/VictoriaMetrics && git diff --name-only)
	@echo ""
	@read -p "Commit changes to enterprise-single-node? (y/n): " answer; \
	if [ "$$answer" != "y" ]; then \
		echo "Aborting. Rolling back changes..."; \
		(cd tmp/VictoriaMetrics && git checkout .); \
		exit 1; \
	fi
	(cd tmp/VictoriaMetrics && git add -A && git commit -m "app/vmui: update vmui build for release $(NEXT_TAG)")
	@echo "Changes committed to enterprise-single-node"
	@echo ""
	@echo "=== Cherry-picking to enterprise-cluster branch ==="
	(cd tmp/VictoriaMetrics && git checkout enterprise-cluster && git cherry-pick enterprise-single-node)
	@echo "Cherry-picked to enterprise-cluster"
	@echo ""
	@echo "SUCCESS: VMUI updated on enterprise-single-node and enterprise-cluster branches"
