ENTERPRISE_REMOTE_EXISTS=$(shell grep -Fx '[remote "enterprise"]' .git/config > /dev/null && echo 1 || echo 0)
GH_RELEASE="/tmp/vm-github-release"


enterprise-remote-check:
ifeq ($(ENTERPRISE_REMOTE_EXISTS), 0)
	$(error enterprise remote is missing)
endif

create-tags: enterprise-remote-check
	git fetch --all
	git checkout -b enterprise enterprise/enterprise && git tag -s $(TAG)-enterprise -m "$(TAG)"
	git checkout -b enterprise-cluster enterprise/enterprise-cluster && git tag -s $(TAG)-enterprise-cluster -m "$(TAG)"
	git checkout -b cluster origin/cluster && git tag -s $(TAG)-cluster -m "$(TAG)"
	git checkout -b master origin/master && git tag -s $(TAG) -m "$(TAG)"

push-tags: enterprise-remote-check
	git push origin $(TAG)
	git push origin $(TAG)-cluster
	git push enterprise $(TAG)-enterprise
	git push enterprise $(TAG)-enterprise-cluster

github-token-check:
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif

github-create-release: github-token-check push-tags
	@result=$$(curl -o $(GH_RELEASE) -s -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token ${GITHUB_TOKEN}" \
		https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases \
		-d '{"tag_name":"$(TAG)","name":"$(TAG)","body":"$(TAG)","draft":true,"prerelease":false,"generate_release_notes":false}'); \
		if [ $${result} = 201 ]; then \
			printf "Created release for ${BOLD}${BLUE}${TAG}${RESET}\n"; \
		else \
			printf "Failed to create release for ${BOLD}${RED}${TAG}${RESET}\n"; \
			exit 1; \
		fi
	
github-upload-assets:
	@release_id=$$(cat $(GH_RELEASE) | grep '"id"' -m 1 | sed -E 's/.* ([[:digit:]]+)\,/\1/'); \
	$(foreach file, $(wildcard bin/*.zip), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/zip" $(MAKE) github-upload-asset || exit 1;) \
	$(foreach file, $(wildcard bin/*.tar.gz), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/x-gzip" $(MAKE) github-upload-asset || exit 1;) \
	$(foreach file, $(wildcard bin/*_checksums.txt), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="text/plain" $(MAKE) github-upload-asset || exit 1;) 

github-upload-asset:
	@printf "Uploading ${BOLD}$(FILE)${RESET}\n"
	@result=$$(curl -o /dev/null -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token ${GITHUB_TOKEN}" \
		-H "Content-Type: $(CONTENT_TYPE)" \
		--data-binary "@$(FILE)" \
		https://uploads.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases/$(RELEASE_ID)/assets?name=$(notdir $(FILE))); \
		if [ $${result} = 201 ]; then \
			printf "Upload OK: $${result}\n"; \
		elif [ $${result} = 422 ]; then
			printf "Asset already uploaded, you need to delete it from UI if you want to re-upload\n"; \
		else \
			printf "Upload ${RED}Failed${RESET}: $${result}\n"; \
			exit 1; \
		fi

github-delete-release:
	@release_id=$$(cat $(GH_RELEASE) | grep '"id"' -m 1 | sed -E 's/.* ([[:digit:]]+)\,/\1/'); \
	result=$$(curl -o /dev/null -s -w "%{http_code}" \
		-X DELETE \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token ${GITHUB_TOKEN}" \
		https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases/$${release_id}); \
		if [ $${result} = 204 ]; then \
			printf "Deleted release for ${BOLD}${TAG}${RESET}\n"; \
		else \
			printf "Failed to delete release for ${BOLD}${RED}${TAG}${RESET}\n"; \
			exit 1; \
		fi
