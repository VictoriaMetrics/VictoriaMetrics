# These commands must be run from the VictoriaMetrics repository root.

cspell-build:
	@docker build cspell \
		--tag cspell

cspell-check:
	@CMD="cspell --no-progress '**'" $(MAKE) cspell-run-command

cspell-run-command:
	@docker run \
		--entrypoint /bin/sh \
		--mount type=bind,src=".",dst=/victoriametrics \
		--rm \
		--tty \
		cspell -c "$(CMD)"

cspell-update: cspell-build
	CMD="cd /victoriametrics/cspell && npm update && rm -rf ./node_modules" $(MAKE) cspell-run-command
	
spellcheck: cspell-check
