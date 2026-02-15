GITHUB_RELEASE_SPEC_FILE="/tmp/vm-github-release-$(TAG)"
GITHUB_DEBUG_FILE="/tmp/vm-github-debug"

GITHUB_ASSETS_COUNT ?= 112

github-token-check:
ifndef GITHUB_TOKEN
	$(error missing GITHUB_TOKEN env var. It must contain github token for VictoriaMetrics project obtained from https://github.com/settings/tokens)
endif

github-tag-check:
ifndef TAG
	$(error missing TAG env var. It must contain github release tag to create)
endif

github-create-release: github-token-check github-tag-check
	@result=$$(curl -o $(GITHUB_RELEASE_SPEC_FILE) -s -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token $(GITHUB_TOKEN)" \
		https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases \
		-d '{"tag_name":"$(TAG)","name":"$(TAG)","body":"TODO: put here the changelog for $(TAG) release from docs/CHANGELOG.md","draft":true,"prerelease":false,"generate_release_notes":false}'); \
		if [ $${result} = 201 ]; then \
			release_id=$$(cat $(GITHUB_RELEASE_SPEC_FILE) | grep '"id"' -m 1 | sed -E 's/.* ([[:digit:]]+)\,/\1/'); \
			printf "Created release $(TAG) with id=$${release_id}\n"; \
		else \
			printf "Failed to create release $(TAG)\n"; \
			cat $(GITHUB_RELEASE_SPEC_FILE); \
			exit 1; \
		fi

github-upload-assets: github-get-release
	@release_id=$$(jq '.id' "$(GITHUB_RELEASE_SPEC_FILE)"); \
	$(foreach file, $(wildcard bin/*.zip), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/zip" $(MAKE) github-upload-asset || exit 1;) \
	$(foreach file, $(wildcard bin/*.tar.gz), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="application/x-gzip" $(MAKE) github-upload-asset || exit 1;) \
	$(foreach file, $(wildcard bin/*_checksums.txt), FILE=$(file) RELEASE_ID=$${release_id} CONTENT_TYPE="text/plain" $(MAKE) github-upload-asset || exit 1;) 

github-upload-asset: github-token-check
ifndef FILE
	$(error missing FILE env var. It must contain path to file to upload to github release)
endif
	@printf "Uploading $(FILE)\n"
	@result=$$(curl -o $(GITHUB_DEBUG_FILE) -w "%{http_code}" \
		-X POST \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token $(GITHUB_TOKEN)" \
		-H "Content-Type: $(CONTENT_TYPE)" \
		--data-binary "@$(FILE)" \
		https://uploads.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases/$(RELEASE_ID)/assets?name=$(notdir $(FILE))); \
		if [ $${result} = 201 ]; then \
			printf "Upload OK: $${result}\n"; \
		elif [ $${result} = 422 ]; then \
			printf "Asset already uploaded, you need to delete it from UI if you want to re-upload it\n"; \
		else \
			printf "Upload failed: $${result}\n"; \
			cat $(GITHUB_DEBUG_FILE); \
			exit 1; \
		fi

github-delete-release: github-token-check github-tag-check github-get-release
	@release_id=$$(jq '.id' "$(GITHUB_RELEASE_SPEC_FILE)"); \
	result=$$(curl -o $(GITHUB_DEBUG_FILE) -s -w "%{http_code}" \
		-X DELETE \
		-H "Accept: application/vnd.github+json" \
		-H "Authorization: token $(GITHUB_TOKEN)" \
		https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases/$${release_id}); \
		if [ $${result} = 204 ]; then \
			printf "Deleted release with id=$${release_id}\n"; \
		else \
			printf "Failed to delete release with id=$${release_id}\n"; \
			cat $(GITHUB_DEBUG_FILE); \
			exit 1; \
		fi

github-verify-release: github-token-check github-tag-check github-get-release
	@ASSET_COUNT=$$(jq '.assets | length' "$(GITHUB_RELEASE_SPEC_FILE)"); \
	if [ "$${ASSET_COUNT}" -ne "$(GITHUB_ASSETS_COUNT)" ]; then \
		echo "Expected $(GITHUB_ASSETS_COUNT) assets, found $${ASSET_COUNT}"; \
		exit 1; \
	fi; \
	FAILED_COUNT=$$(jq '[.assets[] | select(.state != "uploaded")] | length' "$(GITHUB_RELEASE_SPEC_FILE)"); \
	if [ "$${FAILED_COUNT}" -ne "0" ]; then \
		echo "Found $${FAILED_COUNT} failed assets"; \
		exit 1; \
	fi; \
	echo "GitHub release $(TAG) has $${ASSET_COUNT} assets, all uploaded successfully"

github-get-release: github-token-check github-tag-check
	@echo "Fetching GitHub release $(TAG)..."
	@curl --fail -sSL \
		-H "Authorization: Bearer $(GITHUB_TOKEN)" \
		"https://api.github.com/repos/VictoriaMetrics/VictoriaMetrics/releases?per_page=5" \
		| jq '.[] | select(.draft == true and .tag_name == "$(TAG)")' > "$(GITHUB_RELEASE_SPEC_FILE)"
	@if [ ! -s "$(GITHUB_RELEASE_SPEC_FILE)" ]; then \
		echo "Error: Release file is empty"; \
		exit 1; \
	fi
	@echo "Release data stored in $(GITHUB_RELEASE_SPEC_FILE)"
